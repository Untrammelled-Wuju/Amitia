#!/usr/bin/env bash
set -euo pipefail

NODE_ROOT="${NODE_ROOT:-/opt/amitia/node}"
NODE_BIN="$NODE_ROOT/bin/node"
NPM_CLI="$NODE_ROOT/lib/node_modules/npm/bin/npm-cli.js"
NPX_CLI="$NODE_ROOT/lib/node_modules/npm/bin/npx-cli.js"

log() {
    echo "[amitia-node-probe] $*"
}

error() {
    echo "[amitia-node-probe] ERROR: $*" >&2
    exit 1
}

probe_node() {
    if [[ ! -d "$NODE_ROOT" ]]; then
        error "Node root directory not found: $NODE_ROOT"
    fi

    if [[ ! -x "$NODE_BIN" ]]; then
        error "Node binary not found or not executable: $NODE_BIN"
    fi

    local node_version
    node_version=$("$NODE_BIN" --version 2>/dev/null) || error "Failed to get Node version"
    log "Node version: $node_version"

    if [[ ! -f "$NPM_CLI" ]]; then
        error "npm CLI not found: $NPM_CLI"
    fi

    if [[ ! -f "$NPX_CLI" ]]; then
        error "npx CLI not found: $NPX_CLI"
    fi

    log "Node runtime probe passed"
}

probe_node
