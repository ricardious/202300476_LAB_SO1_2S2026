# Manual Técnico — Proyecto 2 de Sistemas Operativos 1 (202300476)

Sonda de kernel en C + Daemon en Go para la telemetría y gestión autónoma de
contenedores, con observabilidad eBPF y dashboard en Grafana.

**Carnet:** 202300476 · **2do Semestre 2026** · Universidad de San Carlos de Guatemala

---

## 1. Arquitectura general

```
Linux kernel
  ├─ módulo C (continfo.ko) ────> /proc/continfo_pr2_so1_202300476 (JSON)
  └─ eBPF CO-RE kill/tgkill ────> ring buffer (16 MiB) ─┐
                                                         │
Docker Engine API <──────────── daemon Go <──────────────┘
          ├─ política (mínimos 3 low / 2 high, excluye protegidos)
          ├─ cronjob (5 contenedores aleatorios cada 2 min)
          └─ Valkey (so1:p2:*)  <── Grafana (datasource + dashboard provisionados)
```

| Componente | Rol | Ubicación |
|---|---|---|
| Módulo de kernel | Expone memoria del sistema y procesos (PID, nombre, cmdline, container_id, VSZ, RSS, %mem, %cpu) en JSON por `/proc` | `kernel/continfo.c` |
| Sonda eBPF | Audita `kill`/`tgkill`; emite eventos con timestamp, PID emisor, PID receptor y señal | `ebpf/kill_monitor.bpf.c` |
| Daemon Go | Lee `/proc`, decide qué contenedores eliminar, persiste en Valkey, correlaciona eBPF | `daemon/` |
| Cronjob | Genera 5 contenedores aleatorios cada 2 minutos desde 4 imágenes | `scripts/generate-containers.sh` |
| Valkey | Base de datos de telemetría y eventos | `deploy/docker-compose.yml` |
| Grafana | Dashboard con los paneles de la rúbrica | `deploy/grafana/` |

---

## 2. Módulo de kernel (C)

### 2.1 Interfaz

El módulo registra un archivo en `/proc`:

```
/proc/continfo_pr2_so1_202300476
```

Se usa `proc_create` con permisos `0444` y `seq_file` con `single_open`, lo que
garantiza que la salida se construye de forma consistente y completa para cada
lectura (no se entrega al lector un documento a medias).

### 2.2 Contrato JSON

```json
{
  "schema_version": 1,
  "timestamp_unix_ns": 1757100000000000000,
  "memory": { "total_kb": 8388608, "free_kb": 4194304, "used_kb": 4194304 },
  "processes": [
    {
      "pid": 1234,
      "name": "python3",
      "cmdline": "python3 /opt/memory-high/memory_high.py",
      "container_id": "abcdef1234567890...64hex",
      "vsz_kb": 900000,
      "rss_kb": 500000,
      "memory_percent": 5.96,
      "cpu_percent": 0.10
    }
  ]
}
```

### 2.3 Memoria del sistema

- `total_kb` = `totalram_pages() << (PAGE_SHIFT - 10)` → **MemTotal**.
- `free_kb` = `freeram_pages() << (PAGE_SHIFT - 10)` → **MemFree**.
- `used_kb` = `total_kb - free_kb`.

Mismas fuentes que `free(1)`; las unidades son KB y se documentan en el propio
contrato. Consistencia verificada por `tests/kernel/validate_schema.py --strict-memory`.

### 2.4 Recorrido de procesos (task_struct)

- Se itera la lista global de tareas con `for_each_process(p)` bajo **RCU**.
- Se toma una referencia real (`get_task_struct`) por proceso para poder
  trabajar fuera de RCU sin carreras con el `exit`.
- VSZ y RSS se obtienen con `get_task_mm` + `mmap_read_lock`:
  - `vsz_kb = mm->total_vm << (PAGE_SHIFT - 10)`
  - `rss_kb = get_mm_rss(mm) << (PAGE_SHIFT - 10)`
