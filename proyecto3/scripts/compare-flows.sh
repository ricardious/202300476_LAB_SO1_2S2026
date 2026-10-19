#!/usr/bin/env bash
source "$(dirname "$0")/env.sh"
cd "$P3_ROOT"
users="${USERS:-5}"
duration="${DURATION:-30s}"
for flow in grpc dapr; do
  date --iso-8601=seconds
  SO1_LOAD_PROFILE="$SO1_P3_DATA/runtime/input/load-profile.json" \
  SO1_LOAD_PATH="/$flow-202300476" \
  "$SO1_P3_DATA/venv/bin/locust" -f load/locustfile.py --headless \
    -u "$users" -r 1 -t "$duration" --host "http://127.0.0.1:$SO1_P3_GATEWAY_PORT" \
    --csv "docs/evidencias/compare-$flow" --only-summary
  k top pods --containers
done
