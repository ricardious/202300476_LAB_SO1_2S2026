# Checklist de Entrega — Proyecto 2 SO1 (202300476)

Runbook de **preentrega y defensa** para la rama `ricardious/p2-delivery`.
Complementa a `GUIA_INSTALACION.md` y `GUIA_DEMO.md`; aquí está el orden exacto
para una **VM limpia**, la **lista de capturas**, el **guion de defensa** y la
**entrega Git**.

> Herramientas de esta entrega:
> - `bash scripts/capture-evidence.sh` — captura automática de evidencias (texto).
> - `bash scripts/verify-delivery.sh` — preentrega estricta (código 0/1/2).
> - `make evidence` / `make verify-delivery` — atajos del Makefile.
> - Rúbrica y auditoría: `docs/AUDITORIA_RUBRICA.md` (estimación ≈ 93/100, techo 98–100).

---

## 1. Prerrequisitos de la VM limpia

| Requisito | Verificación |
|---|---|
| Linux con headers del kernel **idénticos** a `uname -r` | `ls /lib/modules/$(uname -r)/build/Makefile` |
| `gcc`, `make`, `clang` | `gcc --version`, `make --version`, `clang --version` |
| Go ≥ 1.25 | `go version` |
| Docker Engine + Compose v2 | `docker info` y `docker compose version` |
| `python3`, `jq`, `crontab`, `git` | `command -v` de cada uno |
| Red (una sola vez) | build de imágenes y plugin Redis de Grafana |

Instalación típica (Fedora):

```bash
sudo dnf install -y kernel-devel-$(uname -r) gcc make clang git jq python3 crontabs
sudo dnf install -y go docker-ce docker-compose-plugin   # o los paquetes de la distro
sudo systemctl enable --now docker
```

> En Debian/Ubuntu: `sudo apt install linux-headers-$(uname -r) build-essential
> clang golang docker.io docker-compose-v2 python3 python3-yaml jq cron git`.

## 2. Clonar y preparar

```bash
git clone https://github.com/ricardious/202300476_LAB_SO1_2S2026.git
cd 202300476_LAB_SO1_2S2026
git checkout ricardious/p2-delivery        # rama de entrega final
cd proyecto2
cp .env.example .env                       # opcional; los defaults son seguros
# Corrija en .env la ruta SO1_P2_PROJECT_DIR a su clon real (workloads/).
```

Estado de salud inicial:

```bash
make doctor                       # sin privilegios
bash scripts/capture-evidence.sh --out-dir /tmp/pre-ev   # respaldo de entorno previo
```

## 3. Orden de ejecución (VM limpia)

> `make build`/`make test` compilan y prueban el daemon; el **módulo y eBPF**
> necesitan root y headers. Los pasos privilegiados están marcados `sudo`.

### Fase A — Base reproducible

```bash
make build                    # bin/so1-p2-daemon (no se commitea bin/)
make test                     # go test + -race + shell + Compose (sin root)
bash scripts/verify-delivery.sh --quick   # precheck rápido fuente
```

### Fase B — Stack de observabilidad

```bash
make up
docker ps --format '{{.Names}} {{.Status}}' | grep so1-p2   # valkey + grafana healthy
curl -fsS http://localhost:3000/api/health                  # "database":"ok"
```

### Fase C — Módulo de kernel (root)

```bash
make module                           # compila kernel/continfo.ko contra headers reales
sudo make module-load                 # insmod idempotente
ls -l /proc/continfo_pr2_so1_202300476
cat /proc/continfo_pr2_so1_202300476 | python3 -m json.tool | head -40
python3 tests/kernel/validate_schema.py /proc/continfo_pr2_so1_202300476 --strict-memory
sudo dmesg | tail -n 5                # mensaje de carga del módulo
# Contraste con ps/free (rubrica 2.2):
free -k
ps -eo pid,comm,vsz,rss,%mem,%cpu --sort=-rss | head
sudo bash tests/kernel/test-module-lifecycle.sh   # ciclo completo privilegiado
sudo make module-unload               # (opcional) verificar descarga idempotente
sudo make module-load                 # volver a dejarlo cargado para la demo
```

