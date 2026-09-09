#!/usr/bin/env bash
# ============================================================================
# build-images.sh - Construye las imágenes de las 3 APIs del Proyecto 1
# Carnet: 202300476 · Sistemas Operativos 1 · 2do Semestre 2026
#
# Uso:
#   bash scripts/build-images.sh                 # usa docker por defecto
#   CONTAINER_RUNTIME=podman bash scripts/build-images.sh   # o podman
#
# NOTA SOBRE EL NOMBRE DE LAS IMÁGENES:
# El enunciado pide nombres con mayúsculas (API1-202300476, etc.), pero Docker
# y Podman exigen que el nombre de repositorio de una imagen sea MINÚSCULAS.
# Por eso la imagen se construye como  api1-202300476  y el nombre canónico
# API1-202300476 se conserva como LABEL dentro de la imagen (documentado).
# Tags: libres (aquí se usa :1.0).
# ============================================================================
set -euo pipefail

# Runtime de contenedores (docker por defecto, se puede cambiar con CONTAINER_RUNTIME)
RUNTIME="${CONTAINER_RUNTIME:-docker}"

# Imágenes finales (minúsculas por restricción de Docker/Podman)
API1_IMG="api1-202300476:1.0"
API2_IMG="api2-202300476:1.0"
API3_IMG="api3-202300476:1.0"

echo ">> Runtime de contenedores: $RUNTIME"

echo
echo "== Construyendo API1 ($API1_IMG) =="
"$RUNTIME" build -t "$API1_IMG" -f api1/Dockerfile api1/

echo
echo "== Construyendo API2 ($API2_IMG) =="
"$RUNTIME" build -t "$API2_IMG" -f api2/Dockerfile api2/

echo
echo "== Construyendo API3 ($API3_IMG) =="
"$RUNTIME" build -t "$API3_IMG" -f api3/Dockerfile api3/

echo
echo "== Imágenes construidas =="
echo "Nombre canónico (mayúsculas, solo LABEL) -> Nombre real (minúsculas):"
echo "  API1-202300476 -> $API1_IMG"
echo "  API2-202300476 -> $API2_IMG"
echo "  API3-202300476 -> $API3_IMG"
echo
"$RUNTIME" images | grep -E "api[123]-202300476" || true
