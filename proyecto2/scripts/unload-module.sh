#!/usr/bin/env bash
# ============================================================================
# unload-module.sh — Descarga el módulo de kernel del Proyecto 2 SO1
# Carnet: 202300476 · 2do Semestre 2026
#
# Idempotente: si el módulo no está cargado, no hace nada y termina con éxito.
# Verifica que /proc/continfo_pr2_so1_202300476 desaparezca tras la descarga.
#
# Uso:
#   bash proyecto2/scripts/unload-module.sh   # requiere root (o sudo)
# ============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KERNEL_DIR="${SCRIPT_DIR}/../kernel"
MODNAME="continfo"
PROC_FILE="/proc/continfo_pr2_so1_202300476"

# ---- privilegios ---------------------------------------------------------
if [ "$(id -u)" -ne 0 ]; then
    if command -v sudo >/dev/null 2>&1; then
        echo ">> Re-ejecutando con sudo..."
        exec sudo -E bash "$0" "$@"
    else
        echo "ERROR: se requieren privilegios de root (o sudo) para descargar el módulo." >&2
        exit 1
    fi
fi

# ---- estado actual --------------------------------------------------------
is_loaded() {
    grep -qw "$MODNAME" /proc/modules 2>/dev/null
}

# ---- paso 1: ya descargado (idempotencia) ---------------------------------
if ! is_loaded && [ ! -e "$PROC_FILE" ]; then
    echo ">> Módulo $MODNAME no está cargado. Nada que hacer."
    exit 0
fi

# ---- paso 2: descargar -----------------------------------------------------
echo ">> Descargando módulo $MODNAME ..."
if is_loaded; then
    rmmod "$MODNAME" || {
        echo "ERROR: falló rmmod $MODNAME" >&2
        exit 1
    }
fi

# ---- paso 3: verificar -----------------------------------------------------
if is_loaded; then
    echo "ERROR: el módulo sigue en /proc/modules tras rmmod." >&2
    exit 1
fi
if [ -e "$PROC_FILE" ]; then
    echo "ERROR: $PROC_FILE sigue existiendo tras descargar el módulo." >&2
    exit 1
fi

echo ">> OK: módulo descargado y $PROC_FILE eliminado."