#!/usr/bin/env python3
"""Valida el camino público real sin vaciar los datos existentes."""
import datetime, json, os, subprocess, time, urllib.error, urllib.request, uuid
from pathlib import Path
DATA=Path(os.environ.get('SO1_P3_DATA','/mnt/fedora-extra/so1-proyecto3'))
os.environ['KUBECONFIG']=str(DATA/'runtime/kubeconfig')
K=[str(DATA/'bin/kubectl'),'-n','so1-p3']
def integer(key):
    s=subprocess.check_output(K+['exec','valkey-0','--','valkey-cli','--raw','HGET','so1:p3:stats',key],text=True).strip();return int(s or 0)
def post(body,id=None,path='/grpc-202300476'):
    req=urllib.request.Request('http://127.0.0.1:'+os.environ.get('SO1_P3_GATEWAY_PORT','8088')+path,json.dumps(body).encode(),headers={'Content-Type':'application/json','Idempotency-Key':id or str(uuid.uuid4())})
    try:
        with urllib.request.urlopen(req,timeout=8) as response:return response.status,json.load(response)
    except urllib.error.HTTPError as e:return e.code,e.read().decode()
before=integer('total');esp=integer('country:ESP')
r={'country':'ESP','warplanes_in_air':0,'warships_in_water':30,'timestamp':datetime.datetime.now(datetime.timezone.utc).isoformat()}
id=str(uuid.uuid4())
assert post(r,id)[0]==202
assert post(r,id)[0]==202
for bad in [dict(r,country='GMT'),dict(r,warplanes_in_air=51),dict(r,warships_in_water=-1),dict(r,timestamp='invalid'),dict(r,extra=1)]:
    code,_=post(bad);assert 400<=code<500,code
assert post(r,path='/missing')[0]==404
for _ in range(30):
    if integer('total')==before+1 and integer('country:ESP')==esp+1:break
    time.sleep(.2)
else:raise AssertionError('No se observó el reporte o se duplicó el conteo')
time.sleep(1)
assert integer('total')==before+1,'Duplicate was counted twice'
print('PASS: Gateway real, REST, gRPC, publicación, consumo, validación y deduplicación')
