# Desktop Pet Final Gate Runner for Windows
# Mirrors the required desktop-pet CI/release gates and fails closed.

param(
    [ValidateSet("format", "build", "unit", "race", "contract", "security", "migration", "frontend", "desktop", "e2e", "package", "all")]
    [string]$Mode = "all"
)

$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent (Split-Path -Parent $scriptDir)

$passed = 0
$failed = 0
$results = @{}

function Invoke-Gate {
    param(
        [string]$Name,
        [scriptblock]$Action
    )

    Write-Output ""
    Write-Output "=== GATE: $Name ==="
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    try {
        $global:LASTEXITCODE = 0
        & $Action
        $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { $LASTEXITCODE }
        if ($exitCode -ne 0) {
            throw "$Name exited with code $exitCode"
        }
        $sw.Stop()
        Write-Output "  [PASS] $Name ($($sw.ElapsedMilliseconds)ms)"
        $script:passed++
        $results[$Name] = @{ Status = "PASS"; DurationMs = $sw.ElapsedMilliseconds }
        return
    }
    catch {
        $sw.Stop()
        Write-Output "  [FAIL] $Name - $($_.Exception.Message)"
        $script:failed++
        $results[$Name] = @{ Status = "FAIL"; DurationMs = $sw.ElapsedMilliseconds; Error = $_.Exception.Message }
        return
    }
}

function Assert-LastExitCode {
    param([string]$Label)
    if ($LASTEXITCODE -ne 0) {
        throw "$Label exited with code $LASTEXITCODE"
    }
}

Push-Location $repoRoot
try {
    if ($Mode -eq "format" -or $Mode -eq "all") {
        Invoke-Gate "format" {
            $gofmtOutput = & gofmt -l backend/internal/desktoppet backend/cmd/server 2>&1
            if ($gofmtOutput) {
                Write-Output $gofmtOutput
                throw "gofmt found unformatted files"
            }
        }
    }

    if ($Mode -eq "build" -or $Mode -eq "all") {
        Invoke-Gate "backend-build-vet" {
            Push-Location "$repoRoot/backend"
            try {
                & go build ./...
                Assert-LastExitCode "go build ./..."
                & go vet ./internal/desktoppet/... ./internal/migration/... ./cmd/...
                Assert-LastExitCode "go vet"
            }
            finally {
                Pop-Location
            }
        }
    }

    if ($Mode -eq "unit" -or $Mode -eq "all") {
        Invoke-Gate "desktop-pet-unit" {
            Push-Location "$repoRoot/backend"
            try {
                & go test ./internal/desktoppet/... -count=1
                Assert-LastExitCode "desktop pet unit tests"
            }
            finally {
                Pop-Location
            }
        }
    }

    if ($Mode -eq "race" -or $Mode -eq "all") {
        Invoke-Gate "desktop-pet-race" {
            Push-Location "$repoRoot/backend"
            try {
                & go test -race ./internal/desktoppet/... -count=1
                Assert-LastExitCode "desktop pet race tests"
            }
            finally {
                Pop-Location
            }
        }
    }

    if ($Mode -eq "contract" -or $Mode -eq "all") {
        Invoke-Gate "desktop-pet-contract" {
            Push-Location "$repoRoot/backend"
            try {
                & go test ./internal/desktoppet/contracts/... -count=1
                Assert-LastExitCode "desktop pet contract tests"
            }
            finally {
                Pop-Location
            }
        }
    }

    if ($Mode -eq "security" -or $Mode -eq "all") {
        Invoke-Gate "desktop-pet-security" {
            & "$repoRoot/scripts/audit/desktop_pet_forbidden_patterns.ps1" -Paths @("backend/internal/desktoppet", "backend/cmd/server")
            Assert-LastExitCode "desktop pet static audit"
            & node "$repoRoot/scripts/audit/verify-desktop-pet-runtime-singletrack.mjs"
            Assert-LastExitCode "Runtime V2 single-track gate"
        }
    }

    if ($Mode -eq "migration" -or $Mode -eq "all") {
        Invoke-Gate "desktop-pet-migration" {
            Push-Location "$repoRoot/backend"
            try {
                & go test ./internal/migration/... -count=1
                Assert-LastExitCode "desktop pet migration tests"
            }
            finally {
                Pop-Location
            }
        }
    }

    if ($Mode -eq "frontend" -or $Mode -eq "all") {
        Invoke-Gate "front-build-test" {
            Push-Location "$repoRoot/front"
            try {
                & pnpm typecheck
                Assert-LastExitCode "front typecheck"
                & pnpm test
                Assert-LastExitCode "front test"
                & pnpm build
                Assert-LastExitCode "front build"
            }
            finally {
                Pop-Location
            }
        }
    }

    if ($Mode -eq "desktop" -or $Mode -eq "all") {
        Invoke-Gate "desktop-build-test" {
            Push-Location "$repoRoot/desktop"
            try {
                & pnpm typecheck
                Assert-LastExitCode "desktop typecheck"
                & pnpm test
                Assert-LastExitCode "desktop test"
                & pnpm build
                Assert-LastExitCode "desktop build"
                & pnpm run verify:pet-player-singleton
                Assert-LastExitCode "desktop player singleton gate"
                & pnpm run verify:desktop-pet-runtime-singletrack
                Assert-LastExitCode "desktop Runtime V2 single-track gate"
            }
            finally {
                Pop-Location
            }
        }
    }

    if ($Mode -eq "e2e" -or $Mode -eq "all") {
        Invoke-Gate "desktop-pet-electron-golden-e2e" {
            Push-Location "$repoRoot/desktop"
            try {
                & pnpm run test:electron-pet-smoke
                Assert-LastExitCode "desktop pet Electron golden E2E"
            }
            finally {
                Pop-Location
            }
        }
    }

    if ($Mode -eq "package" -or $Mode -eq "all") {
        Invoke-Gate "desktop-package" {
            Push-Location "$repoRoot/desktop"
            try {
                & pnpm run package:ci
                Assert-LastExitCode "desktop package gate"
            }
            finally {
                Pop-Location
            }
        }
    }
}
finally {
    Pop-Location
}

Write-Output ""
Write-Output "=== RESULTS ==="
Write-Output "Passed: $passed"
Write-Output "Failed: $failed"

if ($failed -gt 0) {
    Write-Output ""
    Write-Output "FAILED GATES:"
    foreach ($result in $results.GetEnumerator()) {
        if ($result.Value.Status -eq "FAIL") {
            Write-Output "  - $($result.Key)"
        }
    }
    exit 1
}

Write-Output ""
Write-Output "All required desktop-pet gates passed."
exit 0
