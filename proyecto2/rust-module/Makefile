# SPDX-License-Identifier: GPL-2.0

MODULE := hola_rust_202300476

ifneq ($(KERNELRELEASE),)

obj-m := $(MODULE).o
$(MODULE)-y := module.o rust_code.o

RUSTC ?= rustc
RUST_MODULE_FLAGS := \
	--crate-name hola_rust_202300476 \
	--crate-type lib \
	--edition=2024 \
	--emit=obj=$(obj)/rust_code.o \
	-C panic=abort \
	-C relocation-model=static \
	-C code-model=kernel \
	-C no-redzone=yes \
	-Z retpoline \
	-Z retpoline-external-thunk \
	-Z function-return=thunk-extern \
	-Z function-sections=no \
	-C unsafe-allow-abi-mismatch=retpoline,retpoline-external-thunk \
	-C opt-level=2 \
	-C codegen-units=1 \
	-C force-unwind-tables=no \
	-C strip=debuginfo \
	-D warnings

quiet_cmd_rust_module = RUSTC [M] $@
      cmd_rust_module = $(RUSTC) $(RUST_MODULE_FLAGS) $<

$(obj)/rust_code.o: $(src)/hola_rust.rs FORCE
	$(call if_changed,rust_module)

targets += rust_code.o
clean-files += rust_code.o .rust_code.o.cmd

else

KDIR ?= /lib/modules/$(shell uname -r)/build
PWD  := $(CURDIR)

.PHONY: all check load unload verify clean

all: check
	$(MAKE) -C $(KDIR) M=$(PWD) modules

check:
	@command -v rustc >/dev/null 2>&1 || { \
		echo "ERROR: rustc no esta instalado"; exit 2; }
	@test -f "$(KDIR)/Makefile" || { \
		echo "ERROR: faltan los headers para $$(uname -r)"; exit 2; }
	@grep -q '^CONFIG_RUST=y' "$(KDIR)/.config" || { \
		echo "ERROR: el kernel $$(uname -r) no tiene CONFIG_RUST=y"; exit 2; }
	@echo ">> Rust disponible: $$(rustc --version)"
	@echo ">> Kernel con CONFIG_RUST=y: $$(uname -r)"

load: all
	@if grep -qw $(MODULE) /proc/modules; then \
		echo ">> Modulo $(MODULE) ya cargado"; \
	else \
		sudo insmod $(PWD)/$(MODULE).ko; \
	fi
	@grep -qw $(MODULE) /proc/modules
	@echo ">> Modulo Rust cargado; verifique con: sudo dmesg | grep 'Hola Mundo 202300476'"

unload:
	@if grep -qw $(MODULE) /proc/modules; then \
		sudo rmmod $(MODULE); \
	else \
		echo ">> Modulo $(MODULE) ya estaba descargado"; \
	fi
	@! grep -qw $(MODULE) /proc/modules

verify:
	@sudo dmesg | grep -F 'Hola Mundo 202300476' | tail -n 1

clean:
	$(MAKE) -C $(KDIR) M=$(PWD) clean 2>/dev/null || true
	rm -f *.o *.ko *.mod *.mod.c *.order *.symvers .*.cmd

endif
