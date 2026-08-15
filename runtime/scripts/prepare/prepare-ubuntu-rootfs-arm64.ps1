param(
    [switch]$Offline,
    [string]$CacheDir,
    [string]$StagingDir,
    [string]$OutputDir,
    [switch]$SkipVerify,
    [switch]$DevMode
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ShScript = Join-Path $ScriptDir "prepare-ubuntu-rootfs-arm64.sh"

if (-not (Test-Path $ShScript)) {
    Write-Error "[FATAL] Canonical shell script not found: $ShScript"
    exit 1
}

$WslCheck = Get-Command wsl.exe -ErrorAction SilentlyContinue
if (-not $WslCheck) {
    Write-Error "[FATAL] WSL is not available. The canonical release authority is prepare-ubuntu-rootfs-arm64.sh and must be invoked via WSL."
    exit 1
}

if ($SkipVerify -and -not $DevMode) {
    Write-Error "[FATAL] -SkipVerify is not allowed in release mode"
    exit 1
}

$WslDistro = wsl.exe -l --quiet 2>&1 | Select-Object -First 1
if ([string]::IsNullOrWhiteSpace($WslDistro)) {
    Write-Error "[FATAL] No WSL distribution available"
    exit 1
}

Write-Host "============================================"
Write-Host " Ubuntu ARM64 Rootfs Prepare (WSL Wrapper)"
Write-Host "============================================"
Write-Host " Canonical authority: prepare-ubuntu-rootfs-arm64.sh"
Write-Host " WSL distro: $WslDistro"
Write-Host "============================================"

$WslScriptPath = wsl.exe wslpath -u ("""$ShScript""") 2>&1
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($WslScriptPath)) {
    Write-Error "[FATAL] Failed to convert script path for WSL: $WslScriptPath"
    exit 1
}

$ArgumentsList = @()
if ($Offline) { $ArgumentsList += "--offline" }
if ($DevMode) { $ArgumentsList += "--dev-mode" }
if ($SkipVerify) { $ArgumentsList += "--skip-verify" }
if (-not [string]::IsNullOrWhiteSpace($CacheDir)) {
    $WslCache = wsl.exe wslpath -u ("""$CacheDir""") 2>&1
    if ($LASTEXITCODE -ne 0) { Write-Error "[FATAL] Failed to convert cache path"; exit 1 }
    $ArgumentsList += @("--cache-dir", $WslCache)
}
if (-not [string]::IsNullOrWhiteSpace($StagingDir)) {
    $WslStaging = wsl.exe wslpath -u ("""$StagingDir""") 2>&1
    if ($LASTEXITCODE -ne 0) { Write-Error "[FATAL] Failed to convert staging path"; exit 1 }
    $ArgumentsList += @("--staging-dir", $WslStaging)
}
if (-not [string]::IsNullOrWhiteSpace($OutputDir)) {
    $WslOutput = wsl.exe wslpath -u ("""$OutputDir""") 2>&1
    if ($LASTEXITCODE -ne 0) { Write-Error "[FATAL] Failed to convert output path"; exit 1 }
    $ArgumentsList += @("--output-dir", $WslOutput)
}

Write-Host "[WSL] Invoking canonical shell script..."
Write-Host "[WSL]   $WslScriptPath"
Write-Host "[WSL]   args: $($ArgumentsList -join ' ')"

wsl.exe bash "$WslScriptPath" @ArgumentsList
$ExitCode = $LASTEXITCODE

if ($ExitCode -ne 0) {
    Write-Error "[FATAL] Canonical rootfs prepare failed with exit code $ExitCode"
    exit $ExitCode
}

Write-Host "[DONE] WSL prepare completed successfully"
exit 0
