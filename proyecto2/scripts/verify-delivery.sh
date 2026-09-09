#!/usr/bin/env bash
# ============================================================================
# verify-delivery.sh — Preentrega estricta del Proyecto 2 SO1 (202300476)
#
# Verifica que el árbol de proyecto2/ esté listo para entregar, ejecutando
# (sin privilegios salvo indicación) los mismos checks que el estudiante
# repetirá en la VM:
#
#   1. Prerrequisitos (herramientas) y archivos requeridos.
#   2. Sintaxis shell de scripts y tests.
#   3. JSON/YAML válidos (dashboard, fixtures, provisioning, Compose).
#   4. Go: build, vet, gofmt, test y -race.
#   5. Tests de kernel NO privilegiados (test-all.sh; los privilegiados se
#      omiten solos con [SKIP] cuando no hay root/headers).
#   6. Observabilidad: tests/integration/observability/run.sh (hermético;
#      solo recursos etiquetados so1.p2.* del proyecto).
#   7. Compose config + estado operativo (doctor.sh).
#   8. Ausencia de secretos y artefactos accidentales (versión git + disco).
#
# Salida:
#   - Código 0  si no hay fallos reales (los [SKIP] por privilegios/entorno
#     no se consideran fallos).
#   - Código 1  si existe al menos un [FAIL] real.
#   - Código 2  si se invoca mal.
#
# Uso:
#   bash proyecto2/scripts/verify-delivery.sh                # todo
#   bash proyecto2/scripts/verify-delivery.sh --no-docker    # sin gates Docker
#   bash proyecto2/scripts/verify-delivery.sh --quick        # rápido (sin -race/smoke)
#   bash proyecto2/scripts/verify-delivery.sh --json         # resumen JSON final
# ============================================================================
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NO_DOCKER=0
QUICK=0
JSON_OUT=0

for arg in "$@"; do
  case "$arg" in
    --no-docker) NO_DOCKER=1 ;;
    --quick) QUICK=1 ;;
    --json) JSON_OUT=1 ;;
    -h|--help) sed -n '2,40p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "opción desconocida: $arg" >&2; exit 2 ;;
  esac
done

PASS=0; WARN=0; SKIP=0; FAIL=0
declare -a FAILURES=() SKIPPED=() WARNINGS=()

ok()   { PASS=$((PASS + 1)); printf '  [OK]      %s\n' "$*"; }
warn() { WARN=$((WARN + 1)); WARNINGS+=("$*"); printf '  [AVISO]   %s\n' "$*"; }
skip() { SKIP=$((SKIP + 1)); SKIPPED+=("$*"); printf '  [PEND]    %s\n' "$*"; }
fail() { FAIL=$((FAIL + 1)); FAILURES+=("$*"); printf '  [FALLO]   %s\n' "$*"; }
section() { printf '\n== %s ==\n' "$*"; }

DOCKER_OK=0
if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
  DOCKER_OK=1
fi
COMPOSE_OK=0
if [ "$DOCKER_OK" -eq 1 ] && docker compose version >/dev/null 2>&1; then
  COMPOSE_OK=1
fi

# ---------------------------------------------------------------------------
# 1. Prerrequisitos y archivos requeridos
# ---------------------------------------------------------------------------
section "Prerrequisitos"
for t in bash make go python3 git jq; do
  if command -v "$t" >/dev/null 2>&1; then ok "binario $t"; else fail "binario $t ausente"; fi
done
[ "$DOCKER_OK" -eq 1 ] && ok "daemon Docker responde" || skip "daemon Docker no responde en este host (gates Docker se omitirán)"
[ "$COMPOSE_OK" -eq 1 ] && ok "docker compose disponible" || skip "docker compose no disponible"

