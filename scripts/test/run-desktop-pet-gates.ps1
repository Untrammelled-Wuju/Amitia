# Desktop Pet Test Gate Runner for Windows
# Tests execute in the same order as CI workflow

param(
    [ValidateSet("format", "unit", "contract", "security", "migration", "e2e", "all")]
    [string]$Mode = "all"
)

$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent (Split-Path -Parent $scriptDir)

$passed = 0
$failed = 0
$skipped = 0
$results = @{}

function Invoke-Gate {
    param(
        [string]$Name,
        [scriptblock]$Action,
        [switch]$Optional
    )
    Write-Output ""
    Write-Output "=== GATE: $Name ==="
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    try {
        $output = & $Action 2>&1
        $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { $LASTEXITCODE }
        $sw.Stop()
        if ($exitCode -eq 0) {
            Write-Output "  [PASS] $Name ($($sw.ElapsedMilliseconds)ms)"
            $script:passed++
            $results[$Name] = @{ Status = "PASS"; DurationMs = $sw.ElapsedMilliseconds }
            return $true
        } else {
            Write-Output "  [FAIL] $Name (exit $exitCode)"
            if ($output) { Write-Output $output }
            $script:failed++
            $results[$Name] = @{ Status = "FAIL"; DurationMs = $sw.ElapsedMilliseconds }
            return $false
        }
    } catch {
        $sw.Stop()
        if ($Optional) {
            Write-Output "  [SKIP] $Name - $($_.Exception.Message)"
            $script:skipped++
            $results[$Name] = @{ Status = "SKIP"; DurationMs = $sw.ElapsedMilliseconds }
            return $true
        }
        Write-Output "  [FAIL] $Name - $($_.Exception.Message)"
        $script:failed++
        $results[$Name] = @{ Status = "FAIL"; DurationMs = $sw.ElapsedMilliseconds; Error = $_.Exception.Message }
        return $false
    }
}

Push-Location $repoRoot

try {
    if ($Mode -eq "format" -or $Mode -eq "all") {
        Invoke-Gate "format" {
            $gofmtOutput = & gofmt -l backend/internal/desktoppet backend/cmd/server 2>&1
            if ($gofmtOutput) {
                Write-Output $gofmtOutput
                $LASTEXITCODE = 1
            }
        }
    }

    if ($Mode -eq "unit" -or $Mode -eq "all") {
        Invoke-Gate "unit-test" {
            go test ./backend/internal/desktoppet/... -count=1 2>&1 | Out-String
        } -Optional
    }

    if ($Mode -eq "contract" -or $Mode -eq "all") {
        Invoke-Gate "contract" {
            go test ./backend/internal/desktoppet/contracts/... -count=1 2>&1 | Out-String
        } -Optional
    }

    if ($Mode -eq "security" -or $Mode -eq "all") {
        Invoke-Gate "security" {
            & "$scriptDir\..\audit\desktop_pet_forbidden_patterns.ps1"
        }
    }

    if ($Mode -eq "migration" -or $Mode -eq "all") {
        Invoke-Gate "migration-test" {
            go test ./backend/internal/migration/... -count=1 2>&1 | Out-String
        } -Optional
    }
} finally {
    Pop-Location
}

Write-Output ""
Write-Output "=== RESULTS ==="
Write-Output "Passed:  $passed"
Write-Output "Failed:  $failed"
Write-Output "Skipped: $skipped"

if ($failed -gt 0) {
    Write-Output ""
    Write-Output "FAILED GATES:"
    foreach ($r in $results.GetEnumerator()) {
        if ($r.Value.Status -eq "FAIL") {
            Write-Output "  - $($r.Key)"
        }
    }
    exit 1
}

Write-Output ""
Write-Output "All gates passed."
exit 0
