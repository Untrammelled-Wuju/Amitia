# 使用 git archive 基于 git tracked 白名单打包源码，彻底避免 node_modules/.git 等开发依赖打入包
param(
    [string]$OutputName = "U-Ai-source"
)

$scriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $PSCommandPath }
$workspace = Split-Path -Parent $scriptDir
$outputFile = Join-Path $workspace "$OutputName.zip"
$tempDir = Join-Path $env:TEMP "pack-source-$([guid]::NewGuid().ToString('N'))"

if (Test-Path $outputFile) { Remove-Item $outputFile -Force }

function Add-ZipEntry {
    param([string]$ZipPath, [string]$SourcePath)
    $src = [System.IO.Compression.ZipFile]::Open($SourcePath, 'Read')
    try {
        $dst = [System.IO.Compression.ZipFile]::Open($ZipPath, 'Update')
        try {
            foreach ($entry in $src.Entries) {
                $ms = New-Object System.IO.MemoryStream
                $entry.Open().CopyTo($ms)
                $ms.Position = 0
                $newEntry = $dst.CreateEntry($entry.FullName)
                $entryStream = $newEntry.Open()
                $ms.CopyTo($entryStream)
                $entryStream.Dispose()
                $ms.Dispose()
            }
        } finally {
            $dst.Dispose()
        }
    } finally {
        $src.Dispose()
    }
}

function Get-ZipEntryCount {
    param([string]$ZipPath)
    $zip = [System.IO.Compression.ZipFile]::OpenRead($ZipPath)
    try {
        return $zip.Entries.Count
    } finally {
        $zip.Dispose()
    }
}

Add-Type -AssemblyName System.IO.Compression.FileSystem 2>$null

Set-Location $workspace

Write-Host "Packing source (git archive) -> $outputFile"
$startTime = Get-Date

if (-not (Test-Path $tempDir)) { New-Item -ItemType Directory -Path $tempDir | Out-Null }

try {
    $mainZip = Join-Path $tempDir "main.zip"
    git archive --format=zip -o $mainZip HEAD 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "git archive failed for main repo (exit code $LASTEXITCODE)"
    }

    Copy-Item $mainZip $outputFile -Force

    $submodules = git config --file .gitmodules --get-regexp path 2>&1
    if ($submodules) {
        foreach ($line in $submodules) {
            $smPath = ($line -split ' ', 2)[1]
            $smFullPath = Join-Path $workspace $smPath
            $gitFile = Join-Path $smFullPath ".git"
            if (-not (Test-Path $gitFile)) { continue }

            Write-Host "  Packing submodule: $smPath"
            $smZip = Join-Path $tempDir "$($smPath -replace '[\\/]','_').zip"
            Push-Location $smFullPath
            try {
                git archive --prefix="$smPath/" --format=zip -o $smZip HEAD 2>&1
                if ($LASTEXITCODE -ne 0) {
                    Write-Host "  WARNING: git archive failed for submodule $smPath" -ForegroundColor Yellow
                    continue
                }

                Add-ZipEntry -ZipPath $outputFile -SourcePath $smZip
            } finally {
                Pop-Location
            }
        }
    }

    $elapsed = (Get-Date) - $startTime
    $size = (Get-Item $outputFile).Length
    $fileCount = Get-ZipEntryCount -ZipPath $outputFile

    Write-Host "Done in $($elapsed.ToString('mm\:ss'))"
    Write-Host "Output: $outputFile"
    Write-Host "Files: $fileCount"
    Write-Host "Size: $([math]::Round($size / 1MB, 2)) MB"
} finally {
    if (Test-Path $tempDir) { Remove-Item $tempDir -Recurse -Force }
}
