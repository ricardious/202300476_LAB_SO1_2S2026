# Auditoría final contra la rúbrica — Proyecto 2 SO1 (202300476)

**Rama auditada:** `ricardious/p2-integration` · **Fecha:** 2026-09-09
**Criterio de terminado:** cero brechas críticas conocidas sin documentar,
árbol listo para commit, reporte exacto de cambios/pruebas/riesgos.

---

## 1. Resumen ejecutivo

Se auditó requisito por requisito el 100 % de la rúbrica (85 % técnica +
15 % habilidades). Todos los componentes obligatorios están implementados y
probados: módulo `/proc`, daemon Go, cron + 4 imágenes, Valkey/Grafana con
dashboard provisionado y eBPF CO-RE con correlación de eliminaciones.

Se encontraron y **corrigieron** tres brechas concretas en esta auditoría:

1. **Dashboard RAM vacío (bug real):** los paneles `RAM total / usada / libre`
   hacían `HGET` con los campos `total_kb/used_kb/free_kb`, pero el daemon
   persiste el hash `so1:p2:system:latest` con `mem_total_kb/mem_used_kb/
   mem_free_kb`. Las tarjetas no mostraban nada. Se alinearon los campos del
   dashboard (verificado contra Valkey real vía `/api/ds/query`).
2. **Cron/instalación del generador:** el generador instalado en
   `/usr/local/bin` (cron/daemon) no encontraba `workloads/` y, si las imágenes
   faltaban, no podía construirlas → **cero contenedores** en una VM limpia.
   Ahora resuelve la raíz del proyecto por `SO1_P2_PROJECT_DIR`, por symlink
   (`readlink -f`) o por cwd; `make install` e `install-cron.sh` instalan el
   generador como **symlink** y la línea de cron exporta `SO1_P2_PROJECT_DIR`.
3. **Series temporales sucias:** el stream `kills` no tenía un campo de
   conteo y los paneles temporales incluían columnas no-métricas (`ts`,
   `process_count`, etc.). Se añadió `n=1` a cada evento de kill y
   transformaciones "organize" a los dos paneles temporales para graficar solo
   `mem_used_kb` y `n`.

Además se **creó la documentación de entrega** que faltaba para el criterio
4.1: `MANUAL_TECNICO.md`, `GUIA_INSTALACION.md`, `GUIA_DEMO.md` y `DEFENSA.md`.

> Los ítems privilegiados (cargar el módulo, adjuntar eBPF) **no pueden
> ejecutarse en este host** (sin root/headers): los tests existentes los omiten
> con mensaje claro y quedan listados en §5 como evidencia obligatoria en la VM.

---

## 2. Tabla requisito → puntos → archivo/prueba → estado

Leyenda de estado: ✅ verificado en este host · 🟡 verificado por código/tests,
falta evidencia privilegiada de VM · 🔴 brecha (corregida o pendiente).

