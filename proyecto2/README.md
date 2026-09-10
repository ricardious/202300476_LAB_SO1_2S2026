# Proyecto 2 — Sonda de Kernel + Daemon Go (202300476)

Sistema Linux de telemetría y gestión autónoma de contenedores: un **módulo de
kernel C** expone métricas por `/proc`, una **sonda eBPF CO-RE** audita las
syscalls `kill`/`tgkill`, un **daemon Go** decide qué contenedores eliminar
(conservando 3 de bajo consumo y 2 de alto consumo), y **Valkey + Grafana**
persisten y visualizan los datos. Como actividad adicional, incluye un módulo
**Rust** que registra `Hola Mundo 202300476` en el log del kernel.

```
Linux kernel
  ├─ módulo C ──────────> /proc/continfo_pr2_so1_202300476 (JSON)
  └─ eBPF kill/tgkill ──> ring buffer ─┐
                                        │
Docker API <──────── daemon Go <────────┘
          ├─ política (mínimos 3 low / 2 high, protegidos excluidos)
          ├─ cronjob (5 contenedores aleatorios cada 2 min)
          └─ Valkey  <── Grafana (datasource + dashboard provisionados)
```

## Arquitectura de despliegue

El **daemon corre en el HOST**, no en un contenedor: necesita leer
`/proc/continfo_pr2_so1_202300476` (módulo) y cargar eBPF con privilegios.
**Compose ejecuta Valkey y Grafana** como servicios aislados. El daemon levanta
el stack Compose automáticamente si Valkey no responde
(`SO1_P2_COMPOSE_FILE`); `make up` hace lo mismo manualmente.

## Quickstart

```bash
cd proyecto2
cp .env.example .env          # opcional; los defaults son seguros
make build                    # bin/so1-p2-daemon
make up                       # Valkey + Grafana (Compose, proyecto so1-p2)
make test                     # go test + pruebas shell/Compose sin privilegios
make doctor                   # diagnóstico del entorno
make evidence                 # captura automática de evidencias (docs/evidencias/)
make verify-delivery          # preentrega estricta (0=lista · 1=fallos · 2=uso)
sudo make install             # daemon + generador en /usr/local/bin
make run                      # daemon en el host (módulo + eBPF con root)
```

Con `sudo` también disponible:

```bash
bash scripts/bootstrap.sh --with-module --with-cron   # entorno completo
make demo                                             # demo de punta a punta
```

## Contratos compartidos

| Contrato | Valor |
|---|---|
| Interfaz `/proc` | `/proc/continfo_pr2_so1_202300476` (JSON, `schema_version: 1`) |
| Prefijo Valkey | `so1:p2:` (`system:latest`, `system:timeline`, `containers:timeline`, `rank:ram`, `rank:cpu`, `kills`, `kills:count`) |
| Perfiles de carga | `memory-high`, `cpu-high`, `low`, `intruder` (4 imágenes `so1/load:202300476-<perfil>`) |
| Etiqueta gestionada | `so1.p2.managed=1` |
| Protegidos por nombre | `so1-p2-grafana`, `so1-p2-valkey` (+ `grafana`, `valkey`) |
| Protegidos por etiqueta | `so1.p2.protect=1` |
| Cron | `*/2 * * * *` → `/usr/local/bin/so1-p2-generate-containers.sh` (5 contenedores aleatorios) |
| Señal | `SIGKILL`; la eliminación solo se registra en Valkey tras evento eBPF correlacionado |

El ranking histórico guarda el miembro `"<id>|pid=<pid>"` para que los paneles
"Top 5 histórico por RAM/CPU" muestren PID e ID.

> `make install` e `install-cron.sh` instalan el generador como **enlace
> simbólico** al script del repositorio y la línea de cron exporta
> `SO1_P2_PROJECT_DIR`, de modo que el generador encuentre `workloads/` aunque
> cron lo invoque desde otro directorio de trabajo.

## eBPF y degradación

