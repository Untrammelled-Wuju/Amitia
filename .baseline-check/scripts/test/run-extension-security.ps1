param(
    [switch]$Verbose
)

$ErrorActionPreference = "Stop"
$backendDir = Join-Path $PSScriptRoot "..\..\backend"

Write-Host "=== Extension Security Tests ===" -ForegroundColor Cyan

$securityPatterns = @(
    "TestPackageArchive",
    "TestSecurity",
    "TestSecret",
    "TestPermission",
    "TestPathTraversal",
    "TestPathSecurity",
    "TestZIP",
    "TestSignature",
    "TestRedact",
    "TestScope",
    "TestIsolation",
    "TestSensor",
    "TestCapability",
    "TestCircuit",
    "TestUnsafe",
    "TestReject",
    "TestLimit",
    "TestForbidden"
)

$goArgs = @("test", "-count=1", "-timeout", "180s")
if ($Verbose) { $goArgs += "-v" }
$goArgs += @("-run", ($securityPatterns -join "|"))

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

Write-Host "All security tests passed." -ForegroundColor Green
