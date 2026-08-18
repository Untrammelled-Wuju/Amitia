# 此文件为工作区代码打包工具，不可更改文件内任何内容，除非用户允许
param(
    [string]$OutputName = "U-Ai-source",
    [switch]$IncludeUntracked
)

$scriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $PSCommandPath }
$workspace = Split-Path -Parent $scriptDir
$parentDir = Split-Path $workspace -Parent
$folderName = Split-Path $workspace -Leaf
$outputFile = Join-Path $workspace "$OutputName.tar.gz"

if (Test-Path $outputFile) { Remove-Item $outputFile -Force }

Set-Location $workspace

$gitAvailable = $null -ne (Get-Command git -ErrorAction SilentlyContinue)
if (-not $gitAvailable) {
    Write-Error "git command not found. pack-source requires git."
    exit 1
}

$gitCheck = git rev-parse --is-inside-work-tree 2>&1
if ($gitCheck -ne "true") {
    Write-Error "Not inside a git working tree. pack-source requires git repository."
    exit 1
}

Write-Host "Packing source: $folderName -> $outputFile"
$startTime = Get-Date

if ($IncludeUntracked) {
    $allowedFiles = git ls-files -co --exclude-standard 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Error "git ls-files failed: $allowedFiles"
        exit 1
    }
} else {
    $allowedFiles = git ls-files 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Error "git ls-files failed: $allowedFiles"
        exit 1
    }
}

if ($allowedFiles.Count -eq 0) {
    Write-Error "No files to pack. Allowed list is empty."
    exit 1
}

$illegalPatterns = @(
    '^C:', '^D:', '^E:', '^F:',
    '\\build\\', '/build/',
    '\\logs\\', '/logs/',
    '\\tmp\\', '/tmp/',
    '\\snapshots\\', '/snapshots/',
    'build_errors\.txt$', 'errors\.txt$', 'vet_errors\.txt$',
    '\.db$', '\.db-shm$', '\.db-wal$', '\.db-journal$',
    '\.exe$', '\.pyc$', '\.bak$', '\.tmp$', '\.orig$',
    'node_modules', '\.git$', '\.git/',
    'desktop/dist', 'desktop/build', 'desktop/release', 'desktop/dist-types',
    'desktop/resources/core',
    'front/dist',
    'mobile_app/build', 'mobile_app/android/app/build',
    'mobile_app/android/amitia-runtime/build', 'mobile_app/android/.gradle',
    'backend/pkg/gameplugin/sdk/game-plugin/dist',
    'sdk/plugin-sdk/dist', 'sdk/plugin-sdk/node_modules',
    'qdrant/storage', 'surrealdb/surreal\.exe',
    'backend/data', 'backend/cmd/data', 'data',
    'runtime/out',
    'backend/server_linux_amd64', 'backend/server_linux_arm64', 'backend/server$',
    'backend/surrealdb/surreal\.zip', 'backend/qdrant/qdrant\.zip',
    'backend/node/node\.exe\.zip',
    'desktop/resources/qdrant/qdrant\.zip',
    'desktop/resources/surrealdb/surrealdb/surreal\.zip',
    'desktop/resources/surrealdb/surreal\.zip',
    'desktop/resources/core/node/node\.zip',
    'mobile_app/android/app/src/main/assets/runtime-package',
    'backend/server\.exe', 'backend/server\.exe~',
    'backend/cmd/server/server\.exe', 'backend/cmd/server/backend\.exe',
    'backend/cmd/server/backend', 'backend/amitia-ext\.exe',
    'backend/amitiax\.exe', 'backend/extension\.test\.exe',
    'backend/kernel\.test\.exe', 'backend/legacy-package-migrate\.exe',
    'backend/worker\.test\.exe', 'backend/server_.*\.exe',
    '\.tar$', '\.tar\.gz$', '\.zip$',
    'AmitiaData',
    '\.publish-config\.json$',
    '\.env$', '\.env\.local$',
    '__pycache__', '\.DS_Store$', 'Thumbs\.db$',
    '\.vscode', '\.idea'
)

$illegalFiles = @()
$filteredFiles = @()
foreach ($file in $allowedFiles) {
    $isIllegal = $false
    foreach ($pattern in $illegalPatterns) {
        if ($file -match $pattern) {
            $isIllegal = $true
            $illegalFiles += $file
            break
        }
    }
    if (-not $isIllegal) {
        $filteredFiles += $file
    }
}

if ($illegalFiles.Count -gt 0) {
    Write-Error "FAIL CLOSED: Found $($illegalFiles.Count) illegal paths in allowed list:"
    foreach ($f in $illegalFiles) {
        Write-Error "  - $f"
    }
    exit 1
}

if ($filteredFiles.Count -eq 0) {
    Write-Error "No files remain after filtering. Aborting."
    exit 1
}

$tempFile = [System.IO.Path]::GetTempFileName()
$filteredFiles | Out-File -FilePath $tempFile -Encoding UTF8

Set-Location $parentDir
& tar -czf $outputFile -C $workspace --files-from=$tempFile 2>&1
$tarExitCode = $LASTEXITCODE
Remove-Item $tempFile -Force

if ($tarExitCode -ne 0) {
    Write-Error "tar command failed with exit code $tarExitCode"
    if (Test-Path $outputFile) { Remove-Item $outputFile -Force }
    exit 1
}

$elapsed = (Get-Date) - $startTime
$size = (Get-Item $outputFile).Length

Write-Host "Verifying archive contents..."
$tarList = & tar -tf $outputFile 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Error "Failed to list archive contents. Deleting corrupted archive."
    Remove-Item $outputFile -Force
    exit 1
}

foreach ($item in $tarList) {
    foreach ($pattern in $illegalPatterns) {
        if ($item -match $pattern) {
            Write-Error "FAIL CLOSED: Archive contains illegal item: $item"
            Remove-Item $outputFile -Force
            exit 1
        }
    }
}

Write-Host "Done in $($elapsed.ToString('mm\:ss'))"
Write-Host "Output: $outputFile"
Write-Host "Size: $([math]::Round($size / 1MB, 2)) MB"
Write-Host "Files: $($tarList.Count)"
