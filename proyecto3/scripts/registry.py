#!/usr/bin/env python3
"""Registro Zot local con TLS y credenciales fuera de Git."""
import base64, datetime, ipaddress, json, os, secrets, shutil, signal, ssl, subprocess, time, urllib.request
from pathlib import Path
import bcrypt
from cryptography import x509
from cryptography.x509.oid import NameOID, ExtendedKeyUsageOID
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.hazmat.primitives import serialization, hashes
DATA=Path(os.environ.get("SO1_P3_DATA","/mnt/fedora-extra/so1-proyecto3"))
def ensure():
    for d in ['runtime','certs','certs/client','registry']:(DATA/d).mkdir(parents=True,exist_ok=True)
    c=DATA/'certs';r=DATA/'runtime'
    if not (c/'ca.crt').exists():
        now=datetime.datetime.now(datetime.timezone.utc);key=rsa.generate_private_key(public_exponent=65537,key_size=2048);name=x509.Name([x509.NameAttribute(NameOID.COMMON_NAME,'SO1 Proyecto3 Local CA')])
        ca=x509.CertificateBuilder().subject_name(name).issuer_name(name).public_key(key.public_key()).serial_number(x509.random_serial_number()).not_valid_before(now-datetime.timedelta(minutes=5)).not_valid_after(now+datetime.timedelta(days=365)).add_extension(x509.BasicConstraints(ca=True,path_length=0),True).add_extension(x509.SubjectKeyIdentifier.from_public_key(key.public_key()),False).add_extension(x509.AuthorityKeyIdentifier.from_issuer_public_key(key.public_key()),False).add_extension(x509.KeyUsage(False,False,False,False,False,True,True,False,False),True).sign(key,hashes.SHA256())
        server=rsa.generate_private_key(public_exponent=65537,key_size=2048)
        cert=x509.CertificateBuilder().subject_name(x509.Name([x509.NameAttribute(NameOID.COMMON_NAME,'localhost')])).issuer_name(name).public_key(server.public_key()).serial_number(x509.random_serial_number()).not_valid_before(now-datetime.timedelta(minutes=5)).not_valid_after(now+datetime.timedelta(days=90)).add_extension(x509.BasicConstraints(ca=False,path_length=None),True).add_extension(x509.SubjectKeyIdentifier.from_public_key(server.public_key()),False).add_extension(x509.AuthorityKeyIdentifier.from_issuer_public_key(key.public_key()),False).add_extension(x509.KeyUsage(True,False,True,False,False,False,False,False,False),True).add_extension(x509.ExtendedKeyUsage([ExtendedKeyUsageOID.SERVER_AUTH]),False).add_extension(x509.SubjectAlternativeName([x509.DNSName('localhost'),x509.DNSName('host.k3d.internal'),x509.IPAddress(ipaddress.ip_address('127.0.0.1'))]),False).sign(key,hashes.SHA256())
        for n,o in [('ca.crt',ca),('server.crt',cert)]:(c/n).write_bytes(o.public_bytes(serialization.Encoding.PEM))
        (c/'server.key').write_bytes(server.private_bytes(serialization.Encoding.PEM,serialization.PrivateFormat.PKCS8,serialization.NoEncryption()));(c/'server.key').chmod(0o600)
    shutil.copyfile(c/'ca.crt',c/'client/ca.crt')
    if not (r/'secrets.json').exists():
        (r/'secrets.json').write_text(json.dumps({k:secrets.token_hex(16) for k in ['registry_password','rabbit_password','grafana_password']}));(r/'secrets.json').chmod(0o600)
    s=json.loads((r/'secrets.json').read_text())
    if not (r/'htpasswd').exists():(r/'htpasswd').write_bytes(b'so1:'+bcrypt.hashpw(s['registry_password'].encode(),bcrypt.gensalt())+b'\n');(r/'htpasswd').chmod(0o600)
    config={'distSpecVersion':'1.1.0','storage':{'rootDirectory':str(DATA/'registry'),'gc':True},'http':{'address':'0.0.0.0','port':'5005','tls':{'cert':str(c/'server.crt'),'key':str(c/'server.key')},'auth':{'htpasswd':{'path':str(r/'htpasswd')}}},'log':{'level':'warn','output':str(r/'zot.log')}}
    (r/'zot.json').write_text(json.dumps(config));(r/'zot.json').chmod(0o600)
    return s

def pids():
    found=[]
    for p in Path('/proc').iterdir():
        if p.name.isdigit():
            try:
                args=(p/'cmdline').read_bytes().split(b'\0')
                if args[:1]==[str(DATA/'bin/zot').encode()] and str(DATA/'runtime/zot.json').encode() in args:found.append(int(p.name))
            except (PermissionError,FileNotFoundError):pass
    return found

def start():
    s=ensure()
    if not pids():
        log=open(DATA/'runtime/zot-stdout.log','ab');subprocess.Popen([str(DATA/'bin/zot'),'serve',str(DATA/'runtime/zot.json')],stdin=subprocess.DEVNULL,stdout=log,stderr=subprocess.STDOUT,start_new_session=True);log.close()
    ctx=ssl.create_default_context(cafile=str(DATA/'certs/ca.crt'))
    for _ in range(30):
        try:
            req=urllib.request.Request('https://localhost:5005/v2/',headers={'Authorization':'Basic '+base64.b64encode(('so1:'+s['registry_password']).encode()).decode()})
            with urllib.request.urlopen(req,context=ctx,timeout=2) as resp:
                if resp.status==200:break
        except Exception:time.sleep(0.2)
    else:raise RuntimeError('Zot no está listo; revisar runtime/zot.log')
    for cmd in [[str(DATA/'bin/oras'),'login','localhost:5005','--plain-http=false','--ca-file',str(DATA/'certs/ca.crt'),'--registry-config',str(DATA/'runtime/registry-auth.json'),'-u','so1','--password-stdin'],['podman','--root',str(DATA/'podman'),'login','--tls-verify=true','--cert-dir',str(DATA/'certs/client'),'--authfile',str(DATA/'runtime/podman-auth.json'),'-u','so1','--password-stdin','localhost:5005']]:
        subprocess.run(cmd,input=s['registry_password'].encode(),check=True,stdout=subprocess.DEVNULL)
    for n in ['registry-auth.json','podman-auth.json']:(DATA/'runtime'/n).chmod(0o600)
    print('Zot listo en https://localhost:5005')
if __name__=='__main__':start()
