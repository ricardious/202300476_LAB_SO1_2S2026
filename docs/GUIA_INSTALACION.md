# GUÍA DE INSTALACIÓN — Proyecto 1

> Sistemas Operativos 1 · 2do Semestre 2026 · Carnet: **202300476**
> Repositorio: `202300476_LAB_SO1_2S2026`
> Esta guía permite **reconstruir el proyecto desde cero** con los comandos reales usados.

---

## 1. Requisitos previos

- Host Linux con **KVM/libvirt** (probado en Fedora 42, x86_64, AMD-V).
- Paquetes: `qemu-kvm`, `libvirt`, `virt-install`, `virt-manager`, `cloud-image-utils`
  (en Fedora: `virt-install`, `golang` para compilar).
- Herramientas en el host: `docker` (para construir/transferir imágenes), `qemu-img`,
  `genisoimage`, `sshpass` (opcional).
- Imagen cloud base: `debian-12-genericcloud-amd64.qcow2`.
- CPU con virtualización (AMD-V/Intel VT-x) y `/dev/kvm` disponible.

---

## 2. Preparación del entorno KVM (host)

### 2.1 Verificar virtualización

```bash
grep -o -m1 -E 'vmx|svm' /proc/cpuinfo   # svm (AMD) o vmx (Intel)
ls -l /dev/kvm
```

### 2.2 Instalar y habilitar libvirt + virt-manager

```bash
sudo dnf install -y virt-install virt-manager
sudo systemctl enable --now libvirtd
sudo usermod -aG libvirt ricardious      # relogin para aplicar
```

### 2.3 Conectar siempre a qemu:///system

```bash
virsh -c qemu:///system list --all
```

> `virsh` por defecto usa `qemu:///session` (sin permisos para crear bridges). Usar
> siempre `-c qemu:///system`.

---

## 3. Configuración de la red virtual NAT

Red `lab-so1` (bridge `virbr1`, subred `192.168.100.0/24`, gateway `192.168.100.1`).

`/tmp/opencode/lab-so1-final.xml`:
```xml
<network>
  <name>lab-so1</name>
  <forward mode='nat'/>
  <bridge name='virbr1' stp='on' delay='0'/>
  <ip address='192.168.100.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.100.2' end='192.168.100.250'/>
      <host mac='52:54:00:aa:00:01' ip='192.168.100.10'/>
      <host mac='52:54:00:aa:00:02' ip='192.168.100.20'/>
      <host mac='52:54:00:aa:00:03' ip='192.168.100.30'/>
    </dhcp>
  </ip>
</network>
```

Aplicar:
```bash
virsh -c qemu:///system net-destroy lab-so1 2>/dev/null
virsh -c qemu:///system net-undefine lab-so1 2>/dev/null
virsh -c qemu:///system net-define /tmp/opencode/lab-so1-final.xml
virsh -c qemu:///system net-start lab-so1
virsh -c qemu:///system net-autostart lab-so1
```

> Nota: libvirt rechaza IP estática igual al inicio del rango DHCP, por eso el rango
> empieza en `.2` y las reservas están en `.10/.20/.30`.

### 3.1 Acceso a Internet de las VMs (NAT)

Si las VMs no salen a Internet (firewalld activo), habilitar masquerade y forwarding:

```bash
sudo firewall-cmd --permanent --zone=libvirt --add-masquerade
sudo firewall-cmd --reload
sudo iptables -t nat -A POSTROUTING -s 192.168.100.0/24 ! -o virbr1 -j MASQUERADE
sudo iptables -I FORWARD -i virbr1 -o wlo1 -j ACCEPT
sudo iptables -I FORWARD -i wlo1 -o virbr1 -m state --state ESTABLISHED,RELATED -j ACCEPT
```

---

## 4. Creación de las 3 VMs (KVM)

### 4.1 Descargar la imagen base cloud

```bash
cd /var/lib/libvirt/images
curl -sL -o debian-12-genericcloud-amd64.qcow2 \
  https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-genericcloud-amd64.qcow2
```

