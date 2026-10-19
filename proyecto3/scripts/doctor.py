#!/usr/bin/env python3
import os,shutil,subprocess
from pathlib import Path
p=Path(os.environ.get("SO1_P3_DATA","/mnt/fedora-extra/so1-proyecto3"))
for d in ["/","/mnt/development",str(p)]:
 s=shutil.disk_usage(d);print(d,"libre GiB",round(s.free/2**30,2))
subprocess.run(["free","-h"])
for name in ["docker","podman","go","rustc","cargo","kubectl","helm","k3d","oras","zot","operator-sdk"]:print(name,shutil.which(name) or "FALTA")
subprocess.run(["docker","stats","--no-stream","--format","{{.Name}} {{.MemUsage}} {{.CPUPerc}}"])
print("La instalación es local y no requiere cuentas ni recursos de Google Cloud.")
