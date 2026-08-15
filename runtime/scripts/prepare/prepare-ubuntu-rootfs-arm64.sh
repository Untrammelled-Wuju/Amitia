#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RUNTIME_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BUILDER="$RUNTIME_ROOT/build/rootfs/linux-arm64/build.py"

exec python3 "$BUILDER" "$@"
