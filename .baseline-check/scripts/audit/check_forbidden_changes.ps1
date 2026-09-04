param(
    [string]$RepoRoot = "D:\桌面\跟进项目\U-Ai"
)

$ErrorActionPreference = "Stop"
$exitCode = 0

Write-Host "=== 边界守卫检查 ===" -ForegroundColor Cyan
Write-Host "检查时间: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"

$manifestPath = Join-Path $PSScriptRoot "allowed_dirs.json"
if (-not (Test-Path $manifestPath)) {
    Write-Host "错误: 找不到allowed_dirs.json" -ForegroundColor Red
    exit 1
}

$manifest = Get-Content $manifestPath -Raw | ConvertFrom-Json

$forbiddenDirs = $manifest.forbidden_directories
$forbiddenFiles = $manifest.forbidden_files

$modifiedFiles = git -C $RepoRoot diff --name-only HEAD 2>&1
$stagedFiles = git -C $RepoRoot diff --name-only --cached 2>&1

$allChanges = @($modifiedFiles) + @($stagedFiles) | Where-Object { $_ -and $_ -ne "" }

foreach ($file in $allChanges) {
    foreach ($dir in $forbiddenDirs) {
        if ($file -like "$dir*") {
            Write-Host "禁止修改: $file (位于禁止目录 $dir)" -ForegroundColor Red
            $exitCode = 1
        }
    }
    
    foreach ($f in $forbiddenFiles) {
        if ($file -like "*$f*" -and $file -notlike "scripts*" -and $file -notlike "docs*") {
            Write-Host "禁止修改: $file (匹配禁止文件模式 $f)" -ForegroundColor Red
            $exitCode = 1
        }
    }
}

$sourceFiles = $allChanges | Where-Object { $_ -match "\.(go|ts|vue|js)$" }
foreach ($file in $sourceFiles) {
    if (Test-Path (Join-Path $RepoRoot $file)) {
        $content = Get-Content (Join-Path $RepoRoot $file) -Raw -Encoding UTF8 -ErrorAction SilentlyContinue
        if ($content -match "//.*TODO.*解释|//.*说明|//.*注意|//.*备注|/\*.*\*/") {
            Write-Host "注释警告: $file 可能包含解释性注释" -ForegroundColor Yellow
        }
    }
}

if ($exitCode -eq 0) {
    Write-Host "边界守卫检查通过" -ForegroundColor Green
} else {
    Write-Host "边界守卫检查失败" -ForegroundColor Red
}

exit $exitCode