El consumidor **CO-RE real** (`daemon/internal/ebpf`) se adjunta a
`syscalls:sys_enter_kill` y `syscalls:sys_enter_tgkill`. `SO1_P2_EBPF_MODE`:

- `auto` (default): carga el puente real; si el entorno no tiene root/CAP_BPF,
  degrada a un consumidor nulo con un mensaje explícito (sin falsos positivos:
  ninguna eliminación se marca confirmada sin evento real).
- `required`: sin privilegios el daemon **no arranca** y termina con un error
  accionable.
- `off`: desactiva eBPF explícitamente.

## Estructura

```
proyecto2/
├── Makefile  .env.example  README.md
├── kernel/    # continfo.c (módulo /proc) + Makefile
├── rust-module/ # actividad extra: módulo Hola Mundo con lógica Rust
├── ebpf/      # kill_monitor.bpf.c + Makefile
├── daemon/    # cmd/daemon + internal/{config,procfs,docker,policy,valkey,ebpf,cron,lifecycle}
├── workloads/ # memory-high, cpu-high, low, intruder (4 imágenes)
├── deploy/    # docker-compose.yml + grafana/provisioning + dashboard
├── scripts/   # bootstrap, doctor, demo, cleanup, load/unload-module, generate-containers,
│              # install/remove-cron, captura de evidencias y validación
├── tests/     # fixtures + kernel + integration/observability
└── docs/      # enunciado, manual técnico, guía de instalación y evidencias
    └── evidencias/ # capturas y resultados de ejecuciones reales
```

## Pruebas

```bash
make test                          # go test (con -race) + shell/Compose + kernel no privilegiado
make verify-delivery               # preentrega estricta (manifest, JSON, shell, Go, kernel, observabilidad, secretos)
SO1_SKIP_DOCKER=1 bash tests/integration/observability/run.sh
sudo bash tests/kernel/test-module-lifecycle.sh    # prueba privilegiada (opt-in)
sudo go test ./internal/ebpf/ -run Privilegiada -v # eBPF real (opt-in)
make rust-module                   # compila la actividad extra en Rust
sudo make rust-module-load         # carga y registra Hola Mundo 202300476
sudo make rust-module-unload       # descarga el módulo extra
```

Las pruebas privilegiadas (módulo, eBPF real) se **omiten automáticamente**
con mensajes claros cuando el host no tiene root/CAP_BPF o headers del kernel.

## Actividad extra en Rust

`rust-module/hola_rust.rs` implementa el mensaje y el ciclo lógico de carga y
descarga. Un adaptador C mínimo registra `module_init/module_exit`, porque el
paquete `kernel-devel` de Fedora no distribuye el crate interno completo de
Rust for Linux necesario para compilar un módulo externo puramente Rust. El
resultado sigue siendo un único módulo cargable `hola_rust_202300476.ko` y la
prueba automatizada valida nombre, `vermagic` y el mensaje con el carnet.

```bash
make rust-module
sudo make rust-module-load
sudo dmesg | grep -E 'Hola Mundo 202300476|Modulo Rust 202300476'
sudo make rust-module-unload
```

## Evidencias y entrega

- `bash scripts/capture-evidence.sh` (o `make evidence`) registra el entorno,
  el módulo, los contenedores, el cron, Valkey y Grafana sin incluir secretos.
- [`docs/evidencias/README.md`](docs/evidencias/README.md) presenta las capturas
  reales junto con una explicación breve de cada prueba.

## Limpieza segura

```bash
bash scripts/cleanup.sh            # retira SOLO recursos del proyecto (pide confirmación)
bash scripts/cleanup.sh --no-confirm --module --purge
```

Nunca elimina contenedores/volúmenes/redes ajenos.

Documentación: [`docs/MANUAL_TECNICO.md`](docs/MANUAL_TECNICO.md),
[`docs/GUIA_INSTALACION.md`](docs/GUIA_INSTALACION.md),
[`docs/evidencias/README.md`](docs/evidencias/README.md) y
[`ebpf/README.md`](ebpf/README.md).
