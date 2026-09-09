#!/usr/bin/env bash
# ============================================================================
# test-module-lifecycle.sh — Prueba PRIVILEGIADA del ciclo de vida del módulo.
# Carnet: 202300476 · Proyecto 2 SO1
#
# Requiere root y headers del kernel. Si falta alguno, se omite con mensaje
# claro (prueba privilegiada pendiente de correr en la VM objetivo).
#
# Cubre (rúbrica 1.2, 1.3, 2.1, 2.2):
#   1. Idempotencia de carga/descarga (scripts load/unload).
#   2. Creación y eliminación de /proc/continfo_pr2_so1_202300476.
#   3. JSON válido contra el esquema del proyecto.
#   4. Memoria vs /proc/meminfo dentro de tolerancia documentada.
#   5. Proceso conocido presente (PID 1 y el shell actual) con datos coherentes.
#   6. Porcentaje de CPU de un proceso acotado en el tiempo (spin ~2 s).
#
# Uso:
#   sudo bash proyecto2/tests/kernel/test-module-lifecycle.sh
# ============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
VALIDATOR="${SCRIPT_DIR}/validate_schema.py"
LOAD="${ROOT}/scripts/load-module.sh"
UNLOAD="${ROOT}/scripts/unload-module.sh"
PROC_FILE="/proc/continfo_pr2_so1_202300476"
KDIR="/lib/modules/$(uname -r)/build"
TMPDIR="$(mktemp -d)"
BUSY_PID=""

cleanup() {
    [ -n "$BUSY_PID" ] && kill "$BUSY_PID" 2>/dev/null || true
    if [ -e "$PROC_FILE" ]; then
        bash "$UNLOAD" >/dev/null 2>&1 || true
    fi
    rm -rf "$TMPDIR"
}
trap cleanup EXIT

echo "=================================================================="
echo "  test-module-lifecycle — Ciclo de vida del módulo (PRIVILEGIADO)"
echo "=================================================================="

# ---- prerrequisitos -------------------------------------------------------
if [ "$(id -u)" -ne 0 ]; then
    echo "  [SKIP] se requieren privilegios de root."
    echo "         Prueba privilegiada PENDIENTE: ejecutar en la VM objetivo con"
    echo "         sudo bash proyecto2/tests/kernel/test-module-lifecycle.sh"
    exit 0
fi
if [ ! -e "${KDIR}/Makefile" ] || [ ! -e "${KDIR}/include/generated/autoconf.h" ]; then
    echo "  [SKIP] no hay headers de desarrollo del kernel en $KDIR"
    echo "         Prueba privilegiada PENDIENTE: instalar kernel-devel-$(uname -r)."
    exit 0
fi
if ! command -v python3 >/dev/null 2>&1; then
    echo "  [SKIP] python3 no está disponible."
    exit 0
fi

# ---- 1. Idempotencia: cargar dos veces -----------------------------------
echo ">> 1. Carga idempotente (dos llamadas consecutivas)"
bash "$LOAD" >/dev/null
bash "$LOAD" >/dev/null
if [ -e "$PROC_FILE" ] && grep -qw continfo /proc/modules; then
    echo "  [OK] cargado y archivo presente tras la 2a llamada"
else
    echo "  [FALLO] módulo/archivo ausente tras load" >&2
    exit 1
fi

# ---- 2. Permisos y lectura ------------------------------------------------
echo ">> 2. Permisos y lectura del archivo /proc"
perms=$(stat -c '%a' "$PROC_FILE")
if [ "$perms" != "444" ]; then
    echo "  [AVISO] permisos $perms (se esperaba 444); no es bloqueante." >&2
fi
cat "$PROC_FILE" > "${TMPDIR}/live.json"
if [ -s "${TMPDIR}/live.json" ]; then
    echo "  [OK] lectura de $(( $(wc -c < "${TMPDIR}/live.json") )) bytes"
else
    echo "  [FALLO] el archivo está vacío" >&2
    exit 1
fi

# ---- 3. JSON válido contra el esquema -------------------------------------
echo ">> 3. JSON válido (esquema + consistencia de memoria)"
if python3 "$VALIDATOR" "${TMPDIR}/live.json" --strict-memory >/dev/null 2>&1; then
    echo "  [OK] JSON válido"
else
    echo "  [FALLO] JSON inválido" >&2
    python3 "$VALIDATOR" "${TMPDIR}/live.json" --strict-memory || true
    exit 1
fi

