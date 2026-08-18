# 此文件为工作区代码打包工具；用户已授权本次修复将其改为 Git 感知的源码允许列表打包。
param(
    [string]$OutputName = "U-Ai-source"
)

$ErrorActionPreference = "Stop"
$scriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $PSCommandPath }
$workspace = (Resolve-Path (Split-Path -Parent $scriptDir)).Path
$parentDir = Split-Path $workspace -Parent
$folderName = Split-Path $workspace -Leaf
$outputFile = Join-Path $workspace "$OutputName.tar.gz"
$listFile = Join-Path ([System.IO.Path]::GetTempPath()) ("amitia-source-files-" + [guid]::NewGuid().ToString("N") + ".txt")

function Test-SourcePathUnsafe([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path)) { return $true }
    $p = $Path.Replace('\\', '/').TrimStart([char[]]@('.', '/'))
    if ($p -match '(^|/)(\.\.|\.)($|/)') { return $true }
    if ($p -match '(^|/)[A-Za-z][\x3A\uFF1A\uF03A]') { return $true }
    if ($p -match '(^|/)(node_modules|logs?|tmp)(/|$)') { return $true }
    if ($p -match '(^|/)runtime/(build|out)(/|$)') { return $true }
    if ($p -match '(^|/)snapshots/tmp(/|$)') { return $true }
    if ($p -match '(^|/)desktop/(build|dist|release|dist-types)(/|$)') { return $true }
    if ($p -match '(^|/)front/dist(/|$)') { return $true }
    if ($p -match '(^|/)mobile_app/(build|android/build|android/app/build|android/amitia-runtime/build)(/|$)') { return $true }
    if ($p -match '(^|/)sdk/[^/]+/(dist|node_modules)(/|$)') { return $true }
    if ($p -match '(^|/)(build_errors|errors|vet_errors)\.txt$') { return $true }
    if ($p -match '(^|/)U-Ai-source[^/]*\.(tar|gz|zip)$') { return $true }
    return $false
}

try {
    if (Test-Path $outputFile) { Remove-Item $outputFile -Force }

    $gitRoot = (& git -C $workspace rev-parse --show-toplevel 2>$null | Select-Object -First 1)
    if ([string]::IsNullOrWhiteSpace($gitRoot)) {
        throw "pack-source: Git repository root could not be resolved"
    }
    $gitRoot = (Resolve-Path $gitRoot.Trim()).Path
    if ($gitRoot -ne $workspace) {
        throw "pack-source: expected workspace to be Git root; got '$gitRoot'"
    }

    # Include tracked files plus non-ignored untracked source. Generated/unwanted files are
    # not silently excluded: the fail-closed preflight below rejects them.
    $relativeFiles = @(& git -C $workspace ls-files --cached --others --exclude-standard)
    if ($LASTEXITCODE -ne 0) {
        throw "pack-source: git ls-files failed with exit code $LASTEXITCODE"
    }
    $relativeFiles = @($relativeFiles | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne "" } | Sort-Object -Unique)
    if ($relativeFiles.Count -eq 0) {
        throw "pack-source: no source files selected"
    }

    $unsafe = @($relativeFiles | Where-Object { Test-SourcePathUnsafe $_ })
    if ($unsafe.Count -gt 0) {
        $preview = ($unsafe | Select-Object -First 20) -join [Environment]::NewLine
        throw "pack-source: unsafe/generated paths detected; clean the workspace before packing:`n$preview"
    }

    foreach ($rel in $relativeFiles) {
        $full = Join-Path $workspace $rel
        if (-not (Test-Path -LiteralPath $full -PathType Leaf)) {
            throw "pack-source: selected file disappeared: $rel"
        }
    }

    $archivePaths = @($relativeFiles | ForEach-Object { "$folderName/$($_.Replace('\\','/'))" })
    [System.IO.File]::WriteAllLines($listFile, $archivePaths, (New-Object System.Text.UTF8Encoding($false)))

    Write-Host "Packing source allow-list: $folderName -> $outputFile"
    $startTime = Get-Date
    Push-Location $parentDir
    try {
        & tar -czf $outputFile -T $listFile
        if ($LASTEXITCODE -ne 0) {
            throw "pack-source: tar failed with exit code $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }

    $archiveEntries = @(& tar -tzf $outputFile)
    if ($LASTEXITCODE -ne 0) {
        throw "pack-source: post-pack archive listing failed"
    }
    $badEntries = @()
    foreach ($entry in $archiveEntries) {
        $normalized = $entry.Replace('\\', '/')
        if (-not $normalized.StartsWith("$folderName/")) {
            $badEntries += $entry
            continue
        }
        $relative = $normalized.Substring($folderName.Length + 1)
        if (Test-SourcePathUnsafe $relative) {
            $badEntries += $entry
        }
    }
    if ($badEntries.Count -gt 0) {
        $preview = ($badEntries | Select-Object -First 20) -join [Environment]::NewLine
        Remove-Item $outputFile -Force -ErrorAction SilentlyContinue
        throw "pack-source: archive validation rejected generated/unsafe entries:`n$preview"
    }

    if ($archiveEntries.Count -ne $archivePaths.Count) {
        Remove-Item $outputFile -Force -ErrorAction SilentlyContinue
        throw "pack-source: archive entry count mismatch (expected $($archivePaths.Count), got $($archiveEntries.Count))"
    }

    $elapsed = (Get-Date) - $startTime
    $size = (Get-Item $outputFile).Length
    Write-Host "Done in $($elapsed.ToString('mm\:ss'))"
    Write-Host "Output: $outputFile"
    Write-Host "Files: $($archiveEntries.Count)"
    Write-Host "Size: $([math]::Round($size / 1MB, 2)) MB"
} catch {
    if (Test-Path $outputFile) { Remove-Item $outputFile -Force -ErrorAction SilentlyContinue }
    Write-Error $_
    exit 1
} finally {
    Remove-Item $listFile -Force -ErrorAction SilentlyContinue
}