section "Archivos requeridos (manifest)"
REQUIRED=(
  README.md Makefile .env.example .gitignore
  kernel/continfo.c kernel/Makefile
  ebpf/kill_monitor.bpf.c ebpf/kill_monitor.h ebpf/Makefile ebpf/README.md
  daemon/go.mod daemon/go.sum daemon/cmd/daemon/main.go
  daemon/internal/config/config.go
  daemon/internal/procfs/procfs.go
  daemon/internal/docker/docker.go
  daemon/internal/policy/policy.go
  daemon/internal/valkey/valkey.go
  daemon/internal/cron/cron.go
  daemon/internal/lifecycle/lifecycle.go
  daemon/internal/ebpf/ebpf.go daemon/internal/ebpf/bpf_consumer.go daemon/internal/ebpf/killmonitor_bpf.go
  daemon/internal/ebpf/killmonitor_bpf.o
  workloads/low/Dockerfile workloads/low/low.py
  workloads/cpu-high/Dockerfile workloads/cpu-high/cpu_high.py
  workloads/memory-high/Dockerfile workloads/memory-high/memory_high.py
  workloads/intruder/Dockerfile workloads/intruder/intruder.py
  deploy/docker-compose.yml
  deploy/grafana/Dockerfile
  deploy/grafana/provisioning/datasources/datasources.yaml
  deploy/grafana/provisioning/dashboards/dashboards.yaml
  deploy/grafana/dashboards/so1-p2-overview.json
  scripts/bootstrap.sh scripts/cleanup.sh scripts/capture-evidence.sh scripts/demo.sh scripts/doctor.sh
  scripts/generate-containers.sh scripts/install-cron.sh scripts/load-module.sh
  scripts/remove-cron.sh scripts/unload-module.sh scripts/verify-delivery.sh
  tests/kernel/test-all.sh tests/kernel/test-build.sh tests/kernel/test-module-lifecycle.sh
  tests/kernel/test-schema.sh tests/kernel/validate_schema.py tests/kernel/fixtures/sample-snapshot.json
  tests/integration/observability/run.sh tests/integration/observability/test_containers.sh
  tests/integration/observability/test_cron.sh tests/integration/observability/test_deploy.sh
  tests/integration/observability/test_stack.sh
  docs/GUIA_INSTALACION.md docs/MANUAL_TECNICO.md
  docs/evidencias/README.md
)
MISSING=0
for f in "${REQUIRED[@]}"; do
  if [ -s "$ROOT/$f" ]; then :; else
    MISSING=1
    fail "falta archivo requerido: $f"
  fi
done
[ "$MISSING" -eq 0 ] && ok "manifest completo (${#REQUIRED[@]} archivos)"

section "Carpetas de evidencias por rubro"
for d in entorno kernel daemon-politica cron-workloads valkey-grafana ebpf; do
  if [ -d "$ROOT/docs/evidencias/$d" ] && [ -f "$ROOT/docs/evidencias/$d/.gitkeep" ]; then
    ok "docs/evidencias/$d/.gitkeep"
  else
    fail "docs/evidencias/$d/.gitkeep ausente"
  fi
done

# ---------------------------------------------------------------------------
# 2. Sintaxis shell
# ---------------------------------------------------------------------------
section "Sintaxis shell (bash -n)"
SYNTAX_FAIL=0
while IFS= read -r f; do
  if bash -n "$f" 2>/dev/null; then
    :
  else
    SYNTAX_FAIL=1
    fail "sintaxis shell: $f"
  fi
done < <(find "$ROOT/scripts" "$ROOT/tests" -type f -name '*.sh' | sort)
[ "$SYNTAX_FAIL" -eq 0 ] && ok "todos los scripts con bash -n OK"

# ---------------------------------------------------------------------------
# 3. JSON / YAML
# ---------------------------------------------------------------------------
section "JSON válido"
DASH="${ROOT}/deploy/grafana/dashboards/so1-p2-overview.json"
if command -v jq >/dev/null 2>&1; then
  if jq -e . "$DASH" >/dev/null 2>&1; then ok "dashboard JSON válido (jq)"; else fail "dashboard JSON inválido"; fi
  if jq -e . "$ROOT/tests/kernel/fixtures/sample-snapshot.json" >/dev/null 2>&1; then
    ok "fixture kernel JSON válido"
  else
    fail "fixture kernel inválido"
  fi
else
  skip "jq no disponible"
