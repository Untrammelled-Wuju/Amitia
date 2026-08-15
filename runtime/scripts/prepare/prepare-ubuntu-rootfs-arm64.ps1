param(
    [switch]$Offline,
    [string]$CacheDir,
    [string]$StagingDir,
    [string]$OutputDir,
    [switch]$SkipVerify
)

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent (Split-Path -Parent (Split-Path -Parent $ScriptDir))
$LockFile = Join-Path $ProjectRoot "runtime\artifacts\ubuntu-rootfs\linux-arm64\ubuntu-rootfs-lock.json"
$PolicyFile = Join-Path $ProjectRoot "runtime\artifacts\ubuntu-rootfs\linux-arm64\rootfs-policy.json"

if (-not (Test-Path $LockFile)) {
    Write-Error "Lock file not found: $LockFile"
    exit 1
}

$Lock = Get-Content $LockFile -Raw | ConvertFrom-Json
$Release = $Lock.release
$ArchiveFileName = $Lock.archiveFileName
$ExpectedSha = $Lock.sha256
$SourceUrl = $Lock.sourceUrl

Write-Host "============================================" -ForegroundColor Cyan
Write-Host " Ubuntu ARM64 Rootfs Prepare" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host " Release:        $Release"
Write-Host " Archive:        $ArchiveFileName"
Write-Host " Expected SHA256: $ExpectedSha"
Write-Host " Source:         $SourceUrl"
Write-Host "============================================" -ForegroundColor Cyan

if ($CacheDir) {
    $CachePath = $CacheDir
} else {
    $CachePath = Join-Path $ProjectRoot "runtime\.cache\ubuntu-rootfs"
}
if ($StagingDir) {
    $StagingPath = $StagingDir
} else {
    $StagingPath = Join-Path $ProjectRoot "runtime\build\staging\ubuntu-rootfs-arm64\$((Get-Date).ToString('yyyyMMddHHmmss'))"
}
if ($OutputDir) {
    $OutputPath = $OutputDir
} else {
    $OutputPath = Join-Path $ProjectRoot "runtime\build\out\rootfs\linux-arm64"
}

New-Item -ItemType Directory -Force -Path $CachePath | Out-Null
New-Item -ItemType Directory -Force -Path $StagingPath | Out-Null
New-Item -ItemType Directory -Force -Path $OutputPath | Out-Null

$ArchiveFile = Join-Path $CachePath $ArchiveFileName

if (Test-Path $ArchiveFile) {
    $CachedSha = (Get-FileHash -Path $ArchiveFile -Algorithm SHA256).Hash.ToLower()
    if ($CachedSha -eq $ExpectedSha) {
        Write-Host "[CACHE] Cached archive SHA matches" -ForegroundColor Green
    } else {
        Write-Host "[CACHE] Cached archive SHA mismatch, re-downloading" -ForegroundColor Yellow
        Remove-Item $ArchiveFile -Force
    }
}

if (-not (Test-Path $ArchiveFile)) {
    if ($Offline) {
        Write-Error "Offline mode and no cached archive available"
        exit 1
    }
    Write-Host "[DOWNLOAD] $SourceUrl" -ForegroundColor Cyan
    $TmpFile = "$ArchiveFile.tmp"
    try {
        Invoke-WebRequest -Uri $SourceUrl -OutFile $TmpFile -UseBasicParsing -ErrorAction Stop
        $DownloadSha = (Get-FileHash -Path $TmpFile -Algorithm SHA256).Hash.ToLower()
        if ($DownloadSha -ne $ExpectedSha) {
            Write-Error "SHA256 mismatch after download! Expected: $ExpectedSha, Got: $DownloadSha"
            Remove-Item $TmpFile -Force -ErrorAction SilentlyContinue
            exit 1
        }
        Move-Item -Path $TmpFile -Destination $ArchiveFile -Force
    } catch {
        Write-Error "Download failed: $_"
        Remove-Item $TmpFile -Force -ErrorAction SilentlyContinue
        exit 1
    }
}

Write-Host "[VERIFY] Verifying archive SHA256..." -ForegroundColor Cyan
$ActualSha = (Get-FileHash -Path $ArchiveFile -Algorithm SHA256).Hash.ToLower()
if ($ActualSha -ne $ExpectedSha) {
    Write-Error "SHA256 mismatch! Expected: $ExpectedSha, Got: $ActualSha"
    exit 1
}
Write-Host "[PASS] SHA256 verified: $ActualSha" -ForegroundColor Green

Write-Host "[EXTRACT] Extracting to staging..." -ForegroundColor Cyan
$ExtractDir = Join-Path $StagingPath "rootfs"
if (Test-Path $ExtractDir) {
    Remove-Item $ExtractDir -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $ExtractDir | Out-Null

try {
    tar xzf $ArchiveFile -C $ExtractDir
} catch {
    Write-Error "Extraction failed: $_"
    exit 1
}

Write-Host "[EXTRACT] Extraction complete: $ExtractDir" -ForegroundColor Green

$BuildRecord = @{
    schemaVersion = 1
    component = "ubuntu-rootfs"
    distribution = $Lock.distribution
    flavor = $Lock.flavor
    release = $Lock.release
    codename = $Lock.codename
    architecture = $Lock.architecture
    guestPlatform = $Lock.guestPlatform
    runtimeKind = $Lock.runtimeKind
    source = @{
        url = $Lock.sourceUrl
        archiveFileName = $Lock.archiveFileName
        expectedSha256 = $ExpectedSha
        actualSha256 = $ActualSha
    }
    stagingPath = $ExtractDir
    timestamp = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")
    offline = [bool]$Offline
}

$BuildRecordPath = Join-Path $StagingPath "rootfs-build-record.json"
$BuildRecord | ConvertTo-Json -Depth 5 | Set-Content -Path $BuildRecordPath -Encoding UTF8

Write-Host "[RECORD] Build record written: $BuildRecordPath" -ForegroundColor Green

if (-not $SkipVerify) {
    Write-Host "[VERIFY] Running static validation..." -ForegroundColor Cyan
    $ValidatorPath = Join-Path $ProjectRoot "runtime\validation\linux-arm64\rootfs_validator.py"
    if (Test-Path $ValidatorPath) {
        $VerifyArgs = @("--rootfs", $ExtractDir, "--lock", $LockFile, "--policy", $PolicyFile)
        & python $ValidatorPath @VerifyArgs
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Validation failed with exit code $LASTEXITCODE"
            exit 1
        }
        Write-Host "[PASS] Static validation passed" -ForegroundColor Green
    } else {
        Write-Host "[SKIP] Validator not found: $ValidatorPath" -ForegroundColor Yellow
    }
}

Write-Host "============================================" -ForegroundColor Cyan
Write-Host " Ubuntu ARM64 Rootfs Prepare Complete" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host " Staging: $ExtractDir"
Write-Host " Build Record: $BuildRecordPath"
Write-Host "============================================" -ForegroundColor Cyan

return @{
    RootfsPath = $ExtractDir
    BuildRecordPath = $BuildRecordPath
    ArchiveSha256 = $ActualSha
    StagingDir = $StagingPath
    OutputDir = $OutputPath
}
