#!/usr/bin/env bash
# ============================================================================
# validate-registry.sh - Valida el registro privado Zot del Proyecto 1
# Carnet: 202300476 · Sistemas Operativos 1
#
# NO destructivo: solo hace lecturas (curl al catálogo) y, opcionalmente,
# un "pull" de verificación (el pull es aditivo, no elimina nada).
#
# Uso:
#   bash scripts/validation/validate-registry.sh                 # catálogo (default)
#   bash scripts/validation/validate-registry.sh <IP_VM3>        # catálogo a esa IP
#   bash scripts/validation/validate-registry.sh <IP_VM3> pull-podman     # + pull en VM2
#   bash scripts/validation/validate-registry.sh <IP_VM3> pull-containerd # + pull en VM1
#
# Runetime por defecto del host de ejecución: docker.
# ============================================================================
set -euo pipefail

REGISTRY="${1:-192.168.100.30:5000}"
MODE="${2:-catalog}"

# Repositorios que deben existir en Zot (nombres en minúsculas por restricción de Docker)
REPOS="api1-202300476 api2-202300476 api3-202300476"

echo ">> Registro Zot: $REGISTRY"
echo ">> Modo: $MODE"

# 1) ¿El registro responde?
echo
echo "== 1. Conectividad a $REGISTRY/v2/_catalog =="
CATALOG=$(curl -sS --max-time 8 "http://$REGISTRY/v2/_catalog" || { echo "ERROR: no se pudo contactar el registro"; exit 1; })
echo "Catálogo: $CATALOG"

# 2) ¿Están los 3 repositorios?
echo
echo "== 2. Repositorios esperados =="
FAIL=0
for repo in $REPOS; do
  if echo "$CATALOG" | grep -q "$repo"; then
    echo "  [OK] $repo"
  else
    echo "  [FALTA] $repo"
    FAIL=1
  fi
done
if [ "$FAIL" -ne 0 ]; then
  echo "ERROR: faltan repositorios en el registro."
  exit 1
fi

# 3) Pull de verificación (opcional, no destructivo)
if [ "$MODE" = "pull-podman" ]; then
  echo
  echo "== 3. Pull (podman) de api1-202300476 desde $REGISTRY =="
  podman pull --tls-verify=false "$REGISTRY/api1-202300476:1.0"
  podman images | grep "$REGISTRY/api1-202300476" || true
elif [ "$MODE" = "pull-containerd" ]; then
  echo
  echo "== 3. Pull (containerd) de api3-202300476 desde $REGISTRY =="
  ctr -n default images pull --plain-http "$REGISTRY/api3-202300476:1.0"
  ctr -n default images ls | grep "$REGISTRY/api3-202300476" || true
fi

echo
echo ">> Validación completada correctamente."