### Fase D — Daemon, cron y generador

```bash
sudo make install                     # daemon + generador (symlink) en /usr/local/bin
make install-cron                     # cronjob */2 * * * * (bloque delimitado)
crontab -l                            # debe incluir # >>> so1-p2-202300476:begin >>>
```

Arrancar el daemon con root (módulo + eBPF + loop):

```bash
sudo make run                         # consola; o:
# sudo nohup /usr/local/bin/so1-p2-daemon >> /var/log/so1-p2-daemon.log 2>&1 &
```

Verificar en los logs: módulo cargado, cron instalado, Compose up, eBPF
adjunto (`auto`). Con `SO1_P2_EBPF_MODE=required` el daemon **no arranca** sin
privilegios (prueba opcional del modo estricto).

### Fase E — Acumular carga y provocar la política

El cron crea 5 contenedores cada 2 min. Para acelerar (esperado 10–15):

```bash
bash scripts/generate-containers.sh    # +5
bash scripts/generate-containers.sh    # +5
docker ps --filter label=so1.p2.managed=1 --format '{{.Names}} {{.Image}} {{.Status}}'
```

Con más de 5 gestionados, el daemon (loop 20 s) debe **conservar ≥3 low y ≥2
high**, solicitar la terminación de los excedentes y confirmar cada kill con un
**evento eBPF correlacionado**. Esperar ~1–2 min y verificar:

```bash
docker exec so1-p2-valkey valkey-cli GET so1:p2:kills:count     # > 0
docker exec so1-p2-valkey valkey-cli XRANGE so1:p2:kills - + | head
docker ps --format '{{.Names}}' | grep -E 'so1-p2-(grafana|valkey)'   # intactos
```

### Fase F — eBPF privilegiado (criterios 3.1/3.2)

```bash
sudo go test ./internal/ebpf/ -run Privilegiada -v -count=1
```

Debe mostrar la captura real de un `kill` con PID/señal y confirmar la
correlación. Si el kernel rechaza el programa, el daemon lo reporta (modo
`auto` degrada; `required` falla accionable).

### Fase G — Captura de evidencias

```bash
bash scripts/capture-evidence.sh                  # texto automático por rubro
bash scripts/capture-evidence.sh --privileged     # + dmesg (sudo, opt-in)
make evidence                                     # igual a capture-evidence.sh
```

Luego las **capturas manuales** de la tabla de §4 (Grafana y GitHub).

### Fase H — Preentrega final

```bash
make verify-delivery              # o bash scripts/verify-delivery.sh
```

Requisito: **cero `[FALLO]`**. Los `[PEND]` (SKIP) deben ser solo los de
privilegios/entorno (p. ej. `test-module-lifecycle` si ya se corrió con sudo,
`doctor` con valkey/grafana abajo). El comando devuelve 0 si la entrega está
lista y 1 si hay fallos reales.

## 4. Lista exacta de capturas

Convención de nombres: `NN-descripcion.{txt|png}`. Las carpetas están en
`proyecto2/evidencias/` (ver su `README.md`). Los archivos de texto del
`capture-evidence.sh` tienen nombre fijo por rubro y se pueden conservar tal
cual.

