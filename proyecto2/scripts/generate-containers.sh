#!/usr/bin/env bash
#
# generate-containers.sh
#
# Genera 5 contenedores de perfil aleatorio (memory-high, cpu-high, low,
# intruder) cada dos minutos, con límites de recursos, etiquetas y nombres
# únicos. Está pensado para ser invocado por el cronjob del proyecto, pero
# puede ejecutarse a mano.
#
# Propiedades:
#   - Selecciona EXACTAMENTE 5 perfiles por invocación (aleatorio con
#     reemplazo, semilla fija configurable vía SO1_SEED).
#   - Aplica límites de recursos por perfil (memoria + cpus).
#   - Etiqueta cada contenedor para que el daemon lo identifique como
#     gestionado y para que jamás sea un candidato protegido.
#   - Usa nombres únicos con timestamp + PID + índice.
#   - Es seguro ante ejecuciones simultáneas (bloqueo con mkdir).
#   - Idempotente en el sentido de que no falla si algún contenedor
#     ya existe (los nombres son únicos, por lo que nunca colisiona).

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuración
# ---------------------------------------------------------------------------
readonly IMAGE_TAG="so1/load:202300476"
readonly PROFILE_LABEL="so1.p2.profile"
readonly MANAGED_LABEL="so1.p2.managed=true"
readonly PROJECT_LABEL="so1.p2.project=202300476"
readonly DEFAULT_IMAGE_PREFIX="so1-load-"

# Número de contenedores a crear por invocación (fijado en 5 por el plan).
readonly COUNT="${SO1_CONTAINERS_COUNT:-5}"

# Duración nominal de cada workload (minutos).
readonly RUN_MINUTES="${SO1_RUN_MINUTES:-10}"

# Perfiles disponibles con sus límites por defecto.
declare -A MEM_LIMIT=(
  [memory-high]="768m"
  [cpu-high]="256m"
  [low]="128m"
  [intruder]="128m"
)
declare -A CPU_LIMIT=(
  [memory-high]="0.5"
  [cpu-high]="1.0"
  [low]="0.25"
  [intruder]="0.25"
)
declare -A IMAGE_NAME=(
  [memory-high]="memory-high"
  [cpu-high]="cpu-high"
  [low]="low"
  [intruder]="intruder"
)

PROFILES=(memory-high cpu-high low intruder)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
log() {
  printf '[generate-containers] %s\n' "$*" >&2
}

# ---------------------------------------------------------------------------
# Resolución robusta del directorio del proyecto.
#
# El script puede invocarse:
#   1) desde el repositorio (scripts/generate-containers.sh),
#   2) desde una copia/enlace instalado en /usr/local/bin (cron/daemon).
#
# Orden de resolución:
#   - SO1_P2_PROJECT_DIR (explícito, recomendado para cron/instalación).
#   - El directorio padre del script REAL (readlink -f) si contiene workloads/.
#   - El directorio de trabajo actual si contiene workloads/.
# ---------------------------------------------------------------------------
resolve_project_root() {
  if [ -n "${SO1_P2_PROJECT_DIR:-}" ] && [ -d "${SO1_P2_PROJECT_DIR}/workloads" ]; then
    printf '%s\n' "${SO1_P2_PROJECT_DIR}"
    return 0
  fi
  local real_dir real
  real="$(readlink -f "${BASH_SOURCE[0]}" 2>/dev/null || true)"
  if [ -n "${real}" ]; then
    real_dir="$(dirname "${real}")"
    if [ -d "${real_dir}/../workloads" ]; then
      printf '%s\n' "$(cd "${real_dir}/.." && pwd)"
      return 0
    fi
  fi
  if [ -d "$(pwd)/workloads" ]; then
    printf '%s\n' "$(pwd)"
    return 0
  fi
  return 1
}

die() {
  log "ERROR: $*"
  exit 1
}

# Bloqueo simple para evitar corridas simultáneas.
LOCKDIR="${TMPDIR:-/tmp}/so1-p2-generate.lock"
acquire_lock() {
  if ! mkdir "${LOCKDIR}" 2>/dev/null; then
    die "another instance is already running (${LOCKDIR} exists)"
  fi
  trap 'rmdir "${LOCKDIR}" 2>/dev/null || true' EXIT
}

# ---------------------------------------------------------------------------
# Validación de prerrequisitos
# ---------------------------------------------------------------------------
require_docker() {
  command -v docker >/dev/null 2>&1 || die "docker not found in PATH"
  docker info >/dev/null 2>&1 || die "docker daemon is not reachable"
}

ensure_image() {
  local profile="$1"
  local image="${IMAGE_NAME[$profile]}"
  if ! docker image inspect "${IMAGE_TAG}-${image}" >/dev/null 2>&1; then
    log "image ${IMAGE_TAG}-${image} not found; building it..."
    docker build -t "${IMAGE_TAG}-${image}" \
      "${PROJECT_ROOT}/workloads/${image}" >/dev/null
  fi
}

# Selecciona COUNT perfiles al azar con reemplazo.
pick_profiles() {
  local seed="${SO1_SEED:-}"
  if [[ -n "${seed}" ]]; then
    # Semilla determinista para pruebas.
    RANDOM="${seed}"
  else
    RANDOM="${RANDOM}"
  fi
  local i
  for ((i = 0; i < COUNT; i++)); do
    printf '%s\n' "${PROFILES[$((RANDOM % ${#PROFILES[@]}))]}"
  done
}

# ---------------------------------------------------------------------------
# Construcción de un contenedor
# ---------------------------------------------------------------------------
launch_one() {
  local profile="$1"
  local index="$2"
  local timestamp
  timestamp="$(date +%s)"
  local name="${DEFAULT_IMAGE_PREFIX}${profile}-${timestamp}-$$-${index}"
  local image="${IMAGE_TAG}-${IMAGE_NAME[$profile]}"
  local mem="${MEM_LIMIT[$profile]}"
  local cpus="${CPU_LIMIT[$profile]}"

  log "launching ${name} (profile=${profile}, mem=${mem}, cpus=${cpus})"

  docker run -d \
    --name "${name}" \
    --memory "${mem}" \
    --cpus "${cpus}" \
    --label "${MANAGED_LABEL}" \
    --label "${PROJECT_LABEL}" \
    --label "${PROFILE_LABEL}=${profile}" \
    --env "SO1_RUN_MINUTES=${RUN_MINUTES}" \
    --env "SO1_PROFILE=${profile}" \
    "${image}"
}

# ---------------------------------------------------------------------------
# main
# ---------------------------------------------------------------------------
main() {
  acquire_lock
  require_docker

  if ! PROJECT_ROOT="$(resolve_project_root)"; then
    die "no se pudo ubicar el directorio del proyecto (workloads/). Use SO1_P2_PROJECT_DIR."
  fi
  readonly PROJECT_ROOT
  log "project root: ${PROJECT_ROOT}"

  local i=0
  while IFS= read -r profile; do
    [[ -n "${profile}" ]] || continue
    ensure_image "${profile}"
    launch_one "${profile}" "${i}"
    i=$((i + 1))
  done < <(pick_profiles)

  log "created ${i} container(s)"
}

main "$@"
