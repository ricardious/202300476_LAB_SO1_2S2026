#!/usr/bin/env bash
# ============================================================================
# test-all.sh - Validación completa de las APIs del Proyecto 1
# Carnet: 202300476 · Sistemas Operativos 1
#
# Ejecuta las comprobaciones PRINCIPALES SIN modificar el sistema (solo lecturas
# HTTP con curl + jq). Opcionalmente la prueba de error:
#   bash scripts/validation/test-all.sh                 # comprobaciones principales
#   bash scripts/validation/test-all.sh --error-test    # + detener/levantar API3
#
# La prueba de error detiene y vuelve a levantar API3, garantizando que al
# finalizar API3 queda EN EJECUCIÓN (no se dejan servicios detenidos).
# ============================================================================
set -uo pipefail

CARNET="202300476"
API1="http://192.168.100.10:8081"
API2="http://192.168.100.10:8082"
API3="http://192.168.100.20:8083"
SSH="ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i $HOME/.ssh/id_ed25519"
VM2="192.168.100.20"

ERROR_TEST=0
[ "${1:-}" = "--error-test" ] && ERROR_TEST=1

FAIL=0

req() { # req <nombre> <url> <connection_esperado>
  local name="$1" url="$2" exp="${3:-true}"
  local body code conn carnet
  body=$(curl -s --max-time 10 "$url")
  code=$(curl -s -o /dev/null -w "%{http_code}" --max-time 10 "$url")
  conn=$(printf '%s' "$body" | jq -r '.connection' 2>/dev/null)
  carnet=$(printf '%s' "$body" | jq -r '.carnet // "n/a"' 2>/dev/null)
  if [ "$code" = "200" ] && [ "$conn" = "$exp" ] && [ "$carnet" = "$CARNET" ]; then
    echo "  [OK] $name -> HTTP=$code connection=$conn carnet=$carnet"
  else
    echo "  [FALLO] $name -> HTTP=$code connection=$conn carnet=$carnet (esperado=$exp)"
    FAIL=1
  fi
}

echo "=================================================================="
echo "  Validación Proyecto 1 - Carnet $CARNET"
echo "=================================================================="

echo "== 1. Endpoints /health (curl | jq) =="
for pair in "API1:$API1" "API2:$API2" "API3:$API3"; do
  n="${pair%%:*}"; u="${pair#*:}"
  st=$(curl -s --max-time 8 "$u/health" | jq -r '.status' 2>/dev/null)
  vm=$(curl -s --max-time 8 "$u/health" | jq -r '.VM' 2>/dev/null)
  carnet=$(curl -s --max-time 8 "$u/health" | jq -r '.carnet' 2>/dev/null)
  if [ "$st" = "UP" ] && [ "$carnet" = "$CARNET" ]; then
    echo "  [OK] $n /health -> status=$st VM=$vm carnet=$carnet"
  else
    echo "  [FALLO] $n /health -> status=$st carnet=$carnet"
    FAIL=1
  fi
done

echo "== 2. Seis endpoints de llamada (connection esperado = true) =="
req "API1->API2" "$API1/api1/$CARNET/call-api2" true
req "API1->API3" "$API1/api1/$CARNET/call-api3" true
req "API2->API1" "$API2/api2/$CARNET/call-api1" true
req "API2->API3" "$API2/api2/$CARNET/call-api3" true
req "API3->API1" "$API3/api3/$CARNET/call-api1" true
req "API3->API2" "$API3/api3/$CARNET/call-api2" true

if [ "$ERROR_TEST" = "1" ]; then
  echo "== 3. Prueba de error (detener/levantar API3) =="
  $SSH root@"$VM2" 'podman stop api3' >/dev/null 2>&1
  sleep 2
  req "API1->API3 (API3 caida)" "$API1/api1/$CARNET/call-api3" false
  $SSH root@"$VM2" 'podman start api3' >/dev/null 2>&1
  sleep 3
  req "API1->API3 (API3 recuperada)" "$API1/api1/$CARNET/call-api3" true
  echo "  (API3 quedó en ejecución)"
fi

echo
if [ "$FAIL" = "0" ]; then
  echo ">> RESULTADO: TODAS LAS COMPROBACIONES OK"
else
  echo ">> RESULTADO: HAY FALLOS"
  exit 1
fi
