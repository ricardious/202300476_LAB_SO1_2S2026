#!/usr/bin/env bash
#
# remove-cron.sh
#
# Retira de forma idempotente la entrada de cron del proyecto (delimitada
# por marcadores) sin tocar ninguna otra entrada del usuario.
#
# Idempotente: si la sección no existe, simplemente no hay nada que quitar
# y el script termina con éxito.

set -euo pipefail

readonly MARKER_BEGIN="# >>> so1-p2-202300476:begin >>>"
readonly MARKER_END="# <<< so1-p2-202300476:end <<<"

log() {
  printf '[remove-cron] %s\n' "$*" >&2
}

die() {
  log "ERROR: $*"
  exit 1
}

remove_cron() {
  local tmp_backup tmp_new
  tmp_backup="$(mktemp)"
  tmp_new="$(mktemp)"

  if crontab -l >"${tmp_backup}" 2>/dev/null; then
    : # hay crontab previo
  else
    # No existe crontab: nada que quitar.
    rm -f "${tmp_backup}" "${tmp_new}"
    log "no crontab present; nothing to remove"
    return 0
  fi

  # Elimina la sección delimitada completa (si existe).
  awk -v begin="${MARKER_BEGIN}" -v end="${MARKER_END}" '
    $0 == begin { skip = 1; next }
    $0 == end   { skip = 0; next }
    !skip       { print }
  ' "${tmp_backup}" > "${tmp_new}"

  if ! crontab "${tmp_new}"; then
    die "failed to update crontab"
  fi

  rm -f "${tmp_backup}" "${tmp_new}"
  log "removed project cron entry (other entries untouched)"
}

remove_cron