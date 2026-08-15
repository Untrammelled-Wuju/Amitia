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
    "backend",
    "node",
    "qdrant",
    "rootfs",
    "sidecar",
    "qqSidecar"
)

$BuildRecords = @{}
foreach ($component in $RequiredRecords) {
    $recordPath = Join-Path $RuntimeRoot "build\out\$component\linux-arm64\$component-build-record.json"
    if (-not (Test-Path $recordPath)) {
        $altPath = Join-Path $RuntimeRoot "build\out\$component\linux-arm64\$($component)-build-record.json"
        if (Test-Path $altPath) {
            $recordPath = $altPath
        }
    }
    if (-not (Test-Path $recordPath)) {
        Write-Error "[FATAL] Missing build record for $component : $recordPath"
        Write-Error "All frozen input build records are required. Cannot auto-build missing components."
        exit 1
    }
    $BuildRecords[$component] = Get-Content $recordPath -Raw | ConvertFrom-Json
    Write-Host "[INPUT] $component build record loaded: $recordPath"
}

$NodeRecord = $BuildRecords["node"]
if ($NodeRecord.version -ne $Lock.components.node.version) {
    Write-Error "[FATAL] Node version mismatch: lock=$($Lock.components.node.version) actual=$($NodeRecord.version)"
    exit 1
}

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
if ($NodeActualTreeSha -ne $Lock.components.node.treeSha256) {
    Write-Error "[FATAL] Node tree SHA mismatch: lock=$($Lock.components.node.treeSha256) actual=$NodeActualTreeSha"
    exit 1
}
Write-Host "[VERIFY] Node tree SHA verified: $NodeActualTreeSha"

$RootfsRecord = $BuildRecords["rootfs"]
$RootfsTar = Join-Path $OutputDir "..\..\..\..\rootfs\linux-arm64\ubuntu-rootfs-arm64.tar"
$RootfsTar = Resolve-Path $RootfsTar -ErrorAction SilentlyContinue
if (-not $RootfsTar -or -not (Test-Path $RootfsTar)) {
    Write-Error "[FATAL] Rootfs frozen archive not found. Run prepare-ubuntu-rootfs-arm64.sh first."
    exit 1
}
Write-Host "[INPUT] Rootfs archive: $RootfsTar"

if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
}

$StagingId = [guid]::NewGuid().ToString("N").Substring(0, 12)
$StagingPath = Join-Path $StagingDir $StagingId
if (Test-Path $StagingPath) { Remove-Item $StagingPath -Recurse -Force }
New-Item -ItemType Directory -Force -Path $StagingPath | Out-Null

try {
    $PayloadDir = Join-Path $StagingPath "payload"
    $MetadataDir = Join-Path $StagingPath "metadata"
    New-Item -ItemType Directory -Force -Path $PayloadDir | Out-Null
    New-Item -ItemType Directory -Force -Path $MetadataDir | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $MetadataDir "component-build-records") | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $PayloadDir "program") | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $PayloadDir "rootfs") | Out-Null

    $ProgramDir = Join-Path $PayloadDir "program"
    Copy-Item -Path $NodeArtifacts -Destination (Join-Path $ProgramDir "node") -Recurse

    foreach ($recName in $RequiredRecords) {
        $srcRec = Join-Path $RuntimeRoot "build\out\$recName\linux-arm64\$recName-build-record.json"
        if (-not (Test-Path $srcRec)) {
            $srcRec = Join-Path $RuntimeRoot "build\out\$recName\linux-arm64\$($recName)-build-record.json"
        }
        if (Test-Path $srcRec) {
            Copy-Item -Path $srcRec -Destination (Join-Path $MetadataDir "component-build-records" "$recName-build-record.json") -Force
        }
    }

    Copy-Item -Path $NodeFilesSha -Destination (Join-Path $MetadataDir "component-build-records" "node-files.sha256") -Force

    $PackageManifest = @{
        schemaVersion = 1
        runtimeVersion = $RuntimeVersion
        packageFormatVersion = $PackageFormatVersion
        guestOs = "linux"
        guestArchitecture = "arm64"
        buildMode = "release"
        createdAt = [DateTimeOffset]::UtcNow.ToString("o")
        components = @{
            node = @{
                version = $Lock.components.node.version
                treeSha256 = $Lock.components.node.treeSha256
            }
            rootfs = @{
                rootfsId = $Lock.components.rootfs.rootfsId
                sha256 = $Lock.components.rootfs.sha256
            }
        }
    } | ConvertTo-Json -Depth 10
    $PackageManifest | Set-Content -Path (Join-Path $MetadataDir "package-manifest.json") -Encoding UTF8

    $PackageFileName = "amitia-runtime-$RuntimeVersion-linux-arm64.zip"
    $PackagePath = Join-Path $OutputDir $PackageFileName
    $TempPackagePath = "$PackagePath.tmp.$StagingId"

    if (Test-Path $TempPackagePath) { Remove-Item $TempPackagePath -Force }

    Write-Host "[PACK] Creating package archive..."
    Compress-Archive -Path "$StagingPath\*" -DestinationPath $TempPackagePath -Force

    $PackageSha = (Get-FileHash -Path $TempPackagePath -Algorithm SHA256).Hash.ToLower()
    $PackageSize = (Get-Item $TempPackagePath).Length

    if (Test-Path $PackagePath) { Remove-Item $PackagePath -Force }
    Move-Item -Path $TempPackagePath -Destination $PackagePath -Force

    Write-Host "[PACK] Package created: $PackagePath"
    Write-Host "[PACK] SHA256: $PackageSha"
    Write-Host "[PACK] Size: $PackageSize bytes"

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
            version = $Lock.components.node.version
            treeSha256 = $Lock.components.node.treeSha256
            verifiedSha = $NodeActualTreeSha
        }
        rootfs = @{
            rootfsId = $Lock.components.rootfs.rootfsId
            sha256 = $Lock.components.rootfs.sha256
        }
        createdAt = [DateTimeOffset]::UtcNow.ToString("o")
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
