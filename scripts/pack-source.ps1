param(
    [string]$OutputName = "U-Ai-source"
)

$scriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $PSCommandPath }
$workspace = Split-Path -Parent $scriptDir
$outputFile = Join-Path $workspace "$OutputName.tar.gz"

if (-not (Test-Path (Join-Path $workspace ".git"))) {
    Write-Error "No .git directory found in $workspace. pack-source requires a git repository."
    exit 1
}

if (Test-Path $outputFile) { Remove-Item $outputFile -Force }

Set-Location $workspace

Write-Host "Packing source via git archive: $workspace -> $outputFile"
$startTime = Get-Date

git archive --format tar HEAD | & gzip > $outputFile

if ($LASTEXITCODE -ne 0) {
    Write-Error "git archive failed with exit code $LASTEXITCODE"
    if (Test-Path $outputFile) { Remove-Item $outputFile -Force }
    exit $LASTEXITCODE
}

$elapsed = (Get-Date) - $startTime
$size = (Get-Item $outputFile).Length

Write-Host "Done in $($elapsed.ToString('mm\:ss'))"
Write-Host "Output: $outputFile"
Write-Host "Size: $([math]::Round($size / 1MB, 2)) MB"
