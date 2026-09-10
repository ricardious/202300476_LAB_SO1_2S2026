#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODULE_DIR="${ROOT}/rust-module"
KO="${MODULE_DIR}/hola_rust_202300476.ko"

echo "=================================================================="
echo "  Rust extra — compilación del módulo Hola Mundo"
echo "=================================================================="

command -v rustc >/dev/null 2>&1 || {
    echo "[FALLO] rustc no está instalado"
    exit 1
}

make -C "${MODULE_DIR}" clean >/dev/null
make -C "${MODULE_DIR}"

test -s "${KO}"
test "$(modinfo -F name "${KO}")" = "hola_rust_202300476"
modinfo -F vermagic "${KO}" | grep -q "$(uname -r)"
strings "${KO}" | grep -F "Hola Mundo 202300476" >/dev/null

echo "[OK] ${KO} compilado"
echo "[OK] nombre, vermagic y mensaje 202300476 verificados"
