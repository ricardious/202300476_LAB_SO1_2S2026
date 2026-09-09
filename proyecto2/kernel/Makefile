# ============================================================================
# Makefile del módulo de kernel — Proyecto 2 SO1 (Carnet 202300476)
#
# Construye continfo.ko que expone /proc/continfo_pr2_so1_202300476
#
# Uso:
#   make            # compila contra los headers del kernel en ejecución
#   make KDIR=/usr/src/kernels/...   # compilar contra otro árbol
#   make clean
#   make check-headers   # verifica que los headers del kernel existen
#
# Rúbrica 1.1: "Compilación módulo" -> make -C proyecto2/kernel
# ============================================================================
KDIR ?= /lib/modules/$(shell uname -r)/build
PWD  := $(CURDIR)

obj-m := continfo.o

# Señales de un árbol de headers útil (generados + Makefile de Kbuild).
HEADER_MARKERS = $(KDIR)/include/generated/autoconf.h \
                 $(KDIR)/include/generated/uapi/linux/version.h \
                 $(KDIR)/Makefile

.PHONY: all clean check-headers

all: check-headers
	$(MAKE) -C $(KDIR) M=$(PWD) modules

# Solo mensaje claro si el árbol de headers no existe; si existe,
# dejamos que Kbuild reporte cualquier error real.
check-headers:
	@for f in $(HEADER_MARKERS); do \
		if [ ! -e "$$f" ]; then \
			echo "ERROR: no se encontro $$f"; \
			echo; \
			echo "Los headers de desarrollo del kernel NO estan instalados."; \
			echo "Instale el paquete kernel-devel que coincida con:"; \
			echo "  uname -r -> $(shell uname -r)"; \
			echo "Ejemplos (Fedora/RHEL):"; \
			echo "  sudo dnf install kernel-devel-$(shell uname -r)"; \
			echo "  sudo dnf install kernel-devel"; \
			echo "En Debian/Ubuntu:"; \
			echo "  sudo apt install linux-headers-$(shell uname -r)"; \
			echo; \
			echo "Alternativa: apunte a otro arbol con KDIR=..."; \
			exit 2; \
		fi; \
	done

clean:
	$(MAKE) -C $(KDIR) M=$(PWD) clean 2>/dev/null || true
	rm -f *.o *.ko *.mod *.mod.c *.order *.symvers .*.cmd .*.o.cmd .tmp_versions