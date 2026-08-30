param(
    [string]$OutputName = "flutter-backend-source"
)

$scriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $PSCommandPath }
$workspace = Split-Path -Parent $scriptDir
$parentDir = Split-Path $workspace -Parent
$folderName = Split-Path $workspace -Leaf
$outputFile = Join-Path $workspace "$OutputName.tar.gz"
$tempTar = "$env:TEMP\$OutputName.tar"
$tempGz = "$env:TEMP\$OutputName.tar.gz"

if (Test-Path $outputFile) { Remove-Item $outputFile -Force }
if (Test-Path $tempTar) { Remove-Item $tempTar -Force }
if (Test-Path $tempGz) { Remove-Item $tempGz -Force }

$excludes = @(
    # Common
    "--exclude=.git"
    "--exclude=node_modules"
    "--exclude=.vscode"
    "--exclude=.idea"
    "--exclude=.env"
    "--exclude=.env.local"
    "--exclude=.DS_Store"
    "--exclude=Thumbs.db"
    "--exclude=*.log"
    "--exclude=*.bak"
    "--exclude=*.tmp"
    "--exclude=*.pyc"
    "--exclude=__pycache__"

    # Flutter / mobile_app build artifacts
    "--exclude=$folderName/mobile_app/build"
    "--exclude=$folderName/mobile_app/.dart_tool"
    "--exclude=$folderName/mobile_app/.flutter-plugins-dependencies"
    "--exclude=$folderName/mobile_app/pubspec.lock"
    "--exclude=$folderName/mobile_app/android/.gradle"
    "--exclude=$folderName/mobile_app/android/.kotlin"
    "--exclude=$folderName/mobile_app/android/build"
    "--exclude=$folderName/mobile_app/android/local.properties"
    "--exclude=$folderName/mobile_app/android/.cxx"
    "--exclude=$folderName/mobile_app/android/app/.cxx"
    "--exclude=$folderName/mobile_app/android/app/build"
    "--exclude=$folderName/mobile_app/android/amitia-runtime/.cxx"
    "--exclude=$folderName/mobile_app/android/amitia-runtime/build"
    "--exclude=$folderName/mobile_app/android/amitia-runtime/scripts/proot-build-record.json"
    "--exclude=$folderName/mobile_app/android/scripts/proot-build-record.json"
    "--exclude=$folderName/mobile_app/ios/Flutter/ephemeral"
    "--exclude=$folderName/mobile_app/windows/flutter/ephemeral"
    "--exclude=$folderName/mobile_app/linux/flutter/ephemeral"
    "--exclude=$folderName/mobile_app/macos/Flutter/ephemeral"
    "--exclude=$folderName/mobile_app/ios/Podfile.lock"
    "--exclude=$folderName/mobile_app/ios/.symlinks"
    "--exclude=$folderName/mobile_app/ios/Flutter/Flutter.framework"
    "--exclude=$folderName/mobile_app/ios/Flutter/App.framework"
    "--exclude=$folderName/mobile_app/macos/Podfile.lock"
    "--exclude=$folderName/mobile_app/macos/.symlinks"
    "--exclude=$folderName/mobile_app/linux/flutter/plugins"
    "--exclude=$folderName/mobile_app/web/.dart_tool"

    # Backend Go build artifacts & runtime data
    "--exclude=$folderName/backend/server.exe"
    "--exclude=$folderName/backend/server.exe~"
    "--exclude=$folderName/backend/server"
    "--exclude=$folderName/backend/server_linux_amd64"
    "--exclude=$folderName/backend/server_linux_arm64"
    "--exclude=$folderName/backend/server_*.exe"
    "--exclude=$folderName/backend/amitia-ext.exe"
    "--exclude=$folderName/backend/amitiax.exe"
    "--exclude=$folderName/backend/extension.test.exe"
    "--exclude=$folderName/backend/kernel.test.exe"
    "--exclude=$folderName/backend/legacy-package-migrate.exe"
    "--exclude=$folderName/backend/worker.test.exe"
    "--exclude=$folderName/backend/cmd/server/server.exe"
    "--exclude=$folderName/backend/cmd/server/backend.exe"
    "--exclude=$folderName/backend/cmd/server/backend"
    "--exclude=$folderName/backend/AmitiaData"
    "--exclude=$folderName/backend/data"
    "--exclude=$folderName/backend/cmd/data"
    "--exclude=$folderName/backend/logs"
    "--exclude=$folderName/backend/log/*.log"
    "--exclude=$folderName/backend/qdrant/storage"
    "--exclude=$folderName/backend/qdrant/snapshots"
    "--exclude=$folderName/backend/qdrant/qdrant.exe"
    "--exclude=$folderName/backend/qdrant/qdrant.zip"
    "--exclude=$folderName/backend/surrealdb/data"
    "--exclude=$folderName/backend/surrealdb/data.sdb"
    "--exclude=$folderName/backend/surrealdb/surreal.exe"
    "--exclude=$folderName/backend/surrealdb/surreal.zip"
    "--exclude=$folderName/backend/node/node.exe"
    "--exclude=$folderName/backend/node/node.exe.zip"
    "--exclude=$folderName/backend/sidecar/node_modules"
    "--exclude=$folderName/backend/sidecar/bundle.mjs"
    "--exclude=$folderName/backend/sidecar/launcher.mjs"
    "--exclude=$folderName/backend/qq-sidecar/node_modules"
    "--exclude=$folderName/backend/qq-sidecar/bundle.mjs"
    "--exclude=$folderName/backend/qq-sidecar/launcher.mjs"
    "--exclude=$folderName/backend/qq-sidecar/data"
    "--exclude=$folderName/backend/bin"
    "--exclude=$folderName/backend/target"
    "--exclude=$folderName/backend/build"
    "--exclude=$folderName/backend/snapshots"
    "--exclude=$folderName/backend/storage"
    "--exclude=$folderName/backend/runtime/tasks"
    "--exclude=$folderName/backend/testdata/extensions"
    "--exclude=$folderName/backend/pkg/gameplugin/sdk/game-plugin/dist"
    "--exclude=$folderName/backend/*.db"
    "--exclude=$folderName/backend/*.db-shm"
    "--exclude=$folderName/backend/*.db-wal"
    "--exclude=$folderName/backend/*.db-journal"

    # Exclude other project parts (only keep mobile_app + backend)
    "--exclude=$folderName/desktop"
    "--exclude=$folderName/front"
    "--exclude=$folderName/data"
    "--exclude=$folderName/AmitiaData"
    "--exclude=$folderName/.pub-cache"
    "--exclude=$folderName/.github"
    "--exclude=$folderName/.workbuddy-ai"
    "--exclude=$folderName/.meituan-catpaw"
    "--exclude=$folderName/.qdrant-initialized"
    "--exclude=$folderName/.tool-versions"
    "--exclude=$folderName/.gitattributes"
    "--exclude=$folderName/.gitmodules"
    "--exclude=$folderName/.gitignore"
    "--exclude=$folderName/artifacts"
    "--exclude=$folderName/build-log.txt"
    "--exclude=$folderName/build-log2.txt"
    "--exclude=$folderName/config"
    "--exclude=$folderName/contracts"
    "--exclude=$folderName/docs"
    "--exclude=$folderName/Logo"
    "--exclude=$folderName/Plugin"
    "--exclude=$folderName/runtime"
    "--exclude=$folderName/sdk"
    "--exclude=$folderName/snapshots"
    "--exclude=$folderName/storage"
    "--exclude=$folderName/surrealdb"
    "--exclude=$folderName/testplugins"
    "--exclude=$folderName/trace.atrace"
    "--exclude=$folderName/demo-extension.amitiax"
    "--exclude=$folderName/device-runtime-manifest.json"
    "--exclude=$folderName/appsettings.json"
    "--exclude=$folderName/tsconfig.base.json"
    "--exclude=$folderName/release-gate-report.md"
    "--exclude=$folderName/ANDROID_PROOT_STARTUP_FIX_MANIFEST.txt"
    "--exclude=$folderName/DESKTOP_PET_FINALIZATION_20260830_MANIFEST.txt"
    "--exclude=$folderName/DESKTOP_PET_FINALIZATION_20260830_SHA256SUMS.txt"
    "--exclude=$folderName/dir"
    "--exclude=$folderName/*.tar"
    "--exclude=$folderName/*.tar.gz"
    "--exclude=$folderName/*.tar.xz"
    "--exclude=$folderName/*.zip"
    "--exclude=$folderName/temp_extract_integration"
    "--exclude=$folderName/installed-runtime-package.zip"
    "--exclude=$folderName/amitia-runtime-root-debug.tar.xz"
    "--exclude=$folderName/rootfs-seed-debug.tar.xz"
    "--exclude=$folderName/.codex-temp-patches"
    "--exclude=$folderName/.codex-diff-apply"
    "--exclude=$folderName/classes*.dex.txt"
    "--exclude=$folderName/temp_apk_extract"
    "--exclude=$folderName/U-Ai-source.tar.gz"
    "--exclude=$folderName/flutter-backend-source.tar.gz"
)