fi
if command -v python3 >/dev/null 2>&1; then
  JSON_FAIL=0
  # Los fixtures de tests/fixtures/ son casos de un PARSER tolerante:
  # valid_snapshot / unknown_fields / wrong_schema deben ser JSON válidos,
  # pero truncated_snapshot es un documento CORTADO a propósito (NO debe
  # parsear; así se prueba la robustez del parser).
  for f in valid_snapshot unknown_fields wrong_schema; do
    if python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$ROOT/tests/fixtures/$f.json" >/dev/null 2>&1; then
      :
    else
      JSON_FAIL=1
      fail "JSON inválido: tests/fixtures/$f.json (debería ser válido)"
    fi
  done
  if python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$ROOT/tests/fixtures/truncated_snapshot.json" >/dev/null 2>&1; then
    fail "truncated_snapshot.json parsea (debería estar truncado a propósito)"
  else
    ok "fixtures Go del daemon coherentes (truncado sigue roto, resto válido)"
  fi
  [ "$JSON_FAIL" -eq 0 ] && ok "fixtures Go del daemon JSON válidos"
  if python3 "$ROOT/tests/kernel/validate_schema.py" "$ROOT/tests/kernel/fixtures/sample-snapshot.json" >/dev/null 2>&1; then
    ok "schema del fixture de kernel OK"
  else
    fail "el fixture de kernel no cumple el esquema"
  fi
else
  skip "python3 no disponible"
fi

section "YAML / Compose"
if [ "$COMPOSE_OK" -eq 1 ]; then
  if docker compose -f "$ROOT/deploy/docker-compose.yml" config --quiet 2>/dev/null; then
    ok "docker compose config válido"
  else
    fail "docker compose config inválido"
  fi
else
  skip "docker compose no disponible para validar config"
fi
if command -v python3 >/dev/null 2>&1; then
  if python3 -c 'import yaml' >/dev/null 2>&1; then
    YAML_FAIL=0
    for f in "$ROOT"/deploy/grafana/provisioning/datasources/datasources.yaml \
             "$ROOT"/deploy/grafana/provisioning/dashboards/dashboards.yaml; do
      if python3 -c 'import yaml,sys; list(yaml.safe_load_all(open(sys.argv[1])))' "$f" >/dev/null 2>&1; then
        :
      else
        YAML_FAIL=1
        fail "YAML inválido: $f"
      fi
    done
    [ "$YAML_FAIL" -eq 0 ] && ok "provisioning de Grafana YAML válido"
  else
    skip "módulo python yaml no disponible"
  fi
fi

# ---------------------------------------------------------------------------
# 4. Go: build, vet, gofmt, test, race
# ---------------------------------------------------------------------------
section "Go (build / vet / gofmt / test / race)"
if command -v go >/dev/null 2>&1; then
  (
    cd "$ROOT/daemon" || exit 3
    go build ./... >/dev/null 2>&1 && echo "build OK" || echo "build FAIL"
    go vet ./... >/dev/null 2>&1 && echo "vet OK" || echo "vet FAIL"
    if [ -z "$(gofmt -l .)" ]; then echo "gofmt OK"; else echo "gofmt FAIL"; fi
    go test -count=1 ./... >/dev/null 2>&1 && echo "test OK" || echo "test FAIL"
    if [ "$QUICK" -eq 1 ]; then
      echo "race SKIP (--quick)"
    else
      go test -race -count=1 ./... >/dev/null 2>&1 && echo "race OK" || echo "race FAIL"
    fi
  ) | while IFS= read -r line; do
    case "$line" in
      "build OK"|"vet OK"|"gofmt OK"|"test OK"|"race OK") ok "go ${line%% OK*}" ;;
      "build FAIL"|"vet FAIL"|"gofmt FAIL"|"test FAIL"|"race FAIL") fail "go ${line%% FAIL*}" ;;
      "race SKIP"*) skip "go test -race (${line#race })" ;;
    esac
  done
  # El subshell con tubería pierde el código real de salida de go: se re-evalúa
  # la parte estructural (build/vet) para no falsear un OK.
  if ! (cd "$ROOT/daemon" && go build ./... >/dev/null 2>&1); then
    fail "go build ./... falló (estructural)"
  fi
