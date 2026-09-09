#!/usr/bin/env bash
# ============================================================================
# load-module.sh — Compila y carga el módulo de kernel del Proyecto 2 SO1
# Carnet: 202300476 · 2do Semestre 2026
#
# Idempotente: si el módulo ya está cargado y /proc/continfo_pr2_so1_202300476
# existe, no hace nada y termina con éxito.
#
# Uso:
#   bash proyecto2/scripts/load-module.sh          # requiere root (o sudo)
#   bash proyecto2/scripts/load-module.sh --force  # insmod aunque el archivo
#                                                  # /proc ya exista (solo si
#                                                  # el módulo NO está cargado)
# ============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KERNEL_DIR="${SCRIPT_DIR}/../kernel"
MODNAME="continfo"
KO="${KERNEL_DIR}/${MODNAME}.ko"
PROC_FILE="/proc/continfo_pr2_so1_202300476"

# ---- privilegios ---------------------------------------------------------
if [ "$(id -u)" -ne 0 ]; then
    if command -v sudo >/dev/null 2>&1; then
        echo ">> Re-ejecutando con sudo..."
        exec sudo -E bash "$0" "$@"
    else
        echo "ERROR: se requieren privilegios de root (o sudo) para cargar el módulo." >&2
        exit 1
    fi
fi

# ---- comprobación de estado actual --------------------------------------
is_loaded() {
    grep -qw "$MODNAME" /proc/modules 2>/dev/null
}

# ---- paso 1: ya cargado y archivo presente (idempotencia) ---------------
if is_loaded && [ -e "$PROC_FILE" ]; then
    echo ">> Módulo $MODNAME ya cargado y $PROC_FILE presente. Nada que hacer."
    exit 0
fi

# ---- paso 2: compilar si hace falta -------------------------------------
if [ ! -e "$KO" ]; then
    echo ">> Compilando módulo de kernel en $KERNEL_DIR ..."
    make -C "$KERNEL_DIR"
    if [ ! -e "$KO" ]; then
        echo "ERROR: la compilación no produjo $KO" >&2
        exit 1
    fi
fi

# ---- paso 3: cargar -------------------------------------------------------
echo ">> Cargando módulo $MODNAME ..."
insmod "$KO"

# ---- paso 4: verificar ----------------------------------------------------
if ! is_loaded; then
    echo "ERROR: el módulo no aparece en /proc/modules tras insmod." >&2
    exit 1
fi
if [ ! -e "$PROC_FILE" ]; then
    echo "ERROR: el módulo está cargado pero no se creó $PROC_FILE." >&2
    exit 1
fi

echo ">> OK: módulo cargado. Interfaz disponible en $PROC_FILE"
echo ">> Pruebe: cat $PROC_FILE | python3 -m json.tool"