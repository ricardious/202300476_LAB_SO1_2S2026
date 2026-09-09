#!/usr/bin/env bash
# ============================================================================
# test-build.sh — Compila el módulo de kernel contra los headers disponibles.
# Carnet: 202300476 · Proyecto 2 SO1
#
# Rúbrica 1.1. Si el árbol de headers del kernel NO está instalado, la prueba
# se omite con un mensaje claro (no es un fallo del código del módulo).
#
# Uso:
#   bash proyecto2/tests/kernel/test-build.sh
# ============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KERNEL_DIR="${SCRIPT_DIR}/../../kernel"
KDIR="${KDIR:-/lib/modules/$(uname -r)/build}"
TMP_LOG="$(mktemp)"

echo "=================================================================="
echo "  test-build — Compilación del módulo de kernel"
echo "=================================================================="

if [ ! -e "${KDIR}/Makefile" ] || [ ! -e "${KDIR}/include/generated/autoconf.h" ]; then
    echo "  [SKIP] No hay headers de desarrollo del kernel en:"
    echo "         $KDIR"
    echo "         Instale kernel-devel-$(uname -r) para habilitar esta prueba."
    echo "         (No es un fallo del código: solo se compila si hay headers.)"
    exit 0
fi

echo ">> Headers: $KDIR"
echo ">> make -C ${KERNEL_DIR} KDIR=${KDIR}"

if ! make -C "$KERNEL_DIR" KDIR="$KDIR" >"$TMP_LOG" 2>&1; then
    echo "  [FALLO] la compilación falló. Salida:" >&2
    cat "$TMP_LOG" >&2
    rm -f "$TMP_LOG"
    exit 1
fi
rm -f "$TMP_LOG"

if [ ! -e "${KERNEL_DIR}/continfo.ko" ]; then
    echo "  [FALLO] no se produjo continfo.ko" >&2
    exit 1
fi

echo "  [OK] continfo.ko generado correctamente"
exit 0