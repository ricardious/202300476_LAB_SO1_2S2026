#!/usr/bin/env bash
# ============================================================================
# capture-evidence.sh — Captura automática de evidencias del Proyecto 2 SO1
# (202300476). Rama de entrega: ricardious/p2-delivery
#
# Genera, con timestamp y SIN secretos, archivos de texto por rubro dentro de
# proyecto2/docs/evidencias/. Es SOLO LECTURA del entorno: nunca carga/descarga el
# módulo, nunca toca cron, contenedores ajenos ni Compose, y jamás modifica
# recursos del Proyecto 1.
#
# Acciones privilegiadas (dmesg, doctor --privileged) son OPT-IN: solo se
# ejecutan si el usuario pasa --privileged explícitamente (se usa sudo).
#
# Idempotente: cada corrida sobrescribe el archivo del rubro con un encabezado
# de fecha/hora; no acumula basura entre ejecuciones.
#
# Uso:
#   bash proyecto2/scripts/capture-evidence.sh                  # todos los rubros
#   bash proyecto2/scripts/capture-evidence.sh --rubro kernel   # solo uno
#   bash proyecto2/scripts/capture-evidence.sh --privileged     # + dmesg/doctor root
#   bash proyecto2/scripts/capture-evidence.sh --dry-run        # lista sin escribir
#   bash proyecto2/scripts/capture-evidence.sh --out-dir /tmp/ev
#
# Los rubros y su mapeo con la rúbrica están documentados en
# proyecto2/docs/evidencias/README.md y docs/GUIA_DEMO.md.
# ============================================================================
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PRIVILEGED=0
DRY_RUN=0
OUT_DIR="${SO1_P2_EVIDENCIAS_DIR:-${ROOT}/docs/evidencias}"
REQUESTED_RUBRO=""
GRAFANA_PORT="${GRAFANA_PORT:-3000}"
VALKEY_PORT="${VALKEY_PORT:-6379}"

PROC_FILE="${SO1_P2_PROC_PATH:-/proc/continfo_pr2_so1_202300476}"
MODNAME="continfo"
MARKER_BEGIN="# >>> so1-p2-202300476:begin >>>"
MARKER_END="# <<< so1-p2-202300476:end <<<"

declare -a RUBROS=(entorno kernel daemon-politica cron-workloads valkey-grafana ebpf documentacion-git)

usage() {
  sed -n '2,40p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

# ---------------------------------------------------------------------------
# parsing
# ---------------------------------------------------------------------------
while [ "$#" -gt 0 ]; do
  case "$1" in
    --rubro)
      shift
      REQUESTED_RUBRO="$1"
      ;;
    --privileged) PRIVILEGED=1 ;;
    --dry-run) DRY_RUN=1 ;;
    --out-dir)
      shift
      OUT_DIR="$1"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "opción desconocida: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

if [ -n "$REQUESTED_RUBRO" ]; then
  found=0
  for r in "${RUBROS[@]}"; do
    [ "$r" = "$REQUESTED_RUBRO" ] && found=1
  done
  if [ "$found" -eq 0 ]; then
    echo "rubro inválido: $REQUESTED_RUBRO (válidos: ${RUBROS[*]})" >&2
    exit 2
  fi
fi

# ---------------------------------------------------------------------------
# helpers de estado
# ---------------------------------------------------------------------------
PASS=0; WARN=0; SKIPPED=0; FAIL=0
ok()   { PASS=$((PASS + 1)); printf '  [OK]      %s\n' "$*"; }
warn() { WARN=$((WARN + 1)); printf '  [AVISO]   %s\n' "$*"; }
skip() { SKIPPED=$((SKIPPED + 1)); printf '  [PEND]    %s\n' "$*"; }
fail() { FAIL=$((FAIL + 1)); printf '  [FALLO]   %s\n' "$*"; }

CUR=""          # archivo de captura actual
NOW_UTC="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# escribe una línea en consola y la duplica en el archivo de captura
cap() { # sev  msg  [detail...]
  local sev="$1" msg="$2"
  printf '  [%s]  %s\n' "$sev" "$msg"
  if [ -n "$CUR" ]; then
    printf '[%s]  %s\n' "$sev" "$msg" >> "$CUR"
    shift 2
    for d in "$@"; do printf '       %s\n' "$d" >> "$CUR"; done
  fi
}

