# Guía de Defensa — Proyecto 2 SO1 (202300476)

Preguntas y respuestas esperadas para la defensa oral, ordenadas por tema.

---

## 1. Módulo de kernel

**¿Por qué `seq_file` para el archivo `/proc`?**
`proc_create` con `single_open` entrega una snapshot completa y consistente en
una sola lectura: `seq_file` acumula el documento y el lector nunca recibe un
JSON a medias. Además serializa lecturas concurrentes del archivo.

**¿Cómo se evita una lectura inconsistente?**
Primero se toma una fotografía de referencias (`get_task_struct` para cada
tarea bajo RCU) y solo después se leen `mm`, `cmdline` y cgroup, fuera de los
bloqueos. Así un proceso que sale durante la lectura no invalida el formato.

**¿Qué es `task_struct` y qué se extrae de él?**
Es la estructura que representa cada proceso/hilo en el kernel. De ella se
obtienen `pid`, `comm`, `utime`, `stime` y `start_time`; de su `mm_struct` se
derivan VSZ (`total_vm`) y RSS (`get_mm_rss`).

**¿VSZ vs RSS?**
VSZ es el tamaño de memoria virtual mapeado (`mm->total_vm`); RSS es la
memoria física residente. Un proceso puede tener VSZ alto y RSS bajo.

**¿Cómo se calculan los porcentajes?**
- `memory_percent = rss_kb / total_kb × 100`.
- `cpu_percent = (utime+stime) / (ahora − start_time) × 100` (promedio de
  vida, igual que `ps`). El enunciado admite valores grandes.

**¿Cómo se identifica un contenedor?**
Se lee `/proc/<pid>/cgroup` y se extrae el token hexadecimal largo (Docker usa
64 chars; se aceptan ≥ 32). `cmdline` se conserva como alternativa verificable.

## 2. Daemon Go

**¿Cuál es el flujo de arranque?**
Validar configuración → cargar módulo (script idempotente) → instalar cron →
`Run`: verificar Docker, levantar Valkey/Grafana vía Compose si hace falta,
arrancar eBPF, ejecutar el loop de reconciliación cada 20-60 s.

**¿Cómo decide qué eliminar?**
Ordena los contenedores gestionados por RAM, VSZ, RSS y CPU con desempate
determinista (nombre, ID). Conserva mínimo 3 de bajo consumo y 2 de alto
consumo; el resto son candidatos. Grafana, Valkey y los etiquetados como
protegidos nunca son candidatos (por nombre, por etiqueta y porque no llevan la
etiqueta de gestionado).

**¿Cómo se garantizan los mínimos 3/2?**
`policy.Decide` clasifica por tier y solo marca para eliminación los excedentes
fuera de las cuotas: nunca elimina por debajo de `MinLow` bajos ni `MinHigh`
altos.

**¿Por qué el daemon corre en el host y no en un contenedor?**
Necesita leer el `/proc` del módulo y cargar eBPF con privilegios; ejecutarlo
en un contenedor complicaría mounts/privilegios sin beneficio. Valkey y Grafana
sí van en Compose.

## 3. eBPF

**¿Qué es un tracepoint y cuál se usa?**
Un punto de instrumentación estable en el kernel. Se adjunta a
`syscalls:sys_enter_kill` (y `sys_enter_tgkill`), que se ejecuta antes de la
syscall, capturando el `pid` receptor y la `señal`.

**¿Qué es el verifier y el ring buffer?**
El verifier de eBPF valida el programa (seguridad/terminación) antes de
cargarlo. El ring buffer transporta los eventos del kernel al espacio de
usuario sin latencia de lectura de mapas periódica.

**¿Por qué CO-RE?**
Se compila una vez y las relocalizaciones se resuelven contra el BTF del kernel
en ejecución, evitando dependencias de headers.

**¿Cuándo se confirma una eliminación?**
Solo cuando el evento eBPF coincide con una solicitud pendiente en el PID
receptor y la señal dentro de la ventana temporal. Sin evento real no hay
escritura en `so1:p2:kills`.

## 4. Valkey / Grafana

**¿Qué se persiste?**
`system:latest` (hash), `system:timeline` (stream RAM), `containers:timeline`
(stream), `rank:ram`/`rank:cpu` (sorted sets con `"<id>|pid=<pid>"`), `kills`
(stream) y `kills:count` (contador).

**¿Cómo se relacionan los paneles con las claves?**
Cards RAM = `HGET` sobre `system:latest`; evolución = `XRANGE` sobre
`system:timeline`; eliminaciones = `XLEN`/`GET`/`XRANGE` sobre `kills`;
top 5 = `ZRANGE REV` sobre `rank:*`; eventos = `XREVRANGE` sobre `kills`.

**¿Por qué Grafana nunca se elimina?**
No lleva la etiqueta de gestionado (no entra en el descubrimiento del daemon),
su nombre está en `ProtectedNames` y tiene `so1.p2.protect=1`. Triple vía.

## 5. Cron, idempotencia y operación

**¿Cómo se instala/retira el cronjob?**
Con una entrada delimitada por marcadores. `install-cron.sh`/`remove-cron.sh` y
el daemon Go usan los mismos marcadores y el mismo comando, así que cualquier
vía puede instalar o retirar **solo** ese bloque, sin tocar entradas ajenas.

**¿Qué hace el generador?**
Cada 2 minutos crea exactamente 5 contenedores aleatorios de las 4 imágenes
(3 perfiles + intruso), con límites de recursos, etiquetas y nombres únicos, y
bloqueo contra ejecuciones simultáneas.

**¿Qué pasa al recibir SIGTERM?**
Se detiene el loop, se retira solo el cronjob del proyecto, se libera eBPF y,
según configuración, se descarga el módulo y se baja Compose. Nunca se
eliminan datos ni contenedores protegidos.

**¿Cómo se verifica la idempotencia?**
`install-cron.sh` dos veces → una sola sección; `load-module.sh` dos veces →
un solo `insmod`; `generate-containers.sh` en paralelo → bloquea la segunda.

## 6. Limitaciones reconocidas

- El `%cpu` es promedio de vida, no instantáneo.
- La identificación por cgroup depende del formato Docker (token hex).
- Módulo y eBPF requieren root/CAP_BPF y headers del kernel en la VM.
- La primera carga de Grafana necesita red para instalar el plugin Redis.