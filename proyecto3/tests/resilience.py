#!/usr/bin/env python3
"""Prueba sobre recursos exclusivos de Proyecto 3; conserva el conjunto de datos."""
import json,os,subprocess,time,urllib.request,uuid,datetime
from pathlib import Path
DATA=Path(os.environ.get('SO1_P3_DATA','/mnt/fedora-extra/so1-proyecto3'));os.environ['KUBECONFIG']=str(DATA/'runtime/kubeconfig');K=[str(DATA/'bin/kubectl'),'-n','so1-p3']
def run(*args):return subprocess.check_output(K+list(args),text=True).strip()
def count():return int(run('exec','valkey-0','--','valkey-cli','--raw','HGET','so1:p3:stats','total') or 0)
before=count()
try:
 print(run('scale','deployment/consumer','--replicas=0'),flush=True)
 run('rollout','status','deployment/consumer','--timeout=60s')
 for _ in range(3):
  report={'country':'ESP','warplanes_in_air':42,'warships_in_water':14,'timestamp':datetime.datetime.now(datetime.timezone.utc).isoformat()}
  req=urllib.request.Request('http://127.0.0.1:'+os.environ.get('SO1_P3_GATEWAY_PORT','8088')+'/grpc-202300476',json.dumps(report).encode(),headers={'Content-Type':'application/json','Idempotency-Key':str(uuid.uuid4())})
  with urllib.request.urlopen(req,timeout=10) as r:assert r.status==202
 assert count()==before,'Un consumidor seguía activo durante la pausa'
 print('PASS: broker acepta reportes mientras Consumer está detenido',flush=True)
finally:print(run('scale','deployment/consumer','--replicas=1'),flush=True)
run('rollout','status','deployment/consumer','--timeout=90s')
for _ in range(30):
 if count()==before+3:break
 time.sleep(.5)
else:raise AssertionError('No se recuperaron los reportes pendientes')
print('PASS: consumo recuperado sin perder ni duplicar reportes',flush=True)
before=count();old=run('get','pod','valkey-0','-o','jsonpath={.metadata.uid}')
run('delete','pod','valkey-0','--wait=true')
for _ in range(60):
 result=subprocess.run(K+['get','pod','valkey-0','-o','json'],capture_output=True,text=True)
 if result.returncode==0:
  pod=json.loads(result.stdout)
  if pod['metadata']['uid']!=old and any(c['type']=='Ready' and c['status']=='True' for c in pod['status'].get('conditions',[])):break
 time.sleep(1)
else:raise AssertionError('Valkey no volvió a Ready')
assert count()==before,'Datos perdidos al reemplazar Valkey'
print('PASS: PVC y AOF conservaron los datos después de reemplazar el pod',flush=True)