begin_file() { # rubro  archivo  titulo
  local rubro="$1" fname="$2" title="$3"
  mkdir -p "${OUT_DIR}/${rubro}"
  CUR="${OUT_DIR}/${rubro}/${fname}"
  {
    printf '# ================================================================\n'
    printf '# Proyecto 2 · SO1 (202300476) — %s\n' "$title"
    printf '# Fecha (UTC): %s · Host: %s\n' "$NOW_UTC" "${HOSTNAME:-$(hostname 2>/dev/null || echo unknown)}"
    printf '# Generado por: scripts/capture-evidence.sh (solo lectura)\n'
    printf '# ================================================================\n'
  } > "$CUR"
}

end_file() {
  CUR=""
}

section() { # texto
  printf '\n== %s ==\n' "$*"
  if [ -n "$CUR" ]; then printf '\n== %s ==\n' "$*" >> "$CUR"; fi
}

dump() { # texto a captura (y consola como comentario)
  printf '%s\n' "$*" >> "$CUR"
}

# ---------------------------------------------------------------------------
# rubro: entorno — doctor + sistema + herramientas + lsmod
# ---------------------------------------------------------------------------
capture_entorno() {
  local f="entorno.txt"
  begin_file "entorno" "$f" "Entorno / doctor"
  section "make doctor (scripts/doctor.sh)"
  if [ "$PRIVILEGED" -eq 1 ]; then
    sudo -E bash "${ROOT}/scripts/doctor.sh" --privileged >> "$CUR" 2>&1 \
      && ok "doctor.sh --privileged completado" \
      || warn "doctor.sh --privileged terminó con código distinto de 0 (ver archivo)"
  else
    bash "${ROOT}/scripts/doctor.sh" >> "$CUR" 2>&1 \
      && ok "doctor.sh completado" \
      || warn "doctor.sh terminó con código distinto de 0 (ver archivo)"
    cap OK "ejecute con --privileged para el bloque módulo/eBPF de doctor.sh"
  fi

  section "Sistema operativo"
  dump "$(uname -a)"
  dump "$(cat /proc/version 2>/dev/null || true)"

  section "Herramientas"
  for t in bash make go docker git python3 jq crontab gcc clang; do
    if command -v "$t" >/dev/null 2>&1; then dump "$(command -v "$t")  —  $("$t" --version 2>&1 | head -n 1)"; else dump "$t: NO instalado"; fi
  done

  section "Headers del kernel y BTF (prerequisitos módulo/eBPF)"
  if [ -e "/lib/modules/$(uname -r)/build/Makefile" ]; then
    ok "headers del kernel presentes (/lib/modules/$(uname -r)/build)"
  else
    skip "headers del kernel ausentes (sudo dnf install kernel-devel-$(uname -r))"
  fi
  if [ -r /sys/kernel/btf/vmlinux ]; then
    ok "BTF disponible (CO-RE)"
  else
    warn "BTF ausente (/sys/kernel/btf/vmlinux)"
  fi

  section "lsmod (estado de módulos)"
  if command -v lsmod >/dev/null 2>&1; then
    dump "$(lsmod | head -n 60)"
  else
    skip "lsmod no disponible"
  fi
  end_file
}

