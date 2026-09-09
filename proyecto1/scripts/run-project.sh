#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

VIRSH_URI="qemu:///system"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_ed25519}"
SSH_OPTS=(
  -o BatchMode=yes
  -o ConnectTimeout=5
  -o StrictHostKeyChecking=no
  -o UserKnownHostsFile=/dev/null
)

if [[ -f "$SSH_KEY" ]]; then
  SSH_OPTS+=(-i "$SSH_KEY")
fi

declare -A VM_IPS=(
  [SO1-VM1]="192.168.100.10"
  [SO1-VM2]="192.168.100.20"
  [SO1-VM3]="192.168.100.30"
)

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: se requiere el comando '$1'." >&2
    exit 1
  fi
}

start_vm() {
  local vm="$1"
  local state

  if ! state="$(LC_ALL=C virsh -c "$VIRSH_URI" domstate "$vm" 2>/dev/null)"; then
    echo "ERROR: la VM $vm no existe en $VIRSH_URI." >&2
    exit 1
  fi

  case "$state" in
    running)
      echo ">> $vm ya está en ejecución."
      ;;
    paused)
      echo ">> Reanudando $vm..."
      virsh -c "$VIRSH_URI" resume "$vm" >/dev/null
      ;;
    "shut off"|crashed)
      echo ">> Iniciando $vm..."
      virsh -c "$VIRSH_URI" start "$vm" >/dev/null
      ;;
    *)
      echo "ERROR: estado no soportado para $vm: $state" >&2
      exit 1
      ;;
  esac
}

wait_for_ssh() {
  local vm="$1"
  local ip="${VM_IPS[$vm]}"

  echo ">> Esperando SSH en $vm ($ip)..."
  for _ in {1..45}; do
    if ssh "${SSH_OPTS[@]}" "root@$ip" true >/dev/null 2>&1; then
      return
    fi
    sleep 2
  done

  echo "ERROR: $vm no respondió por SSH." >&2
  exit 1
}

start_services() {
  echo ">> Iniciando Zot en VM3..."
  ssh "${SSH_OPTS[@]}" root@192.168.100.30 '
    if ! docker inspect zot >/dev/null 2>&1; then
      echo "ERROR: no existe el contenedor zot en VM3." >&2
      exit 1
    fi
    docker start zot >/dev/null
  '

  echo ">> Iniciando API3 en VM2..."
  ssh "${SSH_OPTS[@]}" root@192.168.100.20 '
    if ! podman container exists api3; then
      echo "ERROR: no existe el contenedor api3 en VM2." >&2
      exit 1
    fi
    podman start api3 >/dev/null
  '

  echo ">> Iniciando API1 y API2 en VM1..."
  ssh "${SSH_OPTS[@]}" root@192.168.100.10 '
    for container in api1 api2; do
      if ! ctr -n default containers info "$container" >/dev/null 2>&1; then
        echo "ERROR: no existe el contenedor $container en VM1." >&2
        exit 1
      fi
      if ! ctr -n default tasks ls | grep -q "^${container}[[:space:]]"; then
        ctr -n default tasks start --detach "$container"
      fi
    done
  '
}

wait_for_health() {
  local name="$1"
  local url="$2"

  echo ">> Esperando $name en $url..."
  for _ in {1..30}; do
    if curl --fail --silent --max-time 3 "$url" >/dev/null; then
      return
    fi
    sleep 2
  done

  echo "ERROR: $name no respondió correctamente." >&2
  exit 1
}

main() {
  require_command virsh
  require_command ssh
  require_command curl
  require_command jq

  for vm in SO1-VM1 SO1-VM2 SO1-VM3; do
    start_vm "$vm"
  done

  for vm in SO1-VM1 SO1-VM2 SO1-VM3; do
    wait_for_ssh "$vm"
  done

  start_services

  wait_for_health API1 http://192.168.100.10:8081/health
  wait_for_health API2 http://192.168.100.10:8082/health
  wait_for_health API3 http://192.168.100.20:8083/health

  echo ">> Ejecutando validación funcional..."
  bash scripts/validation/test-all.sh "${@}"

  echo
  echo "Proyecto ejecutándose:"
  echo "  API1: http://192.168.100.10:8081/health"
  echo "  API2: http://192.168.100.10:8082/health"
  echo "  API3: http://192.168.100.20:8083/health"
  echo "  Zot:  http://192.168.100.30:5000/v2/_catalog"
}

main "$@"
