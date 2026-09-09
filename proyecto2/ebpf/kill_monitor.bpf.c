// SPDX-License-Identifier: GPL-2.0
/*
 * kill_monitor.bpf.c — monitor CO-RE de la syscall kill.
 *
 * Se adjunta a los tracepoints estables syscalls:sys_enter_kill y
 * syscalls:sys_enter_tgkill y emite, por cada llamada, un evento con
 * timestamp, PID/TGID emisor, PID receptor y señal a través de un ring
 * buffer consumido por el daemon (proyecto2/daemon/internal/ebpf).
 *
 * Compilar (ver Makefile):
 *   clang -g -O2 -Wall -target bpf -D__TARGET_ARCH_x86 \
 *         -c kill_monitor.bpf.c -o kill_monitor.o
 */
#include "kill_monitor.h"

char LICENSE[] SEC("license") = "GPL";

/* Ring buffer hacia el espacio de usuario (16 MiB). */
struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 1 << 24);
} events SEC(".maps");

/*
 * Emite un evento al ring buffer. Se ejecuta desde los tracepoints de
 * kill/tgkill; el emisor es siempre la tarea en curso.
 */
static __always_inline int emit_event(__s32 target_pid, __s32 signal)
{
	struct kill_event *e;
	__u64 pid_tgid = bpf_get_current_pid_tgid();

	e = bpf_ringbuf_reserve(&events, sizeof(*e), 0);
	if (!e)
		return 0;

	e->timestamp_ns = bpf_ktime_get_ns();
	e->emitter_pid = (__s32)(pid_tgid >> 32); /* TGID del emisor */
	e->target_pid = target_pid;
	e->signal = signal;
	bpf_ringbuf_submit(e, 0);
	return 0;
}

SEC("tracepoint/syscalls/sys_enter_kill")
int handle_kill(struct sys_enter_kill_ctx *ctx)
{
	return emit_event(ctx->pid, ctx->sig);
}

SEC("tracepoint/syscalls/sys_enter_tgkill")
int handle_tgkill(struct sys_enter_tgkill_ctx *ctx)
{
	return emit_event(ctx->pid, ctx->sig);
}