#!/usr/bin/env bash
set -euo pipefail

NODE_ROOT="${NODE_ROOT:-/opt/amitia/node}"
NODE_BIN="$NODE_ROOT/bin/node"
NPM_CLI="$NODE_ROOT/lib/node_modules/npm/bin/npm-cli.js"
NPX_CLI="$NODE_ROOT/lib/node_modules/npm/bin/npx-cli.js"

log() {
    echo "[amitia-node-prepare] $*"
}

error() {
    echo "[amitia-node-prepare] ERROR: $*" >&2
    exit 1
}

prepare_node() {
    if [[ ! -d "$NODE_ROOT" ]]; then
        error "Node root directory not found: $NODE_ROOT"
    fi

    if [[ ! -x "$NODE_BIN" ]]; then
        error "Node binary not found or not executable: $NODE_BIN"
    fi

    if [[ ! -f "$NPM_CLI" ]]; then
        error "npm CLI not found: $NPM_CLI"
    fi

    if [[ ! -f "$NPX_CLI" ]]; then
        error "npx CLI not found: $NPX_CLI"
    fi

    log "Node runtime prepared successfully at $NODE_ROOT"
}

prepare_node
