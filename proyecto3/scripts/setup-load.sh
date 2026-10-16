#!/usr/bin/env bash
source "$(dirname "$0")/env.sh"
python3 -m venv "$SO1_P3_DATA/venv"
PIP_CACHE_DIR="$SO1_P3_DATA/build/pip" "$SO1_P3_DATA/venv/bin/pip" install --disable-pip-version-check -r "$P3_ROOT/load/requirements.txt"
