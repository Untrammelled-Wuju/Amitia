# pack-source-lite.ps1 - Only source code, aggressive exclusion for small size
param(
    [string]$OutputName = "U-Ai-source-lite"
)

$scriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $PSCommandPath }
$workspace = Split-Path -Parent $scriptDir
$parentDir = Split-Path $workspace -Parent
$folderName = Split-Path $workspace -Leaf
$outputFile = Join-Path $workspace "$OutputName.tar.gz"
$tempRoot = [System.IO.Path]::GetTempPath()
$tempTar = Join-Path $tempRoot "$OutputName.tar"

if (Test-Path $outputFile) { Remove-Item $outputFile -Force }
if (Test-Path $tempTar) { Remove-Item $tempTar -Force }

# Aggressive excludes - source code only
$excludes = @(
    "--exclude=node_modules"
    "--exclude=.git"
    "--exclude=*.exe"
    "--exclude=*.log"
    "--exclude=*.zip"
    "--exclude=*.tar"
    "--exclude=*.tar.gz"
    "--exclude=*.tar.xz"
    "--exclude=*.apk"
    "--exclude=*.aab"
    "--exclude=*.dex"
    "--exclude=*.class"
    "--exclude=*.jar"
    "--exclude=*.war"
    "--exclude=*.so"
    "--exclude=*.dll"
    "--exclude=*.dylib"
    "--exclude=*.o"
    "--exclude=*.a"
    "--exclude=*.db"
    "--exclude=*.db-shm"
    "--exclude=*.db-wal"
    "--exclude=*.db-journal"
    "--exclude=desktop/dist"
    "--exclude=desktop/build"
    "--exclude=desktop/release"
    "--exclude=desktop/dist-types"
    "--exclude=desktop/resources/core"
    "--exclude=desktop/resources/data"
    "--exclude=front/dist"
    "--exclude=mobile_app/build"
    "--exclude=mobile_app/android/app/build"
    "--exclude=mobile_app/android/amitia-runtime/build"
    "--exclude=mobile_app/android/.gradle"
    "--exclude=$folderName/mobile_app/windows/flutter/ephemeral"
    "--exclude=mobile_app/.flutter-plugins-dependencies"
    "--exclude=$folderName/mobile_app/android/local.properties"
    "--exclude=$folderName/mobile_app/android/.cxx"
    "--exclude=$folderName/mobile_app/android/app/.cxx"
    "--exclude=$folderName/mobile_app/android/amitia-runtime/.cxx"
    "--exclude=$folderName/mobile_app/android/build"
    "--exclude=$folderName/mobile_app/android/.kotlin"
    "--exclude=$folderName/mobile_app/ios/Flutter/ephemeral"
    "--exclude=$folderName/mobile_app/linux/flutter/ephemeral"
    "--exclude=$folderName/mobile_app/macos/Flutter/ephemeral"
    "--exclude=mobile_app/android/app/src/main/assets"
    "--exclude=mobile_app/android/app/src/debug"
    "--exclude=mobile_app/android/app/src/profile"
    "--exclude=backend/pkg/gameplugin/sdk/game-plugin/dist"
    "--exclude=qdrant/storage"
    "--exclude=surrealdb/surreal.exe"
    "--exclude=surrealdb/data"
    "--exclude=surrealdb-data-temp"
    "--exclude=.pub-cache"
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
    "--exclude=backend/surrealdb/data"
    "--exclude=backend/surrealdb/data.sdb"
    "--exclude=backend/qdrant/storage"
    "--exclude=backend/logs"
    "--exclude=backend/AmitiaData"
    "--exclude=backend/bin"
    "--exclude=backend/target"
    "--exclude=backend/node/node.exe"
    "--exclude=backend/node/node.exe.zip"
    "--exclude=backend/sidecar/node_modules"
    "--exclude=backend/qq-sidecar/node_modules"
    "--exclude=backend/qq-sidecar/data"
    "--exclude=$folderName/data"
    "--exclude=runtime/out"
    "--exclude=runtime/mobile_app"
    "--exclude=artifacts"
    "--exclude=testplugins"
    "--exclude=tests"
    "--exclude=**/package-lock.json"
    "--exclude=**/yarn.lock"
    "--exclude=**/.gitignore"
    "--exclude=backend/server_linux_amd64"
    "--exclude=backend/server_linux_arm64"
    "--exclude=backend/server-linux-arm64"
    "--exclude=backend/server-linux-amd64"
    "--exclude=backend/server-linux"
    "--exclude=backend/server-arm64"
    "--exclude=backend/server-amd64"
    "--exclude=backend/sidecar/bundle.mjs"
    "--exclude=backend/qq-sidecar/bundle.mjs"
    "--exclude=backend/sidecar/launcher.mjs"
    "--exclude=backend/qq-sidecar/launcher.mjs"
    "--exclude=backend/server"
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
    "--exclude=temp_apk_extract"
    "--exclude=temp_extract_integration"
    "--exclude=.dart_tool"
    "--exclude=.codex-temp-patches"
    "--exclude=.codex-diff-apply"
    "--exclude=classes*.dex.txt"
    "--exclude=AmitiaData"
    "--exclude=$OutputName.tar.gz"
    "--exclude=U-Ai-source.tar.gz"
    "--exclude=installed-runtime-package.zip"
    "--exclude=amitia-runtime-root-debug.tar.xz"
    "--exclude=rootfs-seed-debug.tar.xz"
    "--exclude=.gradle-proot-build"
    "--exclude=.meituan-catpaw"
    "--exclude=.workbuddy-ai"
    "--exclude=coverage"
    "--exclude=.dart_tool"
    "--exclude=sdk/plugin-cli/dist"
    "--exclude=runtime/task-host/dist"
    "--exclude=runtime/plugin-host/dist"
    "--exclude=*.map"
    "--exclude=backend/build"
    "--exclude=backend/surrealdb/surreal.zip"
    "--exclude=backend/qdrant/qdrant.zip"
    "--exclude=desktop/resources/qdrant/qdrant.zip"
    "--exclude=desktop/resources/surrealdb/surrealdb/surreal.zip"
    "--exclude=desktop/resources/surrealdb/surreal.zip"
    "--exclude=desktop/resources/core/node/node.zip"
    "--exclude=backend/logs"
    "--exclude=backend/surrealdb/data"
    "--exclude=backend/surrealdb/data.sdb"
    "--exclude=backend/surrealdb/surreal.zip"
    "--exclude=backend/surrealdb/surreal.exe"
    "--exclude=backend/qdrant/storage"
    "--exclude=backend/qdrant/qdrant.zip"
    "--exclude=backend/qdrant/qdrant_linux_*"
    "--exclude=backend/node/node.exe"
    "--exclude=backend/node/node.exe.zip"
    "--exclude=$folderName/backend/surrealdb/surreal.exe"
    "--exclude=$folderName/backend/surrealdb/data"
    "--exclude=$folderName/backend/surrealdb/data.sdb"
    "--exclude=$folderName/backend/qdrant/storage"
    "--exclude=$folderName/backend/qdrant/qdrant_linux_*"
    "--exclude=trace.atrace"
    "--exclude=*.atrace"
    "--exclude=.qdrant-initialized"
    "--exclude=*/.qdrant-initialized"
    "--exclude=raft_state.json"
    "--exclude=*.wal"
)

