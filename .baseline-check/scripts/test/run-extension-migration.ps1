param(
    [switch]$Verbose
)

$ErrorActionPreference = "Stop"
$backendDir = Join-Path $PSScriptRoot "..\..\backend"

Write-Host "=== Migration Baseline Tests ===" -ForegroundColor Cyan

$migrationPatterns = @(
    "TestMigration",
    "TestLegacy_AgentSkill_PseudoSkill",
    "TestLegacy_MCP_ToolSync",
    "TestLegacy_MCP_ServerCap",
    "TestLegacy_Registry_ScopeEnabled",
    "TestLegacy_Tool_GenerateBaseline",
    "TestLegacyAdapter",
    "TestPackageRecovery",
    "TestUnifiedLifecycle"
)

$goArgs = @("test", "-count=1", "-timeout", "180s")
if ($Verbose) { $goArgs += "-v" }
$goArgs += @("-run", ($migrationPatterns -join "|"))

$packages = @(
    "./internal/extension/...",
    "./internal/mcp/..."
)

$failed = @()

foreach ($pkg in $packages) {
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

Write-Host "All migration baseline tests passed." -ForegroundColor Green
