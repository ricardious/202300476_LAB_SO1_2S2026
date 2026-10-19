#!/usr/bin/env python3
import datetime,json,os,subprocess
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1];DATA=Path(os.environ.get('SO1_P3_DATA','/mnt/fedora-extra/so1-proyecto3'));out=ROOT/'docs/evidencias';out.mkdir(exist_ok=True)
os.environ['KUBECONFIG']=str(DATA/'runtime/kubeconfig');os.environ['PATH']=str(DATA/'bin')+os.pathsep+os.environ['PATH']
checks={'estado-kubernetes.txt':['kubectl','get','pods','-A','-o','wide'],'recursos.txt':['kubectl','top','pods','-A'],'gateway.txt':['kubectl','-n','so1-p3','get','gateway,httproute','-o','yaml'],'operador.txt':['kubectl','-n','so1-p3','get','valkeyinstances,grafanainstances','-o','yaml'],'hpa.txt':['kubectl','-n','so1-p3','get','hpa,deployments'],'falco.txt':['kubectl','-n','so1-p3','logs','daemonset/falco','--tail=100'],'valkey.txt':['kubectl','-n','so1-p3','exec','valkey-0','--','valkey-cli','HGETALL','so1:p3:stats'],'memoria-host.txt':['free','-h'],'disco.txt':['df','-h','/','/mnt/development','/mnt/fedora-extra'],'contenedores.txt':['docker','stats','--no-stream','--format','{{.Name}} {{.MemUsage}} {{.CPUPerc}}']}
for name,cmd in checks.items():
 r=subprocess.run(cmd,capture_output=True,text=True);(out/name).write_text('Capturado '+datetime.datetime.now(datetime.timezone.utc).isoformat()+'\n'+r.stdout+r.stderr)
print('Evidencias de ejecuciones reales guardadas; no se exportan Secrets')
