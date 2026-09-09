#!/usr/bin/env bash
#
# run.sh — suite de integración del frente de observabilidad (rama 1C).
#
# Verifica, en orden:
#   1. Sintaxis de todos los scripts de proyecto2/scripts y de esta suite.
#   2. Comportamiento real de generate-containers.sh contra un Docker daemon
#      (5 contenedores, límites, etiquetas, nombres únicos) y de los perfiles
#      de carga (low, memory-high, cpu-high, intruder).
#   3. Idempotencia de install-cron.sh / remove-cron.sh usando un `crontab`
#      simulado (el real no siempre está instalado en la VM de desarrollo).
#   4. Validez de docker-compose.yml, provisioning YAML y dashboard JSON,
#      incluida la presencia de todos los paneles de la rúbrica y de las
#      claves con prefijo so1:p2:.
#
# Uso:
#   ./run.sh                # todo
#   SO1_SKIP_DOCKER=1 ./run.sh   # omite las pruebas que requieren Docker
#
# Salida: 0 si todo pasa, 1 si alguna prueba falla.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SCRIPTS_DIR="${ROOT}/scripts"
WORKLOADS_DIR="${ROOT}/workloads"
DEPLOY_DIR="${ROOT}/deploy"
DASHBOARD="${DEPLOY_DIR}/grafana/dashboards/so1-p2-overview.json"

PASS=0
FAIL=0
FAILED_TESTS=()

note()  { printf '\n== %s ==\n' "$*"; }
ok()    { PASS=$((PASS + 1)); printf '  ok: %s\n' "$*"; }
ko()    { FAIL=$((FAIL + 1)); FAILED_TESTS+=("$*"); printf '  FAIL: %s\n' "$*"; }

assert() { # desc, cond
  if [[ "$2" -eq 0 ]]; then ok "$1"; else ko "$1"; fi
}

