#!/usr/bin/env python3
import json, os, signal, subprocess, sys, time, urllib.request, yaml, shutil
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
DATA=Path(os.environ.get('SO1_P3_DATA','/mnt/fedora-extra/so1-proyecto3'))
CLUSTER=os.environ.get('SO1_P3_CLUSTER','so1-p3');NS=os.environ.get('SO1_P3_NAMESPACE','so1-p3')
os.environ['PATH']=str(DATA/'bin')+os.pathsep+os.environ['PATH']
os.environ['KUBECONFIG']=str(DATA/'runtime/kubeconfig')
if CLUSTER!='so1-p3' or NS!='so1-p3':raise SystemExit('Este perfil usa cluster y namespace so1-p3; adapte los manifiestos para renombrarlos.')
from registry import start, ensure, pids
from mirror import mirror

def run(args,**kw):return subprocess.run(args,check=True,**kw)
def output(args):return subprocess.check_output(args,text=True).strip()
def kub(*args):return ['kubectl','-n',NS,*args]

def bootstrap():
    run([sys.executable,str(ROOT/'scripts/install-tools.py')]);start();s=ensure()
    registries={'mirrors':{'localhost:5005':{'endpoint':['https://host.k3d.internal:5005']}},'configs':{'host.k3d.internal:5005':{'tls':{'ca_file':'/etc/so1-p3-ca/ca.crt'},'auth':{'username':'so1','password':s['registry_password']}}}}
    f=DATA/'runtime/registries.yaml';f.write_text(yaml.safe_dump(registries));f.chmod(0o600)
    clusters=json.loads(output(['k3d','cluster','list','-o','json']))
    if not any(x['name']==CLUSTER for x in clusters):
        run(['k3d','cluster','create',CLUSTER,'--image','rancher/k3s:v1.35.9-k3s1','--servers','1','--agents','0','--servers-memory','3584m','--no-lb','--api-port','127.0.0.1:6550','--volume',str(DATA/'k3s')+':/var/lib/rancher/k3s@server:0','--volume',str(DATA/'certs')+':/etc/so1-p3-ca:ro@server:0','--volume',str(ROOT/'deploy/kubelet-local.yaml')+':/var/lib/rancher/k3s/agent/etc/kubelet.conf.d/10-so1-local.conf:ro@server:0','--registry-config',str(f),'--k3s-arg','--disable=traefik,servicelb@server:0','--kubeconfig-update-default=false','--kubeconfig-switch-context=false','--wait'])
    else:run(['k3d','cluster','start',CLUSTER])
    f=DATA/'runtime/kubeconfig';f.write_text(output(['k3d','kubeconfig','get',CLUSTER])+'\n');f.chmod(0o600)
    # Docker reconstruye /etc/hosts al reiniciar el nodo.
    gateway=output(['docker','network','inspect','k3d-'+CLUSTER,'--format','{{range .IPAM.Config}}{{.Gateway}}{{end}}'])
    run(['docker','exec','-i','k3d-'+CLUSTER+'-server-0','sh','-c','grep -v host.k3d.internal /etc/hosts > /tmp/so1-hosts; cat /tmp/so1-hosts > /etc/hosts; cat >> /etc/hosts'],input=(gateway+' host.k3d.internal\n').encode())
    run(['kubectl','wait','node','--all','--for=condition=Ready','--timeout=120s'])

def charts():
    for name,url in [('gateway-helm-v1.9.2.tgz',None),('falco-9.2.0.tgz','https://github.com/falcosecurity/charts/releases/download/falco-9.2.0/falco-9.2.0.tgz')]:
        if (DATA/'downloads'/name).exists():continue
        if url:urllib.request.urlretrieve(url,DATA/'downloads'/name)
        else:run(['helm','pull','oci://docker.io/envoyproxy/gateway-helm','--version','v1.9.2','--destination',str(DATA/'downloads')])

def mirrors():
    items=[('docker.io/valkey/valkey:8.1.3-alpine','library/valkey:8.1.3-alpine'),('docker.io/library/rabbitmq:4.1.4-alpine','library/rabbitmq:4.1.4-alpine'),('docker.io/grafana/grafana:13.2.1','library/grafana:13.2.1'),('docker.io/prom/prometheus:v3.7.2','library/prometheus:v3.7.2'),('docker.io/envoyproxy/gateway:v1.9.2','envoyproxy/gateway:v1.9.2'),('docker.io/envoyproxy/envoy:distroless-v1.39.1','envoyproxy/envoy:distroless-v1.39.1'),('docker.io/falcosecurity/falco:0.45.0','falcosecurity/falco:0.45.0')]
    items.append(('docker.io/daprio/daprd:1.18.7','daprio/daprd:1.18.7'))
    for src,dest in items:mirror(src,dest)

