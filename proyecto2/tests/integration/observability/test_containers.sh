#!/usr/bin/env bash
#
# test_containers.sh — perfil de carga y generate-containers.sh.
# Incluido por run.sh; también puede ejecutarse solo. Requiere Docker.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SCRIPTS_DIR="${ROOT}/scripts"
WORKLOADS_DIR="${ROOT}/workloads"

PASS=0
FAIL=0

ok() { PASS=$((PASS + 1)); printf '  ok: %s\n' "$*"; }
ko() { FAIL=$((FAIL + 1)); printf '  FAIL: %s\n' "$*"; }

docker info >/dev/null 2>&1 || { echo "docker daemon no disponible"; exit 1; }

for img in low memory-high cpu-high intruder; do
  docker build -t "so1/load:test-${img}" "${WORKLOADS_DIR}/${img}" >/dev/null 2>&1 && ok "build ${img}" || ko "build ${img}"
done

SO1_SEED=13 "${SCRIPTS_DIR}/generate-containers.sh" >/dev/null 2>&1 && ok "generate-containers.sh" || ko "generate-containers.sh"

mapfile -t managed < <(docker ps -q --filter "label=so1.p2.project=202300476")
[[ "${#managed[@]}" -eq 5 ]] && ok "exactamente 5 contenedores" || ko "${#managed[@]} contenedores (esperado 5)"

profile_ok=1
for c in "${managed[@]}"; do
  profile=$(docker inspect "$c" --format '{{index .Config.Labels "so1.p2.profile"}}')
  case "$profile" in memory-high|cpu-high|low|intruder) ;; *) profile_ok=0 ;; esac
done
[[ "$profile_ok" -eq 1 ]] && ok "perfiles válidos" || ko "perfil inválido detectado"

docker rm -f "${managed[@]}" >/dev/null 2>&1 || true
printf '  resultados: %d ok, %d fail\n' "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]