| Criterio | Pts máx | Archivo / prueba | Estado | Pts est. |
|---|---|---|---|---|
| **1.1** Compilación y enlazado módulo | 8 | `kernel/Makefile`; `tests/kernel/test-build.sh`; `tests/kernel/VERIFICACION.md` (§1, compilación real contra headers 6.19) | ✅ (compilación real documentada; local SKIP por headers) | 8 |
| **1.2** Carga y descarga del módulo | 8 | `scripts/load-module.sh`, `unload-module.sh` (idempotentes, verifican `/proc` y `/proc/modules`); `tests/kernel/test-module-lifecycle.sh` (privilegiado) | 🟡 código y scripts correctos; ejecución real pendiente en VM | 7 |
| **1.3** Creación/acceso/legible `/proc` | 8 | `kernel/continfo.c` (`proc_create` + `seq_file`); `tests/kernel/validate_schema.py` (fixture OK); nombre exacto `continfo_pr2_so1_202300476` | 🟡 fixture validado; lectura en vivo pendiente en VM | 8 |
| **1.4** Compilación y ejecución daemon GO | 8 | `daemon/` (`go build`, `vet`, `test`, `-race` OK); `bin/so1-p2-daemon`; smoke real del stack | ✅ build/test; ejecución completa (módulo+eBPF) en VM | 8 |
| **1.5** Interacción `/proc` + Valkey | 8 | `internal/procfs` (fixtures válidos/truncados/desconocidos), `internal/valkey` (miniredis); smoke real: daemon persiste `system:latest`/`system:timeline`; ds/query contra Valkey real devuelve datos | ✅ | 8 |
| **2.1** Iteración `task_struct` (kernel) | 7 | `continfo.c` (`for_each_process` + `get_task_struct` bajo RCU + snapshot); revisión de seguridad/concurrencia | ✅ código; contraste con `ps` pendiente en VM | 7 |
| **2.2** Extracción PID/memoria/CPU | 8 | campos completos (PID, name, cmdline, container_id, VSZ, RSS, %mem, %cpu) con unidades y fórmulas `ps`-like; `validate_schema.py` | ✅ código + fixture; comparación contra `free/ps` en VM | 8 |
| **2.3** Parseo en GO → Valkey | 5 | `procfs.Parse` (tipado, valida contrato, tolera desconocidos), `valkey` persistencia; tests unitarios + Valkey real | ✅ | 5 |
| **3.1** eBPF se adjunta a `sys_kill` y captura PID/señal | 12 | `ebpf/kill_monitor.bpf.c` (kill + tgkill, ring buffer 16 MiB); `TestBPFConsumerIntegracionPrivilegiada` (se omite sin root); objeto CO-RE embebido parsea | 🟡 código + parseo del objeto; adjunto real pendiente en VM | 10 |
| **3.2** Evento eBPF → Go → Valkey (confirmación) | 13 | `Correlator` (PID+señal+ventana, sin falsos positivos); `lifecycle` registra kill solo tras evento; tests `TestKillOnlyConfirmedAfterEBPFEvent` + real (opt-in) | 🟡 correlación unitaria probada; cadena real completa en VM | 11 |
| **4.1** Manual técnico y de usuario | 10 | `MANUAL_TECNICO.md`, `GUIA_INSTALACION.md`, `GUIA_DEMO.md`, `README.md`, `ebpf/README.md`, `docs/INFORME_INTEGRACION.md` | ✅ creados/completos | 9 |
| **4.2** Comprensión (defensa) | 5 | `docs/DEFENSA.md` (Q&A kernel, daemon, eBPF, Valkey, cron) | ✅ guía lista (depende del estudiante) | 4 |
| **Total** | **100** | | | **≈ 93 (con servidumbres) · objetivo tras VM ≈ 98–100** |

Estimación **conservadora sobre 100: 93**. Se descuentan puntos en 1.2 (7),
3.1 (10) y 3.2 (11) únicamente porque la ejecución privilegiada en la VM aún
no está evidenciada; una vez capturadas las evidencias de §5, el techo es
98–100 (los 2 puntos restantes dependen de la claridad de la defensa oral).

---

## 3. Requisitos transversales del enunciado

