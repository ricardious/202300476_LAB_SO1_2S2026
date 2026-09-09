#!/usr/bin/env bash
#
# install-cron.sh
#
# Instala de forma idempotente la entrada de cron del proyecto que ejecuta
# scripts/generate-containers.sh cada dos minutos.
#
# La entrada está delimitada por marcadores, de modo que:
#   - Re-ejecutar este script no duplica la entrada (se reemplaza la
#     sección delimitada existente).
#   - remove-cron.sh retira ÚNICAMENTE la sección delimitada y jamás toca
#     entradas ajenas del usuario.
#
# La línea creada apunta al script con rutas absolutas para que cron la
# pueda ejecutar sin depender del PATH del shell.

set -euo pipefail

SCRIPT_SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/generate-containers.sh"

# Marcadores (idempotencia y delimitación). Deben coincidir con los del daemon
# Go (daemon/internal/cron) para que ambas vías operen sobre la misma entrada.
readonly MARKER_BEGIN="# >>> so1-p2-202300476:begin >>>"
readonly MARKER_END="# <<< so1-p2-202300476:end <<<"

# Ruta absoluta del generador instalado. El daemon Go usa exactamente esta
# misma ruta en su entrada de cron (SO1_P2_CRON_COMMAND).
readonly INSTALL_PREFIX="${SO1_P2_INSTALL_PREFIX:-/usr/local/bin}"

# Comando que ejecuta cron. Redirige salida a un log del proyecto.
readonly LOG_FILE="${SO1_P2_LOG_DIR:-${HOME}/.so1-p2/logs}/generate-containers.log"
readonly CRON_LINE="*/2 * * * * ${INSTALL_PREFIX}/so1-p2-generate-containers.sh >> ${LOG_FILE} 2>&1"

log() {
  printf '[install-cron] %s\n' "$*" >&2
}

die() {
  log "ERROR: $*"
  exit 1
}

# Instala (o refresca) el generador en INSTALL_PREFIX para que cron y el daemon
# Go usen la misma ruta absoluta. Best-effort: si no hay permisos, se advierte
# y la instalación de la entrada continúa (make install la resuelve con sudo).
install_generator() {
  local dest="${INSTALL_PREFIX}/so1-p2-generate-containers.sh"
  if [ -x "${dest}" ] && [ "$(readlink -f "${dest}")" = "$(readlink -f "${SCRIPT_SRC}")" ]; then
    return 0
  fi
  log "installing generator to ${dest} (idempotente)..."
  if ! install -m 0755 "${SCRIPT_SRC}" "${dest}" 2>/dev/null; then
    log "no se pudo instalar en ${dest}; hágalo con: sudo make install (continuando)"
  fi
}

backup_crontab() {
  local tmp
  tmp="$(mktemp)"
  if crontab -l >"${tmp}" 2>/dev/null; then
    : # hay crontab previo
  else
    # Sin crontab previo: el temporal queda vacío.
    : >"${tmp}"
  fi
  printf '%s\n' "${tmp}"
}

replace_delimited_entry() {
  local input="$1"
  local output="$2"
  # Elimina la sección delimitada previa (si existe) y agrega la nueva.
  awk -v begin="${MARKER_BEGIN}" -v end="${MARKER_END}" -v line="${CRON_LINE}" '
    $0 == begin { skip = 1; next }
    $0 == end   { skip = 0; next }
    !skip       { print }
    END {
      print begin
      print line
      print end
    }
  ' "${input}" > "${output}"
}

install_cron() {
  install_generator

  # Log dir best-effort (en HOME no requiere root).
  mkdir -p "$(dirname "${LOG_FILE}")" 2>/dev/null || \
    log "no se pudo crear $(dirname "${LOG_FILE}"); el cron logueará a donde pueda"

  local tmp_backup tmp_new
  tmp_backup="$(backup_crontab)"
  tmp_new="$(mktemp)"

  replace_delimited_entry "${tmp_backup}" "${tmp_new}"

  # Validación mínima: la nueva entrada debe tener exactamente una sección.
  local begin_count end_count
  begin_count="$(grep -c "${MARKER_BEGIN}" "${tmp_new}" || true)"
  end_count="$(grep -c "${MARKER_END}" "${tmp_new}" || true)"
  if [[ "${begin_count}" -ne 1 || "${end_count}" -ne 1 ]]; then
    die "delimited section malformed (begin=${begin_count}, end=${end_count})"
  fi

  if ! crontab "${tmp_new}"; then
    die "failed to install crontab"
  fi

  rm -f "${tmp_backup}" "${tmp_new}"
  log "installed cron entry every 2 minutes (${INSTALL_PREFIX}/so1-p2-generate-containers.sh)"
}

install_cron