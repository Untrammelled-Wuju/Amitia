param(
    [switch]$Verbose,
    [string]$Filter = ""
)

$ErrorActionPreference = "Stop"
$backendDir = Join-Path $PSScriptRoot "..\..\backend"

Write-Host "=== MCP Integration Tests ===" -ForegroundColor Cyan

$goArgs = @("test", "-count=1", "-timeout", "120s")
if ($Verbose) { $goArgs += "-v" }
if ($Filter) { $goArgs += @("-run", $Filter) }

$mcpPackages = @(
    "./internal/mcp/...",
    "./internal/mcpapi/..."
)

$failed = @()

foreach ($pkg in $mcpPackages) {
    Write-Host "Testing: $pkg" -ForegroundColor Yellow
    $proc = Start-Process -FilePath "go" -ArgumentList ($goArgs + $pkg) -NoNewWindow -Wait -PassThru -WorkingDirectory $backendDir
    if ($proc.ExitCode -ne 0) {
        $failed += $pkg
    }
}

if ($failed.Count -gt 0) {
    Write-Host "FAILED packages:" -ForegroundColor Red
    $failed | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
    exit 1
}

Write-Host "All MCP integration tests passed." -ForegroundColor Green