else
  skip "go no instalado (no se puede comprobar build/test)"
fi

# ---------------------------------------------------------------------------
# 5. Tests de kernel no privilegiados
# ---------------------------------------------------------------------------
section "Tests de kernel (no privilegiados; los root se omiten solos)"
if bash "$ROOT/tests/kernel/test-all.sh"; then
  ok "tests/kernel/test-all.sh completado (con SKIP documentados)"
else
  fail "tests/kernel/test-all.sh terminó con errores"
fi

# ---------------------------------------------------------------------------
# 6. Observabilidad (suite integración, hermética)
# ---------------------------------------------------------------------------
section "Observabilidad (run.sh)"
if [ "$NO_DOCKER" -eq 1 ] || [ "$DOCKER_OK" -eq 0 ]; then
  skip "suite de observabilidad con Docker omitida (sin daemon o --no-docker)"
elif [ "$QUICK" -eq 1 ]; then
  skip "suite de observabilidad con Docker omitida (--quick)"
else
  echo "  (hermética: solo toca contenedores etiquetados so1.p2.project=202300476)"
  if SO1_RUN_STACK=0 bash "$ROOT/tests/integration/observability/run.sh"; then
    ok "run.sh (integración) OK"
  else
    fail "run.sh (integración) reportó fallos"
  fi
fi

# ---------------------------------------------------------------------------
# 7. Compose / estado operativo (doctor.sh)
# ---------------------------------------------------------------------------
section "Estado operativo (doctor.sh)"
if [ "$NO_DOCKER" -eq 1 ] || [ "$DOCKER_OK" -eq 0 ]; then
  skip "doctor.sh omitido (sin daemon Docker o --no-docker)"
elif [ "$QUICK" -eq 1 ]; then
  skip "doctor.sh omitido (--quick)"
else
  if bash "$ROOT/scripts/doctor.sh"; then
    ok "doctor.sh: entorno apto"
  else
    fail "doctor.sh reportó fallos operativos (revise el detalle arriba)"
  fi
fi

# ---------------------------------------------------------------------------
# 8. Secretos y artefactos accidentales
# ---------------------------------------------------------------------------
section "Secretos y artefactos accidentales"
if command -v git >/dev/null 2>&1 && git -C "$ROOT" rev-parse --show-toplevel >/dev/null 2>&1; then
  # listado de archivos versionados bajo proyecto2/
  TRACKED="$(git -C "$ROOT" ls-files --full-name | grep '^proyecto2/' || true)"
  if printf '%s\n' "$TRACKED" | grep -qx 'proyecto2/.env'; then
    fail "proyecto2/.env está versionado (nunca debe commitearse)"
  else
    ok "proyecto2/.env NO está versionado"
  fi
  if printf '%s\n' "$TRACKED" | grep -E '\.(key|pem|p12|pfx)$' | grep -q .; then
    fail "hay claves/credenciales versionadas (.key/.pem/.p12/.pfx)"
  else
    ok "sin archivos de credenciales versionados"
  fi
  BADARTIFACTS="$(printf '%s\n' "$TRACKED" | grep -E '\.(ko|mod|o|out|log)$' | grep -v 'daemon/internal/ebpf/killmonitor_bpf.o' | grep -v 'go\.mod$' || true)"
  if [ -n "$BADARTIFACTS" ]; then
    fail "artefactos de build versionados bajo proyecto2: $(echo "$BADARTIFACTS" | tr '\n' ' ')"
  else
    ok "sin artefactos de build versionados (objeto eBPF embebido permitido)"
  fi
  if printf '%s\n' "$TRACKED" | grep -q '^proyecto2/bin/' || printf '%s\n' "$TRACKED" | grep -q '^proyecto2/logs/'; then
    fail "bin/ o logs/ versionados bajo proyecto2"
  else
    ok "sin bin/ ni logs/ versionados"
  fi
  # objeto eBPF embebido: debe estar versionado para que el build sea reproducible
  if printf '%s\n' "$TRACKED" | grep -qx 'proyecto2/daemon/internal/ebpf/killmonitor_bpf.o'; then
    ok "objeto eBPF embebido versionado (build reproducible sin clang)"
  else
    fail "killmonitor_bpf.o NO está versionado: un clon no podría buildar el daemon"
  fi
  # contenido sospechoso en archivos de texto versionados
  SECRETS="$(git -C "$ROOT" grep -n -I -E \
    'BEGIN (RSA |EC |OPENSSH |DSA )?PRIVATE KEY|ghp_[A-Za-z0-9]{36}|github_pat_[A-Za-z0-9_]{20,}|AKIA[0-9A-Z]{16}|xox[baprs]-[A-Za-z0-9-]{10,}|sk-[A-Za-z0-9]{20,}' \
    -- 'proyecto2/*' 2>/dev/null || true)"
  if [ -n "$SECRETS" ]; then
    fail "posibles secretos en archivos versionados:"
    printf '%s\n' "$SECRETS" | sed 's/^/    /'
  else
    ok "sin patrones de secretos en archivos versionados"
  fi
  # archivos sin commitear: se listan como aviso (no fallan)
  UNTRACKED="$(git -C "$ROOT" status --porcelain | grep '^??' | grep 'proyecto2/' || true)"
  if [ -n "$UNTRACKED" ]; then
    warn "hay archivos nuevos sin commitear bajo proyecto2 (agregar antes de entregar):"
    printf '%s\n' "$UNTRACKED" | sed 's/^    /    /'
  else
    ok "sin archivos sin commitear bajo proyecto2"
  fi
