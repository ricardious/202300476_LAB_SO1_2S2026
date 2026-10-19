#!/usr/bin/env python3
import hashlib, io, json, os, platform, tarfile, urllib.request, zipfile
from pathlib import Path
if platform.machine()!="x86_64":raise SystemExit("Este perfil está validado en Linux x86_64")
p=Path(os.environ.get("SO1_P3_DATA","/mnt/fedora-extra/so1-proyecto3"))
for n in ["bin","downloads","build","k3s","registry","certs","runtime","podman","evidence"]:(p/n).mkdir(parents=True,exist_ok=True)
for item in json.loads(Path(__file__).with_name("tools.lock.json").read_text()):
    target=p/"bin"/item["name"]
    if target.exists() and hashlib.sha256(target.read_bytes()).hexdigest()==item["sha256"]:continue
    print("Instalando",item["name"],flush=True)
    raw=urllib.request.urlopen(item["url"],timeout=120).read()
    if item["member"]:
        if item["url"].endswith(".zip"):
            with zipfile.ZipFile(io.BytesIO(raw)) as z:raw=z.read(item["member"])
        else:
            with tarfile.open(fileobj=io.BytesIO(raw)) as t:raw=t.extractfile(item["member"]).read()
    if hashlib.sha256(raw).hexdigest()!=item["sha256"]:raise SystemExit("Checksum incorrecto para "+item["name"])
    target.write_bytes(raw);target.chmod(0o755)
print("Herramientas verificadas")