| # | Carpeta | Captura | Comando / acción |
|---|---|---|---|
| E1 | `entorno/` | `entorno.txt` (auto) | `bash scripts/capture-evidence.sh` (doctor + OS + herramientas + lsmod + headers/BTF) |
| K1 | `kernel/` | `01-compilacion-modulo.txt/png` | `sudo make module` (salida CC/LD del .ko) |
| K2 | `kernel/` | `02-modulo-cargado.txt/png` | `sudo make module-load` + `lsmod \| grep continfo` + `dmesg` |
| K3 | `kernel/` | `03-proc-json.txt/png` | `cat /proc/continfo_* \| python3 -m json.tool` |
| K4 | `kernel/` | `04-schema.txt` | `validate_schema.py ... --strict-memory` |
| K5 | `kernel/` | `05-contraste-ps-free.txt` | `free -k` + `ps --sort=-rss` contra el JSON |
| K6 | `kernel/` | `kernel.txt` (auto) | estado módulo + salida exacta + validación `jq` |
| D1 | `daemon-politica/` | `01-build-daemon.txt` | `make build` + `go vet`/`go test` |
| D2 | `daemon-politica/` | `02-daemon-logs.txt/png` | logs de arranque (módulo, cron, Compose, eBPF) |
| D3 | `daemon-politica/` | `03-supervivientes.txt/png` | `docker ps` con ≥3 low y ≥2 high |
| D4 | `daemon-politica/` | `04-protegidos.txt` | grafana/valkey presentes y NO gestionados |
| D5 | `daemon-politica/` | `politica.txt` (auto) | gestionados/etiquetas/perfiles + protegidos |
| C1 | `cron-workloads/` | `01-crontab.txt/png` | `crontab -l` (bloque delimitado cada 2 min) |
| C2 | `cron-workloads/` | `02-imagenes.txt` | `docker images \| grep so1/load` (4 imágenes) |
| C3 | `cron-workloads/` | `03-carga.txt/png` | `docker ps` tras 2-3 corridas (10-15 ctrs) |
| C4 | `cron-workloads/` | `cron.txt` (auto) | cron + generador + imágenes + log |
| V1 | `valkey-grafana/` | `01-valkey-hgetall.txt` | `HGETALL so1:p2:system:latest` |
| V2 | `valkey-grafana/` | `02-valkey-timeline.txt` | `XRANGE so1:p2:system:timeline - +` |
| V3 | `valkey-grafana/` | `03-grafana-dashboard-ram.png` | panel “RAM total/usada/libre” |
| V4 | `valkey-grafana/` | `04-grafana-evolucion.png` | panel “Evolución temporal de RAM” |
| V5 | `valkey-grafana/` | `05-grafana-eliminaciones.png` | paneles XLEN/GET/serie de eliminaciones |
| V6 | `valkey-grafana/` | `06-grafana-top5.png` | “Top 5 histórico por RAM/CPU (PID e ID)” |
| V7 | `valkey-grafana/` | `07-grafana-tabla-ebpf.png` | “Tabla de eventos kill capturados por eBPF” |
| V8 | `valkey-grafana/` | `08-grafana-timeline-ctrs.png` | “Timeline de contenedores” |
| V9 | `valkey-grafana/` | `observabilidad.txt` (auto) | claves, system/timeline, rankings, salud Grafana |
| B1 | `ebpf/` | `01-test-privilegiado.txt/png` | `sudo go test ./internal/ebpf/ -run Privilegiada -v` |
| B2 | `ebpf/` | `02-kills-stream.txt` | `XLEN so1:p2:kills` + `GET kills:count` > 0 + `XRANGE` |
| B3 | `ebpf/` | `ebpf.txt` (auto) | BTF, modo, objeto embebido, stream kills |
| G1 | `documentacion-git/` | `01-clone.txt` | `git clone` + `git checkout ricardious/p2-delivery` |
| G2 | `documentacion-git/` | `02-git-log.txt` | `git log --oneline -15` de la rama |
| G3 | `documentacion-git/` | `03-docs-arbol.txt` | `find docs evidencias \| sort` |
| G4 | `documentacion-git/` | `04-github-repo.png` | página del repositorio |
| G5 | `documentacion-git/` | `05-github-commits.png` | pestaña *commits* de la rama |
| G6 | `documentacion-git/` | `git.txt` (auto) | rama, remoto, log, estado git |

