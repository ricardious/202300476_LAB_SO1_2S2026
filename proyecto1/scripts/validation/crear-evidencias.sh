#!/usr/bin/env bash
# ============================================================================
# crear-evidencias.sh - Genera todas las capturas de evidencia del Proyecto 1
# Carnet: 202300476 · Sistemas Operativos 1
#
# Uso:
#   bash scripts/validation/crear-evidencias.sh
#
# Genera salidas REALES de comandos como evidencia (archivos .txt) en las
# carpetas evidencias/01-... hasta 07-.... Algunas capturas (virt-manager,
# GitHub web) se toman manualmente; este script cubre todo lo que se puede
# capturar por terminal.
# ============================================================================
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
EV="$ROOT/evidencias"
CARNET="202300476"
KEY="${HOME}/.ssh/id_ed25519"
S="ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i $KEY"
VM1=192.168.100.10
VM2=192.168.100.20
VM3=192.168.100.30
ZOT=192.168.100.30:5000

mkdir -p "$EV"/0{1,2,3,4,5,6,7}-*

cap() { # cap <carpeta> <archivo> <titulo> <comando>
  local carpeta="$1" arch="$2" titulo="$3"; shift 3
  { echo "CAPTURA: $titulo"; echo "Comando: $*"; echo "Fecha: $(date)"; echo "----"; "$@"; } > "$carpeta/$arch"
}

echo "== 01-entorno =="
cap "$EV/01-entorno" 01-virt-manager-3vms.txt "virt-manager muestra las 3 VMs (SO1-VM1/2/3)" bash -c "virsh -c qemu:///system list --all"
cap "$EV/01-entorno" 02-virsh-list-all.txt "VMs activas bajo KVM/libvirt" bash -c "virsh -c qemu:///system list --all; echo '---- /dev/kvm ----'; ls -l /dev/kvm"

echo "== 02-runtimes =="
cap "$EV/02-runtimes" 01-vm1-ctr-namespace-ls.txt "Containerd namespaces VM1" bash -c "$S root@$VM1 'sudo ctr namespace ls'"
cap "$EV/02-runtimes" 02-vm1-systemctl-containerd.txt "servicio Containerd VM1" bash -c "$S root@$VM1 'sudo systemctl status containerd --no-pager | head -n5'"
cap "$EV/02-runtimes" 03-vm1-docker-ausente.txt "Docker AUSENTE en VM1" bash -c "$S root@$VM1 'docker info 2>&1 | head -n1'"
cap "$EV/02-runtimes" 04-vm2-podman-info.txt "Podman info VM2" bash -c "$S root@$VM2 'podman info 2>&1 | grep -E \"arch|cgroupVersion|hostname|os\"'"
cap "$EV/02-runtimes" 05-vm2-podman-v.txt "Podman version VM2" bash -c "$S root@$VM2 'podman -v'"
cap "$EV/02-runtimes" 06-vm2-docker-ausente.txt "Docker AUSENTE en VM2" bash -c "$S root@$VM2 'docker info 2>&1 | head -n1'"
cap "$EV/02-runtimes" 07-vm3-docker-info.txt "Docker info VM3" bash -c "$S root@$VM3 'docker info 2>&1 | grep -E \"Server Version|Storage Driver|Containers:\"'"
cap "$EV/02-runtimes" 08-vm3-docker-v.txt "Docker version VM3" bash -c "$S root@$VM3 'docker -v'"

echo "== 03-apis =="
cap "$EV/03-apis" 01-vm1-ctr-containers-ls.txt "contenedores VM1 (containerd)" bash -c "$S root@$VM1 'sudo ctr -n default containers ls'"
cap "$EV/03-apis" 02-vm2-podman-ps.txt "contenedores VM2 (podman)" bash -c "$S root@$VM2 'podman ps'"
cap "$EV/03-apis" 03-vm3-docker-ps.txt "contenedores VM3 (docker) - solo zot" bash -c "$S root@$VM3 'docker ps'"
cap "$EV/03-apis" 04-vm1-ctr-images-ls.txt "imagenes VM1 (containerd)" bash -c "$S root@$VM1 'sudo ctr -n default images ls'"
cap "$EV/03-apis" 05-vm2-podman-images.txt "imagenes VM2 (podman)" bash -c "$S root@$VM2 'podman images'"
cap "$EV/03-apis" 06-vm3-docker-images.txt "imagenes VM3 (docker)" bash -c "$S root@$VM3 'docker images'"
cap "$EV/03-apis" 07-dockerfiles.txt "Dockerfiles de las APIs" bash -c "echo '===== api1 ====='; cat $ROOT/api1/Dockerfile; echo; echo '===== api2 ====='; cat $ROOT/api2/Dockerfile; echo; echo '===== api3 ====='; cat $ROOT/api3/Dockerfile"
cap "$EV/03-apis" 08-health-api1.txt "/health API1" bash -c "curl -s http://$VM1:8081/health | jq ."
cap "$EV/03-apis" 09-health-api2.txt "/health API2" bash -c "curl -s http://$VM1:8082/health | jq ."
cap "$EV/03-apis" 10-health-api3.txt "/health API3" bash -c "curl -s http://$VM2:8083/health | jq ."

