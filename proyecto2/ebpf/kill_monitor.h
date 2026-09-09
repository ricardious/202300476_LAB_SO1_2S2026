// SPDX-License-Identifier: GPL-2.0
/*
 * kill_monitor.h — definiciones mínimas y autocontenidas para el programa
 * CO-RE kill_monitor.bpf.c.
 *
 * No depende de headers del kernel ni de vmlinux.h: el verifier y el cargador
 * de libbpf (vía cilium/ebpf) resuelven las relocalizaciones CO-RE contra el
 * BTF del kernel en ejecución (/sys/kernel/btf/vmlinux) al cargar.
 */
#ifndef __KILL_MONITOR_H
#define __KILL_MONITOR_H

#define __BPF__ 1

typedef unsigned char __u8;
typedef unsigned short __u16;
typedef unsigned int __u32;
typedef unsigned long long __u64;
typedef signed int __s32;
typedef signed long long __s64;

#define SEC(name) __attribute__((section(name), used))
#define __always_inline inline __attribute__((always_inline))
#define __uint(name, val) int (*name)[val]
#define __type(name, val) typeof(val) *name

/* Valores de enum bpf_map_type y bpf_func_id estables (uapi). */
#define BPF_MAP_TYPE_RINGBUF 27

#define BPF_FUNC_ktime_get_ns 5
#define BPF_FUNC_get_current_pid_tgid 14
#define BPF_FUNC_ringbuf_reserve 131
#define BPF_FUNC_ringbuf_submit 132

/*
 * Cabecera común que precede los datos de todo tracepoint (struct trace_entry
 * del kernel). Su layout es ABI estable.
 */
struct __bpf_trace_entry {
	__u16 type;
	__u8 flags;
	__u8 preempt_count;
	__s32 pid;
};

/*
 * Layout común de syscalls:sys_enter_* publicado por tracefs:
 *   common_* (8 bytes) | long syscall_nr @8 | unsigned long args[6] @16
 *
 * Los argumentos no comienzan inmediatamente después de trace_entry: primero
 * aparece el identificador de la syscall. Modelar pid/sig directamente en los
 * offsets 8/12 leería syscall_nr y padding en kernels de 64 bits.
 */
struct sys_enter_ctx {
	struct __bpf_trace_entry ent;
	__s64 id;
	__u64 args[6];
};

/*
 * Evento emitido al ring buffer. Los campos coinciden 1:1 con el contrato
 * Go ebpf.Event (emitter pid/tgid, target pid, señal y timestamp en ns).
 * Layout: 8 + 4 + 4 + 4 + 4 = 24 bytes, alineación 8.
 */
struct kill_event {
	__u64 timestamp_ns; /* bpf_ktime_get_ns() */
	__s32 emitter_pid;  /* TGID del proceso que emitió kill */
	__s32 target_pid;   /* argumento pid de la syscall (puede ser negativo) */
	__s32 signal;       /* número de señal (9 = SIGKILL) */
	__u32 reserved;     /* padding para alinear a 8 bytes */
};

/* Helpers BPF (declaración clásica de libbpf: la dirección es el id). */
static __u64 (*bpf_ktime_get_ns)(void) = (void *)BPF_FUNC_ktime_get_ns;
static __u64 (*bpf_get_current_pid_tgid)(void) = (void *)BPF_FUNC_get_current_pid_tgid;
static void *(*bpf_ringbuf_reserve)(void *ringbuf, __u64 size, __u64 flags) =
	(void *)BPF_FUNC_ringbuf_reserve;
static void (*bpf_ringbuf_submit)(void *data, __u64 flags) =
	(void *)BPF_FUNC_ringbuf_submit;

#endif /* __KILL_MONITOR_H */