def deploy():
    charts();s=ensure()
    for o in [{'apiVersion':'v1','kind':'Namespace','metadata':{'name':NS}},{'apiVersion':'v1','kind':'Secret','metadata':{'name':'p3-auth','namespace':NS},'type':'Opaque','stringData':{'rabbit-password':s['rabbit_password'],'grafana-password':s['grafana_password'],'amqp-url':'amqp://so1:'+s['rabbit_password']+'@rabbitmq:5672/'}}]:run(['kubectl','apply','-f','-'],input=json.dumps(o).encode())
    run(['kubectl','apply','-f',str(ROOT/'operator/config/crd/bases')])
    for file in ['operator.yaml','instances.yaml','workloads.yaml','dapr.yaml']:run(['kubectl','apply','-f',str(ROOT/'deploy'/file)])
    run(['helm','upgrade','--install','eg',str(DATA/'downloads/gateway-helm-v1.9.2.tgz'),'-n','envoy-gateway-system','--create-namespace','-f',str(ROOT/'deploy/envoy-values.yaml'),'--wait','--timeout','5m'])
    run(['kubectl','apply','-f',str(ROOT/'deploy/gateway.yaml')])
    run(['helm','upgrade','--install','falco',str(DATA/'downloads/falco-9.2.0.tgz'),'-n',NS,'-f',str(ROOT/'deploy/falco-values.yaml'),'--wait','--timeout','4m'])
    run(kub('rollout','restart','deployment/api-rust','deployment/go-ingest','deployment/go-writer','deployment/consumer','deployment/p3-operator','deployment/go-dapr'))
    for name in ['api-rust','go-ingest','go-writer','consumer','rabbitmq','prometheus','p3-operator','grafana','go-dapr']:run(kub('rollout','status','deployment/'+name,'--timeout=180s'))
    run(kub('rollout','status','statefulset/valkey','--timeout=120s'))

def tunnel(name,resource,mapping,namespace=NS):
    f=DATA/'runtime'/('tunnel-'+name+'.pid')
    if f.exists():
        try:
            if b'port-forward' in Path('/proc/'+f.read_text().strip()+'/cmdline').read_bytes():return
        except FileNotFoundError:pass
    log=open(DATA/'runtime'/('tunnel-'+name+'.log'),'ab')
    proc=subprocess.Popen(['kubectl','-n',namespace,'port-forward','--address','127.0.0.1',resource,mapping],stdin=subprocess.DEVNULL,stdout=log,stderr=subprocess.STDOUT,start_new_session=True);log.close();f.write_text(str(proc.pid))

def tunnels():
    services=json.loads(output(['kubectl','get','svc','-A','-l','gateway.envoyproxy.io/owning-gateway-name=military','-o','json']))['items']
    if not services:raise SystemExit('Gateway todavía no tiene Service')
    tunnel('gateway','svc/'+services[0]['metadata']['name'],os.environ.get('SO1_P3_GATEWAY_PORT','8088')+':80',namespace=services[0]['metadata']['namespace'])
    tunnel('grafana','svc/grafana',os.environ.get('SO1_P3_GRAFANA_PORT','3003')+':3000')
    print('Gateway http://127.0.0.1:'+os.environ.get('SO1_P3_GATEWAY_PORT','8088')+'/grpc-202300476 — Grafana http://127.0.0.1:'+os.environ.get('SO1_P3_GRAFANA_PORT','3003'))

def locust_ui():
    executable=DATA/'venv/bin/locust'
    if not executable.exists():raise SystemExit('Falta Locust: ejecutar make setup-load')
    profile=DATA/'runtime/input/load-profile.json'
    if not profile.exists():run(['bash',str(ROOT/'scripts/artifact.sh')])
    found=[]
    for process in Path('/proc').iterdir():
        if not process.name.isdigit():continue
        try:
            args=(process/'cmdline').read_bytes().split(b'\0')
            if str(executable).encode() in args and b'--web-port' in args and os.environ.get('SO1_P3_LOCUST_PORT','8089').encode() in args:found.append(int(process.name))
        except (PermissionError,FileNotFoundError):pass
    if found:pid=found[0]
    else:
        log=open(DATA/'runtime/locust-ui.log','ab');environment=dict(os.environ,SO1_LOAD_PROFILE=str(profile))
        proc=subprocess.Popen([str(executable),'-f',str(ROOT/'load/locustfile.py'),'--host','http://127.0.0.1:'+os.environ.get('SO1_P3_GATEWAY_PORT','8088'),'--web-host','127.0.0.1','--web-port',os.environ.get('SO1_P3_LOCUST_PORT','8089')],stdin=subprocess.DEVNULL,stdout=log,stderr=subprocess.STDOUT,start_new_session=True,env=environment);log.close();pid=proc.pid
    (DATA/'runtime/locust-ui.pid').write_text(str(pid));print('Locust listo en http://127.0.0.1:'+os.environ.get('SO1_P3_LOCUST_PORT','8089')+'; no se inicia tráfico automáticamente')

def stop():
    f=DATA/'runtime/locust-ui.pid'
    if f.exists():
        try:
            pid=int(f.read_text());args=Path('/proc/'+str(pid)+'/cmdline').read_bytes().split(b'\0')
            if str(DATA/'venv/bin/locust').encode() in args:os.kill(pid,signal.SIGTERM)
        except (FileNotFoundError,ProcessLookupError):pass
        f.unlink(missing_ok=True)
    for f in (DATA/'runtime').glob('tunnel-*.pid'):
        try:
            pid=int(f.read_text());args=Path('/proc/'+str(pid)+'/cmdline').read_bytes()
            if b'kubectl' in args and b'port-forward' in args:os.kill(pid,signal.SIGTERM)
        except (FileNotFoundError,ProcessLookupError):pass
        f.unlink(missing_ok=True)
    run(['k3d','cluster','stop',CLUSTER])
    for pid in pids():os.kill(pid,signal.SIGTERM)
    print('Proyecto detenido; datos persistentes conservados')

if __name__=='__main__':
    action=sys.argv[1]
    {'bootstrap':bootstrap,'mirror':mirrors,'deploy':deploy,'tunnels':tunnels,'stop':stop,'registry':start,'locust-ui':locust_ui}[action]()
