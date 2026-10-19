#!/usr/bin/env python3
"""Copia linux/amd64 a Zot; convierte el manifiesto Docker v2 a OCI si hace falta."""
import base64, json, os, subprocess, sys, urllib.request, ssl
from pathlib import Path
DATA=Path(os.environ.get("SO1_P3_DATA","/mnt/fedora-extra/so1-proyecto3"))
def mirror(src,dest):
    auth=str(DATA/"runtime/registry-auth.json");ca=str(DATA/"certs/ca.crt")
    result=subprocess.run([str(DATA/"bin/oras"),"cp","--platform","linux/amd64","--concurrency","1","--to-plain-http=false","--to-ca-file",ca,"--to-registry-config",auth,src,"localhost:5005/"+dest],capture_output=True,text=True)
    if result.returncode:
        if "manifest invalid" not in result.stderr:
            raise RuntimeError(result.stderr[-2000:])
        raw=subprocess.check_output([str(DATA/"bin/oras"),"manifest","fetch","--platform","linux/amd64",src])
        manifest=json.loads(raw)
        manifest["mediaType"]="application/vnd.oci.image.manifest.v1+json"
        manifest["config"]["mediaType"]="application/vnd.oci.image.config.v1+json"
        for layer in manifest["layers"]:
            if layer["mediaType"]=="application/vnd.docker.image.rootfs.diff.tar.gzip":layer["mediaType"]="application/vnd.oci.image.layer.v1.tar+gzip"
        repo,tag=dest.rsplit(":",1)
        secret=json.loads((DATA/"runtime/secrets.json").read_text())["registry_password"]
        req=urllib.request.Request("https://localhost:5005/v2/"+repo+"/manifests/"+tag,data=json.dumps(manifest).encode(),method="PUT",headers={"Content-Type":manifest["mediaType"],"Authorization":"Basic "+base64.b64encode(("so1:"+secret).encode()).decode()})
        with urllib.request.urlopen(req,context=ssl.create_default_context(cafile=ca),timeout=30) as response:
            if response.status!=201:raise RuntimeError("OCI manifest rejected")
    print("Zot OCI listo:",dest,flush=True)
if __name__=="__main__":mirror(sys.argv[1],sys.argv[2])
