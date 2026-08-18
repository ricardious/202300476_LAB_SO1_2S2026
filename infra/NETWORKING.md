# NETWORKING — Máquinas Virtuales del Proyecto 1

> Sistemas Operativos 1 · 2do Semestre 2026 · Carnet: **202300476**
> Virtualización: **KVM / libvirt** (host Fedora 42). Sin reemplazo por VirtualBox/VMware.

## Resumen

Tres máquinas virtuales Linux creadas con KVM/libvirt sobre la red NAT `lab-so1`
(bridge `virbr1`, subred `192.168.100.0/24`, gateway `192.168.100.1`).
Cada VM tiene IP estática identificable, comunicación VM↔VM en los 6 sentidos,
acceso a Internet (NAT) y capacidad de recibir conexiones HTTP entre ellas.

## Tabla de máquinas virtuales

| Nombre (libvirt) | Hostname | IP | Rol | Runtime (posterior) | Servicios que ejecutará |
|---|---|---|---|---|---|
| SO1-VM1 | so1-vm1 | 192.168.100.10 | Servicios / desarrollo de APIs | Containerd | API1, API2 |
| SO1-VM2 | so1-vm2 | 192.168.100.20 | Servicios / desarrollo de APIs | Podman | API3 |
| SO1-VM3 | so1-vm3 | 192.168.100.30 | Registro de imágenes | Docker | Zot Registry |

### Distribución final (obligatoria, no se cambia)

```
VM1  -> Containerd -> API1 y API2
VM2  -> Podman     -> API3
VM3  -> Docker     -> Zot
```

## Detalle por VM

### SO1-VM1 (192.168.100.10)
- Sistema: Debian 12 (Linux)
- vCPU: 2 · RAM: 1280 MiB · Disco: 12 GB (qcow2 con base compartida)
- MAC: `52:54:00:aa:00:01`
- Runtime futuro: **Containerd** · Contenedores: **API1, API2**
- Acceso: SSH `root@192.168.100.10`

### SO1-VM2 (192.168.100.20)
- Sistema: Debian 12 (Linux)
- vCPU: 1 · RAM: 1024 MiB · Disco: 12 GB (qcow2 con base compartida)
- MAC: `52:54:00:aa:00:02`
- Runtime futuro: **Podman** · Contenedor: **API3**
- Acceso: SSH `root@192.168.100.20`

### SO1-VM3 (192.168.100.30)
- Sistema: Debian 12 (Linux)
- vCPU: 2 · RAM: 1536 MiB · Disco: 12 GB (qcow2 con base compartida)
- MAC: `52:54:00:aa:00:03`
- Runtime futuro: **Docker** · Contenedor: **Zot Registry** (puerto 5000)
- Acceso: SSH `root@192.168.100.30`

## Red

- Red libvirt: **lab-so1** (NAT)
- Bridge: **virbr1** — IP del host en la red: `192.168.100.1`
- Subred: `192.168.100.0/24`
- Gateway/DNS para las VMs: `192.168.100.1` (el host hace NAT)
- DHCP: `192.168.100.2 – 192.168.100.250` (las VMs usan IP estática)

| Elemento | Valor |
|---|---|
| VM1 | 192.168.100.10 |
| VM2 | 192.168.100.20 |
| VM3 | 192.168.100.30 |
| Gateway | 192.168.100.1 |

## Conectividad verificada

- [x] Las 3 VMs bootean correctamente (Debian 12, multi-user).
- [x] IPs identificables: .10 / .20 / .30.
- [x] Comunicación VM↔VM (ping en los 6 sentidos: VM1↔VM2, VM1↔VM3, VM2↔VM3).
- [x] HTTP entre VMs (ej.: desde VM2 y VM3 a `http://192.168.100.10:8080` → HTTP 200).
- [x] Acceso a Internet desde las 3 VMs (HTTP 200 a https://www.debian.org).
- [x] Aparecen en `virsh list --all` y en virt-manager.
- [x] SSH funcional (clave pública).

## Acceso

- Usuario: **root**
- Autenticación SSH por clave pública del host (`~/.ssh/id_ed25519`) en `/root/.ssh/authorized_keys`.
  > La contraseña de acceso a las VMs no se documenta en el repositorio (se entrega por
  > otro medio durante la calificación).

## Comandos de administración

```bash
virsh -c qemu:///system list --all          # listar VMs
virsh -c qemu:///system start SO1-VM1       # iniciar
virsh -c qemu:///system shutdown SO1-VM1    # apagado
virsh -c qemu:///system destroy SO1-VM1     # forzar apagado
virsh -c qemu:///system net-list --all      # redes (lab-so1)
ssh root@192.168.100.10                     # acceso SSH a VM1
```
