# Build reproducible del frente eBPF del proyecto 2.
#
# Requisitos: clang (>= 14, genera BTF) y, para regenerar los bindings Go,
# bpf2go (github.com/cilium/ebpf/cmd/bpf2go). bpftool es OPCIONAL y solo se
# usa para emitir vmlinux.h como documentación/alternativa.
#
# La carga al kernel resuelve las relocalizaciones CO-RE contra el BTF del
# kernel en ejecución (/sys/kernel/btf/vmlinux), por lo que el objeto no
# depende de headers del kernel en tiempo de compilación.

CLANG ?= clang
ARCH  ?= x86
# Si bpf2go no está instalado en PATH, se ejecuta vía `go run` con la versión
# fijada en go.mod del daemon (ruta reproducible sin binarios globales).
BPF2GO ?= go run github.com/cilium/ebpf/cmd/bpf2go@v0.22.0
EBPF_GO_DIR := ../daemon/internal/ebpf

# Objeto eBPF de inspección/documentación (BTF incluido).
OBJ := kill_monitor.o

.PHONY: all object bindings vmlinux clean

all: object bindings

# Compila el programa CO-RE a un objeto ELF/BTF. Es la misma compilación que
# bpf2go hace internamente; este target existe para inspección y para hosts
# sin Go.
object: $(OBJ)

$(OBJ): kill_monitor.bpf.c kill_monitor.h
	$(CLANG) -g -O2 -Wall -target bpf -D__TARGET_ARCH_$(ARCH) \
		-c kill_monitor.bpf.c -o $@

# Regenera los bindings Go (killmonitor_bpf.go + killmonitor_bpf.o) dentro del
# paquete ebpf del daemon. Alternativa sin bpf2go: go generate ./internal/ebpf
bindings:
	cd $(EBPF_GO_DIR) && $(BPF2GO) -cc clang -target bpf -go-package ebpf \
		killmonitor ../../../ebpf/kill_monitor.bpf.c

# Documentación de la ruta vmlinux.h (requiere bpftool).
vmlinux:
	@echo "Genera el header CO-RE del kernel en ejecución:"
	@echo "  bpftool btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h"
	@echo "kill_monitor.bpf.c es autocontenido y no lo requiere; se ofrece"
	@echo "como alternativa para reescrituras con tipos completos del kernel."

clean:
	rm -f $(OBJ) $(EBPF_GO_DIR)/killmonitor_bpf.go $(EBPF_GO_DIR)/killmonitor_bpf.o