# ---------------------------------------------------------------------------
# rubro: kernel — módulo /proc + lsmod del módulo + dmesg (opt-in)
# ---------------------------------------------------------------------------
capture_kernel() {
  local f="kernel.txt"
  begin_file "kernel" "$f" "Módulo de kernel y /proc"
  section "Estado del módulo (${MODNAME})"
  if grep -qw "${MODNAME}" /proc/modules 2>/dev/null; then
    ok "${MODNAME} cargado en /proc/modules"
    dump "modules: $(grep "${MODNAME}" /proc/modules)"
  else
    skip "${MODNAME} NO cargado (sudo bash scripts/load-module.sh)"
  fi
  if [ -e "$PROC_FILE" ]; then
    ok "$PROC_FILE existe"
    dump "permisos: $(stat -c '%A %a' "$PROC_FILE" 2>/dev/null || true)"
  else
    skip "$PROC_FILE no existe (requiere cargar el módulo con root)"
  fi

  section "Salida EXACTA de ${PROC_FILE} + validación con jq"
  if [ -r "$PROC_FILE" ]; then
    local exact="${OUT_DIR}/kernel/proc-snapshot.json"
    cp "$PROC_FILE" "$exact" 2>/dev/null
    local sz
    sz="$(wc -c < "$exact" 2>/dev/null || echo 0)"
    dump "se copió la salida exacta a ${exact} (${sz} bytes)"
    if command -v jq >/dev/null 2>&1; then
      if jq -e . "$exact" >/dev/null 2>&1; then
        ok "jq: JSON válido"
        dump "jq -e . >/dev/null → OK"
      else
        fail "jq: el JSON NO es válido"
        dump "jq -e . → ERROR (revise el archivo)"
      fi
    else
      skip "jq no disponible; validación omitida"
    fi
    if command -v python3 >/dev/null 2>&1; then
      if python3 "${ROOT}/tests/kernel/validate_schema.py" "$exact" --strict-memory >/dev/null 2>&1; then
        ok "validate_schema.py --strict-memory: OK"
      else
        warn "validate_schema.py --strict-memory no pasó (revise detalle)"
      fi
    fi
    dump "--- primeras líneas ---"
    dump "$(head -n 20 "$exact")"
  else
    skip "no se pudo leer $PROC_FILE (módulo no cargado o sin permisos)"
  fi

  section "Contraste con ps / free (rubrica 2.2)"
  if command -v ps >/dev/null 2>&1 && command -v free >/dev/null 2>&1; then
    dump "--- free -k ---"
    dump "$(free -k 2>/dev/null || true)"
    dump "--- top 10 por RSS ---"
    dump "$(ps -eo pid,comm,vsz,rss,%mem,%cpu --sort=-rss 2>/dev/null | head -n 11 || true)"
  else
    skip "ps/free no disponibles"
  fi

  if [ "$PRIVILEGED" -eq 1 ]; then
    section "dmesg (eventos del módulo, opt-in con sudo)"
    if sudo dmesg 2>/dev/null | grep -qi "continfo"; then
      ok "eventos continfo encontrados en dmesg"
      dump "$(sudo dmesg 2>/dev/null | grep -i continfo | tail -n 10)"
    else
      warn "sin eventos continfo en dmesg (cargue el módulo para verlos)"
    fi
  else
    cap OK "omita dmesg (requiere root); re-ejecute con --privileged"
  fi
  end_file
}

# ---------------------------------------------------------------------------
# rubro: daemon-politica — daemon, docker, gestionados, política
# ---------------------------------------------------------------------------
capture_daemon_politica() {
  local f="politica.txt"
  begin_file "daemon-politica" "$f" "Daemon Go y política de conservación"
  section "Daemon"
  if [ -x "${ROOT}/bin/so1-p2-daemon" ]; then
    ok "binario compilado presente (${ROOT}/bin/so1-p2-daemon)"
  else
    warn "binario no compilado (make build)"
  fi
  if pgrep -af "so1-p2-daemon" >/dev/null 2>&1; then
    ok "daemon en ejecución:"
    dump "$(pgrep -af so1-p2-daemon)"
  else
    skip "daemon NO en ejecución (sudo make run, o /usr/local/bin/so1-p2-daemon)"
  fi
  if [ -r /var/log/so1-p2-daemon.log ]; then
    dump "--- cola de /var/log/so1-p2-daemon.log ---"
    dump "$(tail -n 30 /var/log/so1-p2-daemon.log)"
  fi

  section "Docker"
  if docker info >/dev/null 2>&1; then
    ok "daemon Docker responde"
  else
    skip "daemon Docker no disponible en este host"
  fi

  section "Contenedores gestionados (etiqueta so1.p2.project=202300476)"
  if docker info >/dev/null 2>&1; then
    local total running
    total="$(docker ps -a --filter "label=so1.p2.project=202300476" --format '{{.ID}}' 2>/dev/null | wc -l)"
    running="$(docker ps --filter "label=so1.p2.project=202300476" --format '{{.ID}}' 2>/dev/null | wc -l)"
    dump "total: ${total} · en ejecución: ${running}"
    dump "--- detalle (name, image, profile, status, mem) ---"
    docker ps -a --filter "label=so1.p2.project=202300476" \
      --format '{{.Names}}\t{{.Image}}\t{{.Status}}' >> "$CUR" 2>/dev/null
    dump "--- perfiles ---"
    docker ps -a --filter "label=so1.p2.project=202300476" --format '{{.Names}}' \
      | awk -F- '{print $3}' | sort | uniq -c >> "$CUR" 2>/dev/null
  else
    skip "sin Docker; no se listan contenedores"
  fi

  section "Protegidos (nunca candidatos) — Grafana y Valkey"
  if docker info >/dev/null 2>&1; then
    for name in so1-p2-grafana so1-p2-valkey; do
      if docker ps --format '{{.Names}}' | grep -qx "$name"; then
        ok "$name presente (protegido)"
        dump "$(docker inspect "$name" --format '  labels: {{json .Config.Labels}}' 2>/dev/null || true)"
      else
        warn "$name no está corriendo (make up)"
      fi
    done
  fi
  end_file
}

