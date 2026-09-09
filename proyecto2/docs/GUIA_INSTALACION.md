# Guía de Instalación — Proyecto 2 SO1 (202300476)

Guía paso a paso para instalar y verificar el sistema completo en una VM Linux
(Fedora/Debian/Ubuntu) con Docker y privilegios de root.

---

## 0. Prerrequisitos

| Requisito | Versión mínima | Verificación |
|---|---|---|
| Linux con kernel con headers | kernel-devel == `uname -r` | `ls /lib/modules/$(uname -r)/build/Makefile` |
| GCC / make | — | `gcc --version` |
| Go | 1.25 | `go version` |
| Docker + compose | engine moderno | `docker info` y `docker compose version` |
| clang | 14+ (build eBPF, si se regenera) | `clang --version` |
| BTF del kernel | presente | `ls /sys/kernel/btf/vmlinux` |
| Python 3 | — | `python3 --version` (validadores) |
| Root (o sudo) | — | para módulo, eBPF y `make install` |

> En Fedora: `sudo dnf install kernel-devel-$(uname -r) gcc make go docker-ce`
> En Debian/Ubuntu: `sudo apt install linux-headers-$(uname -r) build-essential golang docker.io`

## 1. Obtener el código

```bash
git clone <repo> <carpeta>
cd <carpeta>/proyecto2
```

## 2. Configuración

```bash
cp .env.example .env      # opcional; los defaults son seguros
```

Revise (si lo necesita) `SO1_P2_*` en `.env`. Los defaults cumplen el
enunciado: loop 20 s, mínimos 3/2, cron `*/2 * * * *`, SIGKILL, eBPF `auto`.

## 3. Compilar

```bash
make build                # daemon Go → bin/so1-p2-daemon
make module               # módulo de kernel (continfo.ko)
make ebpf                 # objeto + bindings eBPF (opcional si no cambió el C)
```

## 4. Levantar Valkey y Grafana

```bash
make up
docker ps --format '{{.Names}} {{.Status}}' | grep so1-p2
```

Deben aparecer `so1-p2-valkey` (healthy) y `so1-p2-grafana` (healthy). El
dashboard estará en `http://localhost:3000` (admin/admin).

> **Nota (volumen Grafana):** los dashboards provisionados se copian dentro del
> volumen `so1-p2-grafana-data`, que tiene prioridad sobre la imagen. En una VM
> **nueva** no hay volumen y se usa la versión de la imagen (correcta). Si el
> stack ya se construyó antes y se actualizó `so1-p2-overview.json`, reconstruya
> la imagen y sincronice el archivo en el contenedor:
> `docker compose -f deploy/docker-compose.yml build grafana` y
> `docker cp deploy/grafana/dashboards/so1-p2-overview.json so1-p2-grafana:/var/lib/grafana/dashboards/`
> (el provisioner recarga en ~30 s).

## 5. Cargar el módulo de kernel

```bash
sudo make module-load     # compila si falta e insmod (idempotente)
cat /proc/continfo_pr2_so1_202300476 | python3 -m json.tool   # ver JSON
sudo make module-unload   # rmmod idempotente
```

## 6. Instalar el daemon y el generador

```bash
sudo make install         # daemon + generador (symlink) en /usr/local/bin
make install-cron         # cronjob cada 2 min (idempotente, delimitado)
```

Verifique el cron:

```bash
crontab -l
# debe incluir el bloque delimitado # >>> so1-p2-202300476:begin >>>
```

## 7. Ejecutar el daemon

```bash
make run                  # lee .env y ejecuta el daemon en el host
# o en background:
sudo nohup /usr/local/bin/so1-p2-daemon >> /var/log/so1-p2-daemon.log 2>&1 &
```

El daemon, al arrancar:
1. Carga el módulo (idempotente).
2. Instala/refresca su cronjob.
3. Si Valkey no responde, levanta Compose.
4. Adjunta eBPF (modo `auto`; sin privilegios degrada con mensaje claro).
5. Ejecuta el loop cada 20 s.

Detener con `Ctrl+C` o `kill -TERM <pid>`: retira solo el cronjob del proyecto
y libera eBPF.

## 8. Pruebas

Sin privilegios:

```bash
make test                                     # go test + -race + shell + Compose
SO1_SKIP_DOCKER=1 bash tests/integration/observability/run.sh
bash tests/kernel/test-all.sh                 # compilación si hay headers; schema
```

Privilegiadas (VM objetivo):

```bash
sudo bash tests/kernel/test-module-lifecycle.sh
sudo go test ./internal/ebpf/ -run Privilegiada -v -count=1
```

## 9. Diagnóstico

```bash
make doctor               # checks no privilegiados
sudo bash scripts/doctor.sh --privileged      # + módulo/eBPF
```

## 10. Limpieza segura

```bash
bash scripts/cleanup.sh                       # solo recursos del proyecto (pide confirmación)
bash scripts/cleanup.sh --no-confirm --module --purge
```

`cleanup.sh` solo toca recursos con la etiqueta `so1.p2.project=202300476` y el
proyecto Compose `so1-p2`; los recursos del Proyecto 1 quedan intactos.

## 11. Verificación final

Una instalación correcta cumple estas condiciones:

- `make build`, `make module` y `make up` terminan sin errores.
- `/proc/continfo_pr2_so1_202300476` aparece al cargar el módulo y desaparece
  al descargarlo.
- El JSON de `/proc` pasa `tests/kernel/validate_schema.py`.
- Grafana presenta datos en todos los paneles solicitados.
- Con más de cinco contenedores, el daemon conserva al menos tres de bajo
  consumo y dos de alto consumo.
- `so1:p2:kills:count` aumenta únicamente después de una señal confirmada por
  eBPF.
- Las pruebas privilegiadas del módulo y de eBPF terminan correctamente.
