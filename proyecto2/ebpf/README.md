# Frente eBPF — monitor de la syscall `kill`

Componente CO-RE (Compile Once, Run Everywhere) del plan maestro
(`proyecto2/docs/PLAN_PROYECTO2.md`, sección 3.3 y criterio 3.1/3.2 de la
rúbrica). Captura las syscalls `kill`/`tgkill`, emite eventos con timestamp,
PID/TGID emisor, PID receptor y señal a través de un ring buffer, y los
reenvía al daemon Go para correlacionar eliminaciones de contenedores.

## Arquitectura

```text
syscalls:sys_enter_kill ──┐
                          ├─> kill_monitor.bpf.c ─> ring buffer ─> BPFConsumer (Go)
syscalls:sys_enter_tgkill ┘                               (daemon/internal/ebpf)
```

- `kill_monitor.bpf.c`: dos programas, `handle_kill` y `handle_tgkill`,
  adjuntos a los tracepoints estables `syscalls:sys_enter_kill` y
  `syscalls:sys_enter_tgkill`.
- `kill_monitor.h`: definiciones mínimas y autocontenidas (tipos, macros de
  mapa, helpers BPF y layout de los argumentos del tracepoint).
- El evento `kill_event` (24 bytes) transporta:
  - `timestamp_ns`: `bpf_ktime_get_ns()` del instante de la syscall.
  - `emitter_pid`: TGID del proceso que emite la señal.
  - `target_pid`: argumento `pid` de la syscall (puede ser negativo en
    `kill(-pgid)`).
  - `signal`: número de señal (9 = SIGKILL).

## Dependencias de headers / vmlinux

- El objeto es **CO-RE**: clang emite secciones `.BTF`/`.BTF.ext` y el
  cargador (cilium/ebpf, `libbpf`) resuelve las relocalizaciones contra el
  BTF del kernel en ejecución (`/sys/kernel/btf/vmlinux`). No se compila
  contra headers del kernel.
- El layout de `sys_enter_kill_ctx`/`sys_enter_tgkill_ctx` reproduce el
  formato publicado por tracefs:
  `/sys/kernel/tracing/events/syscalls/sys_enter_kill/format` (y tgkill).
  Es ABI estable; si un kernel cambiara esos offsets, el verifier/loader lo
  detectará al cargar y el consumidor reportará el error.
- Para una alternativa con tipos completos del kernel:
  `bpftool btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h`
  (ver target `make vmlinux`).

## Build

```sh
# Objeto eBPF para inspección (clang, requiere BTF)
make object

# Bindings Go dentro del paquete del daemon (requiere bpf2go en PATH)
make bindings
#   o bien: cd ../daemon/internal/ebpf && go generate ./...

# Ambos
make
```

Requisitos: clang >= 14, `bpf2go` (cilium/ebpf). bpftool es opcional.

## Carga y adjunto

La carga al kernel requiere privilegios (root / CAP_BPF). El consumidor Go
(`proyecto2/daemon/internal/ebpf/bpf_consumer.go`) implementa la interfaz
`ebpf.Consumer` existente y es quien carga el objeto, adjunta los
tracepoints, lee el ring buffer y cierra todos los recursos.

## Pruebas

- Sin privilegios: `go test ./internal/ebpf/...` valida el parseo del objeto,
  el decodificado binario del evento y los contratos de correlación.
- Privilegiadas (VM objetivo): el test de integración carga el consumidor
  real, envía un `SIGKILL` controlado a un proceso hijo y verifica que el
  evento llega con el PID y la señal esperados, confirmando la correlación.
  Se omite automáticamente si el host no concede permisos de carga.