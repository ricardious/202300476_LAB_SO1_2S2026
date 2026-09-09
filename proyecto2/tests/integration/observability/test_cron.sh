#!/usr/bin/env bash
#
# test_cron.sh — idempotencia de install-cron.sh / remove-cron.sh con un
# `crontab` simulado (PATH). Incluido por run.sh; también puede ejecutarse
# solo sin necesidad de cron real.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SCRIPTS_DIR="${ROOT}/scripts"

PASS=0
FAIL=0

ok() { PASS=$((PASS + 1)); printf '  ok: %s\n' "$*"; }
ko() { FAIL=$((FAIL + 1)); printf '  FAIL: %s\n' "$*"; }

MOCK_BIN="$(mktemp -d)"
MOCK_CRON="$(mktemp)"
trap 'rm -rf "$MOCK_BIN" "$MOCK_CRON"' EXIT

cat > "${MOCK_BIN}/crontab" <<'MOCK'
#!/usr/bin/env bash
CRONTAB_FILE="${MOCK_CRONTAB_FILE:?}"
if [[ "$1" == "-l" ]]; then
  if [[ -f "$CRONTAB_FILE" ]]; then cat "$CRONTAB_FILE"; else exit 1; fi
else
  cp "$1" "$CRONTAB_FILE"
fi
MOCK
chmod +x "${MOCK_BIN}/crontab"
export MOCK_CRONTAB_FILE="${MOCK_CRON}"
rm -f "$MOCK_CRON"

MARKER_BEGIN="# >>> so1-p2-202300476:begin >>>"
USER_ENTRY='*/5 * * * * /usr/bin/touch /tmp/user-kept.txt'

if PATH="${MOCK_BIN}:${PATH}" "${SCRIPTS_DIR}/install-cron.sh" >/dev/null 2>&1; then
  b=$(grep -c "$MARKER_BEGIN" "$MOCK_CRON" || true)
  [[ "$b" -eq 1 ]] && ok "instalación inicial: 1 sección" || ko "instalación inicial: $b secciones"
else
  ko "install-cron.sh falló"
fi

PATH="${MOCK_BIN}:${PATH}" "${SCRIPTS_DIR}/install-cron.sh" >/dev/null 2>&1 || true
b=$(grep -c "$MARKER_BEGIN" "$MOCK_CRON" || true)
[[ "$b" -eq 1 ]] && ok "reinstalación idempotente" || ko "reinstalación duplicó ($b)"

printf '%s\n' "$USER_ENTRY" > "$MOCK_CRON"
PATH="${MOCK_BIN}:${PATH}" "${SCRIPTS_DIR}/install-cron.sh" >/dev/null 2>&1 || true
grep -Fq "$USER_ENTRY" "$MOCK_CRON" && ok "instalación preserva entrada ajena" || ko "perdió entrada ajena"

PATH="${MOCK_BIN}:${PATH}" "${SCRIPTS_DIR}/remove-cron.sh" >/dev/null 2>&1 || true
b=$(grep -c "$MARKER_BEGIN" "$MOCK_CRON" || true)
[[ "$b" -eq 0 ]] && ok "remove retira sección del proyecto" || ko "remove no retiró sección"
grep -Fq "$USER_ENTRY" "$MOCK_CRON" && ok "remove conserva entrada ajena" || ko "remove borró entrada ajena"

PATH="${MOCK_BIN}:${PATH}" "${SCRIPTS_DIR}/remove-cron.sh" >/dev/null 2>&1 \
  && ok "remove sin sección es no-op" || ko "remove sin sección falló"

printf '  resultados: %d ok, %d fail\n' "$PASS" "$FAIL"
[[ "$FAIL" -eq 0 ]]