else
  skip "git no disponible o proyecto2 no está dentro de un repositorio (chequeos de versionado omitidos)"
fi

section "Artefactos en disco (fuera de git, p. ej. build local)"
DISK_BAD="$(find "$ROOT" -type f \( -name '.env' -o -name '*.key' -o -name '*.pem' -o -name '*.p12' -o -name '*.pfx' -o -name 'id_rsa' -o -name 'id_ed25519' \) -not -path '*/.git/*' 2>/dev/null || true)"
if [ -n "$DISK_BAD" ]; then
  fail "archivos sensibles en disco: $(echo "$DISK_BAD" | tr '\n' ' ')"
else
  ok "sin archivos sensibles en disco"
fi
if [ -d "$ROOT/bin" ]; then
  warn "existe proyecto2/bin/ (build local, ignorado por git; no se commitea)"
else
  ok "sin bin/ de build local"
fi
if find "$ROOT" -type f -name '*.log' -not -path '*/.git/*' 2>/dev/null | grep -q .; then
  warn "hay archivos .log en disco (ignorados por git; no se commitean)"
else
  ok "sin .log en disco"
fi

# ---------------------------------------------------------------------------
# Resumen
# ---------------------------------------------------------------------------
section "RESULTADO"
printf '  OK: %d   AVISOS: %d   PENDIENTES(SKIP): %d   FALLOS: %d\n' "$PASS" "$WARN" "$SKIP" "$FAIL"
if [ "${#SKIPPED[@]}" -gt 0 ]; then
  printf '  Omisiones (SKIP, no bloqueantes):\n'
  for s in "${SKIPPED[@]}"; do printf '    - %s\n' "$s"; done
fi
if [ "${#FAILURES[@]}" -gt 0 ]; then
  printf '  Fallos reales:\n'
  for f in "${FAILURES[@]}"; do printf '    - %s\n' "$f"; done
fi

if [ "$JSON_OUT" -eq 1 ]; then
  printf '{"ok":%d,"warn":%d,"skip":%d,"fail":%d,"failures":[%s],"skips":[%s]}\n' \
    "$PASS" "$WARN" "$SKIP" "$FAIL" \
    "$(printf '"%s",' "${FAILURES[@]}" | sed 's/,$//')" \
    "$(printf '"%s",' "${SKIPPED[@]}" | sed 's/,$//')"
fi

if [ "$FAIL" -gt 0 ]; then
  echo
  echo "  VEREDICTO: la entrega NO está lista (corrija los fallos reales)."
  exit 1
fi
echo
echo "  VEREDICTO: preentrega OK (los SKIP son por entorno/privilegios y deben"
echo "  ejecutarse en la máquina donde se demostrará el proyecto)."
exit 0