> Los `(auto)` los genera `capture-evidence.sh`; los `(manual)` se toman a mano.
> No se sube ningún archivo vacío: si una captura no aplica, se omite.

## 5. Guion de defensa (≈ 5–8 min)

1. **Contexto (30 s):** “Sonda de kernel + daemon Go que lee `/proc` del módulo,
   decide qué contenedores eliminar con una política 3 low / 2 high y confirma
   cada eliminación con un evento eBPF antes de registrarla en Valkey.”
2. **Demo en vivo (2–3 min):** `make doctor` → `docker ps` (supervivientes 3/2) →
   `valkey-cli HGETALL so1:p2:system:latest` → `GET so1:p2:kills:count` → panel
   Grafana con datos.
3. **Módulo (si preguntan):** `single_open`/`seq_file` para snapshot consistente;
   snapshot de `task_struct` bajo RCU y formateo fuera de locks; VSZ=`total_vm`,
   RSS=`get_mm_rss`, `%cpu` promedio de vida (convención `ps`).
4. **Política:** ordena por RAM/VSZ/RSS/CPU con desempate determinista; conserva
   mínimos 3/2; Grafana/Valkey protegidos por nombre, etiqueta y ausencia de
   `so1.p2.managed` (triple vía).
5. **eBPF:** tracepoints `sys_enter_kill`/`tgkill`, ring buffer 16 MiB, CO-RE
   contra BTF; solo se escribe `so1:p2:kills` si el evento coincide con un PID+
   señal pendiente dentro de la ventana (sin falsos positivos).
6. **Cierre (30 s):** limpieza segura y evidencia de que el Proyecto 1 queda
   intacto (§6).

Para el Q&A completo por tema usar `docs/DEFENSA.md`.

## 6. Limpieza segura y entrega Git

```bash
# Retira SOLO recursos del proyecto 2 (etiqueta so1.p2.project y Compose so1-p2)
bash scripts/cleanup.sh --no-confirm
# Si además se quiere descargar el módulo y borrar volúmenes del proyecto:
bash scripts/cleanup.sh --no-confirm --module --purge

# Comprobar que el Proyecto 1 está intacto (contenedores smartegg / api1-3 / zot):
docker ps -a | grep -Ev 'so1-(load|p2-)'   # deben seguir los recursos del P1
```

Entrega Git (tras la defensa, cuando las capturas estén completas):

```bash
cd proyecto2
git add evidencias/ docs/CHECKLIST_ENTREGA.md scripts/capture-evidence.sh scripts/verify-delivery.sh
git status                        # solo archivos intencionados
git commit -m "docs(entrega): evidencias y herramientas de preentrega (P2)"
git push origin ricardious/p2-delivery
```

Reglas de entrega:
- **No** versionar `.env`, `bin/`, `logs/`, `kernel/*.ko|*.o`, `ebpf/kill_monitor.o`.
- El único objeto `.o` versionado es `daemon/internal/ebpf/killmonitor_bpf.o`
  (embebido; necesario para build reproducible sin clang).
- Reproducible: un clon limpio + los comandos de §2–§3 debe dejar el sistema
  funcionando con los mismos resultados.

## 7. Criterio de terminado (autoverificación)

- [ ] `make build`, `make test`, `make verify-delivery` (0 fallos reales).
- [ ] `sudo make module-load` → `/proc/continfo_pr2_so1_202300476` legible y JSON válido.
- [ ] `sudo bash tests/kernel/test-module-lifecycle.sh` y `sudo go test ./internal/ebpf/ -run Privilegiada` pasan.
- [ ] `kills:count > 0` con timestamps coherentes a las eliminaciones.
- [ ] Dashboard con los 14 paneles con datos (no vacíos).
- [ ] Evidencias por rubro completas (§4) y Proyecto 1 intacto tras `cleanup.sh`.
- [ ] Rama `ricardious/p2-delivery` pusheada con los cambios finales.