- `cmdline` se lee con `access_process_vm` sobre `[arg_start, arg_end)` (misma
  semántica que `/proc/<pid>/cmdline`), convirtiendo los `\0` separadores en
  espacios.
- `container_id` se detecta leyendo `/proc/<pid>/cgroup` y buscando el token
  hexadecimal más largo (≥ 32 chars; Docker usa 64). Cuando no hay contenedor,
  el campo queda `""`.
- Porcentajes (unidades y redondeo a 2 decimales):
  - `memory_percent = rss_kb / total_kb * 100`
  - `cpu_percent = (utime + stime) / (now - start_time) * 100`

**CPU:** la fórmula es el promedio de uso sobre la vida del proceso, la misma
convención que `ps(1)`. El enunciado admite explícitamente valores grandes
derivados de cálculos diferenciales del kernel.

### 2.5 Seguridad y concurrencia

- Referencias tomadas antes de tocar `mm`/`fs`; los bloqueos de mmap se
  mantienen solo el tiempo mínimo para copiar punteros.
- El formateo JSON ocurre fuera de cualquier bloqueo de tareas; `seq_file`
  serializa lecturas concurrentes del mismo archivo.
- Escaping JSON completo de `"`, `\`, `\n`, `\r`, `\t` y controles (`\u00xx`).
- Cota de procesos por lectura (`MAX_TASKS_SNAPSHOT = 65536`) para proteger
  contra uso ilimitado de memoria.

### 2.6 Build y ciclo de vida

- `make -C proyecto2/kernel` compila `continfo.ko` contra los headers en
  ejecución (`KDIR` configurable).
- `scripts/load-module.sh` / `scripts/unload-module.sh` son **idempotentes**:
  verifican estado en `/proc/modules` y que el archivo `/proc` aparezca/
  desaparezca. Se auto-re-ejecutan con `sudo`.

---

## 3. Sonda eBPF (CO-RE)

### 3.1 Adjunto

Dos programas adjuntos a los **tracepoints estables**:

- `syscalls:sys_enter_kill`
- `syscalls:sys_enter_tgkill`

### 3.2 Evento emitido (24 bytes)

```
struct kill_event {
  __u64 timestamp_ns;  // bpf_ktime_get_ns()
  __s32 emitter_pid;   // TGID del emisor
  __s32 target_pid;    // kill: arg pid | tgkill: tgid del grupo
  __s32 signal;        // 9 = SIGKILL, 15 = SIGTERM ...
  __u32 reserved;      // alineación
};
```

Los eventos viajan por un **ring buffer** (`BPF_MAP_TYPE_RINGBUF`, 16 MiB). En
`tgkill` se reporta el **TGID** (y no el TID del hilo) porque el daemon
correlaciona contra el PID de host del proceso raíz del contenedor.

### 3.3 Carga y correlación

- El objeto es **CO-RE**: se compila una vez con clang y se carga contra el
  BTF del kernel en ejecución (`/sys/kernel/btf/vmlinux`). No requiere headers
  del kernel en tiempo de compilación.
- `daemon/internal/ebpf/bpf_consumer.go` (cilium/ebpf) carga el objeto
  embebido, adjunta los tracepoints y lee el ring buffer.
- El daemon registra solicitudes de terminación pendientes
  (`Correlator.Track(container, hostPID, señal)`) antes de pedir el `kill` a
  Docker. Solo un evento eBPF que coincida en **PID receptor + señal** dentro
  de la ventana (`SO1_P2_CONFIRMATION_TIMEOUT`) confirma la eliminación.

---

## 4. Daemon Go

### 4.1 Paquetes

| Paquete | Responsabilidad |
|---|---|
| `config` | Carga/valida variables `SO1_P2_*` |
| `procfs` | Lee y valida el JSON de `/proc` (parser estricto/tolerante) |
| `docker` | Cliente mínimo de la API Docker (unix/http) |
| `policy` | Ranking determinista y cuotas 3 low / 2 high |
| `valkey` | Persistencia de telemetría y eventos (claves `so1:p2:*`) |
| `ebpf` | Consumidor CO-RE real + correlación kill |
| `cron` | Instalación/retiro idempotente del cronjob del proyecto |
| `lifecycle` | Orquestación del arranque, loop y apagado |

### 4.2 Arranque (`main`)

1. Validar configuración y construir dependencias.
2. Cargar el módulo `/proc` si `SO1_P2_LOAD_MODULE_ON_START=true`
   (`scripts/load-module.sh`, idempotente).
3. Instalar el cronjob del proyecto (delimitado por marcadores; no toca
   entradas ajenas).
4. `Run`:
   - verificar Docker (`_ping`);
   - levantar Valkey/Grafana vía Compose si Valkey no responde;
   - arrancar el consumidor eBPF (`auto` degrada solo sin privilegios;
     `required` falla; `off` desactiva);
   - ejecutar el loop de reconciliación cada `SO1_P2_LOOP_INTERVAL` (20-60 s).

### 4.3 Ciclo de reconciliación

1. Leer y deserializar `/proc` (snapshot completa).
2. Descubrir contenedores gestionados (`so1.p2.managed=1`) por la API Docker.
3. Enriquecer cada contenedor con PID de host (`Inspect`) y métricas del kernel
   (buscar el PID en la snapshot).
4. Aplicar la política:
   - excluir protegidos (Grafana/Valkey por nombre o por etiqueta
     `so1.p2.protect=1`) y contenedores sin métricas/PID válido;
   - ordenar por **RAM, VSZ, RSS, CPU** con desempate determinista
     (nombre, ID);
   - conservar como mínimo **3 de bajo consumo** y **2 de alto consumo**
     (`SO1_P2_MIN_LOW`/`SO1_P2_MIN_HIGH`);
   - el resto son candidatos a eliminación.
5. Persistir en Valkey:
   - `so1:p2:system:latest` (hash),
   - `so1:p2:system:timeline` (stream de RAM),
   - `so1:p2:containers:timeline` (stream de contenedores activos e inactivos),
   - `so1:p2:rank:ram` / `so1:p2:rank:cpu` (sorted sets con miembro
     `"<id>|pid=<pid>"`).
6. Para cada candidato: `Correlator.Track` + `POST /containers/{id}/kill`.
7. El consumidor de eventos (`runEventConsumer`) correlaciona el evento eBPF y
   **solo entonces** escribe `so1:p2:kills` (stream) e incrementa
   `so1:p2:kills:count`.

### 4.4 Claves de Valkey

| Clave | Tipo | Contenido |
|---|---|---|
| `so1:p2:system:latest` | hash | `mem_total_kb`, `mem_free_kb`, `mem_used_kb`, `process_count`, `timestamp_unix_ns` |
| `so1:p2:system:timeline` | stream | `ts`, `mem_total_kb`, `mem_free_kb`, `mem_used_kb`, `process_count` |
| `so1:p2:containers:timeline` | stream | `container_id`, `name`, `host_pid`, `mem_percent`, `vsz_kb`, `rss_kb`, `cpu`, `ts` |
| `so1:p2:rank:ram` / `rank:cpu` | sorted set | miembro `"<id>|pid=<pid>"`, score = RSS / CPU |
| `so1:p2:kills` | stream | `container_id`, `host_pid`, `signal`, `ts`, `n=1` |
| `so1:p2:kills:count` | string | contador total de eliminaciones confirmadas |

### 4.5 Apagado ordenado

`SIGINT`/`SIGTERM` (via `signal.NotifyContext`): se detiene el loop, se retira
**solo** la entrada de cron del proyecto, se descarga el módulo si
`SO1_P2_UNLOAD_MODULE_ON_STOP=true`, se baja Compose si
`SO1_P2_COMPOSE_DOWN_ON_STOP=true`. Nunca se eliminan datos ni contenedores
protegidos.

---

## 5. Cronjob y cargas de trabajo

### 5.1 Cron

- Línea: `*/2 * * * *` (cada 2 minutos).
- Comando: `/usr/local/bin/so1-p2-generate-containers.sh` (instalado como
  **symlink** al repositorio por `make install`/`install-cron.sh`; la línea de
  cron exporta `SO1_P2_PROJECT_DIR`).
- La entrada está **delimitada por marcadores**
  (`# >>> so1-p2-202300476:begin >>>` … `# <<< ...:end <<<`) para que
  `install-cron.sh`/`remove-cron.sh` y el daemon Go operen sobre la misma
  entrada sin borrar entradas ajenas.