### 4.2 Discos overlay (base compartida, ahorra espacio)

```bash
for i in 1 2 3; do
  qemu-img create -f qcow2 -F qcow2 -b debian-12-genericcloud-amd64.qcow2 -o size=12G SO1-VM$i.qcow2
done
```

### 4.3 Seed de cloud-init (hostname, usuario, red)

Cada VM usa un seed ISO (volid `cidata`) con `user-data`, `meta-data` y `network-config`
(este último configura DHCP por MAC). Ejemplo `user-data`:
```yaml
#cloud-config
hostname: so1-vm1
users:
  - name: ricardious
    sudo: ALL=(ALL) NOPASSWD:ALL
    lock_passwd: false
    passwd: <hash>
    shell: /bin/bash
ssh_pwauth: true
```

`network-config` (por VM, con su MAC):
```yaml
version: 2
ethernets:
  eth0:
    match:
      macaddress: "52:54:00:aa:00:01"
    dhcp4: true
```

Generar ISO:
```bash
(cd <seed_dir> && genisoimage -quiet -output seed-so1-vm1.iso -volid cidata -joliet -rock user-data meta-data network-config)
```

### 4.4 Crear cada VM con virt-install

```bash
virt-install --connect qemu:///system \
  --name SO1-VM1 --memory 1024 --vcpus 2 --cpu host-passthrough \
  --os-variant debian11 \
  --disk path=/var/lib/libvirt/images/SO1-VM1.qcow2,format=qcow2,bus=virtio \
  --disk path=/var/lib/libvirt/images/seed-so1-vm1.iso,device=cdrom \
  --network network=lab-so1,model=virtio,mac=52:54:00:aa:00:01 \
  --graphics vnc,listen=127.0.0.1 --import --boot hd --noautoconsole
```

Repetir para VM2 (768 MB, 1 vCPU, MAC `...:02`) y VM3 (1024 MB, 2 vCPU, MAC `...:03`).

> Si cloud-init no configuró la red (imagen sin `network-config`), se configura el
> filesystem directamente con `qemu-nbd` (ver §13). En esta implementación la red se fijó
> por `network-config` + reservas DHCP.

---

## 5. Instalación de runtimes en las VMs

### 5.1 VM1 — Containerd

```bash
ssh root@192.168.100.10
apt-get update && apt-get install -y containerd
systemctl enable --now containerd
```
Verificación:
```bash
sudo ctr namespace ls
sudo systemctl status containerd
docker info      # debe fallar (command not found)
```

### 5.2 VM2 — Podman

```bash
ssh root@192.168.100.20
apt-get update && apt-get install -y podman
```
Verificación:
```bash
podman info
podman -v
docker info      # debe fallar (command not found)
```

### 5.3 VM3 — Docker

```bash
ssh root@192.168.100.30
apt-get update && apt-get install -y docker.io
systemctl enable --now docker
```
Verificación:
```bash
docker info
docker -v
```

| VM | Runtime | Versión |
|---|---|---|
| VM1 | Containerd | ctr 1.6.20 |
| VM2 | Podman | 4.3.1 |
| VM3 | Docker | 20.10.24 |

> Docker NO debe instalarse en VM1 ni VM2.

---

## 6. Compilación de las APIs (Go)

Cada API es un módulo Go independiente. Desde el host (con Go instalado):

```bash
cd api1 && go build . && go test ./...
cd api2 && go build . && go test ./...
cd api3 && go build . && go test ./...
```

- Estructura por API: `go.mod`, `main.go`, `main_test.go`, `Dockerfile`.
- Módulos: `github.com/202300476/api1`, `api2`, `api3` (Go 1.21).

---

## 7. Creación de las imágenes (Dockerfiles)

Dockerfile multi-stage (builder `golang:1.21-alpine` → runtime `alpine:3.20`), binario
estático, `USER 65534:65534`, `EXPOSE` del puerto correspondiente.