# ---------------------------------------------------------------------------
# 1. Sintaxis shell
# ---------------------------------------------------------------------------
note "Sintaxis shell"
for f in "${SCRIPTS_DIR}"/*.sh "${ROOT}"/tests/integration/observability/test_*.sh; do
  if bash -n "$f" 2>/dev/null; then
    ok "bash -n ${f}"
  else
    ko "bash -n ${f}"
  fi
done

# ---------------------------------------------------------------------------
# 2. Contenedores de carga (requiere Docker)
# ---------------------------------------------------------------------------
if [[ "${SO1_SKIP_DOCKER:-0}" != "1" ]] && docker info >/dev/null 2>&1; then
  note "Imágenes de carga"
  for img in low memory-high cpu-high intruder; do
    if docker build -t "so1/load:test-${img}" "${WORKLOADS_DIR}/${img}" >/dev/null 2>&1; then
      ok "build ${img}"
    else
      ko "build ${img}"
    fi
  done

  note "generate-containers.sh (5 contenedores, etiquetas, límites)"
  SO1_SEED=7 SO1_CONTAINERS_COUNT=5 "${SCRIPTS_DIR}/generate-containers.sh" >/dev/null 2>&1 \
    && ok "generate-containers.sh con semilla fija" \
    || ko "generate-containers.sh con semilla fija"

  mapfile -t managed < <(docker ps -q --filter "label=so1.p2.project=202300476")
  if [[ "${#managed[@]}" -eq 5 ]]; then ok "creó exactamente 5 contenedores"; else ko "creó ${#managed[@]} contenedores (esperado 5)"; fi

  names=$(docker ps --format '{{.Names}}' --filter "label=so1.p2.project=202300476")
  if [[ $(printf '%s\n' "$names" | wc -l) -eq $(printf '%s\n' "$names" | sort -u | wc -l) ]]; then
    ok "nombres únicos"
  else
    ko "nombres no únicos"
  fi

  label_ok=1
  for c in "${managed[@]}"; do
    l=$(docker inspect "$c" --format '{{json .Config.Labels}}' 2>/dev/null || true)
    if [[ "$l" != *'"so1.p2.project":"202300476"'* || "$l" != *'"so1.p2.managed":"true"'* || "$l" != *'"so1.p2.profile":'* ]]; then
      label_ok=0
    fi
  done
  [[ "$label_ok" -eq 1 ]] && ok "etiquetas so1.p2.* en todos los contenedores" || ko "etiquetas faltantes"

  mem_ok=1
  for c in "${managed[@]}"; do
    profile=$(docker inspect "$c" --format '{{index .Config.Labels "so1.p2.profile"}}')
    mem=$(docker inspect "$c" --format '{{.HostConfig.Memory}}')
    case "$profile" in
      memory-high) [[ "$mem" -eq $((768 * 1024 * 1024)) ]] || mem_ok=0 ;;
      cpu-high)    [[ "$mem" -eq $((256 * 1024 * 1024)) ]] || mem_ok=0 ;;
      low|intruder) [[ "$mem" -eq $((128 * 1024 * 1024)) ]] || mem_ok=0 ;;
    esac
  done
  [[ "$mem_ok" -eq 1 ]] && ok "límites de memoria por perfil" || ko "límites de memoria incorrectos"

  # Bloqueo anti-corridas simultáneas
  LOCKDIR="${TMPDIR:-/tmp}/so1-p2-generate.lock"
  mkdir -p "$LOCKDIR"
  if "${SCRIPTS_DIR}/generate-containers.sh" >/dev/null 2>&1; then
    ko "permitió corrida simultánea (debería fallar con lock presente)"
  else
    ok "bloquea ejecución simultánea"
  fi
  rmdir "$LOCKDIR" 2>/dev/null || true

  # Limpieza de los contenedores de prueba
  docker rm -f "${managed[@]}" >/dev/null 2>&1 || true
  ok "limpieza de contenedores de prueba"
else
  note "Contenedores de carga (omito por SO1_SKIP_DOCKER=1 o sin daemon)"
fi

# ---------------------------------------------------------------------------
# 3. Idempotencia de cron (con crontab simulado)
# ---------------------------------------------------------------------------
note "install-cron.sh / remove-cron.sh (idempotencia)"
MOCK_BIN="$(mktemp -d)"
MOCK_CRON="$(mktemp)"
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

# Caso A: instalación sobre crontab vacío
if PATH="${MOCK_BIN}:${PATH}" "${SCRIPTS_DIR}/install-cron.sh" >/dev/null 2>&1; then
  b=$(grep -c "$MARKER_BEGIN" "$MOCK_CRON" || true)
  [[ "$b" -eq 1 ]] && ok "instalación inicial: una sola sección" || ko "instalación inicial: $b secciones begin"
  grep -q '\*/2 \* \* \* \*' "$MOCK_CRON" && ok "línea cron cada 2 minutos" || ko "línea cron cada 2 minutos ausente"
else
  ko "install-cron.sh falló sobre crontab vacío"
fi

# Caso B: reinstalación no duplica
PATH="${MOCK_BIN}:${PATH}" "${SCRIPTS_DIR}/install-cron.sh" >/dev/null 2>&1 || true
b=$(grep -c "$MARKER_BEGIN" "$MOCK_CRON" || true)
[[ "$b" -eq 1 ]] && ok "reinstalación idempotente (1 sección)" || ko "reinstalación duplicó a $b secciones"

# Caso C: entrada ajena preservada
printf '%s\n' "$USER_ENTRY" > "$MOCK_CRON"
PATH="${MOCK_BIN}:${PATH}" "${SCRIPTS_DIR}/install-cron.sh" >/dev/null 2>&1 || true
grep -Fq "$USER_ENTRY" "$MOCK_CRON" && ok "preserva entradas ajenas tras instalar" || ko "perdió entrada ajena al instalar"

# Caso D: remove deja solo la entrada ajena
PATH="${MOCK_BIN}:${PATH}" "${SCRIPTS_DIR}/remove-cron.sh" >/dev/null 2>&1 || true
b=$(grep -c "$MARKER_BEGIN" "$MOCK_CRON" || true)
[[ "$b" -eq 0 ]] && ok "remove retira la sección del proyecto" || ko "remove no retiró la sección"
grep -Fq "$USER_ENTRY" "$MOCK_CRON" && ok "remove conserva entradas ajenas" || ko "remove borró entrada ajena"

