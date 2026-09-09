# Informe de integración — Proyecto 2 (rama `ricardious/p2-integration`)

**Carnet:** 202300476 · **Fecha:** 2026-09-08 · **Estado:** árbol listo para que
Muse lo divida en commits (sin commits realizados).

## 1. Resumen

Se completó la integración funcional de punta a punta del Proyecto 2: el daemon
Go ahora usa el **consumidor eBPF CO-RE real** (con degradación explícita y
configurable solo ante falta de privilegios), el **ciclo de vida del módulo de
kernel** se ejecuta de verdad mediante los scripts idempotentes existentes, y el
arranque de **Valkey/Grafana, módulo, cron y daemon** quedó reproducible y
documentado (daemon en host, Compose para servicios). Se añadieron el Makefile
de alto nivel, `.env.example`, README, `bootstrap/doctor/demo/cleanup.sh` y
pruebas de integración no privilegiadas (dobleces y smoke tests seguros).

## 2. Objetivos obligatorios y su estado

| # | Objetivo | Estado | Dónde |
|---|---|---|---|
| 1 | Consumidor eBPF CO-RE real con degradación configurable + correlación kill/tgkill antes de persistir | ✅ | `daemon/cmd/daemon/main.go`, `daemon/internal/ebpf/`, `daemon/internal/lifecycle/lifecycle.go` |
| 2 | Ciclo de vida del módulo por configuración (carga/descarga reales, scripts seguros y rutas configurables) | ✅ | `daemon/internal/lifecycle/module.go`, `main.go`, `internal/config` |
| 3 | Arranque reproducible Valkey/Grafana + módulo + cron + daemon; daemon en host; Grafana/Valkey protegidos | ✅ | `lifecycle/compose.go`, `deploy/docker-compose.yml`, README |
| 4 | Makefile + `.env.example` + README + bootstrap/doctor/demo/cleanup idempotentes | ✅ | raíz de `proyecto2/` |
| 5 | Contratos corregidos (protegidos `so1-p2-*`, `/proc`, esquema, claves Valkey, dashboard, cron 5 contenedores / 4 imágenes, mínimos 3/2) | ✅ | `config`, `valkey`, `scripts/install-cron.sh`, dashboard |
| 6 | Pruebas no privilegiadas y/o dobles + tests privilegiados opt-in con mensajes claros | ✅ | `lifecycle_extra_test.go`, `config_test.go`, `docker_test.go`, `test_stack.sh` |
| 7 | gofmt, vet, build, test, test -race, kernel no privilegiado, observabilidad, shell/Compose, smoke Docker seguro | ✅ | ver §5 |
| 8 | Todo dentro de `proyecto2/`, Proyecto 1 intacto, sin evidencias inventadas, listo para commits de Muse | ✅ | ver §3 |

## 3. Archivos cambiados / creados

**Modificados:**
- `daemon/cmd/daemon/main.go` — wiring real: consumidor CO-RE, módulo (Load/Unload),
  cron desde configuración, Compose down opcional, exit≠0 ante error de Run.
- `daemon/internal/config/config.go` — `EBPFMode` (auto|required|off),
  `ComposeFile` (vacío desactiva), `ModuleLoadScript/Unload`, `CronSchedule/Command`,
  nombres protegidos reales `so1-p2-grafana/so1-p2-valkey`, validaciones y overrides.
- `daemon/internal/config/config_test.go` — tests de los nuevos campos y contratos.
- `daemon/internal/ebpf/ebpf.go` — `IsPrivilegeError` (EPERM/EACCES/EOPNOTSUPP).
- `daemon/internal/ebpf/ebpf_test.go` — tests de `IsPrivilegeError`.
- `daemon/internal/ebpf/killmonitor_bpf.o` — regenerado (ver `kill_monitor.bpf.c`).
- `daemon/internal/lifecycle/lifecycle.go` — `startConsumer` con fallback por modo,
  `ComposeBootstrap` en `Run`, flag `degraded` atómico, `Degraded()`.
- `daemon/internal/valkey/valkey.go` — `AddRankings` codifica `"<id>|pid=<pid>"`.
- `daemon/internal/valkey/valkey_test.go` — ranking con PID.
- `daemon/internal/cron/cron.go` + `cron_test.go` — marcadores unificados con
  `install-cron.sh`/`remove-cron.sh`.
- `daemon/internal/docker/docker.go` + `docker_test.go` — **bug corregido**: la ruta
  de la API Docker requiere `/v<versión>` (antes `1.41` → 404 en `/_ping`).
- `deploy/docker-compose.yml` — `name: so1-p2`, labels `so1.p2.protect=1` en
  Valkey y Grafana.
- `ebpf/kill_monitor.bpf.c` + `README.md` — `handle_tgkill` reporta `tgid` (no el
  TID del hilo) para correlacionar con el PID de host del contenedor.
- `scripts/install-cron.sh` — instala el generador en `/usr/local/bin` y unifica
  la línea de cron con el comando del daemon.
- `tests/integration/observability/run.sh` — integra `test_stack.sh` (opt-in).

**Creados:**
- `Makefile` — targets `build test up down install run doctor demo ebpf module
  module-load module-unload install-cron remove-cron fmt vet lint clean purge`.
