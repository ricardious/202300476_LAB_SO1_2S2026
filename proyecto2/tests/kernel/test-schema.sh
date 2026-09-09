#!/usr/bin/env bash
# ============================================================================
# test-schema.sh — Valida el contrato JSON del módulo (sin privilegios).
# Carnet: 202300476 · Proyecto 2 SO1
#
# 1. Valida el fixture de referencia.
# 2. Si el módulo está cargado, valida la salida EN VIVO de /proc.
#
# Rúbrica 1.3 (estructura). Uso:
#   bash proyecto2/tests/kernel/test-schema.sh
# ============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VALIDATOR="${SCRIPT_DIR}/validate_schema.py"
FIXTURE="${SCRIPT_DIR}/fixtures/sample-snapshot.json"
PROC_FILE="/proc/continfo_pr2_so1_202300476"
FAIL=0

echo "=================================================================="
echo "  test-schema — Contrato JSON de /proc/continfo_pr2_so1_202300476"
echo "=================================================================="

if ! command -v python3 >/dev/null 2>&1; then
    echo "  [SKIP] python3 no está disponible."
    exit 0
fi

echo ">> 1. Fixture de referencia"
if python3 "$VALIDATOR" "$FIXTURE" >/dev/null 2>&1; then
    echo "  [OK] fixture válido"
else
    echo "  [FALLO] el fixture no pasa el validador" >&2
    FAIL=1
fi

echo ">> 2. Salida en vivo del módulo (si está cargado)"
if [ -r "$PROC_FILE" ]; then
    if python3 "$VALIDATOR" "$PROC_FILE" --strict-memory >/dev/null 2>&1; then
        echo "  [OK] salida en vivo válida (esquema + consistencia de memoria)"
    else
        echo "  [FALLO] la salida en vivo no es válida" >&2
        python3 "$VALIDATOR" "$PROC_FILE" --strict-memory || true
        FAIL=1
    fi
else
    echo "  [SKIP] módulo no cargado ($PROC_FILE no existe); se valida solo el fixture."
fi

echo "=================================================================="
if [ "$FAIL" -eq 0 ]; then
    echo "  RESULTADO: OK"
    exit 0
else
    echo "  RESULTADO: FALLO"
    exit 1
fi