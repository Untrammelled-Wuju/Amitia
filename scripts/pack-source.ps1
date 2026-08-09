$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$desktop = [Environment]::GetFolderPath("Desktop")
$stageDir = Join-Path $desktop "U-Ai-source-archive"
$outZip = Join-Path $desktop "U-Ai-source.zip"

if (Test-Path $stageDir) { Remove-Item $stageDir -Recurse -Force }
if (Test-Path $outZip) { Remove-Item $outZip -Force }

# Directory names to skip (matched against any path component)
$skipDirs = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase, @(
    "node_modules", ".git", "dist", "build", "dist-types", "release", "logs",
    "AmitiaData", "data", "migration_backups", "migration_backups_prev",
    "surrealdb", "dev", "tmp", "D:", "__pycache__", ".dart_tool",
    ".github", ".workbuddy-ai", ".reasonix", ".trae-html-share-packages",
    "build-installer", "installer", "installer-v2", "installer-v3",
    "installer-v4", "installer-v5", "installer-out", "installer-final",
    "release-dist", "dist-release", "out", "android"
))

# These specific subdirectory paths (relative to each source root) should also be skipped
$skipSubPaths = @(
    "cmd/data", "qdrant/storage", "qdrant/snapshots", "resources/core",
    "resources/data", "resources/qdrant/storage", "resources/qdrant/snapshots",
    "resources/surrealdb/data", "backend/qdrant", "backend/surrealdb"
)

# File extensions to skip
$skipExts = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase, @(
    ".exe", ".dll", ".pdb", ".log", ".db", ".db-shm", ".db-wal", ".db-journal",
    ".zip", ".tar", ".orig", ".swp", ".swo", ".pyc", ".iml", ".py"
))

# Specific filenames to skip
$skipNames = [System.Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase, @(
    ".DS_Store", "Thumbs.db", "local-token", "mcp-secrets.key", "device.json",
    "qrcode.png", "server", "server.exe", "backend.exe", "backend",
    "amitia-ext.exe", "amitiax.exe", "extension.test.exe", "worker.test.exe",
    "kernel.test.exe", "legacy-package-migrate.exe", "server_linux_amd64",
    "server_linux_arm64", "qdrantprocess.test", "qdrantprocess.test.exe",
    "tmp_check.js", "tmp_check_db.js", "TEMPgradle_out.txt", ":TEMPgradle_out.txt",
    "test_output.txt", "test_nodeenv_race.txt", ".env", ".env.local",
    ".qdrant-initialized", "desktop-instance-id", "raft_state.json"
))

function Test-ShouldSkipDir {
    param([string]$FullPath, [string]$SourceRoot)
    $relPath = $FullPath.Substring($SourceRoot.Length).TrimStart('\', '/')
    $parts = $relPath -split '[\\/]'
    foreach ($p in $parts) {
        if ($skipDirs.Contains($p)) { return $true }
    }
    # Check sub-paths
    $relForward = $relPath -replace '\\', '/'
    foreach ($sp in $skipSubPaths) {
        if ($relForward -like "$sp*" -or $relForward -like "*/$sp*") { return $true }
    }
    return $false
}

function Copy-SourceFiles {
    param([string]$Source, [string]$Dest)
    if (-not (Test-Path $Dest)) { $null = New-Item -ItemType Directory -Path $Dest -Force }

    $dirs = Get-ChildItem -LiteralPath $Source -Directory -Force -ErrorAction SilentlyContinue |
        Where-Object { -not (Test-ShouldSkipDir -FullPath $_.FullName -SourceRoot $Source) }

    $files = Get-ChildItem -LiteralPath $Source -File -Force -ErrorAction SilentlyContinue |
        Where-Object {
            -not $skipExts.Contains($_.Extension) -and
            -not $skipNames.Contains($_.Name)
        }

    foreach ($f in $files) {
        try {
            Copy-Item -LiteralPath $f.FullName -Destination $Dest -Force -ErrorAction Stop
        } catch {
            # Skip locked/inaccessible files
        }
    }

    foreach ($d in $dirs) {
        $subDest = Join-Path $Dest $d.Name
        Copy-SourceFiles -Source $d.FullName -Dest $subDest
    }
}

Write-Host "=== Packing source ===" -ForegroundColor Cyan
Write-Host "Front..." -ForegroundColor Yellow
Copy-SourceFiles -Source (Join-Path $repoRoot "front") -Dest (Join-Path $stageDir "front")
Write-Host "Backend..." -ForegroundColor Yellow
Copy-SourceFiles -Source (Join-Path $repoRoot "backend") -Dest (Join-Path $stageDir "backend")
Write-Host "Desktop..." -ForegroundColor Yellow
Copy-SourceFiles -Source (Join-Path $repoRoot "desktop") -Dest (Join-Path $stageDir "desktop")

Write-Host "Compressing..." -ForegroundColor Yellow
Compress-Archive -Path "$stageDir\*" -DestinationPath $outZip -CompressionLevel Optimal -Force

Remove-Item $stageDir -Recurse -Force

$size = (Get-Item $outZip).Length
Write-Host ""
Write-Host "=== Done ===" -ForegroundColor Green
Write-Host "Archive: $outZip" -ForegroundColor Cyan
Write-Host "Size: $([math]::Round($size / 1MB, 2)) MB" -ForegroundColor Cyan