- `.env.example` — todas las variables `SO1_P2_*` con defaults documentados.
- `README.md` — quickstart, decisión de despliegue y contratos.
- `.gitignore` — artefactos locales (`bin/`, `.env`, objetos), con `!.env.example`.
- `scripts/bootstrap.sh`, `scripts/doctor.sh`, `scripts/demo.sh`, `scripts/cleanup.sh`.
- `daemon/internal/lifecycle/module.go` — ejecutor de scripts de módulo (inyectable).
- `daemon/internal/lifecycle/compose.go` — `ComposeRunner` + `ComposeBootstrap`.
- `daemon/internal/lifecycle/lifecycle_extra_test.go` — degradación/required/off,
  módulo y bootstrap Compose con dobles.
- `tests/integration/observability/test_stack.sh` — smoke real del stack (opt-in).

## 4. Decisiones técnicas

1. **Daemon en host, servicios en Compose.** El daemon necesita `/proc` del módulo
   y cargar eBPF con privilegios; correrlo en un contenedor complicaría
   mounts/privilegios sin beneficio. Compose ejecuta Valkey y Grafana con
   healthchecks; el daemon levanta el stack automáticamente si Valkey no responde
   (`SO1_P2_COMPOSE_FILE`), y `make up/down` lo hace manualmente.
2. **Degradación eBPF por modo** (`auto|required|off`). `auto` degrada SOLO si el
   error es de privilegios (EPERM/EACCES/EOPNOTSUPP); `required` falla con mensaje
   accionable y exit≠0; `off` desactiva explícito. Al degradar, ninguna eliminación
   se registra como confirmada (no hay eventos), con log explícito.
3. **Protección de Grafana/Valkey por triple vía:** no llevan la etiqueta de
   gestionado (no son candidatos), nombres protegidos reales `so1-p2-*`, y labels
   `so1.p2.protect=1` en Compose.
4. **Cron unificado:** daemon Go y `install-cron.sh` usan los mismos marcadores y
   el mismo comando `/usr/local/bin/so1-p2-generate-containers.sh` (`make install`
   lo coloca); `remove-cron.sh` retira solo el bloque delimitado.
5. **Rankings con PID:** el miembro del sorted set es `"<id>|pid=<pid>"` para que
   los paneles "Top 5 por RAM/CPU" muestren PID e ID sin join en Grafana.
6. **tgkill→tgid:** la correlación usa el PID de host del contenedor (TGID); el
   handler de `tgkill` emite `tgid`, no el TID del hilo.
7. **Bug Docker API corregido:** prefijo `/v` en la ruta (`v1.41`), detectado en el
   smoke test real del daemon (404 en `/_ping`).
8. **Exit codes:** `EBPFMode=required` sin privilegios termina con código 1 y
   mensaje accionable; apagado ordenado por señal termina con 0.

## 5. Validaciones ejecutadas (comandos y resultados exactos)

Host: Fedora 42, kernel `6.19.14`, Go 1.25.10, Docker 29.5.3 (Compose v5.1.4),
clang 20, **sin root**, **sin headers de kernel**, **sin crontab**, BTF presente.

| Comando | Resultado |
|---|---|
| `gofmt -l daemon/` | sin archivos listados (limpio) |
| `go vet ./...` | OK |
| `go build ./...` | OK |
| `go test ./...` | 9 paquetes OK |
| `go test -race ./...` | 9 paquetes OK (sin carreras) |
| `bash tests/kernel/test-all.sh` | **OK** con SKIPs documentados (headers ausentes, root ausente) |
| `bash tests/integration/observability/run.sh` | **51 pruebas OK, 0 fallidas** (build 4 imágenes, 5 contenedores con semilla, etiquetas, límites, lock, cron mock, compose, dashboard, claves) |
| `bash -n` sobre todos los scripts | OK |
| `docker compose -f deploy/docker-compose.yml config --quiet` | OK |
| `SO1_RUN_STACK=1 bash tests/integration/observability/test_stack.sh` | **8 OK**: compose válido, proyecto `so1-p2`, valkey PONG, grafana /api/health, protegidos y NO gestionados |
| `make ebpf` (object + bindings) | OK |
| Smoke daemon (modo `auto`, valkey real vía Compose, `/proc`=fixture) | degrada eBPF con mensaje claro, persiste `so1:p2:system:latest` + `system:timeline` en Valkey, apagado limpio por SIGTERM |
| Smoke daemon (`EBPF_MODE=required`, sin root) | error accionable y **exit=1** |
| Smoke daemon (`EBPF_MODE=off`, compose auto) | arranca sin eBPF; ante Valkey caído **levanta Compose** y continúa |
| `make build / up / down / demo / doctor / bootstrap --skip-docker` | OK |
| `scripts/cleanup.sh --no-confirm` | retira solo recursos `so1.p2.*`; los 4 contenedores del Proyecto 1 (smartegg) intactos |

## 6. Riesgos residuales

- **eBPF/ módulo sin probar con privilegios aquí** (host sin root/headers): los tests
  privilegiados quedan como opt-in y omiten con mensaje claro; deben correrse en la
  VM objetivo (`sudo bash tests/kernel/test-module-lifecycle.sh` y el test
  `TestBPFConsumerIntegracionPrivilegiada`).
- **Versión de Compose local** (`v5.1.4`) no soporta `--wait`; el runner reintenta
  `up -d` y espera por healthchecks/ping — cubierto pero a confirmar en la VM.
- **CPU del módulo** usa la convención de promedio de vida (`ps`); la rúbrica admite
  valores diferenciales grandes.
- **`restart: unless-stopped`** en Compose puede reencender servicios tras
  `down`-prematuro; `cleanup.sh` hace `down` y elimina contenedores del proyecto.
- **Dependencia de internet** para `grafana cli plugins install redis-datasource`
  en el build de la imagen Grafana (reproducible pero requiere red la primera vez).