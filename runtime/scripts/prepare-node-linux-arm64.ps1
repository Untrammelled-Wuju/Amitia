$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RuntimeRoot = Split-Path -Parent $ScriptDir
$LockFile = Join-Path $RuntimeRoot "artifacts\node\linux-arm64\node-runtime-lock.json"
$CacheDir = Join-Path $RuntimeRoot ".cache\node"
$StagingDir = Join-Path $RuntimeRoot "build\staging\node-linux-arm64"
$OutputDir = Join-Path $RuntimeRoot "build\out\node\linux-arm64"

if (-not (Test-Path $LockFile)) {
    Write-Error "[FATAL] Lock file not found: $LockFile"
    exit 1
}

$Lock = Get-Content $LockFile -Raw | ConvertFrom-Json

$Version = $Lock.version
$ArchiveFileName = $Lock.archiveFileName
$SourceUrl = $Lock.sourceUrl
$ExpectedSha = $Lock.sha256
$InstallSubdir = $Lock.installSubdir

Write-Host "============================================"
Write-Host " Node Linux ARM64 Prepare"
Write-Host "============================================"
Write-Host "Version:          $Version"
Write-Host "Archive:          $ArchiveFileName"
Write-Host "Source:           $SourceUrl"
Write-Host "Expected SHA256:  $ExpectedSha"
Write-Host "============================================"

if (-not (Test-Path $CacheDir)) {
    New-Item -ItemType Directory -Force -Path $CacheDir | Out-Null
}

$ArchiveFile = Join-Path $CacheDir $ArchiveFileName

if (Test-Path $ArchiveFile) {
    $CachedSha = (Get-FileHash -Path $ArchiveFile -Algorithm SHA256).Hash.ToLower()
    if ($CachedSha -eq $ExpectedSha) {
        Write-Host "[CACHE] Cached archive SHA matches"
    } else {
        Write-Host "[CACHE] Cached archive SHA mismatch, removing"
        Remove-Item $ArchiveFile -Force
    }
}

if (-not (Test-Path $ArchiveFile)) {
    Write-Host "[DOWNLOAD] $SourceUrl"
    $TmpFile = "$ArchiveFile.tmp"
    try {
        Invoke-WebRequest -Uri $SourceUrl -OutFile $TmpFile -UseBasicParsing
        $DownloadSha = (Get-FileHash -Path $TmpFile -Algorithm SHA256).Hash.ToLower()
        if ($DownloadSha -ne $ExpectedSha) {
            Remove-Item $TmpFile -Force -ErrorAction SilentlyContinue
            Write-Error "[FATAL] SHA mismatch: expected=$ExpectedSha actual=$DownloadSha"
            exit 1
        }
        Move-Item $TmpFile $ArchiveFile -Force
        Write-Host "[PASS] SHA verified: $DownloadSha"
    } catch {
        if (Test-Path $TmpFile) { Remove-Item $TmpFile -Force -ErrorAction SilentlyContinue }
        Write-Error "[FATAL] Download failed: $_"
        exit 1
    }
}

$ParentDir = Split-Path $OutputDir -Parent
if (-not (Test-Path $ParentDir)) {
    New-Item -ItemType Directory -Force -Path $ParentDir | Out-Null
}

$StagingRoot = Join-Path $StagingDir ([guid]::NewGuid().ToString("N"))
if (Test-Path $StagingRoot) { Remove-Item $StagingRoot -Recurse -Force }
New-Item -ItemType Directory -Force -Path $StagingRoot | Out-Null

Write-Host "[EXTRACT] Validating archive members before extraction..."
try {
    $ArchiveMembers = & tar -tJf $ArchiveFile 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Error "[FATAL] Failed to list archive members"
        exit 1
    }
    $UnsafeEntries = $ArchiveMembers | Where-Object { $_ -match '^((\.\./)|/)' }
    if ($UnsafeEntries) {
        Write-Error "[FATAL] Archive contains absolute paths or parent traversal entries"
        $UnsafeEntries | ForEach-Object { Write-Error "  Unsafe: $_" }
        exit 1
    }
    Write-Host "[PASS] No absolute path or parent traversal entries"
} catch {
    Write-Error "[FATAL] Archive validation error: $_"
    exit 1
}

Write-Host "[EXTRACT] Extracting to staging..."
try {
    & tar -xJf $ArchiveFile -C $StagingRoot 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Error "[FATAL] tar extraction failed with exit code $LASTEXITCODE"
        exit 1
    }
} catch {
    Write-Error "[FATAL] tar extraction error: $_"
    exit 1
}

