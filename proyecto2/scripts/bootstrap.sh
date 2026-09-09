#!/usr/bin/env bash
# ============================================================================
# bootstrap.sh — Preparación reproducible del Proyecto 2 de SO1 (202300476)
#
# Idempotente: puede ejecutarse cuantas veces se quiera. No borra recursos
# ajenos; solo construye/levanta los recursos del proyecto.
#
# Pasos (cada uno es opcional según flags):
#   1. Verifica prerrequisitos (docker, go, bash).
#   2. Genera .env desde .env.example si no existe.
#   3. Compila el daemon Go (make build).
#   4. Compila el objeto eBPF (make ebpf) y regenera bindings si hace falta.
#   5. Levanta Valkey + Grafana vía Compose (make up).
#   6. (Opcional --with-module) Compila y carga el módulo de kernel.
#   7. (Opcional --with-cron) Instala el cronjob del proyecto.
#
# Uso:
#   bash proyecto2/scripts/bootstrap.sh                # pasos 1-5
#   bash proyecto2/scripts/bootstrap.sh --with-module  # + módulo (root/sudo)
#   bash proyecto2/scripts/bootstrap.sh --with-cron    # + cron
#   bash proyecto2/scripts/bootstrap.sh --skip-docker  # sin Docker (CI)
# ============================================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WITH_MODULE=0
WITH_CRON=0
SKIP_DOCKER=0

for arg in "$@"; do
  case "$arg" in
    --with-module) WITH_MODULE=1 ;;
    --with-cron) WITH_CRON=1 ;;
    --skip-docker) SKIP_DOCKER=1 ;;
    *) echo "opción desconocida: $arg" >&2; exit 2 ;;
  esac
done

log() { printf '[bootstrap] %s\n' "$*"; }
die() { log "ERROR: $*"; exit 1; }

# ---- 1. Prerrequisitos -----------------------------------------------------
command -v bash >/dev/null 2>&1 || die "bash no está en PATH"
command -v make >/dev/null 2>&1 || die "make no está en PATH"
command -v go >/dev/null 2>&1 || die "go no está en PATH (instale Go >= 1.25)"
if [ "$SKIP_DOCKER" -eq 0 ]; then
  command -v docker >/dev/null 2>&1 || die "docker no está en PATH"
  docker info >/dev/null 2>&1 || die "el daemon de Docker no responde"
fi
log "prerrequisitos OK"

# ---- 2. .env desde .env.example -------------------------------------------
ENV_FILE="${ROOT}/.env"
ENV_EXAMPLE="${ROOT}/.env.example"
if [ ! -f "$ENV_FILE" ] && [ -f "$ENV_EXAMPLE" ]; then
  cp "$ENV_EXAMPLE" "$ENV_FILE"
  log "generado .env desde .env.example (edítelo si necesita cambios)"
else
  log ".env ya existe (o no hay .env.example); no se toca"
fi

# ---- 3. Compilar el daemon -------------------------------------------------
log "compilando daemon Go..."
make -C "$ROOT" build

# ---- 4. Compilar el frente eBPF -------------------------------------------
log "compilando objeto eBPF..."
make -C "$ROOT" ebpf

# ---- 5. Levantar Valkey + Grafana ------------------------------------------
if [ "$SKIP_DOCKER" -eq 0 ]; then
  log "levantando Valkey + Grafana (Compose)..."
  make -C "$ROOT" up
else
  log "saltando Docker/Compose (--skip-docker)"
fi

# ---- 6. Módulo de kernel (opcional) ----------------------------------------
if [ "$WITH_MODULE" -eq 1 ]; then
  log "compilando y cargando el módulo de kernel (requiere root/sudo)..."
  bash "${ROOT}/scripts/load-module.sh"
else
  log "módulo de kernel omitido (use --with-module para cargarlo)"
fi

# ---- 7. Cronjob (opcional) --------------------------------------------------
if [ "$WITH_CRON" -eq 1 ]; then
  log "instalando cronjob del proyecto (2 min)..."
  bash "${ROOT}/scripts/install-cron.sh"
else
  log "cronjob omitido (use --with-cron para instalarlo)"
fi

log "bootstrap completo."
log "  - Dashboard Grafana: http://localhost:${GRAFANA_PORT:-3000} (admin/admin)"
log "  - Próximo paso recomendado: make doctor   o   make demo"