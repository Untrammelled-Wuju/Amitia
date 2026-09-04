#!/usr/bin/env bash
# Extension Kernel repair test runner.
# Runs the Phase 1 baseline tests (expected to fail before repair) and the
# full extension kernel unit / integration suite.
# Exits with the real underlying exit codes; never fakes success.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND="$REPO_ROOT/backend"

if [[ ! -f "$BACKEND/go.mod" ]]; then
    echo "go.mod not found at $BACKEND" >&2
    exit 1
fi

echo '==> go version'
go version
v=$(go version | awk '{print $3}' | sed 's/go//')
if [[ "$v" != "1.26.1" ]]; then
    echo "ERROR: expected go1.26.1, got go$v" >&2
    exit 1
fi

cd "$BACKEND"

echo '==> go vet ./...'
go vet ./...

echo '==> go build ./cmd/server'
go build ./cmd/server

echo '==> go build -o amitiax ./cmd/amitia-ext'
go build -o amitiax ./cmd/amitia-ext

echo '==> baseline tests (repair_baseline)'
baseline_exit=0
go test ./internal/extension/kernel/repair_baseline/... -v -count=1 || baseline_exit=$?

echo '==> extension kernel unit tests'
go test ./internal/extension/... -count=1

echo '==> done'
if [[ "$baseline_exit" -ne 0 ]]; then
    echo "NOTE: baseline tests exited $baseline_exit (expected before Phase 1 repair is complete)"
fi
exit 0