Construcción con `scripts/build-images.sh`:
```bash
bash scripts/build-images.sh                                   # usa docker
CONTAINER_RUNTIME=podman bash scripts/build-images.sh          # o podman
```

Imágenes resultantes (tag 1.0):

| Canónico | Imagen real |
|---|---|
| API1-202300476 | `api1-202300476` |
| API2-202300476 | `api2-202300476` |
| API3-202300476 | `api3-202300476` |

> Docker/Podman exigen minúsculas en el nombre de repositorio; el nombre canónico se
> conserva como LABEL en la imagen.

---

## 8. Configuración de Zot (VM3)

```bash
ssh root@192.168.100.30
mkdir -p /opt/zot/data
cat > /opt/zot/config.yaml <<'YAML'
distSpecVersion: 1.1.0
storage:
  rootDirectory: /var/lib/registry
http:
  address: 0.0.0.0
  port: 5000
log:
  level: info
YAML
docker pull ghcr.io/project-zot/zot:latest
docker run -d --name zot --restart unless-stopped \
  -p 5000:5000 \
  -v /opt/zot/config.yaml:/etc/zot/config.yaml \
  -v /opt/zot/data:/var/lib/registry \
  ghcr.io/project-zot/zot:latest
```

Verificación:
```bash
curl http://192.168.100.30:5000/v2/_catalog
```

---

## 9. Push / Pull de imágenes (registro privado)

### 9.1 Transferir imágenes del host a las VMs

```bash
# API1 y API2 → VM1 (containerd)
docker save api1-202300476:1.0 api2-202300476:1.0 | ssh root@192.168.100.10 'ctr -n default images import -'
# API3 → VM2 (podman)
docker save api3-202300476:1.0 | ssh root@192.168.100.20 'podman load'
```

### 9.2 Push desde cada runtime

```bash
# VM1 (containerd): API1 y API2
ssh root@192.168.100.10 '
  ctr -n default images tag docker.io/library/api1-202300476:1.0 192.168.100.30:5000/api1-202300476:1.0
  ctr -n default images push --plain-http 192.168.100.30:5000/api1-202300476:1.0
  ctr -n default images tag docker.io/library/api2-202300476:1.0 192.168.100.30:5000/api2-202300476:1.0
  ctr -n default images push --plain-http 192.168.100.30:5000/api2-202300476:1.0
'
# VM2 (podman): API3
ssh root@192.168.100.20 '
  podman tag api3-202300476:1.0 192.168.100.30:5000/api3-202300476:1.0
  podman push --tls-verify=false 192.168.100.30:5000/api3-202300476:1.0
'
```

> Si el tag ya existe con un digest viejo, borrarlo antes:
> `ctr -n default images rm <ref>` / `podman rmi <ref>`.

### 9.3 Pull cruzado

```bash
ssh root@192.168.100.20 'podman pull --tls-verify=false 192.168.100.30:5000/api1-202300476:1.0'
ssh root@192.168.100.10 'ctr -n default images pull --plain-http 192.168.100.30:5000/api3-202300476:1.0'
```

### 9.4 Registro HTTP / insecure

Zot es HTTP; para push/pull se usa `--plain-http` (ctr) y `--tls-verify=false` (podman),
acotado por comando.

---

## 10. Despliegue de las APIs

### 10.1 Variables de entorno (valores reales)

| API | Puerto | TARGET_API1 | TARGET_API2 | TARGET_API3 |
|---|---|---|---|---|
| API1 | 8081 | — | `http://192.168.100.10:8082/health` | `http://192.168.100.20:8083/health` |
| API2 | 8082 | `http://192.168.100.10:8081/health` | — | `http://192.168.100.20:8083/health` |
| API3 | 8083 | `http://192.168.100.10:8081/health` | `http://192.168.100.10:8082/health` | — |

### 10.2 VM1 — API1 y API2 (containerd, `--net-host`)

