#!/usr/bin/env bash
set -euo pipefail
P3_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
[[ ! -f "$P3_ROOT/.env" ]] || source "$P3_ROOT/.env"
export SO1_P3_DATA="${SO1_P3_DATA:-/mnt/fedora-extra/so1-proyecto3}"
export SO1_P3_CLUSTER="${SO1_P3_CLUSTER:-so1-p3}"
export SO1_P3_NAMESPACE="${SO1_P3_NAMESPACE:-so1-p3}"
export PATH="$SO1_P3_DATA/bin:$PATH"
export KUBECONFIG="$SO1_P3_DATA/runtime/kubeconfig"
export GOCACHE="$SO1_P3_DATA/build/go-cache" GOMODCACHE="$SO1_P3_DATA/build/go-mod"
export CARGO_HOME="$SO1_P3_DATA/build/cargo" CARGO_TARGET_DIR="$SO1_P3_DATA/build/rust"
export GOMAXPROCS=2 CARGO_BUILD_JOBS=1
k() { kubectl -n "$SO1_P3_NAMESPACE" "$@"; }

export SO1_P3_GATEWAY_PORT="${SO1_P3_GATEWAY_PORT:-8088}"
export SO1_P3_GRAFANA_PORT="${SO1_P3_GRAFANA_PORT:-3003}"
export SO1_P3_LOCUST_PORT="${SO1_P3_LOCUST_PORT:-8089}"