# ---------------------------------------------------------------------------
# rubro: cron-workloads — cronjob, generador, imágenes, carga
# ---------------------------------------------------------------------------
capture_cron_workloads() {
  local f="cron.txt"
  begin_file "cron-workloads" "$f" "Cron y generador de carga"
  section "Cronjob del proyecto (bloque delimitado)"
  if command -v crontab >/dev/null 2>&1; then
    if crontab -l 2>/dev/null | grep -q "${MARKER_BEGIN}"; then
      ok "sección del proyecto presente"
      crontab -l 2>/dev/null | awk -v b="${MARKER_BEGIN}" -v e="${MARKER_END}" '
        $0==b {p=1} p {print} $0==e {p=0}' >> "$CUR"
    else
      skip "sin sección del proyecto en crontab (make install-cron)"
    fi
  else
    skip "binario crontab ausente en este host"
  fi

  section "Generador instalado"
  local gen="/usr/local/bin/so1-p2-generate-containers.sh"
  if [ -e "$gen" ]; then
    ok "$gen presente"
    dump "$(ls -l "$gen")"
    dump "resuelve a: $(readlink -f "$gen" 2>/dev/null || echo '(copia, no symlink)')"
  else
    skip "$gen ausente (sudo make install)"
  fi

  section "Imágenes de carga (so1/load:202300476-*)"
  if docker info >/dev/null 2>&1; then
    local imgs
    imgs="$(docker images --format '{{.Repository}}:{{.Tag}}' | grep '^so1/load' || true)"
    if [ -n "$imgs" ]; then
      ok "imágenes presentes"
      dump "$imgs"
    else
      skip "sin imágenes so1/load (bash scripts/generate-containers.sh las construye)"
    fi
  else
    skip "Docker no disponible"
  fi

  section "Carga acumulada (contenedores del proyecto)"
  if docker info >/dev/null 2>&1; then
    dump "conteo actual: $(docker ps -a --filter "label=so1.p2.project=202300476" --format '{{.ID}}' | wc -l)"
  fi

  section "Log del generador (cola)"
  local glog="${HOME}/.so1-p2/logs/generate-containers.log"
  if [ -r "$glog" ]; then
    dump "$(tail -n 15 "$glog")"
  else
    skip "sin log del generador en ${glog}"
  fi
  end_file
}

