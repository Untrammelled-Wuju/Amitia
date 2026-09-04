param(
    [ValidateSet("quick","integration","security","migration","frontend","electron","all")]
    [string]$Group = "quick",
    [switch]$Verbose
)

$ErrorActionPreference = "Continue"
$scriptDir = $PSScriptRoot

$scripts = @{
    "quick"       = @("run-extension-unit.ps1")
    "integration" = @("run-extension-integration.ps1", "run-mcp-integration.ps1")
    "security"    = @("run-extension-security.ps1")
    "migration"   = @("run-extension-migration.ps1")
    "frontend"    = @("run-extension-frontend.ps1")
    "electron"    = @("run-extension-electron.ps1")
}

$exitCode = 0

if ($Group -eq "all") {
    $allGroups = @("quick", "integration", "security", "migration", "frontend", "electron")
} else {
    $allGroups = @($Group)
}

foreach ($grp in $allGroups) {
    Write-Host ""
    Write-Host "=== CI Group: $grp ===" -ForegroundColor Cyan

    foreach ($scriptName in $scripts[$grp]) {
        $scriptPath = Join-Path $scriptDir $scriptName
        $args = @("-File", $scriptPath)
        if ($Verbose) { $args += "-Verbose" }

        $proc = Start-Process -FilePath "pwsh" -ArgumentList $args -NoNewWindow -Wait -PassThru
        if ($proc.ExitCode -ne 0) {
            $exitCode = $proc.ExitCode
            if ($grp -ne "electron") {
                Write-Host "Group '$grp' FAILED (script: $scriptName)" -ForegroundColor Red
            }
        }
    }
}

if ($exitCode -ne 0) {
    Write-Host ""
    Write-Host "Some test groups failed. Check output above." -ForegroundColor Red
} else {
    Write-Host ""
    Write-Host "All test groups passed." -ForegroundColor Green
}

exit $exitCode
