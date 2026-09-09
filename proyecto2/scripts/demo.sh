#!/usr/bin/env bash
# ============================================================================
# demo.sh — Demostración de punta a punta del Proyecto 2 de SO1 (202300476)
#
# Prepara el entorno (sin borrar nada ajeno) y ejecuta una secuencia corta:
#   1. Levanta Valkey + Grafana (Compose).
#   2. Genera 5 contenedores de carga (perfiles aleatorios de las 4 imágenes).
#   3. Muestra el estado y la URL del dashboard.
#   4. Si se solicita (--daemon), lanza el daemon en segundo plano tras
#      verificar prerrequisitos (módulo /proc + eBPF). Con --degraded el
#      daemon arranca aunque el entorno no tenga privilegios.
#
# Uso:
#   bash proyecto2/scripts/demo.sh                    # pasos 1-3
#   sudo bash proyecto2/scripts/demo.sh --daemon      # + daemon real (root)
#   bash proyecto2/scripts/demo.sh --daemon --degraded
#   bash proyecto2/scripts/demo.sh --clean            # limpia SOLO recursos del proyecto
# ============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_DAEMON=0
DEGRADED=0
DO_CLEAN=0

for arg in "$@"; do
  case "$arg" in
    --daemon) RUN_DAEMON=1 ;;
    --degraded) DEGRADED=1 ;;
    --clean) DO_CLEAN=1 ;;
    *) echo "opción desconocida: $arg" >&2; exit 2 ;;
  esac
done

log() { printf '[demo] %s\n' "$*"; }
die() { log "ERROR: $*"; exit 1; }

if [ "$DO_CLEAN" -eq 1 ]; then
  bash "${ROOT}/scripts/cleanup.sh" --no-confirm
  exit $?
fi

command -v docker >/dev/null 2>&1 || die "docker no está en PATH"
docker info >/dev/null 2>&1 || die "el daemon de Docker no responde"

# ---- 1. Compose ------------------------------------------------------------
log "1/4 · levantando Valkey + Grafana (Compose)..."
docker compose -f "${ROOT}/deploy/docker-compose.yml" up -d --wait 2>/dev/null \
  || docker compose -f "${ROOT}/deploy/docker-compose.yml" up -d

# ---- 2. Contenedores de carga ---------------------------------------------
log "2/4 · generando 5 contenedores de perfil aleatorio..."
bash "${ROOT}/scripts/generate-containers.sh"
mapfile -t managed < <(docker ps --filter "label=so1.p2.project=202300476" --format '{{.Names}}\t{{.Image}}\t{{.Status}}')
printf '  %-32s %-28s %s\n' "NOMBRE" "IMAGEN" "ESTADO"
for c in "${managed[@]}"; do printf '  %s\n' "$c"; done

# ---- 3. Dashboard ----------------------------------------------------------
GRAFANA_PORT="${GRAFANA_PORT:-3000}"
log "3/4 · dashboard en http://localhost:${GRAFANA_PORT} (usuario/pass: admin/admin)"

# ---- 4. Daemon (opcional) --------------------------------------------------
if [ "$RUN_DAEMON" -eq 1 ]; then
  if [ "$DEGRADED" -eq 0 ] && [ "$(id -u)" -ne 0 ]; then
    die "el modo --daemon requiere root (módulo + eBPF). Use --daemon --degraded si solo quiere probar el loop."
  fi
  [ "$DEGRADED" -eq 1 ] && export SO1_P2_EBPF_MODE="${SO1_P2_EBPF_MODE:-auto}"
  log "4/4 · arrancando daemon (Ctrl+C para detener)..."
  if [ -x "${ROOT}/bin/so1-p2-daemon" ]; then
    cd "${ROOT}" && exec "${ROOT}/bin/so1-p2-daemon"
  else
    die "daemon no compilado: ejecute make build primero"
  fi
else
  log "4/4 · daemon omitido (use --daemon o --daemon --degraded)"
fi