# ---------------------------------------------------------------------------
# rubro: valkey-grafana — datos Valkey + salud de Grafana
# ---------------------------------------------------------------------------
capture_valkey_grafana() {
  local f="observabilidad.txt"
  begin_file "valkey-grafana" "$f" "Valkey y Grafana"
  local VC="docker exec so1-p2-valkey timeout 10 valkey-cli"

  section "Contenedores del stack"
  if docker info >/dev/null 2>&1; then
    for name in so1-p2-valkey so1-p2-grafana; do
      if docker ps --format '{{.Names}}' | grep -qx "$name"; then
        ok "$name corriendo"
        dump "$(docker ps --filter "name=${name}" --format '  {{.Names}}\t{{.Status}}\t{{.Ports}}')"
      else
        warn "$name NO está corriendo (make up)"
      fi
    done
  fi

  section "Valkey (ping y claves so1:p2:*)"
  if docker ps --format '{{.Names}}' | grep -qx 'so1-p2-valkey' 2>/dev/null; then
    if $VC ping >/dev/null 2>&1; then
      ok "Valkey responde PONG"
      dump "--- KEYS so1:p2:* ---"
      $VC KEYS 'so1:p2:*' >> "$CUR" 2>/dev/null
    else
      warn "Valkey no responde al ping"
    fi
    section "system:latest (hash)"
    dump "$($VC HGETALL so1:p2:system:latest 2>/dev/null || true)"
    section "system:timeline (stream, últimas 30)"
    dump "$($VC XRANGE so1:p2:system:timeline - + COUNT 30 2>/dev/null || true)"
    section "containers:timeline (stream, últimas 30)"
    dump "$($VC XRANGE so1:p2:containers:timeline - + COUNT 30 2>/dev/null || true)"
    section "rank:ram / rank:cpu (top 10)"
    dump "--- ZREVRANGE rank:ram ---"
    dump "$($VC ZREVRANGE so1:p2:rank:ram 0 9 WITHSCORES 2>/dev/null || true)"
    dump "--- ZREVRANGE rank:cpu ---"
    dump "$($VC ZREVRANGE so1:p2:rank:cpu 0 9 WITHSCORES 2>/dev/null || true)"
  else
    skip "so1-p2-valkey no está corriendo"
  fi

  section "Grafana (/api/health)"
  if curl -fsS --max-time 10 "http://localhost:${GRAFANA_PORT}/api/health" >/dev/null 2>&1; then
    ok "Grafana responde en /api/health"
    dump "$(curl -fsS --max-time 10 "http://localhost:${GRAFANA_PORT}/api/health" 2>/dev/null || true)"
  else
    skip "Grafana no responde en :${GRAFANA_PORT} (make up y espere el healthcheck)"
  fi

  section "Capturas visuales manuales"
  dump "Tome una captura por cada panel del dashboard (14 paneles) y guárdelas"
  dump "en valkey-grafana/ con nombres NN-grafana-<panel>.png. Lista en"
  dump "docs/GUIA_DEMO.md -> 'Evidencias a capturar'."
  end_file
}

# ---------------------------------------------------------------------------
# rubro: ebpf — eBPF + cadena de confirmación (kills)
# ---------------------------------------------------------------------------
capture_ebpf() {
  local f="ebpf.txt"
  begin_file "ebpf" "$f" "eBPF y confirmación de eliminaciones"
  section "Prerrequisitos eBPF"
  if [ -r /sys/kernel/btf/vmlinux ]; then
    ok "BTF presente (CO-RE viable)"
  else
    warn "BTF ausente"
  fi
  dump "SO1_P2_EBPF_MODE=${SO1_P2_EBPF_MODE:-auto} (default: auto; required exige root)"
  if [ -f "${ROOT}/daemon/internal/ebpf/killmonitor_bpf.o" ]; then
    ok "objeto eBPF embebido presente (daemon/internal/ebpf/killmonitor_bpf.o)"
  else
    warn "objeto eBPF ausente (make ebpf)"
  fi

  section "Prueba privilegiada (opt-in, requiere root)"
  if [ "$(id -u)" -eq 0 ]; then
    dump "root detectado: puede ejecutar la prueba integrada:"
  else
    dump "usuario sin root: ejecute en la VM:"
  fi
  dump "  sudo go test ./internal/ebpf/ -run Privilegiada -v -count=1"
  dump "y capture la salida (texto o pantalla) en ebpf/"

  section "Cadena de confirmación: so1:p2:kills"
  local VC="docker exec so1-p2-valkey timeout 10 valkey-cli"
  if docker ps --format '{{.Names}}' | grep -qx 'so1-p2-valkey' 2>/dev/null; then
    dump "--- XLEN so1:p2:kills ---"
    dump "$($VC XLEN so1:p2:kills 2>/dev/null || true)"
    dump "--- GET so1:p2:kills:count ---"
    dump "$($VC GET so1:p2:kills:count 2>/dev/null || true)"
    dump "--- XRANGE so1:p2:kills - + (últimas 30) ---"
    dump "$($VC XRANGE so1:p2:kills - + COUNT 30 2>/dev/null || true)"
    if [ -n "$($VC GET so1:p2:kills:count 2>/dev/null || true)" ] && \
       [ "$($VC GET so1:p2:kills:count 2>/dev/null || true)" -gt 0 ] 2>/dev/null; then
      ok "kills:count > 0 — eliminaciones confirmadas por eBPF"
    else
      skip "kills:count = 0 (sin eliminaciones confirmadas todavía; acumule carga)"
    fi
  else
    skip "so1-p2-valkey no está corriendo (make up)"
  fi
  end_file
}

