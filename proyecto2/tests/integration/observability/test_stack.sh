#!/usr/bin/env bash
#
# test_stack.sh — smoke test REAL del stack Valkey + Grafana (opt-in).
#
# No forma parte del flujo default (requiere descargar/buildar imágenes y
# levantar servicios). Se activa con:
#   SO1_RUN_STACK=1 ./test_stack.sh
#   SO1_RUN_STACK=1 bash run.sh
#
# Verifica:
#   1. docker-compose.yml válido y con nombre de proyecto so1-p2.
#   2. `compose up -d --wait` levanta so1-p2-valkey y so1-p2-grafana.
#   3. Valkey responde al ping (dentro del contenedor).
#   4. Grafana responde en /api/health.
#   5. Ambos llevan etiquetas de protección (so1.p2.protect=1) y NUNCA son
#      gestionados (sin so1.p2.managed).
#   6. Baja el stack al final (conserva volúmenes).
#
# Solo toca recursos del proyecto so1-p2: no interfiere con contenedores ajenos.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
DEPLOY_DIR="${ROOT}/deploy"
COMPOSE="${DEPLOY_DIR}/docker-compose.yml"
GRAFANA_PORT="${GRAFANA_PORT:-3000}"

PASS=0
FAIL=0
SKIP=0

ok()   { PASS=$((PASS + 1)); printf '  ok: %s\n' "$*"; }
ko()   { FAIL=$((FAIL + 1)); printf '  FAIL: %s\n' "$*"; }
skip() { SKIP=$((SKIP + 1)); printf '  SKIP: %s\n' "$*"; }

if [[ "${SO1_RUN_STACK:-0}" != "1" ]]; then
  echo "SKIP: SO1_RUN_STACK=1 para ejecutar el smoke del stack (descarga imágenes)."
  exit 0
fi

docker info >/dev/null 2>&1 || { echo "SKIP: docker daemon no disponible."; exit 0; }

docker compose -f "$COMPOSE" config --quiet 2>/dev/null \
  && ok "compose config válido" || ko "compose config inválido"

grep -q '^name: so1-p2' "$COMPOSE" && ok "proyecto Compose explícito so1-p2" || ko "proyecto Compose no fijado"

if ! docker compose -f "$COMPOSE" up -d --wait >/dev/null 2>&1; then
  docker compose -f "$COMPOSE" up -d >/dev/null 2>&1 || true
fi

if docker ps --format '{{.Names}}' | grep -qx 'so1-p2-valkey'; then
  ok "so1-p2-valkey arriba"
  if docker exec so1-p2-valkey valkey-cli ping 2>/dev/null | grep -q PONG; then
    ok "Valkey responde PONG"
  else
    ko "Valkey no responde PONG"
  fi
else
  ko "so1-p2-valkey no está corriendo"
fi

if docker ps --format '{{.Names}}' | grep -qx 'so1-p2-grafana'; then
  ok "so1-p2-grafana arriba"
  if curl -fsS "http://localhost:${GRAFANA_PORT}/api/health" 2>/dev/null | grep -q '"database"'; then
    ok "Grafana responde en /api/health"
  else
    ko "Grafana no responde en /api/health"
  fi
else
  ko "so1-p2-grafana no está corriendo"
fi

prot_ok=1
for name in so1-p2-valkey so1-p2-grafana; do
  labels=$(docker inspect "$name" --format '{{json .Config.Labels}}' 2>/dev/null || echo "")
  if [[ "$labels" != *'"so1.p2.protect":"1"'* ]]; then prot_ok=0; fi
  if [[ "$labels" == *'"so1.p2.managed"'* ]]; then prot_ok=0; fi
done
[[ "$prot_ok" -eq 1 ]] && ok "protegidos y NO gestionados (so1-p2-valkey/grafana)" \
  || ko "etiquetas de protección ausentes o gestión indebida"

docker compose -f "$COMPOSE" down >/dev/null 2>&1 || true
ok "stack bajado (volúmenes conservados)"

printf '  resultados: %d ok, %d fail, %d skip\n' "$PASS" "$FAIL" "$SKIP"
[[ "$FAIL" -eq 0 ]]