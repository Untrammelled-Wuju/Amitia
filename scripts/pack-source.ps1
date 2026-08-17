# 此文件为工作区代码打包工具，不可更改文件内任何内容，除非用户允许
param(
    [string]$OutputName = "U-Ai-source"
)

$scriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $PSCommandPath }
$workspace = Split-Path -Parent $scriptDir
$parentDir = Split-Path $workspace -Parent
$folderName = Split-Path $workspace -Leaf
$outputFile = Join-Path $workspace "$OutputName.tar.gz"
$tempDir = Join-Path $env:TEMP "pack-source-$([guid]::NewGuid().ToString('N'))"

if (Test-Path $outputFile) { Remove-Item $outputFile -Force }

$excludes = @(
    "--exclude=node_modules"
    "--exclude=.git"
    "--exclude=*.exe"
    "--exclude=*.log"
    "--exclude=desktop/dist"
    "--exclude=desktop/build"
    "--exclude=desktop/release"
    "--exclude=front/dist"
    "--exclude=mobile_app/build"
    "--exclude=mobile_app/android/app/build"
    "--exclude=mobile_app/android/amitia-runtime/build"
    "--exclude=mobile_app/android/.gradle"
    "--exclude=backend/pkg/gameplugin/sdk/game-plugin/dist"
    "--exclude=*.db"
    "--exclude=*.db-shm"
    "--exclude=*.db-wal"
    "--exclude=*.db-journal"
    "--exclude=qdrant/storage"
    "--exclude=surrealdb/surreal.exe"
    "--exclude=desktop/release"
    "--exclude=desktop/build"
    "--exclude=desktop/dist-types"
    "--exclude=desktop/resources/core"
    "--exclude=sdk/plugin-sdk/dist"
    "--exclude=sdk/plugin-sdk/node_modules"
    "--exclude=*.pyc"
    "--exclude=__pycache__"
    "--exclude=.DS_Store"
    "--exclude=Thumbs.db"
    "--exclude=*.bak"
    "--exclude=*.tmp"
    "--exclude=*.orig"
    "--exclude=.vscode"
    "--exclude=.idea"
    "--exclude=.env"
    "--exclude=.env.local"
    "--exclude=.publish-config.json"
    "--exclude=backend/data"
    "--exclude=backend/cmd/data"
    "--exclude=data"
    "--exclude=logs"
    "--exclude=runtime/out"
    "--exclude=backend/server_linux_amd64"
    "--exclude=backend/server_linux_arm64"
    "--exclude=backend/server"
    "--exclude=backend/surrealdb/surreal.zip"
    "--exclude=backend/qdrant/qdrant.zip"
    "--exclude=backend/node/node.exe.zip"
    "--exclude=desktop/resources/qdrant/qdrant.zip"
    "--exclude=desktop/resources/surrealdb/surrealdb/surreal.zip"
    "--exclude=desktop/resources/surrealdb/surreal.zip"
    "--exclude=desktop/resources/core/node/node.zip"
    "--exclude=mobile_app/android/app/src/main/assets/runtime-package"
    "--exclude=backend/server.exe"
    "--exclude=backend/server.exe~"
    "--exclude=backend/cmd/server/server.exe"
    "--exclude=backend/cmd/server/backend.exe"
    "--exclude=backend/cmd/server/backend"
    "--exclude=backend/amitia-ext.exe"
    "--exclude=backend/amitiax.exe"
    "--exclude=backend/extension.test.exe"
    "--exclude=backend/kernel.test.exe"
    "--exclude=backend/legacy-package-migrate.exe"
    "--exclude=backend/worker.test.exe"
    "--exclude=backend/server_*.exe"
    "--exclude=*.tar"
    "--exclude=AmitiaData"
)

Set-Location $parentDir

Write-Host "Packing source: $folderName -> $outputFile"
$startTime = Get-Date

& tar -czf $outputFile @excludes $folderName

if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: tar command failed with exit code $LASTEXITCODE" -ForegroundColor Red
    if (Test-Path $tempDir) { Remove-Item $tempDir -Recurse -Force }
    exit 1
}

$elapsed = (Get-Date) - $startTime
$size = (Get-Item $outputFile).Length

Write-Host "Done in $($elapsed.ToString('mm\:ss'))"
Write-Host "Output: $outputFile"
Write-Host "Size: $([math]::Round($size / 1MB, 2)) MB"

Write-Host "`nValidating archive contents..."

if (-not (Test-Path $tempDir)) {
    New-Item -ItemType Directory -Path $tempDir | Out-Null
}

try {
    & tar -tzf $outputFile | Out-File -FilePath "$tempDir/filelist.txt" -Encoding utf8
    if ($LASTEXITCODE -ne 0) {
        Write-Host "ERROR: failed to list archive contents" -ForegroundColor Red
        exit 1
    }

    $violations = @()

    $sensitivePatterns = @(
        "node_modules/",
        "\.git/",
        "\.exe$",
        "\.log$",
        "\.db$",
        "\.db-shm$",
        "\.db-wal$",
        "\.db-journal$",
        "qdrant/storage/",
        "surreal\.exe$",
        "desktop/release/",
        "desktop/build/",
        "desktop/dist-types/",
        "desktop/resources/core/",
        "\.pyc$",
        "__pycache__/",
        "\.env$",
        "\.env\.local$",
        "\.publish-config\.json$",
        "backend/data/",
        "backend/cmd/data/",
        "/data/",
        "/logs/"
    )

    $fileList = Get-Content "$tempDir/filelist.txt"
    foreach ($file in $fileList) {
        foreach ($pattern in $sensitivePatterns) {
            if ($file -match $pattern) {
                $violations += "$file (matched: $pattern)"
                break
            }
        }
    }

    if ($violations.Count -gt 0) {
        Write-Host "`nWARNING: archive contains potentially sensitive files:" -ForegroundColor Yellow
        foreach ($v in $violations) {
            Write-Host "  - $v" -ForegroundColor Yellow
        }
        Write-Host "`nTotal violations: $($violations.Count)" -ForegroundColor Yellow
    } else {
        Write-Host "Validation passed: no sensitive files detected" -ForegroundColor Green
    }

    $totalFiles = $fileList.Count
    Write-Host "Total files in archive: $totalFiles"
} finally {
    if (Test-Path $tempDir) {
        Remove-Item $tempDir -Recurse -Force
        Write-Host "Cleaned up temporary files"
    }
}
