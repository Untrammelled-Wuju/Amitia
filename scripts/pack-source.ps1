param(
    [string]$OutputName = "U-Ai-source"
)

$scriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $PSCommandPath }
$workspace = Split-Path -Parent $scriptDir
$parentDir = Split-Path $workspace -Parent
$folderName = Split-Path $workspace -Leaf
$outputFile = Join-Path $workspace "$OutputName.tar.gz"

Set-Location $workspace

if (-not (Test-Path (Join-Path $workspace ".git"))) {
    Write-Error "Not a git repository. Cannot use git archive."
    exit 1
}

Write-Host "Scanning for polluted paths..."
$pollutedPatterns = @(
    '^[A-Za-z]:[/\\]',
    '\\[A-Za-z]:[/\\]',
    '[^\x00-\x7F]'
)

$trackedFiles = & git ls-files -co --exclude-standard 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Error "git ls-files failed"
    exit 1
}

$pollutedFiles = @()
foreach ($f in $trackedFiles) {
    foreach ($pattern in $pollutedPatterns) {
        if ($f -match $pattern) {
            $pollutedFiles += $f
            break
        }
    }
}

if ($pollutedFiles.Count -gt 0) {
    Write-Error "Polluted paths found in $($pollutedFiles.Count) file(s):"
    foreach ($pf in $pollutedFiles) {
        Write-Error "  $pf"
    }
    Write-Error "Aborting pack due to source pollution."
    exit 1
}

$buildLogTmpPatterns = @(
    '^build[/\\]',
    '^tmp[/\\]',
    '[/\\]build[/\\]',
    '[/\\]tmp[/\\]',
    '[/\\]logs?[/\\]',
    '\.log$'
)

$contaminatedFiles = @()
foreach ($f in $trackedFiles) {
    foreach ($pattern in $buildLogTmpPatterns) {
        if ($f -match $pattern) {
            $contaminatedFiles += $f
            break
        }
    }
}

if ($contaminatedFiles.Count -gt 0) {
    Write-Error "Build/log/tmp contamination found in $($contaminatedFiles.Count) file(s):"
    foreach ($cf in $contaminatedFiles) {
        Write-Error "  $cf"
    }
    Write-Error "Aborting pack due to contamination."
    exit 1
}

Write-Host "No pollution detected. Packing via git archive..."

if (Test-Path $outputFile) { Remove-Item $outputFile -Force }

$tempTar = Join-Path $workspace "$OutputName.tar"
if (Test-Path $tempTar) { Remove-Item $tempTar -Force }

& git archive --format=tar --prefix="$folderName/" HEAD > $tempTar 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Error "git archive failed"
    if (Test-Path $tempTar) { Remove-Item $tempTar -Force }
    exit 1
}

& gzip -f $tempTar
if ($LASTEXITCODE -ne 0) {
    Write-Error "gzip failed"
    if (Test-Path $tempTar) { Remove-Item $tempTar -Force }
    exit 1
}

$finalTarGz = "$tempTar.gz"
if (Test-Path $finalTarGz) {
    Move-Item $finalTarGz $outputFile -Force
}

Write-Host "Verifying archive contents..."
$verifyOutput = & tar -tzf $outputFile 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Error "Archive verification failed"
    exit 1
}

$verifyPolluted = @()
foreach ($entry in $verifyOutput) {
    foreach ($pattern in $pollutedPatterns) {
        if ($entry -match $pattern) {
            $verifyPolluted += $entry
            break
        }
    }
}

if ($verifyPolluted.Count -gt 0) {
    Write-Error "Polluted paths found in archive:"
    foreach ($vp in $verifyPolluted) {
        Write-Error "  $vp"
    }
    Write-Error "Archive verification failed."
    exit 1
}

$size = (Get-Item $outputFile).Length
Write-Host "Done. Output: $outputFile"
Write-Host "Size: $([math]::Round($size / 1MB, 2)) MB"
Write-Host "Entries: $($verifyOutput.Count)"
