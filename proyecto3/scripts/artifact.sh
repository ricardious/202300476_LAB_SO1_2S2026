#!/usr/bin/env bash
source "$(dirname "$0")/env.sh"
cd "$P3_ROOT/load"
oras push --plain-http=false --ca-file "$SO1_P3_DATA/certs/ca.crt" --registry-config "$SO1_P3_DATA/runtime/registry-auth.json" --artifact-type application/vnd.so1.load-profile.v1+json localhost:5005/so1/input:1.0.0 load-profile.json:application/json
mkdir -p "$SO1_P3_DATA/runtime/input"
oras pull --plain-http=false --ca-file "$SO1_P3_DATA/certs/ca.crt" --registry-config "$SO1_P3_DATA/runtime/registry-auth.json" -o "$SO1_P3_DATA/runtime/input" localhost:5005/so1/input:1.0.0
cmp load-profile.json "$SO1_P3_DATA/runtime/input/load-profile.json"