$ExtractedRoot = Join-Path $StagingRoot ("node-v{0}-linux-arm64" -f $Version)
if (-not (Test-Path $ExtractedRoot)) {
    Write-Error "[FATAL] Extracted root not found: $ExtractedRoot"
    if (Test-Path $StagingRoot) { Remove-Item $StagingRoot -Recurse -Force }
    exit 1
}

$NodeBin = Join-Path $ExtractedRoot "bin\node"
$NpmCli = Join-Path $ExtractedRoot "lib\node_modules\npm\bin\npm-cli.js"
$NpxCli = Join-Path $ExtractedRoot "lib\node_modules\npm\bin\npx-cli.js"

if (-not (Test-Path $NodeBin)) {
    Write-Error "[FATAL] node binary not found: $NodeBin"
    if (Test-Path $StagingRoot) { Remove-Item $StagingRoot -Recurse -Force }
    exit 1
}
if (-not (Test-Path $NpmCli)) {
    Write-Error "[FATAL] npm-cli.js not found"
    if (Test-Path $StagingRoot) { Remove-Item $StagingRoot -Recurse -Force }
    exit 1
}
if (-not (Test-Path $NpxCli)) {
    Write-Error "[FATAL] npx-cli.js not found"
    if (Test-Path $StagingRoot) { Remove-Item $StagingRoot -Recurse -Force }
    exit 1
}

$NodeDest = Join-Path $OutputDir $InstallSubdir
$TempDest = "$NodeDest.tmp.$([guid]::NewGuid().ToString('N').Substring(0,8))"
if (Test-Path $TempDest) { Remove-Item $TempDest -Recurse -Force }
if (-not (Test-Path $OutputDir)) { New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null }

Copy-Item -Path $ExtractedRoot -Destination $TempDest -Recurse
if (Test-Path $NodeDest) { Remove-Item $NodeDest -Recurse -Force }
Move-Item -Path $TempDest -Destination $NodeDest -Force
Write-Host "[FREEZE] Node runtime atomically published to: $NodeDest"

$TreeManifest = @()
Get-ChildItem -Path $NodeDest -Recurse -File | Sort-Object { $_.FullName.Substring($NodeDest.Length + 1) } | ForEach-Object {
    $RelPath = $_.FullName.Substring($NodeDest.Length + 1)
    $Hash = (Get-FileHash -Path $_.FullName -Algorithm SHA256).Hash.ToLower()
    $TreeManifest += "$Hash  $RelPath"
}
$TreeManifest | Set-Content -Path (Join-Path $OutputDir "node-files.sha256") -Encoding UTF8
Write-Host "[MANIFEST] node-files.sha256 generated"

$ContentForHash = $TreeManifest -join "`n"
$Sha256Algo = [System.Security.Cryptography.SHA256]::Create()
$Bytes = [System.Text.Encoding]::UTF8.GetBytes($ContentForHash)
$HashBytes = $Sha256Algo.ComputeHash($Bytes)
$TreeSha = ($HashBytes | ForEach-Object { '{0:x2}' -f $_ }) -join ''
$Sha256Algo.Dispose()

Write-Host "[TREE SHA] $TreeSha"

$NpmVersion = "bundled-with-$Version"
$NpxVersion = "bundled-with-$Version"

$BuildRecord = @{
    schemaVersion = 1
    component = "node"
    version = $Version
    platform = "linux"
    architecture = "arm64"
    source = @{
        url = $SourceUrl
        archiveFileName = $ArchiveFileName
        expectedSha256 = $ExpectedSha
        actualSha256 = $ExpectedSha
    }
    runtime = @{
        nodePath = "node/bin/node"
        npmPath = "node/bin/npm"
        npxPath = "node/bin/npx"
        corePackPath = ""
    }
    npmVersion = $NpmVersion
    npxVersion = $NpxVersion
    corepackIncluded = $false
    validation = @{
        staticValidation = "PASS"
        executionValidation = "NOT_EXECUTED"
    }
    treeSha256 = $TreeSha
    frozenRoot = "node"
    frozenAt = [DateTimeOffset]::UtcNow.ToString("o")
} | ConvertTo-Json -Depth 10

$BuildRecord | Set-Content -Path (Join-Path $OutputDir "node-build-record.json") -Encoding UTF8
Write-Host "[RECORD] node-build-record.json generated"

if (Test-Path $StagingRoot) { Remove-Item $StagingRoot -Recurse -Force }

Write-Host "============================================"
Write-Host "[DONE] Node Linux ARM64 prepare complete"
Write-Host "Output: $OutputDir"
Write-Host "  node/                    - frozen runtime"
Write-Host "  node-files.sha256        - tree manifest"
Write-Host "  node-build-record.json   - build record"
Write-Host "============================================"
