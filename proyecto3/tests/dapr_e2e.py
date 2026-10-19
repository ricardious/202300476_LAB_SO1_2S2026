#!/usr/bin/env python3
"""Flujo real Dapr: requiere carga detenida; restaura los recursos que pausa."""
import datetime, json, subprocess, time, uuid
from e2e import K, integer, post

PATH='/dapr-202300476'
def kub(*args):return subprocess.run(K+list(args),check=True)
def wait_total(expected):
    for _ in range(60):
        if integer('total')==expected:break
        time.sleep(.5)
    else:raise AssertionError(f'Conteo esperado {expected}, actual {integer("total")}')
    time.sleep(1)
    assert integer('total')==expected,'Conteo duplicado o carga externa activa'

def main():
    print('Inicio UTC:',datetime.datetime.now(datetime.timezone.utc).isoformat(),flush=True)
    r={'country':'ESP','warplanes_in_air':12,'warships_in_water':7,'timestamp':datetime.datetime.now(datetime.timezone.utc).isoformat()}
    before=integer('total');id=str(uuid.uuid4())
    for path in [PATH,PATH,'/grpc-202300476']:
        code,body=post(r,id,path);assert code==202,(code,body)
    for bad in [dict(r,country='GMT'),dict(r,warplanes_in_air=51),dict(r,warships_in_water=-1),dict(r,timestamp='invalid'),dict(r,extra=1)]:
        code,_=post(bad,path=PATH);assert 400<=code<500,code
    wait_total(before+1)
    print('PASS: SDK pub/sub, RabbitMQ, Valkey, validacion y deduplicacion entre ambos flujos',flush=True)
    try:
        kub('scale','deployment/go-dapr','--replicas=0')
        kub('wait','pod','-l','app=go-dapr','--for=delete','--timeout=60s')
        assert post(r,path=PATH)[0] in (502,503),'Dapr apagado no debe aceptar'
        assert post(r)[0]==202,'El flujo gRPC debe seguir disponible'
        wait_total(before+2)
        print('PASS: Dapr apagado produce error y gRPC continua funcionando',flush=True)
    finally:
        kub('scale','deployment/go-dapr','--replicas=1')
        kub('rollout','status','deployment/go-dapr','--timeout=180s')
    outage_id=str(uuid.uuid4())
    try:
        kub('scale','statefulset/valkey','--replicas=0')
        kub('wait','pod/valkey-0','--for=delete','--timeout=60s')
        code,body=post(r,outage_id,PATH);assert code==202,(code,body)
        time.sleep(4)
        print('PASS: publicacion confirmada durante indisponibilidad de Valkey',flush=True)
    finally:
        kub('scale','statefulset/valkey','--replicas=1')
        kub('rollout','status','statefulset/valkey','--timeout=120s')
    wait_total(before+3)
    assert post(r,outage_id,PATH)[0]==202
    wait_total(before+3)
    print('PASS: reentrega tras recuperar Valkey sin duplicar estadisticas',flush=True)
    queues=json.loads(subprocess.check_output(K+['exec','deploy/rabbitmq','--','rabbitmqctl','list_queues','name','durable','auto_delete','messages','arguments','--formatter=json'],text=True))
    ours=[q for q in queues if 'dapr' in q['name']]
    assert ours,'No hay cola Dapr'
    assert all(q['durable'] and not q['auto_delete'] and q['messages']==0 for q in ours),ours
    print('Colas Dapr:',json.dumps(ours),flush=True)
    print('PASS: cola Dapr durable, sin auto-delete y drenada; sin tocar datos existentes',flush=True)

if __name__=='__main__':main()