### 5.2 Generador

Por invocación crea **exactamente 5 contenedores** eligiendo al azar (con
reemplazo) entre las 4 imágenes:

| Perfil | Imagen | Límite memoria | Límite CPU | Comportamiento |
|---|---|---|---|---|
| `memory-high` | `so1/load:202300476-memory-high` | 768 MB | 0.5 | Reserva y mantiene ~512 MB (RSS real) |
| `cpu-high` | `so1/load:202300476-cpu-high` | 256 MB | 1.0 | Carga de CPU sostenida |
| `low` | `so1/load:202300476-low` | 128 MB | 0.25 | Consumo mínimo ≥ 4 min |
| `intruder` | `so1/load:202300476-intruder` | 128 MB | 0.25 | Lectura controlada de un fixture; bloquea intentos sobre rutas reales del host |

Etiquetas de cada contenedor: `so1.p2.managed=true`,
`so1.p2.project=202300476`, `so1.p2.profile=<perfil>`. Nombres únicos con
timestamp + PID + índice. Bloqueo con `mkdir` contra ejecuciones simultáneas y
semilla fija (`SO1_SEED`) para pruebas reproducibles.

---

## 6. Valkey y Grafana

### 6.1 Compose (`deploy/docker-compose.yml`)

Proyecto explícito `so1-p2`. Servicios:

- `valkey` (`valkey/valkey:9.1.2-alpine`), contenedor `so1-p2-valkey`,
  AOF + save 60s, puerto 6379, healthcheck.
- `grafana` (imagen custom `so1/p2-grafana:202300476`, base 13.2.1 + plugin
  `redis-datasource` 2.2.0), contenedor `so1-p2-grafana`, puerto 3000,
  healthcheck, depende de Valkey sano.

Ambos llevan `so1.p2.protect=1` y `so1.p2.project=202300476` y **no** llevan
`so1.p2.managed` (nunca son candidatos del daemon).

### 6.2 Datasource y dashboard

- Datasource provisionado: `redis://valkey:6379/0`, uid `so1-valkey`, default.
- Dashboard provisionado: `so1-p2-overview.json` con paneles:
  - RAM total / usada / libre (cards, `HGET` sobre `so1:p2:system:latest`);
  - Evolución temporal de RAM (time series, `XRANGE` de `system:timeline`);
  - Eliminaciones confirmadas: `XLEN`, `GET kills:count` y time series de
    `so1:p2:kills`;
  - Tabla de eventos kill capturados por eBPF (`XREVRANGE` de `so1:p2:kills`);
  - Top 5 histórico por RAM y por CPU (tablas, `ZRANGE ... REV` sobre
    `rank:ram`/`rank:cpu`, con PID e ID en el miembro);
  - Timeline de contenedores activos e inactivos (`XREVRANGE` de
    `containers:timeline`).

Los paneles temporales usan transformaciones de organización para graficar
únicamente las métricas relevantes.

---

## 7. Configuración (`SO1_P2_*`)

