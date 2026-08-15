$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RuntimeRoot = Split-Path -Parent $ScriptDir
$LockFile = Join-Path $RuntimeRoot "build\runtime-package\linux-arm64\runtime-package-lock.json"
$OutputDir = Join-Path $RuntimeRoot "build\out\runtime-package\linux-arm64"
$StagingDir = Join-Path $RuntimeRoot "build\staging\runtime-package-linux-arm64"

Write-Host "============================================"
Write-Host " Runtime Package Build - Linux ARM64"
Write-Host "============================================"

if (-not (Test-Path $LockFile)) {
    Write-Error "[FATAL] Lock file not found: $LockFile"
    exit 1
}

$Lock = Get-Content $LockFile -Raw | ConvertFrom-Json
$RuntimeVersion = $Lock.runtimeVersion
$PackageFormatVersion = $Lock.packageFormatVersion

Write-Host "Runtime Version: $RuntimeVersion"
Write-Host "Package Format:  $PackageFormatVersion"
Write-Host "============================================"

$RequiredRecords = @(
    "node",
    "rootfs",
    "backend",
    "qdrant",
    "plugin-host",
    "task-host"
)

foreach ($component in $RequiredRecords) {
    $recordPath = Join-Path $RuntimeRoot "build\out\$component\linux-arm64\$component-build-record.json"
    if (-not (Test-Path $recordPath)) {
        Write-Error "[FATAL] Missing build record for $component : $recordPath"
        Write-Error "All frozen input build records are required. Cannot auto-build missing components."
        exit 1
    }
    Write-Host "[INPUT] $component build record loaded: $recordPath"
}

$NodeVersion = $Lock.components.node.version
$NodeExpectedTreeSha = $Lock.components.node.treeSha256

$NodeArtifacts = Join-Path $RuntimeRoot "build\out\node\linux-arm64\node"
if (-not (Test-Path $NodeArtifacts)) {
    Write-Error "[FATAL] Node frozen artifacts not found: $NodeArtifacts"
    exit 1
}

$NodeFilesSha = Join-Path $RuntimeRoot "build\out\node\linux-arm64\node-files.sha256"
if (-not (Test-Path $NodeFilesSha)) {
    Write-Error "[FATAL] Node tree manifest not found: $NodeFilesSha"
    exit 1
}

$NodeActualTreeSha = (Get-FileHash -Path $NodeFilesSha -Algorithm SHA256).Hash.ToLower()
if ($NodeActualTreeSha -ne $NodeExpectedTreeSha) {
    Write-Error "[FATAL] Node tree SHA mismatch: lock=$NodeExpectedTreeSha actual=$NodeActualTreeSha"
    exit 1
}
Write-Host "[VERIFY] Node tree SHA verified: $NodeActualTreeSha"

$RootfsTar = Join-Path $RuntimeRoot "build\out\rootfs\linux-arm64\ubuntu-rootfs-arm64.tar"
if (-not (Test-Path $RootfsTar)) {
    Write-Error "[FATAL] Rootfs frozen archive not found. Run prepare-ubuntu-rootfs-arm64.sh first."
    exit 1
}
Write-Host "[INPUT] Rootfs archive: $RootfsTar"

$BackendTar = Join-Path $RuntimeRoot "build\out\backend\linux-arm64\amitia-backend-linux-arm64.tar.xz"
if (-not (Test-Path $BackendTar)) {
    Write-Error "[FATAL] Backend frozen archive not found."
    exit 1
}
Write-Host "[INPUT] Backend archive: $BackendTar"

$QdrantTar = Join-Path $RuntimeRoot "build\out\qdrant\linux-arm64\qdrant-linux-arm64.tar.xz"
if (-not (Test-Path $QdrantTar)) {
    Write-Error "[FATAL] Qdrant frozen archive not found."
    exit 1
}
Write-Host "[INPUT] Qdrant archive: $QdrantTar"

if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
}

$BuildId = (Get-Date -Format "yyyyMMddHHmmss") + "-" + $PID
$StagingPath = Join-Path $StagingDir $BuildId
if (Test-Path $StagingPath) { Remove-Item $StagingPath -Recurse -Force }
New-Item -ItemType Directory -Force -Path $StagingPath | Out-Null

