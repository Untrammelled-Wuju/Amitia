param(
    [switch]$Verbose
)

$ErrorActionPreference = "Stop"
$frontDir = Join-Path $PSScriptRoot "..\..\front"

Write-Host "=== Frontend Extension Baseline Tests ===" -ForegroundColor Cyan

$vitestArgs = @("vitest", "run", "--config", "vitest.config.ts")
if ($Verbose) {
    $vitestArgs += "--reporter=verbose"
}

$proc = Start-Process -FilePath "npx" -ArgumentList ($vitestArgs + "src/__tests__/extensions.legacy.baseline.test.ts") -NoNewWindow -Wait -PassThru -WorkingDirectory $frontDir

if ($proc.ExitCode -ne 0) {
    Write-Host "Frontend tests FAILED." -ForegroundColor Red
    exit $proc.ExitCode
}

Write-Host "All frontend extension baseline tests passed." -ForegroundColor Green