echo "== 04-zot =="
cap "$EV/04-zot" 01-zot-container.txt "contenedor Zot en VM3" bash -c "$S root@$VM3 'docker ps'"
cap "$EV/04-zot" 02-zot-catalog.txt "catalogo Zot" bash -c "curl -s http://$ZOT/v2/_catalog"
cap "$EV/04-zot" 03-zot-push-vm1.txt "push API1/API2 desde VM1 (containerd)" bash -c "$S root@$VM1 'ctr -n default images push --plain-http $ZOT/api1-202300476:1.0 2>&1 | tail -n1; ctr -n default images push --plain-http $ZOT/api2-202300476:1.0 2>&1 | tail -n1'"
cap "$EV/04-zot" 04-zot-push-vm2.txt "push API3 desde VM2 (podman)" bash -c "$S root@$VM2 'podman push --tls-verify=false $ZOT/api3-202300476:1.0 2>&1 | tail -n1'"
cap "$EV/04-zot" 05-zot-pull-vm2.txt "pull API1 en VM2 (podman)" bash -c "$S root@$VM2 'podman pull --tls-verify=false $ZOT/api1-202300476:1.0 2>&1 | tail -n1; podman images | grep 202300476'"
cap "$EV/04-zot" 06-zot-pull-vm1.txt "pull API3 en VM1 (containerd)" bash -c "$S root@$VM1 'ctr -n default images pull --plain-http $ZOT/api3-202300476:1.0 2>&1 | tail -n1'"

echo "== 05-comunicacion =="
cap "$EV/05-comunicacion" 01-seis-endpoints-true.txt "6 endpoints (connection=true)" bash -c "for u in http://$VM1:8081/api1/$CARNET/call-api2 http://$VM1:8081/api1/$CARNET/call-api3 http://$VM1:8082/api2/$CARNET/call-api1 http://$VM1:8082/api2/$CARNET/call-api3 http://$VM2:8083/api3/$CARNET/call-api1 http://$VM2:8083/api3/$CARNET/call-api2; do echo \"\$u\"; curl -s \"\$u\" | jq .; echo; done"
{ echo "CAPTURA: prueba de error - API3 detenida (connection=false)"; echo "Fecha: $(date)"; echo "----";
  $S root@$VM2 'podman stop api3' >/dev/null 2>&1; sleep 2;
  echo "== API1 -> call-api3 (API3 detenida) =="; curl -s http://$VM1:8081/api1/$CARNET/call-api3 | jq .; } > "$EV/05-comunicacion/02-error-connection-false.txt"
{ echo "CAPTURA: recuperacion - API3 levantada (connection=true)"; echo "Fecha: $(date)"; echo "----";
  $S root@$VM2 'podman start api3' >/dev/null 2>&1; sleep 3;
  echo "== API1 -> call-api3 (API3 levantada) =="; curl -s http://$VM1:8081/api1/$CARNET/call-api3 | jq .; } > "$EV/05-comunicacion/03-recuperacion-true.txt"

echo "== 06-github =="
cap "$EV/06-github" 01-repo-privado.txt "repositorio privado GitHub" bash -c "gh repo view ricardious/202300476_LAB_SO1_2S2026 --json name,visibility -q '\"Repo: \(.name)  Visibilidad: \(.visibility)\"'"
cap "$EV/06-github" 02-colaboradores.txt "colaboradores" bash -c "gh api repos/ricardious/202300476_LAB_SO1_2S2026/collaborators --jq '.[] | .login'; echo '== invitaciones =='; gh api repos/ricardious/202300476_LAB_SO1_2S2026/invitations --jq '.[] | .invitee.login'"
cap "$EV/06-github" 03-readme.txt "README del repositorio" bash -c "curl -s https://raw.githubusercontent.com/ricardious/202300476_LAB_SO1_2S2026/main/README.md | head -n 25"
cap "$EV/06-github" 04-releases.txt "releases" bash -c "gh release list --repo ricardious/202300476_LAB_SO1_2S2026"

echo "== 07-documentacion =="
cap "$EV/07-documentacion" 01-readme.txt "README (portada)" bash -c "head -n 20 $ROOT/README.md"
cap "$EV/07-documentacion" 02-manual-tecnico.txt "manual tecnico" bash -c "head -n 25 $ROOT/docs/MANUAL_TECNICO.md"
cap "$EV/07-documentacion" 03-guia-instalacion.txt "guia de instalacion" bash -c "head -n 25 $ROOT/docs/GUIA_INSTALACION.md"
cap "$EV/07-documentacion" 04-diagrama-arquitectura.txt "diagrama de arquitectura" bash -c "grep -A15 'Diagrama de arquitectura' $ROOT/docs/MANUAL_TECNICO.md"
cap "$EV/07-documentacion" 05-flujo-sistema.txt "flujo del sistema" bash -c "grep -A8 '## 3. Flujo de comunicacion' $ROOT/docs/MANUAL_TECNICO.md"
cap "$EV/07-documentacion" 06-evidencias-carpeta.txt "estructura de evidencias" bash -c "find $EV -name '*.txt' | sort"

echo
echo "Total de capturas generadas: $(find "$EV" -name '*.txt' | wc -l)"
echo "Listo. Revisa evidencias/ para las capturas."