try {
    $PayloadDir = Join-Path $StagingPath "payload"
    $MetadataDir = Join-Path $StagingPath "metadata"
    $RuntimeDir = Join-Path $PayloadDir "runtime"
    $RootfsDir = Join-Path $PayloadDir "rootfs"
    $ComponentRecordsDir = Join-Path $MetadataDir "component-build-records"

    New-Item -ItemType Directory -Force -Path $RuntimeDir | Out-Null
    New-Item -ItemType Directory -Force -Path $RootfsDir | Out-Null
    New-Item -ItemType Directory -Force -Path $ComponentRecordsDir | Out-Null

    Copy-Item -Path $NodeArtifacts -Destination (Join-Path $RuntimeDir "node") -Recurse
    Copy-Item -Path $BackendTar -Destination (Join-Path $RuntimeDir "backend.tar.xz") -Force
    Copy-Item -Path $QdrantTar -Destination (Join-Path $RuntimeDir "qdrant.tar.xz") -Force
    Copy-Item -Path $RootfsTar -Destination (Join-Path $RootfsDir "rootfs.tar") -Force
    Copy-Item -Path $NodeFilesSha -Destination (Join-Path $ComponentRecordsDir "node-files.sha256") -Force

    foreach ($component in $RequiredRecords) {
        $srcRec = Join-Path $RuntimeRoot "build\out\$component\linux-arm64\$component-build-record.json"
        if (Test-Path $srcRec) {
            Copy-Item -Path $srcRec -Destination (Join-Path $ComponentRecordsDir "$component-build-record.json") -Force
        }
    }

    $PackageIndex = @{
        schemaVersion = 1
        runtimeVersion = $RuntimeVersion
        packageFormatVersion = $PackageFormatVersion
        guestOs = "linux"
        guestArchitecture = "arm64"
        buildMode = "release"
        components = @{
            node = @{
                version = $NodeVersion
                treeSha256 = $NodeExpectedTreeSha
            }
        }
    } | ConvertTo-Json -Depth 10
    $PackageIndex | Set-Content -Path (Join-Path $MetadataDir "package-index.json") -Encoding UTF8

    $ComponentLock = @{
        schemaVersion = 1
        runtimeVersion = $RuntimeVersion
        requiredComponents = @("backend", "node", "qdrant", "rootfs", "plugin-host", "task-host")
    } | ConvertTo-Json -Depth 5
    $ComponentLock | Set-Content -Path (Join-Path $MetadataDir "component-lock.json") -Encoding UTF8

    $PayloadSha = & {
        Get-ChildItem -Path $PayloadDir -Recurse -File |
            Sort-Object { $_.FullName.Substring($PayloadDir.Length + 1) } |
            ForEach-Object {
                $RelPath = $_.FullName.Substring($PayloadDir.Length + 1) -replace '\\', '/'
                $Hash = (Get-FileHash -Path $_.FullName -Algorithm SHA256).Hash.ToLower()
                "$Hash  $RelPath"
            }
    } | & {
        $sha = [System.Security.Cryptography.SHA256]::Create()
        $bytes = [System.Text.Encoding]::UTF8.GetBytes(($input -join "`n"))
        $hash = $sha.ComputeHash($bytes)
        ($hash | ForEach-Object { '{0:x2}' -f $_ }) -join ''
        $sha.Dispose()
    }
    Write-Host "[VERIFY] Payload tree SHA: $PayloadSha"

    $SortedFiles = Get-ChildItem -Path $PayloadDir -Recurse -File | Sort-Object { $_.FullName.Substring($PayloadDir.Length + 1) }
    $Sha256SumsContent = $SortedFiles | ForEach-Object {
        $RelPath = $_.FullName.Substring($PayloadDir.Length + 1) -replace '\\', '/'
        $Hash = (Get-FileHash -Path $_.FullName -Algorithm SHA256).Hash.ToLower()
        "$Hash  $RelPath"
    }
    $Sha256SumsContent | Set-Content -Path (Join-Path $MetadataDir "SHA256SUMS") -Encoding UTF8

    $PackageFileName = "amitia-runtime-$RuntimeVersion-linux-arm64.zip"
    $PackagePath = Join-Path $OutputDir $PackageFileName
    $TempPackagePath = "$PackagePath.tmp.$BuildId"

    if (Test-Path $TempPackagePath) { Remove-Item $TempPackagePath -Force }

    Write-Host "[PACK] Creating package archive..."
    $SortedAllFiles = Get-ChildItem -Path $StagingPath -Recurse -File | Sort-Object { $_.FullName.Substring($StagingPath.Length + 1) }
    $FileNamesFile = [System.IO.Path]::GetTempFileName()
    ($SortedAllFiles | ForEach-Object { $_.FullName }) | Set-Content -Path $FileNamesFile -Encoding UTF8

    try {
        & zip -rq -@ $TempPackagePath --names-stdin "$FileNamesFile" 2>&1
        if ($LASTEXITCODE -ne 0) {
            throw "zip failed with exit code $LASTEXITCODE"
        }
    } finally {
        Remove-Item $FileNamesFile -Force -ErrorAction SilentlyContinue
    }

    $PackageSha = (Get-FileHash -Path $TempPackagePath -Algorithm SHA256).Hash.ToLower()
    $PackageSize = (Get-Item $TempPackagePath).Length

    if (Test-Path $PackagePath) { Remove-Item $PackagePath -Force }
    Move-Item -Path $TempPackagePath -Destination $PackagePath -Force

    Write-Host "[PACK] Package created: $PackagePath"
    Write-Host "[PACK] SHA256: $PackageSha"
    Write-Host "[PACK] Size: $PackageSize bytes"

    $Timestamp = [DateTimeOffset]::UtcNow.ToString("yyyy-MM-ddTHH:mm:ssZ")

    $FinalRecord = @{
        schemaVersion = 1
        runtimeVersion = $RuntimeVersion
        packageFormatVersion = $PackageFormatVersion
        guestOs = "linux"
        guestArchitecture = "arm64"
        buildMode = "release"
        package = @{
            file = $PackageFileName
            sha256 = $PackageSha
            size = $PackageSize
        }
        node = @{
            version = $NodeVersion
            treeSha256 = $NodeExpectedTreeSha
            verifiedSha = $NodeActualTreeSha
        }
        createdAt = $Timestamp
    } | ConvertTo-Json -Depth 10

    $RecordPath = Join-Path $OutputDir "runtime-package-build-record.json"
    $FinalRecord | Set-Content -Path $RecordPath -Encoding UTF8
    Write-Host "[RECORD] runtime-package-build-record.json written"

    "$PackageSha  $PackageFileName" | Set-Content -Path (Join-Path $OutputDir "$PackageFileName.sha256") -Encoding UTF8
    Write-Host "[DONE] $PackageFileName.sha256 written"

} finally {
    if (Test-Path $StagingPath) { Remove-Item $StagingPath -Recurse -Force }
}

Write-Host "============================================"
Write-Host "[DONE] Runtime Package Build Complete"
Write-Host "============================================"
Write-Host " Output: $OutputDir"
Write-Host "  $PackageFileName"
Write-Host "  $PackageFileName.sha256"
Write-Host "  runtime-package-build-record.json"
Write-Host "============================================"
