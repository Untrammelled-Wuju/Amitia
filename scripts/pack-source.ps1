# 此文件为工作区代码打包工具，不可更改文件内任何内容，除非用户允许
param(
    [string]$OutputName = "U-Ai-source"
)

$scriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $PSCommandPath }
$workspace = Split-Path -Parent $scriptDir
$parentDir = Split-Path $workspace -Parent
$folderName = Split-Path $workspace -Leaf
$outputFile = Join-Path $workspace "$OutputName.tar.gz"
$tempRoot = [System.IO.Path]::GetTempPath()
$tempTar = Join-Path $tempRoot "$OutputName.tar"
$tempGz = Join-Path $tempRoot "$OutputName.tar.gz"
$tempExtract = Join-Path $tempRoot "$OutputName-audit"

if (Test-Path $outputFile) { Remove-Item $outputFile -Force }
if (Test-Path $tempTar) { Remove-Item $tempTar -Force }
if (Test-Path $tempGz) { Remove-Item $tempGz -Force }
if (Test-Path $tempExtract) { Remove-Item $tempExtract -Recurse -Force }

# ============================================================
# P0-05: Source Hygiene + canonical Freeze Gate (before packing)
# ============================================================
Write-Host "=== P0-05: Running Source Hygiene / Freeze Gates ==="
Set-Location $workspace

& node scripts/audit/verify-source-hygiene.mjs
if ($LASTEXITCODE -ne 0) {
    Write-Host "FAIL: source hygiene gate did not pass. Aborting source pack."
    exit 1
}

& node desktop/scripts/verify-desktop-pet-finalization.mjs
if ($LASTEXITCODE -ne 0) {
    Write-Host "FAIL: desktop-pet finalization gate did not pass. Aborting source pack."
    exit 1
}

& node scripts/build-freeze-scope.mjs --verify
if ($LASTEXITCODE -ne 0) {
    Write-Host "FAIL: canonical freeze manifest is stale or invalid. Run: node scripts/build-freeze-scope.mjs --write"
    exit 1
}

& node desktop/scripts/release-integrity.mjs --pre-build
if ($LASTEXITCODE -ne 0) {
    Write-Host "FAIL: release integrity pre-build gate did not pass. Aborting source pack."
    exit 1
}
Write-Host "Source Hygiene / Freeze Gates: PASS"

# ============================================================
# P0-07: tar excludes (runtime pollution exclusion)
# ============================================================
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
    "--exclude=$folderName/mobile_app/windows/flutter/ephemeral"
    "--exclude=$folderName/mobile_app/.flutter-plugins-dependencies"
    "--exclude=$folderName/mobile_app/android/local.properties"
    "--exclude=$folderName/mobile_app/android/.cxx"
    "--exclude=$folderName/mobile_app/android/app/.cxx"
    "--exclude=$folderName/mobile_app/android/amitia-runtime/.cxx"
    "--exclude=$folderName/mobile_app/android/build"
    "--exclude=$folderName/mobile_app/android/.kotlin"
    "--exclude=$folderName/mobile_app/ios/Flutter/ephemeral"
    "--exclude=$folderName/mobile_app/linux/flutter/ephemeral"
    "--exclude=$folderName/mobile_app/macos/Flutter/ephemeral"
    "--exclude=backend/pkg/gameplugin/sdk/game-plugin/dist"
    "--exclude=*.db"
    "--exclude=*.db-shm"
    "--exclude=*.db-wal"
    "--exclude=*.db-journal"
    "--exclude=qdrant/storage"
    "--exclude=surrealdb/surreal.exe"
    "--exclude=surrealdb/data"
    "--exclude=surrealdb-data-temp"
    "--exclude=.pub-cache"
    "--exclude=desktop/dist-types"
    "--exclude=desktop/resources/core"
    "--exclude=desktop/resources/data"
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
    "--exclude=backend/sidecar/node_modules"
    "--exclude=backend/qq-sidecar/node_modules"
    "--exclude=backend/qq-sidecar/data"
    "--exclude=$folderName/data"
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
    "--exclude=temp_apk_extract"
    "--exclude=backend/build"
    "--exclude=*.map"
    "--exclude=sdk/plugin-cli/dist"
    "--exclude=runtime/task-host/dist"
    "--exclude=runtime/plugin-host/dist"
    "--exclude=runtime/mobile_app"
    "--exclude=*.so"
    "--exclude=*.dll"
    "--exclude=*.dylib"
    # P0-07: Runtime pollution exclusion rules (precise paths only)
    "--exclude=trace.atrace"
    "--exclude=*.atrace"
    "--exclude=.qdrant-initialized"
    "--exclude=*/.qdrant-initialized"
    "--exclude=$folderName/backend/surrealdb/surreal.exe"
    "--exclude=$folderName/backend/surrealdb/data"
    "--exclude=$folderName/backend/surrealdb/data.sdb"
    "--exclude=$folderName/backend/qdrant/storage"
    "--exclude=$folderName/backend/qdrant/qdrant_linux_*"
    "--exclude=raft_state.json"
    "--exclude=*.wal"
    "--exclude=.gradle-proot-build"
    "--exclude=.meituan-catpaw"
    "--exclude=.workbuddy-ai"
    "--exclude=coverage"
    "--exclude=.DS_Store"
    "--exclude=Thumbs.db"
)