# ---- 4. Memoria vs /proc/meminfo ------------------------------------------
echo ">> 4. Memoria vs /proc/meminfo (tolerancia documentada)"
total_kb=$(python3 -c "import json;print(json.load(open('${TMPDIR}/live.json'))['memory']['total_kb'])")
memtotal_kb=$(awk '/^MemTotal:/{print $2}' /proc/meminfo)
diff=$(( total_kb > memtotal_kb ? total_kb - memtotal_kb : memtotal_kb - total_kb ))
pct_diff=$(( memtotal_kb > 0 ? diff * 100 / memtotal_kb : 100 ))
if [ "$pct_diff" -le 1 ]; then
    echo "  [OK] total_kb=$total_kb vs MemTotal=$memtotal_kb (Δ=$pct_diff%)"
else
    echo "  [FALLO] total_kb=$total_kb vs MemTotal=$memtotal_kb (Δ=$pct_diff%)" >&2
    exit 1
fi

free_kb=$(python3 -c "import json;print(json.load(open('${TMPDIR}/live.json'))['memory']['free_kb'])")
memfree_kb=$(awk '/^MemFree:/{print $2}' /proc/meminfo)
fdiff=$(( free_kb > memfree_kb ? free_kb - memfree_kb : memfree_kb - free_kb ))
if [ "$fdiff" -le 524288 ]; then
    echo "  [OK] free_kb=$free_kb vs MemFree=$memfree_kb (Δ=${fdiff} KB <= 512 MB)"
else
    echo "  [AVISO] free_kb=$free_kb vs MemFree=$memfree_kb (Δ=${fdiff} KB)" >&2
fi

# ---- 5. Procesos conocidos presentes --------------------------------------
echo ">> 5. Procesos conocidos presentes (PID 1 y shell actual)"
if python3 - "$PROC_FILE" <<'PY' >/dev/null 2>&1
import json, sys
doc = json.load(open(sys.argv[1]))
pids = {p["pid"]: p for p in doc["processes"]}
assert 1 in pids, "PID 1 ausente"
assert pids[1]["name"] == "systemd", f"PID 1 no es systemd: {pids[1]['name']!r}"
print("ok")
PY
then
    echo "  [OK] PID 1 presente y es systemd"
else
    echo "  [FALLO] PID 1 no verificado" >&2
    exit 1
fi

SELF_PID=$$
if python3 - "$PROC_FILE" "$SELF_PID" <<'PY' >/dev/null 2>&1
import json, sys
doc = json.load(open(sys.argv[1]))
pid = int(sys.argv[2])
match = [p for p in doc["processes"] if p["pid"] == pid]
assert match, f"PID {pid} ausente"
assert match[0]["cmdline"], f"PID {pid} sin cmdline"
print("ok")
PY
then
    echo "  [OK] el shell actual (PID $SELF_PID) aparece con cmdline"
else
    echo "  [FALLO] el shell actual no aparece con cmdline" >&2
    exit 1
fi

# ---- 6. CPU de un proceso acotado (spin ~2 s) ------------------------------
echo ">> 6. CPU de un proceso con carga acotada (spin ~2 s)"
( end=$((SECONDS + 2)); while [ "$SECONDS" -lt "$end" ]; do :; done ) &
BUSY_PID=$!
sleep 1
cat "$PROC_FILE" > "${TMPDIR}/busy.json"
cpu_busy=$(python3 -c "
import json
doc = json.load(open('${TMPDIR}/busy.json'))
p = [x for x in doc['processes'] if x['pid'] == ${BUSY_PID}]
print(p[0]['cpu_percent'] if p else 'N/A')
")
kill "$BUSY_PID" 2>/dev/null || true
SPIN_PID="$BUSY_PID"
BUSY_PID=""
if [ "$cpu_busy" != "N/A" ]; then
    ok=$(python3 -c "exit(0 if float('$cpu_busy') > 5.0 else 1)" && echo yes || echo no)
    if [ "$ok" = "yes" ]; then
        echo "  [OK] proceso de spin con cpu_percent=$cpu_busy (> 5%)"
    else
        echo "  [FALLO] spin con cpu_percent=$cpu_busy (esperado > 5%)" >&2
        exit 1
    fi
else
    echo "  [FALLO] no se encontró el PID de spin ($SPIN_PID)" >&2
    exit 1
fi

# ---- 7. Descarga idempotente + archivo eliminado ---------------------------
echo ">> 7. Descarga idempotente y eliminación del archivo"
bash "$UNLOAD" >/dev/null
bash "$UNLOAD" >/dev/null
if [ ! -e "$PROC_FILE" ] && ! grep -qw continfo /proc/modules; then
    echo "  [OK] módulo descargado y archivo eliminado"
else
    echo "  [FALLO] tras unload el módulo/archivo sigue presente" >&2
    exit 1
fi

echo "=================================================================="
echo "  RESULTADO: OK — ciclo de vida completo verificado"
exit 0