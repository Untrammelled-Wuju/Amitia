# 此文件为工作区代码打包工具，不可更改文件内任何内容，除非用户允许
param(
    [string]$OutputName = "U-Ai-source"
)

$scriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $PSCommandPath }
$workspace = Split-Path -Parent $scriptDir
$parentDir = Split-Path $workspace -Parent
$folderName = Split-Path $workspace -Leaf
$outputFile = Join-Path $workspace "$OutputName.tar.gz"
$tempTar = "$env:TEMP\$OutputName.tar"
$tempGz = "$env:TEMP\$OutputName.tar.gz"
$tempExtract = "$env:TEMP\$OutputName-audit"

if (Test-Path $outputFile) { Remove-Item $outputFile -Force }
if (Test-Path $tempTar) { Remove-Item $tempTar -Force }
if (Test-Path $tempGz) { Remove-Item $tempGz -Force }
if (Test-Path $tempExtract) { Remove-Item $tempExtract -Recurse -Force }

# ============================================================
# P0-05: Source Hygiene Gate + Freeze Gate (before packing)
# ============================================================
Write-Host "=== P0-05: Running Source Hygiene Gate ==="
Set-Location $workspace

Write-Host "Running verify-desktop-pet-finalization.mjs..."
& node desktop/scripts/verify-desktop-pet-finalization.mjs
if ($LASTEXITCODE -ne 0) {
    Write-Host "FAIL: verify-desktop-pet-finalization.mjs did not pass. Aborting source pack."
    exit 1
}

Write-Host "Running SHA256 freeze verification..."
$shaLines = Get-Content "DESKTOP_PET_FINALIZATION_SHA256SUMS.txt"
$shaOk = 0
$shaFail = 0
foreach ($line in $shaLines) {
    if ($line.Trim() -eq "") { continue }
    $expectedHash = $line.Substring(0, 64)
    $relativePath = $line.Substring(66)
    $fullPath = Join-Path $workspace $relativePath
    if (Test-Path $fullPath) {
        $actualHash = (Get-FileHash $fullPath -Algorithm SHA256).Hash.ToLower()
        if ($actualHash -eq $expectedHash) {
            $shaOk++
        } else {
            $shaFail++
            Write-Host "SHA MISMATCH: $relativePath"
        }
    } else {
        $shaFail++
        Write-Host "MISSING: $relativePath"
    }
}
if ($shaFail -ne 0) {
    Write-Host "FAIL: SHA256 freeze verification failed ($shaFail issues). Aborting source pack."
    exit 1
}
Write-Host "SHA256 freeze verification: $shaOk files OK"

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
    "--exclude=testplugins"
    "--exclude=tests"
    "--exclude=runtime/mobile_app"
    "--exclude=*.zip"
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
# P0-08: Archive revalidation (extract and re-audit)
# ============================================================
Write-Host "=== P0-08: Archive Revalidation ==="

if (Test-Path $tempExtract) { Remove-Item $tempExtract -Recurse -Force }
New-Item -ItemType Directory -Path $tempExtract -Force | Out-Null

Write-Host "Extracting archive for audit..."
& tar -xzf $outputFile -C $tempExtract
if ($LASTEXITCODE -ne 0 -and $LASTEXITCODE -ne 1) {
    Write-Host "FAIL: Could not extract archive for audit"
    exit 1
}

$extractedFolder = Join-Path $tempExtract $folderName
if (-not (Test-Path $extractedFolder)) {
    Write-Host "FAIL: Expected folder not found in archive: $folderName"
    exit 1
}

Write-Host "Running source hygiene audit on extracted archive..."
$archiveRoot = $extractedFolder

# Check for forbidden runtime artifacts in archive
# Only check file-level patterns; directory-level exclusion is handled by tar --exclude rules
$forbiddenFiles = @(
    "trace.atrace",
    "*.atrace",
    ".qdrant-initialized",
    "raft_state.json",
    "*.wal",
    ".DS_Store",
    "Thumbs.db"
)

$archiveEntries = & tar -tzf $outputFile 2>&1
$violations = @()
foreach ($entry in $archiveEntries) {
    $name = Split-Path $entry -Leaf
    foreach ($pattern in $forbiddenFiles) {
        if ($name -like $pattern) {
            $violations += "$entry (matched: $pattern)"
        }
    }
}

if ($violations.Count -ne 0) {
    Write-Host "FAIL: Archive contains forbidden runtime artifacts:"
    foreach ($v in $violations) { Write-Host "  - $v" }
    Remove-Item $tempExtract -Recurse -Force -ErrorAction SilentlyContinue
    exit 1
}
Write-Host "Source hygiene audit: PASS (no forbidden artifacts in archive)"

# Run verify-desktop-pet-finalization.mjs against extracted archive
Write-Host "Running verify-desktop-pet-finalization.mjs against extracted archive..."
$env:DESKTOP_PET_ARCHIVE_ROOT = $archiveRoot
Set-Location $workspace
& node desktop/scripts/verify-desktop-pet-finalization.mjs
if ($LASTEXITCODE -ne 0) {
    Write-Host "FAIL: verify-desktop-pet-finalization.mjs did not pass on archive contents"
    Remove-Item $tempExtract -Recurse -Force -ErrorAction SilentlyContinue
    exit 1
}
Write-Host "Archive finalization gate: PASS"

# Cleanup
Remove-Item $tempExtract -Recurse -Force -ErrorAction SilentlyContinue

# Step 3: Final verification output
Write-Host "Step 3: Final archive verification..."
$verifyResult = & tar -tzf $outputFile 2>&1
if ($LASTEXITCODE -eq 0 -or $LASTEXITCODE -eq 1) {
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
