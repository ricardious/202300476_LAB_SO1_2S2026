#!/usr/bin/env bash
# ============================================================================
# doctor.sh — Diagnóstico del entorno del Proyecto 2 de SO1 (202300476)
#
# Verifica cada componente y reporta OK / FALLO con mensajes accionables.
# No modifica nada. Los checks privilegiados (módulo/eBPF) se reportan como
# PENDIENTE si el entorno no tiene root/CAP_BPF, sin romper la ejecución.
#
# Uso:
#   bash proyecto2/scripts/doctor.sh            # checks no privilegiados
#   sudo bash proyecto2/scripts/doctor.sh --privileged   # + módulo/eBPF
#   bash proyecto2/scripts/doctor.sh --json     # salida JSON (para CI)
# ============================================================================
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PRIVILEGED=0
JSON=0
for arg in "$@"; do
  case "$arg" in
    --privileged) PRIVILEGED=1 ;;
    --json) JSON=1 ;;
    *) echo "opción desconocida: $arg" >&2; exit 2 ;;
  esac
done

PROC_FILE="/proc/continfo_pr2_so1_202300476"
COMPOSE="${ROOT}/deploy/docker-compose.yml"
DASHBOARD="${ROOT}/deploy/grafana/dashboards/so1-p2-overview.json"
GENERATOR="/usr/local/bin/so1-p2-generate-containers.sh"
DAEMON_BIN="${ROOT}/bin/so1-p2-daemon"

PASS=0
FAIL=0
WARN=0
SKIPPED=0
declare -a FAILURES=()

ok()      { PASS=$((PASS + 1)); printf '  [OK]      %s\n' "$*"; }
warn()    { WARN=$((WARN + 1)); printf '  [AVISO]   %s\n' "$*"; }
fail()    { FAIL=$((FAIL + 1)); FAILURES+=("$*"); printf '  [FALLO]   %s\n' "$*"; }
skip()    { SKIPPED=$((SKIPPED + 1)); printf '  [PEND]    %s\n' "$*"; }

section() { printf '\n== %s ==\n' "$*"; }

section "Herramientas base"
for t in bash make go docker git; do
  if command -v "$t" >/dev/null 2>&1; then ok "binario $t"; else fail "binario $t ausente"; fi
done

section "Código y build"
if [ -f "$ROOT/daemon/go.mod" ]; then ok "go.mod del daemon"; else fail "falta daemon/go.mod"; fi
if [ -x "$DAEMON_BIN" ]; then ok "daemon compilado ($DAEMON_BIN)"; else warn "daemon no compilado: ejecute make build"; fi
if [ -f "$ROOT/kernel/continfo.c" ]; then ok "código del módulo de kernel"; else fail "falta kernel/continfo.c"; fi
if [ -f "$ROOT/ebpf/kill_monitor.bpf.c" ]; then ok "código eBPF"; else fail "falta ebpf/kill_monitor.bpf.c"; fi
if [ -f "$ROOT/daemon/internal/ebpf/killmonitor_bpf.o" ]; then ok "objeto eBPF embebido"; else fail "falta killmonitor_bpf.o (make ebpf)"; fi

section "Docker / Compose"
if docker info >/dev/null 2>&1; then ok "daemon Docker responde"; else fail "daemon Docker no responde"; fi
if docker compose version >/dev/null 2>&1; then ok "docker compose disponible"; else fail "docker compose ausente"; fi
if docker compose -f "$COMPOSE" config --quiet >/dev/null 2>&1; then
  ok "docker-compose.yml válido"
else
  fail "docker-compose.yml inválido"
fi

section "Stack de observabilidad (Valkey / Grafana)"
VALKEY_PORT="${VALKEY_PORT:-6379}"
GRAFANA_PORT="${GRAFANA_PORT:-3000}"
if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx 'so1-p2-valkey'; then
  ok "contenedor so1-p2-valkey arriba"
  if docker exec so1-p2-valkey valkey-cli ping >/dev/null 2>&1; then
    ok "Valkey responde al ping"
  else
    fail "Valkey no responde al ping"
  fi
else
  warn "so1-p2-valkey no está corriendo (ejecute: make up)"
fi
if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx 'so1-p2-grafana'; then
  ok "contenedor so1-p2-grafana arriba"
  if curl -fsS "http://localhost:${GRAFANA_PORT}/api/health" >/dev/null 2>&1; then
    ok "Grafana responde en /api/health"
  else
    fail "Grafana no responde en /api/health"
  fi
else
  warn "so1-p2-grafana no está corriendo (ejecute: make up)"
fi
if jq -e . "$DASHBOARD" >/dev/null 2>&1; then ok "dashboard JSON válido"; else fail "dashboard JSON inválido"; fi

section "Módulo de kernel (/proc)"
if [ -r "$PROC_FILE" ]; then
  ok "$PROC_FILE existe y es legible"
  if python3 "${ROOT}/tests/kernel/validate_schema.py" "$PROC_FILE" >/dev/null 2>&1; then
    ok "salida en vivo cumple el esquema JSON"
  else
    fail "la salida en vivo no cumple el esquema (validate_schema.py)"
  fi
  if grep -qw continfo /proc/modules 2>/dev/null; then ok "módulo continfo en /proc/modules"; else warn "módulo continfo cargado pero no listado"; fi
elif [ "$PRIVILEGED" -eq 1 ]; then
  fail "$PROC_FILE no existe (cargue el módulo: sudo make module-load)"
else
  skip "$PROC_FILE no existe (requiere root; use --privileged o cargue el módulo)"
fi
if [ -d "/lib/modules/$(uname -r)/build" ]; then ok "headers del kernel presentes"; else skip "headers del kernel ausentes (la compilación del módulo se omite)"; fi

section "eBPF"
if [ -r /sys/kernel/btf/vmlinux ]; then ok "BTF del kernel disponible (CO-RE)"; else fail "falta /sys/kernel/btf/vmlinux (kernel sin BTF)"; fi
if [ "$(id -u)" -eq 0 ]; then
  ok "root disponible: se puede cargar eBPF"
else
  if [ "$PRIVILEGED" -eq 1 ]; then
    fail "sin root/CAP_BPF no se puede cargar eBPF (SO1_P2_EBPF_MODE=auto degrada; required falla)"
  else
    skip "sin root; use --privileged o SO1_P2_EBPF_MODE=auto para degradar"
  fi
fi

section "Cron"
if command -v crontab >/dev/null 2>&1; then
  if crontab -l 2>/dev/null | grep -q 'so1-p2-202300476'; then
    ok "cronjob del proyecto instalado"
  else
    warn "cronjob del proyecto NO instalado (ejecute: make install-cron)"
  fi
else
  skip "binario crontab ausente en este host"
fi
if [ -x "$GENERATOR" ]; then ok "generador instalado en $GENERATOR"; else warn "generador no instalado en /usr/local/bin (ejecute: sudo make install)"; fi

printf '\n== RESUMEN ==\n'
printf '  OK: %d   AVISOS: %d   PENDIENTES: %d   FALLOS: %d\n' "$PASS" "$WARN" "$SKIPPED" "$FAIL"
if [ "$FAIL" -gt 0 ]; then
  printf '  Fallos:\n'
  for f in "${FAILURES[@]}"; do printf '    - %s\n' "$f"; done
  exit 1
fi
printf '  Entorno apto para el Proyecto 2 (los pendientes son opcionales o requieren root).\n'
exit 0