Set-Location $parentDir

Write-Host "Packing source: $folderName -> $outputFile"
$startTime = Get-Date

# Step 1: Create uncompressed tar
Write-Host "Step 1: Creating uncompressed tar..."
& tar -cf $tempTar @excludes $folderName

if ($LASTEXITCODE -ne 0) {
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
$pythonCommand = if (Get-Command python -ErrorAction SilentlyContinue) { "python" } elseif (Get-Command python3 -ErrorAction SilentlyContinue) { "python3" } else { $null }
if (-not $pythonCommand) {
    Write-Host "FAIL: Python is required for deterministic gzip compression"
    if (Test-Path $tempTar) { Remove-Item $tempTar -Force }
    exit 1
}
& $pythonCommand -c @"
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

# ============================================================
# P0-01: Required Source check
# ============================================================
Write-Host "=== P0-01: Required Source check ==="
$requiredSourceFiles = @(
    "backend/internal/desktoppet/storage/pathguard.go",
    "backend/internal/desktoppet/storage/roots.go",
    "backend/internal/desktoppet/release/storage/filesystem.go",
    "backend/internal/desktoppet/release/storage/filesystem_test.go",
    "backend/internal/gamehost/storage/paths.go",
    "backend/internal/gamehost/storage/directory_manager.go",
    "backend/pkg/database/surrealdb/manager.go",
    "backend/pkg/database/surrealdb/spec.go",
    "backend/pkg/database/qdrant/client.go",
    "backend/pkg/database/qdrant/manager.go",
    "backend/pkg/database/qdrant/spec.go",
    "desktop/src/main/artifacts/artifact-client.ts",
    "desktop/src/renderer/pet-animation-globals.d.ts"
)

$missingFiles = @()
foreach ($relPath in $requiredSourceFiles) {
    $entries = & tar -tzf $outputFile 2>&1 | Select-String -Pattern $relPath
    if (-not $entries) {
        $missingFiles += $relPath
    }
}

if ($missingFiles.Count -gt 0) {
    Write-Host "FAIL: Required source files missing from archive:"
    foreach ($f in $missingFiles) { Write-Host "  - $f" }
    Remove-Item $outputFile -Force -ErrorAction SilentlyContinue
    exit 1
}
Write-Host "Required Source check: PASS ($($requiredSourceFiles.Count) files verified)"

# ============================================================
# P0-08: Archive revalidation (archive-local self verification + clean build)
# ============================================================
Write-Host "=== P0-08: Archive Revalidation ==="
Set-Location $workspace
& node scripts/verify-source-archive.mjs $outputFile --clean-build
if ($LASTEXITCODE -ne 0) {
    Write-Host "FAIL: archive revalidation or clean build failed"
    Remove-Item $outputFile -Force -ErrorAction SilentlyContinue
    exit 1
}
Write-Host "Archive revalidation: PASS"

# Step 3: Final verification output
Write-Host "Step 3: Final archive verification..."
$verifyResult = & tar -tzf $outputFile 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Host "FAIL: final tar listing failed with exit code $LASTEXITCODE"
    Remove-Item $outputFile -Force -ErrorAction SilentlyContinue
    exit 1
}
$entries = ($verifyResult | Measure-Object -Line).Lines
Write-Host "Verified: $entries entries"

$gcFound = $verifyResult | Select-String "game_center_api.dart"
if ($gcFound) {
    Write-Host "game_center_api.dart: FOUND"
} else {
    Write-Host "game_center_api.dart: MISSING!"
}

$goFiles = ($verifyResult | Select-String "\.go$" | Measure-Object).Count
Write-Host ".go files: $goFiles"

$dartFiles = ($verifyResult | Select-String "\.dart$" | Measure-Object).Count
Write-Host ".dart files: $dartFiles"

$elapsed = (Get-Date) - $startTime
$size = (Get-Item $outputFile).Length
Write-Host ""
Write-Host "Done in $($elapsed.ToString('mm\:ss'))"
Write-Host "Output: $outputFile"
Write-Host "Size: $([math]::Round($size / 1MB, 2)) MB"

if ($env:CI -ne "true" -and $env:OS -eq "Windows_NT") {
    Start-Process "explorer.exe" -ArgumentList $workspace
    Start-Process "explorer.exe" -ArgumentList "/select,$outputFile"
}
