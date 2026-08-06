param(
    [string]$SourcePath = "",
    [switch]$Quiet
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
if ([string]::IsNullOrWhiteSpace($SourcePath)) {
    $SourcePath = $repoRoot
}
$SourcePath = (Resolve-Path $SourcePath).Path

$exitCode = 0
$findings = [System.Collections.Generic.List[string]]::new()

function Write-Finding {
    param([string]$Message)
    $findings.Add($Message)
    if (-not $Quiet) {
        Write-Host $Message -ForegroundColor Red
    }
}

Write-Host "=== Source Archive Guard ===" -ForegroundColor Cyan
Write-Host "扫描目录: $SourcePath"
Write-Host "检查时间: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"

$forbiddenFileNames = @(
    "local-token",
    "mcp-secrets.key",
    "device.json",
    "qrcode.png"
)

$forbiddenDirNames = @(
    "AmitiaData",
    "migration_backups",
    "migration_backups_prev",
    "node_modules",
    "data.db"
)

$forbiddenSuffixes = @(
    ".db",
    ".db-wal",
    ".db-shm",
    ".db-journal",
    ".log",
    ".exe",
    ".dll",
    ".pdb"
)

$knownLeakedSecrets = @(
    "c3j22EREBHyFPEcP09s6C5cPSa-xxY3Vfslq2xZILbI",
    "Am1t1a-D3skt0p-L0c4l-S3cr3t-K3y-2026",
    "D3f4ult-R00t-JWT-S3cr3t-K3y-2026-Xyz",
    "Rj2Lf0Lh4RpEe4VwOrKoInJpMuS1bBmO",
    "hMMyAMy8t2JX",
    "sNRZFXQRAynD"
)

$skipDirs = @(
    ".git",
    "node_modules",
    "front/node_modules",
    "desktop/node_modules",
    "sdk/node_modules",
    "mobile_app",
    "wechat-chat-extractor"
)

$allFiles = Get-ChildItem -LiteralPath $SourcePath -Recurse -Force -File -ErrorAction SilentlyContinue |
    Where-Object {
        $rel = $_.FullName.Substring($SourcePath.Length).TrimStart('\', '/')
        $parts = $rel -split '[\\/]'
        $skip = $false
        foreach ($s in $skipDirs) {
            if ($parts -contains $s) { $skip = $true; break }
        }
        -not $skip
    }

foreach ($file in $allFiles) {
    $rel = $file.FullName.Substring($SourcePath.Length).TrimStart('\', '/')
    $parts = $rel -split '[\\/]'

    $relForward = $rel -replace '\\', '/'

    $allowedReleaseCore = $relForward.StartsWith("desktop/resources/core/")
    $allowedNodeRuntime = $relForward.StartsWith("backend/node/") -or
        $relForward.StartsWith("backend/security/node") -or
        $relForward.StartsWith("desktop/resources/core/")

    $isVerifyScript = $relForward -eq "scripts/verify-source-archive.ps1" -or
        $relForward -eq "scripts/verify-source-archive.sh"

    foreach ($dn in $forbiddenDirNames) {
        if ($parts -contains $dn) {
            Write-Finding "违禁目录: $rel"
        }
    }

    foreach ($fn in $forbiddenFileNames) {
        if ($file.Name -eq $fn) {
            Write-Finding "违禁文件: $rel"
        }
    }

    if (-not $allowedReleaseCore -and -not $allowedNodeRuntime) {
        foreach ($sfx in $forbiddenSuffixes) {
            if ($file.Name.EndsWith($sfx, [System.StringComparison]::OrdinalIgnoreCase)) {
                Write-Finding "违禁文件类型: $rel"
                break
            }
        }

        if ($file.Name -match "^(server|backend|amitia-ext|amitiax|surreal|qdrant)(\.exe)?$") {
            Write-Finding "已编译产物: $rel"
        }
    }

    if ($file.Name -match "^(backend-source|backend-src).*\.zip$") {
        Write-Finding "源码归档混入: $rel"
    }

    $isText = $file.Extension -match "^\.(go|ts|tsx|js|vue|dart|json|yml|yaml|md|txt|toml|ps1|sh|bat|py|rs|html|css|sql)$"
    if ($isText -and -not $isVerifyScript) {
        try {
            $content = Get-Content -LiteralPath $file.FullName -Raw -Encoding UTF8 -ErrorAction Stop
            foreach ($secret in $knownLeakedSecrets) {
                if ($content.Contains($secret)) {
                    Write-Finding "含已知泄露凭据($secret): $rel"
                }
            }
        } catch {
        }
    }
}

$runtimeDataDirs = @(
    "qdrant/storage",
    "qdrant/snapshots",
    "surrealdb/data",
    "surrealdb/storage",
    "backend/qdrant/storage",
    "backend/qdrant/snapshots",
    "desktop/resources/qdrant/storage",
    "desktop/resources/qdrant/snapshots"
)

foreach ($rd in $runtimeDataDirs) {
    $full = Join-Path $SourcePath ($rd -replace '/', [System.IO.Path]::DirectorySeparatorChar)
    if (Test-Path $full) {
        Write-Finding "向量库运行时数据: $rd"
    }
}

if ($findings.Count -gt 0) {
    Write-Host ""
    Write-Host "Source Archive Guard 失败: $($findings.Count) 项违禁内容" -ForegroundColor Red
    $exitCode = 1
} else {
    Write-Host ""
    Write-Host "Source Archive Guard 通过，未发现运行数据或泄露凭据" -ForegroundColor Green
}

exit $exitCode