# ---------------------------------------------------------------------------
# rubro: documentacion-git — estado git + docs + evidencias
# ---------------------------------------------------------------------------
capture_documentacion_git() {
  local f="git.txt"
  begin_file "documentacion-git" "$f" "Documentación y estado Git"
  section "Repositorio"
  if git rev-parse --show-toplevel >/dev/null 2>&1; then
    dump "toplevel: $(git rev-parse --show-toplevel)"
    dump "rama actual: $(git branch --show-current)"
    dump "remotes:"
    git remote -v >> "$CUR" 2>/dev/null
    dump "--- últimos 15 commits ---"
    git log --oneline -15 >> "$CUR" 2>/dev/null
    dump "--- estado (archivos sin commitear) ---"
    dump "archivos modificados/sin seguimiento: $(git status --porcelain | wc -l)"
    git status --short >> "$CUR" 2>/dev/null
  else
    skip "no es un repositorio git en $ROOT"
  fi

  section "Documentación (docs/)"
  if [ -d "${ROOT}/docs" ]; then
    find "${ROOT}/docs" -maxdepth 1 -type f -name '*.md' | sort | sed "s|${ROOT}/|proyecto2/|" >> "$CUR"
  fi

  section "Árbol de evidencias (docs/evidencias/)"
  find "${ROOT}/docs/evidencias" -maxdepth 2 | sort | sed "s|${ROOT}/|proyecto2/|" >> "$CUR"

  section "Capturas visuales manuales (GitHub)"
  dump "Capture: página del repo, pestaña commits de la rama ricardious/p2-delivery"
  dump "y (si aplica) releases. Guárdelas en documentacion-git/."
  end_file
}

# ---------------------------------------------------------------------------
# main
# ---------------------------------------------------------------------------
main() {
  if [ "$DRY_RUN" -eq 1 ]; then
    echo "Plan de captura (--dry-run; no se escribe nada):"
    for r in "${RUBROS[@]}"; do
      if [ -z "$REQUESTED_RUBRO" ] || [ "$r" = "$REQUESTED_RUBRO" ]; then
        printf '  %s\n' "${OUT_DIR}/${r}/*.txt"
      fi
    done
    echo
    echo "Rúbrica y contenido de cada rubro: docs/evidencias/README.md"
    exit 0
  fi

  echo "== Captura de evidencias del Proyecto 2 (202300476) =="
  echo "Directorio de salida: ${OUT_DIR}"
  [ "$PRIVILEGED" -eq 1 ] && echo "Modo privilegiado: ON (se usará sudo para dmesg/doctor)"

  for r in "${RUBROS[@]}"; do
    [ -n "$REQUESTED_RUBRO" ] && [ "$r" != "$REQUESTED_RUBRO" ] && continue
    echo
    echo "### rubro: ${r}"
    case "$r" in
      entorno)           capture_entorno ;;
      kernel)            capture_kernel ;;
      daemon-politica)   capture_daemon_politica ;;
      cron-workloads)    capture_cron_workloads ;;
      valkey-grafana)    capture_valkey_grafana ;;
      ebpf)              capture_ebpf ;;
      documentacion-git) capture_documentacion_git ;;
    esac
  done

  echo
  echo "== RESUMEN =="
  printf '  OK: %d   AVISOS: %d   PENDIENTES: %d   FALLOS: %d\n' "$PASS" "$WARN" "$SKIPPED" "$FAIL"
  echo
  echo "Archivos generados en ${OUT_DIR}:"
  find "${OUT_DIR}" -maxdepth 2 -name '*.txt' -o -maxdepth 2 -name '*.json' 2>/dev/null | sort
  echo
  echo "Faltan por captura MANUAL (pantalla):"
  echo "  - Grafana: 14 paneles del dashboard (valkey-grafana/NN-grafana-<panel>.png)."
  echo "  - GitHub: repo, commits de la rama y releases (documentacion-git/)."
  echo "  - Acciones privilegiadas del checklist (sudo make module-load, sudo go test eBPF,"
  echo "    sudo bash tests/kernel/test-module-lifecycle.sh) en kernel/ y ebpf/."
  exit 0
}

main "$@"
