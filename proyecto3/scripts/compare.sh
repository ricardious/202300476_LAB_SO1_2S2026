#!/usr/bin/env bash
source "$(dirname "$0")/env.sh"
cd "$P3_ROOT"
[[ -x "$SO1_P3_DATA/venv/bin/locust" ]] || { echo 'Primero: make setup-load'; exit 1; }
restore() {
  k scale deployment/go-writer --replicas=1 >/dev/null || true
  k patch valkeyinstance valkey --type=merge -p '{"spec":{"replica":{"enabled":false}}}' >/dev/null || true
  k rollout restart deployment/go-ingest >/dev/null || true
}
trap restore EXIT
for replicas in 1 2; do
  k scale deployment/go-writer --replicas="$replicas"
  k rollout status deployment/go-writer --timeout=120s
  k rollout restart deployment/go-ingest
  k rollout status deployment/go-ingest --timeout=120s
  SO1_LOAD_PROFILE="$SO1_P3_DATA/runtime/input/load-profile.json" SO1_LOAD_WAIT_MIN=0 SO1_LOAD_WAIT_MAX=0 "$SO1_P3_DATA/venv/bin/locust" -f load/locustfile.py --headless -u 20 -r 4 -t 45s --host "http://127.0.0.1:$SO1_P3_GATEWAY_PORT" --csv "docs/evidencias/repeticion-writer-$replicas" --only-summary
done
k exec valkey-0 -- valkey-benchmark -h valkey -n 1000 -c 5 --csv HGETALL so1:p3:stats > docs/evidencias/repeticion-valkey-primario.csv
k patch valkeyinstance valkey --type=merge -p '{"spec":{"replica":{"enabled":true}}}'
for attempt in {1..60}; do
  if k get statefulset valkey-replica >/dev/null 2>&1; then break; fi
  sleep 1
done
k rollout status statefulset/valkey-replica --timeout=120s
for attempt in {1..30}; do
  if k exec valkey-replica-0 -- valkey-cli INFO replication | tr -d '\r' | rg -q '^master_link_status:up$'; then break; fi
  sleep 1
done
k exec valkey-replica-0 -- valkey-cli INFO replication | tr -d '\r' | rg '^master_link_status:up$'
k exec valkey-replica-0 -- valkey-benchmark -h valkey-read -n 1000 -c 5 --csv HGETALL so1:p3:stats > docs/evidencias/repeticion-valkey-replica.csv