| Requisito | Implementación | Verificación |
|---|---|---|
| Módulo `/proc` con memoria total/libre/usada | `continfo.c` (`si_meminfo`, `used=total-free`) | `validate_schema.py --strict-memory` |
| Procesos con PID, nombre/cmdline o container id, VSZ, RSS, %mem, %cpu | snapshot `task_struct` + cgroup + cmdline | fixture + (VM) contraste con `ps` |
| Daemon: lectura robusta e intervalo 20-60 s | `config` valida `[20s,60s]`; loop ticker | `config_test` |
| Arranque Grafana/Valkey | `lifecycle.ComposeBootstrap` + `make up` | smoke real del stack |
| Cron y carga: 5 contenedores aleatorios cada 2 min, 4 imágenes (3 perfiles + intruso) | `install-cron.sh` + `generate-containers.sh` (semilla, límites, etiquetas, lock) | run.sh (55 OK) |
| Política: mínimo 3 low y 2 high; nunca Grafana | `policy.Decide` + triple protección (managed=ausente, nombres protegidos, `protect=1`) | `policy_test`, `lifecycle_test`, inspect real |
| Eliminación registrada solo tras confirmación eBPF | `Correlator` + `runEventConsumer` + `RecordKill` | unit + (VM) cadena real |
| Apagado ordenado sin borrar recursos ajenos | `signal.NotifyContext`; `cron.Remove`; `cleanup.sh` etiquetado | tests + (VM) |
| JSON válido, seq_file, seguridad/concurrencia | `single_open`, snapshot bajo RCU, escape completo, cota 65536 | revisión + fixture |
| Dashboard con todos los paneles de la rúbrica | `so1-p2-overview.json` (14 paneles) | run.sh + ds/query real |

---

## 4. Cambios realizados en esta auditoría (sin commit, listos para Muse)

| Archivo | Cambio |
|---|---|
| `deploy/grafana/dashboards/so1-p2-overview.json` | Campos HGET alineados con el hash real (`mem_total_kb/used_kb/free_kb`); transformaciones "organize" en los 2 paneles temporales |
| `daemon/internal/valkey/valkey.go` | `RecordKill` agrega `n=1` al stream `so1:p2:kills` (serie limpia para el panel temporal) |
| `daemon/internal/valkey/valkey_test.go` | Tests: campo `n` del stream; campos del hash `system:latest` alineados con el dashboard |
| `scripts/generate-containers.sh` | Resolución robusta del proyecto (`SO1_P2_PROJECT_DIR` → symlink → cwd); `ensure_image` usa `PROJECT_ROOT/workloads` |
| `scripts/install-cron.sh` | Instala el generador como **symlink**; la línea de cron `cd` + exporta `SO1_P2_PROJECT_DIR` |
| `Makefile` | `install` crea symlink del generador (no copia) |
| `.env.example` | Documenta `SO1_P2_PROJECT_DIR` |
| `README.md` | Nota de instalación symlink + SO1_P2_PROJECT_DIR; referencia a la nueva documentación |
| `tests/integration/observability/run.sh` | Entorno hermético (limpia contenedores previos del proyecto); test del generador instalado (symlink+env); test de `SO1_P2_PROJECT_DIR` en la línea de cron; test de alineación de campos HGET |
| `docs/MANUAL_TECNICO.md`, `docs/GUIA_INSTALACION.md`, `docs/GUIA_DEMO.md`, `docs/DEFENSA.md` | **Nuevos**: documentación de entrega del criterio 4.1/4.2 |
| `docs/AUDITORIA_RUBRICA.md` | Este informe |

## 5. Pruebas ejecutadas en este host (sin privilegios)

| Prueba | Resultado |
|---|---|
| `go build ./...` / `go vet ./...` / `gofmt -l .` | OK / OK / limpio |
| `go test -count=1 ./...` (9 paquetes) | OK |
| `go test -race -count=1 ./...` (9 paquetes) | OK sin carreras |
| `bash tests/kernel/test-all.sh` | OK (SKIP documentados: headers y root) |
| `bash tests/integration/observability/run.sh` | **55 OK / 0 fallidas** (build 4 imágenes, generador repo + instalado, etiquetas, límites, lock, cron mock con SO1_P2_PROJECT_DIR, compose, dashboard y campos HGET) |
| `SO1_RUN_STACK=1 bash tests/integration/observability/test_stack.sh` | 8 OK (Valkey PONG, Grafana /api/health, protegidos y NO gestionados) |
| `docker compose -f deploy/docker-compose.yml config --quiet` | OK |
| Grafana `/api/ds/query` contra Valkey real (todos los targets del dashboard) | Devuelve datos reales (hget/xlen/get/xrange/zrange) |
| `bash -n` de todos los scripts | OK |