# Caso E: remove sobre crontab sin sección es no-op (idempotente)
PATH="${MOCK_BIN}:${PATH}" "${SCRIPTS_DIR}/remove-cron.sh" >/dev/null 2>&1 \
  && ok "remove sin sección es no-op (código 0)" \
  || ko "remove sin sección falló"

rm -rf "$MOCK_BIN" "$MOCK_CRON"

# ---------------------------------------------------------------------------
# 4. Deploy: compose, provisioning y dashboard
# ---------------------------------------------------------------------------
note "docker-compose.yml"
if docker compose -f "${DEPLOY_DIR}/docker-compose.yml" config --quiet 2>/dev/null; then
  ok "compose config válido"
else
  ko "compose config inválido"
fi
if grep -q 'valkey/valkey:9.1.2-alpine' "${DEPLOY_DIR}/docker-compose.yml"; then
  ok "versión Valkey fijada"
else
  ko "versión Valkey no fijada"
fi

note "Dashboard JSON y claves so1:p2:"
if jq -e . "$DASHBOARD" >/dev/null 2>&1; then ok "dashboard es JSON válido"; else ko "dashboard JSON inválido"; fi

REQUIRED_KEYS=(system:latest system:timeline containers:timeline rank:ram rank:cpu kills kills:count)
for k in "${REQUIRED_KEYS[@]}"; do
  if jq -e --arg k "so1:p2:${k}" '.. | strings | select(. == $k)' "$DASHBOARD" >/dev/null 2>&1; then
    ok "clave so1:p2:${k} presente"
  else
    ko "clave so1:p2:${k} ausente"
  fi
done

REQUIRED_PANELS=("RAM total" "RAM usada" "RAM libre" "Evolución temporal de RAM" "Eliminaciones confirmadas (XLEN)" "Eliminaciones confirmadas a lo largo del tiempo" "Top 5 histórico por RAM (PID e ID)" "Top 5 histórico por CPU (PID e ID)" "Tabla de eventos kill capturados por eBPF")
for p in "${REQUIRED_PANELS[@]}"; do
  if jq -e --arg p "$p" '[.panels[]?.title] | index($p) != null' "$DASHBOARD" >/dev/null 2>&1; then
    ok "panel: $p"
  else
    ko "panel faltante: $p"
  fi
done

# El datasource provisionado apunta a la misma uid que usan los paneles.
DS_UID="$(grep -A1 'name: Valkey' "${DEPLOY_DIR}/grafana/provisioning/datasources/datasources.yaml" | grep -o 'uid: .*' | awk '{print $2}')"
if [[ "$DS_UID" == "so1-valkey" ]] && jq -e '.. | select(.uid? == "so1-valkey")' "$DASHBOARD" >/dev/null 2>&1; then
  ok "datasource Valkey provisionado y referenciado por los paneles"
else
  ko "datasource Valkey no alineado entre provisioning y dashboard"
fi

# ---------------------------------------------------------------------------
# 5. Smoke REAL del stack (opt-in: SO1_RUN_STACK=1)
# ---------------------------------------------------------------------------
if [[ "${SO1_RUN_STACK:-0}" == "1" ]]; then
  note "Smoke del stack Valkey + Grafana (opt-in)"
  "${ROOT}/tests/integration/observability/test_stack.sh" \
    || { ko "smoke del stack"; }
else
  note "Smoke del stack (omitido; use SO1_RUN_STACK=1 para levantar Valkey+Grafana)"
fi

# ---------------------------------------------------------------------------
# Resumen
# ---------------------------------------------------------------------------
note "RESULTADO"
printf '  pruebas OK: %d   pruebas fallidas: %d\n' "$PASS" "$FAIL"
if [[ "$FAIL" -gt 0 ]]; then
  printf '  fallaron:\n'
  for t in "${FAILED_TESTS[@]}"; do printf '    - %s\n' "$t"; done
  exit 1
fi
exit 0