#!/usr/bin/env bash
# ============================================================================
# test-all.sh — Ejecuta todas las verificaciones del frente kernel.
# Carnet: 202300476 · Proyecto 2 SO1
#
#   bash proyecto2/tests/kernel/test-all.sh
#
# Cobertura:
#   - test-build.sh           : compila contra headers (SKIP si no hay).
#   - test-schema.sh          : contrato JSON (sin privilegios).
#   - test-module-lifecycle.sh: ciclo de vida (SKIP si no hay root/headers).
# ============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FAIL=0

for t in test-build.sh test-schema.sh test-module-lifecycle.sh; do
    echo
    if bash "${SCRIPT_DIR}/${t}"; then
        :
    else
        FAIL=1
    fi
done

echo
echo "=================================================================="
if [ "$FAIL" -eq 0 ]; then
    echo "  test-all: OK (con omisiones [SKIP] documentadas en cada prueba)"
    exit 0
else
    echo "  test-all: HUBO FALLOS (ver arriba)"
    exit 1
fi