```bash
ssh root@192.168.100.10
ctr -n default run --detach --net-host \
  --env PORT=8081 \
  --env TARGET_API2=http://192.168.100.10:8082/health \
  --env TARGET_API3=http://192.168.100.20:8083/health \
  192.168.100.30:5000/api1-202300476:1.0 api1

ctr -n default run --detach --net-host \
  --env PORT=8082 \
  --env TARGET_API1=http://192.168.100.10:8081/health \
  --env TARGET_API3=http://192.168.100.20:8083/health \
  192.168.100.30:5000/api2-202300476:1.0 api2
```

> `ctr` no publica puertos como docker; con `--net-host` cada API escucha en la IP de VM1.

### 10.3 VM2 — API3 (podman)

```bash
ssh root@192.168.100.20
podman run -d --name api3 -p 8083:8083 \
  -e PORT=8083 \
  -e TARGET_API1=http://192.168.100.10:8081/health \
  -e TARGET_API2=http://192.168.100.10:8082/health \
  192.168.100.30:5000/api3-202300476:1.0
```

### 10.4 Detener / redesplegar

```bash
# VM1 (detener y recrear)
ctr -n default task kill -a api1; ctr -n default container rm api1
# volver a ejecutar el ctr run de API1

# VM2
podman rm -f api3
# volver a ejecutar el podman run

# VM3 (Zot, conserva datos)
docker rm -f zot
# volver a ejecutar el docker run de Zot
```

---

## 11. Pruebas

### 11.1 /health

```bash
curl -s http://192.168.100.10:8081/health | jq .
curl -s http://192.168.100.10:8082/health | jq .
curl -s http://192.168.100.20:8083/health | jq .
```

### 11.2 Seis endpoints de llamada

```bash
curl -s http://192.168.100.10:8081/api1/202300476/call-api2 | jq .
curl -s http://192.168.100.10:8081/api1/202300476/call-api3 | jq .
curl -s http://192.168.100.10:8082/api2/202300476/call-api1 | jq .
curl -s http://192.168.100.10:8082/api2/202300476/call-api3 | jq .
curl -s http://192.168.100.20:8083/api3/202300476/call-api1 | jq .
curl -s http://192.168.100.20:8083/api3/202300476/call-api2 | jq .
```

### 11.3 Scripts de validación

```bash
bash scripts/validation/test-all.sh                # /health + 6 endpoints
bash scripts/validation/test-all.sh --error-test   # + prueba de error (detener/levantar API3)
bash scripts/validation/validate-registry.sh       # catálogo Zot
```

---

## 12. Solución de problemas frecuentes

1. **VMs "en pausa" (sin memoria en el host).**
   ```bash
   virsh -c qemu:///system resume SO1-VM1
   ```
   Cerrar apps del host; si persiste, reducir RAM de la VM (editar `dumpxml`).

2. **`ctr run` falla con "no users found".**
   La imagen usa `USER nobody` (por nombre). Usar `USER 65534:65534` (nobody numérico)
   en el Dockerfile y reconstruir.

3. **`ctr images tag` no actualiza un tag existente.**
   Borrar el ref primero: `ctr -n default images rm 192.168.100.30:5000/<img>:1.0`.

4. **No hay Internet en las VMs.**
   Verificar `lab-so1` activa y las reglas NAT/forwarding (§3.1).

5. **Zot no responde.**
   `docker start zot` en VM3 (datos en `/opt/zot/data`).

6. **Push/pull a registro HTTP rechazado.**
   Usar `--plain-http` (ctr) o `--tls-verify=false` (podman).

7. **SSH a VM falla ("No route to host").**
   La VM está apagada o en pausa; verificar con `virsh list --all` y `virsh start/resume`.

---

## 13. Referencias

- `docs/MANUAL_TECNICO.md` — manual técnico completo.
- `infra/NETWORKING.md` — detalle de red y VMs.
- `scripts/build-images.sh`, `scripts/validation/*.sh`.
