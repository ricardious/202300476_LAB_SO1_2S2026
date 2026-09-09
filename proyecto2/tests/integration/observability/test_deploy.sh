#!/usr/bin/env bash
#
# test_deploy.sh — valida compose, provisioning YAML y dashboard JSON.
# Incluido por run.sh; también puede ejecutarse solo.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
DEPLOY_DIR="${ROOT}/deploy"
DASHBOARD="${DEPLOY_DIR}/grafana/dashboards/so1-p2-overview.json"

PASS=0
FAIL=0

ok() { PASS=$((PASS + 1)); printf '  ok: %s\n' "$*"; }
ko() { FAIL=$((FAIL + 1)); printf '  FAIL: %s\n' "$*"; }

if docker compose -f "${DEPLOY_DIR}/docker-compose.yml" config --quiet 2>/dev/null; then
  ok "compose config válido"
else
  ko "compose config inválido"
fi

if jq -e . "$DASHBOARD" >/dev/null 2>&1; then
  ok "dashboard JSON válido"
else
  ko "dashboard JSON inválido"
fi

for k in system:latest system:timeline containers:timeline rank:ram rank:cpu kills kills:count; do
  jq -e --arg k "so1:p2:${k}" '.. | strings | select(. == $k)' "$DASHBOARD" >/dev/null 2>&1 \
    && ok "clave so1:p2:${k}" || ko "clave so1:p2:${k} ausente"
done

for p in "RAM total" "RAM usada" "RAM libre" "Evolución temporal de RAM" "Eliminaciones confirmadas (XLEN)" "Eliminaciones confirmadas a lo largo del tiempo" "Top 5 histórico por RAM (PID e ID)" "Top 5 histórico por CPU (PID e ID)" "Tabla de eventos kill capturados por eBPF"; do
  jq -e --arg p "$p" '[.panels[]?.title] | index($p) != null' "$DASHBOARD" >/dev/null 2>&1 \
    && ok "panel: $p" || ko "panel faltante: $p"
done

printf '  resultados: %d ok, %d fail\n' "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]