Set-Location $parentDir

Write-Host "Packing Flutter + Backend source: $folderName -> $outputFile"
$startTime = Get-Date

# Step 1: Create uncompressed tar
Write-Host "Step 1: Creating uncompressed tar..."
& tar -cf $tempTar @excludes $folderName/mobile_app $folderName/backend

if ($LASTEXITCODE -ne 0 -and $LASTEXITCODE -ne 1) {
    Write-Host "tar failed with exit code: $LASTEXITCODE"
    if (Test-Path $tempTar) { Remove-Item $tempTar -Force }
    exit 1
}

if (-not (Test-Path $tempTar)) {
    Write-Host "tar did not create output file"
    exit 1
}

$tarSize = (Get-Item $tempTar).Length
Write-Host "Tar created: $([math]::Round($tarSize / 1MB, 2)) MB"

# Step 2: Compress with Python (avoids Windows tar gzip truncation)
Write-Host "Step 2: Compressing with Python..."
python -c @"
import gzip
import shutil
import sys

src = r'$tempTar'
dst = r'$tempGz'

try:
    with open(src, 'rb') as f_in:
        with gzip.open(dst, 'wb', compresslevel=6) as f_out:
            shutil.copyfileobj(f_in, f_out, length=1024*1024)
    print('gzip compression complete')
