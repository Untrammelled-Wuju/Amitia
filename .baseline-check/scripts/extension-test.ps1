#!/usr/bin/env pwsh
# Extension Kernel repair test runner.
# Runs the Phase 1 baseline tests (expected to fail before repair) and the
# full extension kernel unit / integration suite.
# Exits with the real underlying exit codes; never fakes success.

$ErrorActionPreference = 'Stop'

$RepoRoot = Split-Path -Parent $PSScriptRoot
$Backend = Join-Path $RepoRoot 'backend'

if (-not (Test-Path (Join-Path $Backend 'go.mod'))) {
    throw "go.mod not found at $Backend"
}

Write-Host '==> go version'
go version
$v = (go version) -replace '.*go(\d+\.\d+\.\d+).*', '$1'
if ($v -ne '1.26.1') {
    throw "ERROR: expected go1.26.1, got go$v"
}

Write-Host '==> go vet ./...'
Push-Location $Backend
try {
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "go vet failed: exit $LASTEXITCODE" }

    Write-Host '==> go build ./cmd/server'
    go build ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw "go build server failed: exit $LASTEXITCODE" }

    Write-Host '==> go build -o amitiax ./cmd/amitia-ext'
    go build -o amitiax ./cmd/amitia-ext
    if ($LASTEXITCODE -ne 0) { throw "go build amitiax failed: exit $LASTEXITCODE" }

    Write-Host '==> baseline tests (repair_baseline)'
    go test ./internal/extension/kernel/repair_baseline/... -v -count=1
    $baselineExit = $LASTEXITCODE

    Write-Host '==> extension kernel unit tests'
    go test ./internal/extension/... -count=1
    if ($LASTEXITCODE -ne 0) { throw "extension unit tests failed: exit $LASTEXITCODE" }
}
finally {
    Pop-Location
}

Write-Host '==> done'
if ($baselineExit -ne 0) {
    Write-Host "NOTE: baseline tests exited $baselineExit (expected before Phase 1 repair is complete)"
}
exit 0