| Variable | Default | Descripción |
|---|---|---|
| `SO1_P2_PROC_PATH` | `/proc/continfo_pr2_so1_202300476` | Interfaz del módulo |
| `SO1_P2_LOOP_INTERVAL` | `20s` | Período del ciclo (20-60 s) |
| `SO1_P2_DOCKER_HOST` | `unix:///var/run/docker.sock` | Endpoint Docker |
| `SO1_P2_VALKEY_ADDR` | `127.0.0.1:6379` | Valkey |
| `SO1_P2_MIN_LOW` / `SO1_P2_MIN_HIGH` | `3` / `2` | Cuotas de conservación |
| `SO1_P2_PROTECTED_NAMES` | `so1-p2-grafana,so1-p2-valkey,grafana,valkey` | Nunca candidatos |
| `SO1_P2_PROTECTED_LABELS` | `so1.p2.protect=1` | Etiqueta de protección |
| `SO1_P2_MANAGED_LABEL_KEY/VALUE` | `so1.p2.managed` / `1` | Marca gestionados |
| `SO1_P2_KILL_SIGNAL` | `SIGKILL` | Señal de terminación |
| `SO1_P2_CONFIRMATION_TIMEOUT` | `30s` | Ventana de correlación eBPF |
| `SO1_P2_EBPF_MODE` | `auto` | `auto`/`required`/`off` |
| `SO1_P2_COMPOSE_FILE` | `deploy/docker-compose.yml` | Auto-arranque de servicios |
| `SO1_P2_CRON_SCHEDULE` | `*/2 * * * *` | Cronjob |
| `SO1_P2_CRON_COMMAND` | `/usr/local/bin/so1-p2-generate-containers.sh` | Comando de cron |

---

## 8. Decisiones técnicas relevantes

1. **Daemon en host, servicios en Compose**: el daemon necesita `/proc` del
   módulo y eBPF con privilegios; Compose aísla Valkey/Grafana.
2. **Confirmación eBPF estricta**: ninguna eliminación se escribe en Valkey sin
   un evento `kill` correlacionado (PID receptor + señal en ventana temporal).
3. **Triple protección de Grafana/Valkey**: sin etiqueta de gestionado, nombres
   protegidos, y etiqueta `so1.p2.protect=1`.
4. **Degradación explícita de eBPF**: `auto` degrada solo ante EPERM/EACCES/
   EOPNOTSUPP con log claro; `required` falla con exit≠0.
5. **Rankings con PID**: el miembro del sorted set incluye `pid=<pid>` para
   que el dashboard muestre PID e ID sin joins.
6. **Cron idempotente y delimitado**: marcadores únicos compartidos entre el
   daemon Go y los scripts shell.

---

## 9. Limitaciones conocidas

- El `%cpu` es un promedio de vida (`ps`), no instantáneo; la rúbrica admite
  valores grandes.
- La identificación de contenedores se basa en el token cgroup (Docker) y
  `cmdline` como alternativa; no depende de `docker` dentro del módulo.
- La carga eBPF y del módulo requieren root/CAP_BPF y headers del kernel en la
  VM objetivo.
- El datasource de Grafana usa Redis/Valkey vía plugin; la primera vez requiere
  red para instalar el plugin en el build de la imagen.

---

## 10. Actividad extra: módulo de kernel en Rust

El directorio `rust-module/` produce `hola_rust_202300476.ko`. La función que
emite `Hola Mundo 202300476` y la lógica de salida están escritas en Rust
`no_std`; `module.c` se limita a registrar los callbacks de carga y descarga.
Esta separación permite compilar con los headers `kernel-devel` de Fedora,
que habilitan Rust en el kernel pero no incluyen el crate interno completo de
Rust for Linux para módulos externos.

El Makefile usa el `rustc` compatible con el kernel, genera un objeto ELF con
las opciones de ABI del kernel y deja que Kbuild enlace y genere los metadatos
del `.ko`. La validación automatizada comprueba el nombre del módulo, su
`vermagic` y la presencia del mensaje con el carnet.

```bash
make rust-module
bash tests/rust/test-build.sh
sudo make rust-module-load
sudo dmesg | grep -E 'Hola Mundo 202300476|Modulo Rust 202300476'
sudo make rust-module-unload
```
