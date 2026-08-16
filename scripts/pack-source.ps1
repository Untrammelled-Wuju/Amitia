param(
    [string]$OutputName = "U-Ai-source"
)

$scriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $PSCommandPath }
$workspace = Split-Path -Parent $scriptDir
$outputFile = Join-Path $workspace "$OutputName.tar.gz"

if (Test-Path $outputFile) { Remove-Item $outputFile -Force }

Set-Location $workspace

if (-not (Test-Path (Join-Path $workspace ".git"))) {
    Write-Error "Not a git repository. git archive requires a git repo."
    exit 1
}

Write-Host "Packing source via git archive: $workspace -> $outputFile"
$startTime = Get-Date

git archive --format=tar.gz -o $outputFile HEAD

$elapsed = (Get-Date) - $startTime
$size = (Get-Item $outputFile).Length

Write-Host "Done in $($elapsed.ToString('mm\:ss'))"
Write-Host "Output: $outputFile"
Write-Host "Size: $([math]::Round($size / 1MB, 2)) MB"
