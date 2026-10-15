#!/usr/bin/env bash
source "$(dirname "$0")/env.sh"
python3 "$P3_ROOT/scripts/install-tools.py"
cd "$P3_ROOT/services"
CGO_ENABLED=0 go build -p 1 -trimpath -ldflags="-s -w" -o "$SO1_P3_DATA/build/so1-go" ./cmd/so1
cd "$P3_ROOT/api-rust"
cargo build --locked --release -j 1
cp "$CARGO_TARGET_DIR/release/so1-p3-api" "$SO1_P3_DATA/build/so1-rust"
printf 'Archivo de prueba SO1 sin secretos reales\n' > "$SO1_P3_DATA/build/test-secret.txt"
cd "$P3_ROOT"
for component in go rust operator; do
  case "$component" in
    go) containerfile=services/Containerfile; context="$SO1_P3_DATA/build" ;;
    rust) containerfile=api-rust/Containerfile; context="$SO1_P3_DATA/build" ;;
    operator) containerfile=operator/Dockerfile; context=operator ;;
  esac
  image="localhost:5005/so1/$component:1.0.0"
  podman --root "$SO1_P3_DATA/podman" build --format oci --memory 1g -t "$image" -f "$containerfile" "$context"
  podman --root "$SO1_P3_DATA/podman" push --format oci --tls-verify=true --cert-dir "$SO1_P3_DATA/certs/client" --authfile "$SO1_P3_DATA/runtime/podman-auth.json" "$image"
done
