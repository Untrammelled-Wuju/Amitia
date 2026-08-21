# 此文件为工作区代码打包工具，不可更改文件内任何内容，除非用户允许
param(
    [string]$OutputName = "U-Ai-source"
)

$scriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $PSCommandPath }
$workspace = Split-Path -Parent $scriptDir
$parentDir = Split-Path $workspace -Parent
$folderName = Split-Path $workspace -Leaf
$outputFile = Join-Path $workspace "$OutputName.tar.gz"

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
    "--exclude=surrealdb-data-temp"
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
    "--exclude=*.tar.gz"
    "--exclude=*.tar.xz"
    "--exclude=*.zip"
    "--exclude=artifacts"
    "--exclude=.dart_tool"
    "--exclude=temp_extract_integration"
    "--exclude=AmitiaData"
    "--exclude=U-Ai-source.tar.gz"
    "--exclude=installed-runtime-package.zip"
    "--exclude=amitia-runtime-root-debug.tar.xz"
    "--exclude=rootfs-seed-debug.tar.xz"
    "--exclude=.codex-temp-patches"
    "--exclude=.codex-diff-apply"
    "--exclude=classes*.dex.txt"
)

Set-Location $parentDir

Write-Host "Packing source: $folderName -> $outputFile"
$startTime = Get-Date

& tar -czf $outputFile @excludes $folderName

$elapsed = (Get-Date) - $startTime
$size = (Get-Item $outputFile).Length

Write-Host "Done in $($elapsed.ToString('mm\:ss'))"
Write-Host "Output: $outputFile"
Write-Host "Size: $([math]::Round($size / 1MB, 2)) MB"