Set-Location $parentDir

Write-Host "Packing lite source: $folderName -> $outputFile"
$startTime = & Get-Date

# Step 1: Create uncompressed tar
Write-Host "Step 1: Creating uncompressed tar..."
& tar -cf $tempTar @excludes $folderName

if (-not $?) {
    Write-Host "tar failed with exit code: $LASTEXITCODE"
    if (Test-Path $tempTar) { Remove-Item $tempTar -Force }
    exit 1
}

$tarSize = (Get-Item $tempTar).Length
Write-Host "Tar created: $([math]::Round($tarSize / 1MB, 2)) MB"

# Step 2: Compress with Python
Write-Host "Step 2: Python gzip..."
$pythonCommand = if (Get-Command python -ErrorAction SilentlyContinue) { "python" } elseif (Get-Command python3 -ErrorAction SilentlyContinue) { "python3" } else { $null }
if (-not $pythonCommand) {
    Write-Host "FAIL: Python not found"
    if (Test-Path $tempTar) { Remove-Item $tempTar -Force }
    exit 1
}
& $pythonCommand -c @"
import gzip
import shutil
import sys

src = r'$tempTar'
dst = r'$outputFile'

try:
    with open(src, 'rb') as f_in:
        with gzip.open(dst, 'wb', compresslevel=9) as f_out:
            shutil.copyfileobj(f_in, f_out, length=1024*1024)
    print('gzip complete')
except Exception as e:
    print(f'Error: {e}', file=sys.stderr)
    sys.exit(1)
"@

$tempTarPath = $tempTar
if (Test-Path $tempTarPath) { Remove-Item $tempTarPath -Force }

if ($LASTEXITCODE -ne 0) { exit 1 }

# Verify
$finalFile = Get-Item $outputFile
$elapsed = (Get-Date) - $startTime
Write-Host ""
Write-Host "Done in $($elapsed.ToString('mm\:ss'))"
Write-Host "Output: $outputFile"
Write-Host "Size: $([math]::Round($finalFile.Length / 1MB, 2)) MB"

$verifyResult = & tar -tzf $outputFile 2>&1
$entries = ($verifyResult | Measure-Object -Line).Lines
$goFiles = ($verifyResult | Select-String "\.go$" | Measure-Object).Count
$dartFiles = ($verifyResult | Select-String "\.dart$" | Measure-Object).Count
$tsFiles = ($verifyResult | Select-String "\.tsx?$" | Measure-Object).Count
$vueFiles = ($verifyResult | Select-String "\.vue$" | Measure-Object).Count
$pyFiles = ($verifyResult | Select-String "\.py$" | Measure-Object).Count
$ps1Files = ($verifyResult | Select-String "\.ps1$" | Measure-Object).Count
$mjsFiles = ($verifyResult | Select-String "\.mjs$" | Measure-Object).Count
$ktFiles = ($verifyResult | Select-String "\.kt$" | Measure-Object).Count

Write-Host "Entries: $entries"
Write-Host ".go: $goFiles  .dart: $dartFiles  .ts/.tsx: $tsFiles  .vue: $vueFiles"
Write-Host ".py: $pyFiles  .ps1: $ps1Files  .mjs: $mjsFiles  .kt: $ktFiles"
