param(
    [switch]$Offline,
    [string]$CacheDir,
    [string]$StagingDir,
    [string]$OutputDir,
    [switch]$SkipVerify,
    [switch]$DevMode
)

$ErrorActionPreference = "Stop"

$ReleaseMode = -not $DevMode
if ($SkipVerify -and $ReleaseMode) {
    Write-Error "[FATAL] -SkipVerify is not allowed in release mode"
    exit 1
}

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

Write-Host "[CLEAN] Cleaning rootfs before freeze..." -ForegroundColor Cyan
@(
    "$ExtractDir\tmp\*",
    "$ExtractDir\var\tmp\*",
    "$ExtractDir\var\cache\apt\*"
) | ForEach-Object {
    Remove-Item $_ -Force -ErrorAction SilentlyContinue
}
Remove-Item "$ExtractDir\etc\machine-id" -Force -ErrorAction SilentlyContinue
Get-ChildItem "$ExtractDir\etc\ssh" -Filter "ssh_host_*" -ErrorAction SilentlyContinue | Remove-Item -Force
Remove-Item "$ExtractDir\root\.bash_history" -Force -ErrorAction SilentlyContinue
Remove-Item "$ExtractDir\root\.cache" -Recurse -Force -ErrorAction SilentlyContinue
Get-ChildItem "$ExtractDir" -Filter "*.log" -Recurse -ErrorAction SilentlyContinue | Remove-Item -Force
Get-ChildItem "$ExtractDir\var\log" -File -ErrorAction SilentlyContinue | Remove-Item -Force
Write-Host "[CLEAN] Rootfs cleaned" -ForegroundColor Green

if (-not $SkipVerify) {
    Write-Host "[VERIFY] Running static validation on final rootfs tree..." -ForegroundColor Cyan
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
Write-Host "============================================" -ForegroundColor Cyan

Write-Host "[FREEZE] Generating tree manifest..." -ForegroundColor Cyan
$ManifestLines = Get-ChildItem -Path $ExtractDir -Recurse -File | Sort-Object { $_.FullName.Substring($ExtractDir.Length + 1) } | ForEach-Object {
    $RelPath = $_.FullName.Substring($ExtractDir.Length + 1) -replace '\\', '/'
    $Hash = (Get-FileHash -Path $_.FullName -Algorithm SHA256).Hash.ToLower()
    "$Hash  $RelPath"
}
$ManifestLines | Set-Content -Path (Join-Path $OutputPath "rootfs-files.sha256") -Encoding UTF8
Write-Host "[PASS] rootfs-files.sha256 generated" -ForegroundColor Green

$Sha256Algo = [System.Security.Cryptography.SHA256]::Create()
$ManifestBytes = [System.Text.Encoding]::UTF8.GetBytes(($ManifestLines -join "`n"))
$HashBytes = $Sha256Algo.ComputeHash($ManifestBytes)
$TreeSha = ($HashBytes | ForEach-Object { '{0:x2}' -f $_ }) -join ''
$Sha256Algo.Dispose()
Write-Host "[TREE SHA] $TreeSha" -ForegroundColor Green

$FrozenTarName = "ubuntu-rootfs-arm64.tar"
$FrozenTarPath = Join-Path $OutputPath $FrozenTarName
$TempTarPath = "$FrozenTarPath.tmp.$([guid]::NewGuid().ToString('N').Substring(0,8))"

if (Test-Path $TempTarPath) { Remove-Item $TempTarPath -Force }

Write-Host "[FREEZE] Creating deterministic frozen tar archive..." -ForegroundColor Cyan
$SortedFiles = Get-ChildItem -Path $ExtractDir -Recurse -File | Sort-Object { $_.FullName.Substring($ExtractDir.Length + 1) } | ForEach-Object { $_.FullName }
$FileNamesFile = [System.IO.Path]::GetTempFileName()
$SortedFiles | Set-Content -Path $FileNamesFile -Encoding UTF8

try {
    & tar --null -T $FileNamesFile --mtime=@0 --owner=0 --group=0 --numeric-owner -cf $TempTarPath -C $ExtractDir 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "tar failed with exit code $LASTEXITCODE"
    }
} finally {
    Remove-Item $FileNamesFile -Force -ErrorAction SilentlyContinue
}

$FrozenSha = (Get-FileHash -Path $TempTarPath -Algorithm SHA256).Hash.ToLower()

if (Test-Path $FrozenTarPath) { Remove-Item $FrozenTarPath -Force }
Move-Item -Path $TempTarPath -Destination $FrozenTarPath -Force
Write-Host "[FREEZE] Frozen archive created: $FrozenTarPath" -ForegroundColor Green
Write-Host "[FREEZE] Frozen SHA256: $FrozenSha" -ForegroundColor Green

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
    frozen = @{
        archiveFileName = $FrozenTarName
        archiveSha256 = $FrozenSha
        treeSha256 = $TreeSha
    }
    outputPath = $OutputPath
    timestamp = [DateTimeOffset]::UtcNow.ToString("o")
    offline = [bool]$Offline
} | ConvertTo-Json -Depth 5

$BuildRecordPath = Join-Path $OutputPath "rootfs-build-record.json"
$BuildRecord | Set-Content -Path $BuildRecordPath -Encoding UTF8
Write-Host "[RECORD] Final build record: $BuildRecordPath" -ForegroundColor Green

Write-Host "============================================" -ForegroundColor Cyan
Write-Host "[DONE] Ubuntu ARM64 Rootfs Freeze Complete" -ForegroundColor Cyan
Write-Host "============================================" -ForegroundColor Cyan
Write-Host " Output: $OutputPath"
Write-Host "  $FrozenTarName   - frozen rootfs archive"
Write-Host "  rootfs-files.sha256     - tree manifest"
Write-Host "  rootfs-build-record.json - final build record"
Write-Host "============================================" -ForegroundColor Cyan

return @{
    RootfsPath = $ExtractDir
    BuildRecordPath = $BuildRecordPath
    ArchiveSha256 = $ActualSha
    FrozenSha256 = $FrozenSha
    TreeSha256 = $TreeSha
    StagingDir = $StagingPath
    OutputDir = $OutputPath
}
