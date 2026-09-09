#!/usr/bin/env bash
# ============================================================================
# cleanup.sh — Retira SOLO los recursos del Proyecto 2 de SO1 (202300476)
#
# Nunca toca contenedores, volúmenes ni redes ajenas. Los recursos del
# proyecto se identifican por etiqueta (so1.p2.project=202300476) y por el
# nombre de proyecto Compose "so1-p2".
#
# Acciones (idempotentes):
#   1. Elimina los contenedores de carga con la etiqueta del proyecto.
#   2. Retira la entrada de cron del proyecto (solo el bloque delimitado).
#   3. Baja el stack Compose del proyecto (conservando volúmenes).
#   4. (Opcional --module) Descarga el módulo de kernel si está cargado.
#   5. (Opcional --purge) Elimina los volúmenes NOMBRADOS del proyecto.
#
# Uso:
#   bash proyecto2/scripts/cleanup.sh                 # pide confirmación
#   bash proyecto2/scripts/cleanup.sh --no-confirm    # sin prompt (CI/demo)
#   bash proyecto2/scripts/cleanup.sh --module --purge
# ============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIRM=1
UNLOAD_MODULE=0
PURGE=0

for arg in "$@"; do
  case "$arg" in
    --no-confirm) CONFIRM=0 ;;
    --module) UNLOAD_MODULE=1 ;;
    --purge) PURGE=1 ;;
    *) echo "opción desconocida: $arg" >&2; exit 2 ;;
  esac
done

log() { printf '[cleanup] %s\n' "$*"; }

# ---- 0. Confirmación -------------------------------------------------------
if [ "$CONFIRM" -eq 1 ]; then
  printf 'Esto retirará SOLO los recursos del proyecto 2 (etiqueta so1.p2.project=202300476 y Compose so1-p2). ¿Continuar? [y/N] '
  read -r resp
  case "$resp" in
    y|Y|s|S) ;;
    *) log "abortado por el usuario"; exit 0 ;;
  esac
fi

# ---- 1. Contenedores de carga etiquetados ---------------------------------
mapfile -t project_containers < <(docker ps -a --filter "label=so1.p2.project=202300476" --format '{{.ID}}' 2>/dev/null || true)
if [ "${#project_containers[@]}" -gt 0 ]; then
  log "eliminando ${#project_containers[@]} contenedor(es) del proyecto..."
  docker rm -f "${project_containers[@]}" >/dev/null 2>&1 || true
  log "contenedores del proyecto eliminados"
else
  log "no hay contenedores del proyecto que eliminar"
fi

# ---- 2. Cronjob del proyecto -----------------------------------------------
if [ -f "${ROOT}/scripts/remove-cron.sh" ]; then
  log "retirando entrada de cron del proyecto..."
  bash "${ROOT}/scripts/remove-cron.sh" || true
fi

# ---- 3. Compose del proyecto -----------------------------------------------
COMPOSE_FILE="${ROOT}/deploy/docker-compose.yml"
if [ -f "$COMPOSE_FILE" ]; then
  log "bajando stack Compose del proyecto (so1-p2)..."
  docker compose -f "$COMPOSE_FILE" down >/dev/null 2>&1 || true
  if [ "$PURGE" -eq 1 ]; then
    log "eliminando volúmenes NOMBRADOS del proyecto..."
    docker volume rm so1-p2-valkey-data so1-p2-grafana-data >/dev/null 2>&1 || true
  fi
fi

# ---- 4. Módulo de kernel (opcional) ----------------------------------------
if [ "$UNLOAD_MODULE" -eq 1 ]; then
  if [ "$(id -u)" -eq 0 ]; then
    log "descargando módulo de kernel..."
    bash "${ROOT}/scripts/unload-module.sh" || true
  else
    log "descarga del módulo omitida (requiere root; use: sudo bash $0 --module)"
  fi
fi

log "limpieza completada. Recursos ajenos intactos."