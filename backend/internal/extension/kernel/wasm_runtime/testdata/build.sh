#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WAT_FILE="$SCRIPT_DIR/echo.wat"
WASM_FILE="$SCRIPT_DIR/echo_prebuilt.wasm"

if command -v wat2wasm >/dev/null 2>&1; then
    echo "Using wat2wasm to compile echo.wat"
    wat2wasm "$WAT_FILE" -o "$WASM_FILE"
    echo "Built: $WASM_FILE"
else
    echo "wat2wasm not found, falling back to Go generator"
    if command -v go >/dev/null 2>&1; then
        cd "$SCRIPT_DIR"
        go run generate_wasm.go
        echo "Built: $WASM_FILE"
    else
        echo "Error: neither wat2wasm nor go is available" >&2
        exit 1
    fi
fi

if [ -f "$WASM_FILE" ]; then
    SIZE=$(wc -c < "$WASM_FILE")
    echo "WASM module size: $SIZE bytes"
else
    echo "Error: WASM file was not created" >&2
    exit 1
fi