## 6. Evidencias que el usuario debe capturar en la VM (checklist)

Privilegiadas (con `sudo`), en orden de la rúbrica:

1. `sudo make module` → salida de compilación y `ls proyecto2/kernel/*.ko`.
2. `sudo make module-load` → `dmesg | tail`, `ls -l /proc/continfo_pr2_so1_202300476`,
   `cat ... | python3 -m json.tool` (JSON válido), y `sudo make module-unload` →
   archivo eliminado. (Criterios 1.1, 1.2, 1.3, 2.1, 2.2.)
3. Contraste de métricas: `ps -eo pid,comm,vsz,rss,%mem,%cpu --sort=-rss | head` y
   `free -k` frente a la salida del módulo (2.2).
4. `make build` y arranque del daemon con root:
   `sudo make run` (o `sudo /usr/local/bin/so1-p2-daemon`) → logs de carga de
   módulo, cron instalado, Compose up, eBPF adjunto (1.4, 1.5).
5. `docker exec so1-p2-valkey valkey-cli HGETALL so1:p2:system:latest` y
   `XRANGE so1:p2:system:timeline - +` con datos (1.5, 2.3).
6. eBPF real: `sudo go test ./internal/ebpf/ -run Privilegiada -v -count=1`
   → confirma evento `kill` con PID/señal (3.1).
7. Cadena completa 3.2: ejecutar `bash scripts/generate-containers.sh` 2-3 veces
   (10-15 contenedores), esperar el loop del daemon y mostrar:
   `docker ps` (5 supervivientes: ≥3 low y ≥2 high; Grafana intacta),
   `valkey-cli XRANGE so1:p2:kills - +` y `GET so1:p2:kills:count` > 0
   con timestamps coherentes a los kills solicitados (3.2).
8. Cron: `crontab -l` (bloque delimitado cada 2 min) y, con el daemon detenido,
   `crontab -l` de nuevo (bloque retirado) (apagado ordenado).
9. Dashboard: captura de pantalla de **cada panel** con datos reales
   (cards RAM, evolución temporal, eliminaciones, top 5 RAM/CPU, tabla eBPF).
10. `bash scripts/cleanup.sh --no-confirm` y comprobar que los recursos del
    Proyecto 1 (contenedores `smartegg`) siguen intactos.

## 7. Riesgos residuales (documentados, no bloqueantes)

- **eBPF y módulo sin ejecutar en este host** (sin root/headers). El código y
  los tests existen; la evidencia de la cadena real debe capturarse en la VM
  (§6.6, §6.7). Si el kernel de la VM no publicara los tracepoints o rechazara
  el programa, el daemon lo reporta explícitamente (modo `auto` degrada; modo
  `required` falla).
- **Renderizado de los paneles:** todos los targets del dashboard fueron
  consultados contra Valkey real vía `/api/ds/query` y devuelven datos
  (cards RAM 8388608/4194304/4194304 KB, series temporales, XLEN/GET, top 5,
  tablas). Aun así debe confirmarse visualmente en Grafana dentro de la VM
  (§6.9).
- **Primer arranque de Grafana requiere red** para instalar `redis-datasource`
  en el build de la imagen (reproducible, una sola vez).
- **`%cpu` promedio de vida** (convención `ps`): la rúbrica admite valores
  grandes; se documenta en el manual.
- **Compose `restart: unless-stopped`**: `cleanup.sh` hace `down` y elimina los
  contenedores del proyecto; los datos en volúmenes se conservan.

## 8. Criterio de terminado

- Cero brechas críticas conocidas sin documentar: ✅ (las tres brechas
  encontradas se corrigieron; los ítems privilegiados están identificados y
  tienen evidencia asignada en §6).
- Árbol listo para commit: ✅ (`git status` solo muestra los cambios
  intencionados; sin commits realizados, listo para Muse).
- Reporte exacto de cambios/pruebas/riesgos: ✅ (este documento).