except Exception as e:
    print(f'Error: {e}', file=sys.stderr)
    sys.exit(1)
"@

if ($LASTEXITCODE -ne 0) {
    Write-Host "gzip compression failed"
    if (Test-Path $tempTar) { Remove-Item $tempTar -Force }
    if (Test-Path $tempGz) { Remove-Item $tempGz -Force }
    exit 1
}

# Move to final location
Move-Item $tempGz $outputFile -Force
if (Test-Path $tempTar) { Remove-Item $tempTar -Force }

# Step 3: Verify
Write-Host "Step 3: Verifying archive..."
$verifyResult = & tar -tzf $outputFile 2>&1
if ($LASTEXITCODE -eq 0 -or $LASTEXITCODE -eq 1) {
    $entries = ($verifyResult | Measure-Object -Line).Lines
    Write-Host "Verified: $entries entries"

    $dartFiles = ($verifyResult | Select-String "\.dart$" | Measure-Object).Count
    Write-Host ".dart files: $dartFiles"

    $goFiles = ($verifyResult | Select-String "\.go$" | Measure-Object).Count
    Write-Host ".go files: $goFiles"

    $pubspecFound = $verifyResult | Select-String "pubspec.yaml"
    if ($pubspecFound) { Write-Host "pubspec.yaml: FOUND" } else { Write-Host "pubspec.yaml: MISSING!" }

    $goModFound = $verifyResult | Select-String "go.mod"
    if ($goModFound) { Write-Host "go.mod: FOUND" } else { Write-Host "go.mod: MISSING!" }
} else {
    Write-Host "Verification failed"
}

$elapsed = (Get-Date) - $startTime
$size = (Get-Item $outputFile).Length
Write-Host ""
Write-Host "Done in $($elapsed.ToString('mm\:ss'))"
Write-Host "Output: $outputFile"
Write-Host "Size: $([math]::Round($size / 1MB, 2)) MB"

Start-Process "explorer.exe" -ArgumentList $workspace
Start-Process "explorer.exe" -ArgumentList "/select,$outputFile"
