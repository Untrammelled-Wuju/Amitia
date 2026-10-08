$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$backendDir = Join-Path $root "backend"
$desktopDir = Join-Path $root "desktop"
$adminSystemDir = Join-Path $root "admin-system"
$adminLogDir = Join-Path $root "logs"
$nodeExe = Join-Path $root "desktop\resources\core\node\node.exe"
$surrealExe = Join-Path $backendDir "surrealdb\surreal.exe"
$qdrantExe = Join-Path $backendDir "qdrant\qdrant.exe"
    $serverExe = Join-Path $backendDir "server.codex2.exe"
    if (-not (Test-Path $serverExe)) {
        $serverExe = Join-Path $backendDir "server.codex.exe"
    }
    if (-not (Test-Path $serverExe)) {
        $serverExe = Join-Path $backendDir "server.runtime.exe"
    }
    if (-not (Test-Path $serverExe)) {
        $serverExe = Join-Path $backendDir "server.current.exe"
    }
    if (-not (Test-Path $serverExe)) {
        $serverExe = Join-Path $backendDir "server.new.exe"
    }
if (-not (Test-Path $serverExe)) {
    $serverExe = Join-Path $backendDir "server.exe"
}
$adminServerExe = Join-Path $backendDir "admin-server.exe"
$goExe = "C:\Code\Go\bin\go.exe"
$surrealPass = "AmitiaSurrealDBRootPassword20260831Securex"
$adminMySQLConfigPath = Join-Path $root "config\admin-mysql.local.json"
$adminMySQLConfig = $null
if (Test-Path $adminMySQLConfigPath) {
    $adminMySQLConfig = Get-Content -Raw $adminMySQLConfigPath | ConvertFrom-Json
}

$env:AMITIA_RUNTIME_ROOT = Join-Path $root "AmitiaData"
$env:AMITIA_CONFIG_DIR = Join-Path $env:AMITIA_RUNTIME_ROOT "config"
$env:AMITIA_WORKSPACE_DIR = $root
$env:AMITIA_DATA_DIR = Join-Path $root "AmitiaData"
$env:AMITIA_RUN_MODE = "desktop"
$env:AMITIA_SURREAL_USER = "root"
$env:AMITIA_SURREAL_PASSWORD = $surrealPass
$env:AMITIA_EXTENSION_DEV_MODE = "true"

function Stop-ProjectProcess {
    param([int]$ProcessId)
    $process = Get-CimInstance Win32_Process -Filter "ProcessId = $ProcessId" -ErrorAction SilentlyContinue
    if ($null -eq $process -or [string]::IsNullOrWhiteSpace($process.ExecutablePath)) {
        return
    }
    if (-not $process.ExecutablePath.StartsWith($root + "\", [System.StringComparison]::OrdinalIgnoreCase)) {
        return
    }
    Stop-Process -Id $ProcessId -Force -ErrorAction SilentlyContinue
}

Write-Host "=== U-Ai 完整启动脚本 ===" -ForegroundColor Cyan

Write-Host "`n[1/8] 清理项目旧进程..." -ForegroundColor Yellow
$projectNames = @("server", "server.codex2", "server.codex", "server.runtime", "server.current", "server.new", "admin-server", "AmitiaCore", "qdrant", "surreal", "mysqld", "node", "electron")
Get-CimInstance Win32_Process -ErrorAction SilentlyContinue |
    Where-Object {
        $_.Name -and [System.IO.Path]::GetFileNameWithoutExtension($_.Name) -in $projectNames -and
        $_.ExecutablePath -and $_.ExecutablePath.StartsWith($root, [System.StringComparison]::OrdinalIgnoreCase)
    } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }

$projectPorts = @(18899, 18998, 18000, 19178, 15178, 15179)
if ($adminMySQLConfig -and $adminMySQLConfig.managed -and $adminMySQLConfig.port) {
    $projectPorts += [int]$adminMySQLConfig.port
}
foreach ($port in $projectPorts) {
    Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue |
        ForEach-Object {
            if ($port -eq [int]$adminMySQLConfig.port) {
                Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue
            } else {
                Stop-ProjectProcess -ProcessId $_.OwningProcess
            }
        }
}
Start-Sleep -Seconds 3

Write-Host "`n[2/8] 启动管理服务 MySQL..." -ForegroundColor Yellow
if ($adminMySQLConfig -and $adminMySQLConfig.managed) {
    $mysqlBaseDir = [string]$adminMySQLConfig.baseDir
    $mysqlDataDir = [string]$adminMySQLConfig.dataDir
    if (-not [System.IO.Path]::IsPathRooted($mysqlDataDir)) {
        $mysqlDataDir = Join-Path $root $mysqlDataDir
    }
    $mysqlRuntimeDir = Split-Path -Parent $mysqlDataDir
    $mysqldExe = Join-Path $mysqlBaseDir "bin\mysqld.exe"
    $mysqlExe = Join-Path $mysqlBaseDir "bin\mysql.exe"
    $mysqlAdminExe = Join-Path $mysqlBaseDir "bin\mysqladmin.exe"
    if (-not (Test-Path $mysqldExe) -or -not (Test-Path $mysqlExe)) {
        Write-Host "未找到 MySQL 服务端程序: $mysqlBaseDir" -ForegroundColor Red
        exit 1
    }
    $mysqlPort = [int]$adminMySQLConfig.port
    $mysqlErrorLog = Join-Path $mysqlRuntimeDir "mysqld.err"
    $mysqlIniPath = Join-Path $mysqlRuntimeDir "my.ini"
    $mysqlIniContent = @"
[mysqld]
basedir="$($mysqlBaseDir -replace '\\','/')"
datadir="$($mysqlDataDir -replace '\\','/')"
port=$mysqlPort
bind-address=127.0.0.1
mysqlx=0
character-set-server=utf8mb4
collation-server=utf8mb4_unicode_ci
log-error="$($mysqlErrorLog -replace '\\','/')"
"@
    New-Item -ItemType Directory -Force -Path $mysqlRuntimeDir | Out-Null
    New-Item -ItemType Directory -Force -Path $mysqlDataDir | Out-Null
    Set-Content -LiteralPath $mysqlIniPath -Value $mysqlIniContent -Encoding UTF8
    if (-not (Test-Path (Join-Path $mysqlDataDir "mysql"))) {
        & $mysqldExe "--defaults-file=$($mysqlIniPath -replace '\\','/')" "--initialize-insecure"
        if ($LASTEXITCODE -ne 0) {
            Write-Host "MySQL 数据目录初始化失败" -ForegroundColor Red
            exit 1
        }
    }
    Start-Process -FilePath $mysqldExe `
        -ArgumentList "--defaults-file=$($mysqlIniPath -replace '\\','/')" `
        -WindowStyle Hidden
    $mysqlReady = $false
    for ($i = 0; $i -lt 30; $i++) {
        & $mysqlAdminExe "--host=127.0.0.1" "--port=$mysqlPort" "--user=root" "--protocol=TCP" "ping" 2>$null | Out-Null
        if ($LASTEXITCODE -eq 0) {
            $mysqlReady = $true
            break
        }
        Start-Sleep -Seconds 1
    }
    if (-not $mysqlReady) {
        Write-Host "管理服务 MySQL 启动失败，请查看 $mysqlErrorLog" -ForegroundColor Red
        exit 1
    }
    $mysqlPassword = [string]$adminMySQLConfig.password
    $mysqlDatabase = [string]$adminMySQLConfig.database
    $mysqlUser = [string]$adminMySQLConfig.username
    $provisionSQL = "CREATE DATABASE IF NOT EXISTS $mysqlDatabase CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci; CREATE USER IF NOT EXISTS '$mysqlUser'@'127.0.0.1' IDENTIFIED BY '$mysqlPassword'; ALTER USER '$mysqlUser'@'127.0.0.1' IDENTIFIED BY '$mysqlPassword'; GRANT ALL PRIVILEGES ON $mysqlDatabase.* TO '$mysqlUser'@'127.0.0.1'; FLUSH PRIVILEGES;"
    & $mysqlExe "--host=127.0.0.1" "--port=$mysqlPort" "--user=root" "--protocol=TCP" "--execute=$provisionSQL"
    if ($LASTEXITCODE -ne 0) {
        Write-Host "管理服务 MySQL 用户或数据库初始化失败" -ForegroundColor Red
        exit 1
    }
} else {
    Write-Host "使用外部 MySQL: $($env:AMITIA_ADMIN_MYSQL_HOST):$($env:AMITIA_ADMIN_MYSQL_PORT)" -ForegroundColor DarkGray
}
Start-Sleep -Seconds 2

Write-Host "`n[3/8] 启动 SurrealDB..." -ForegroundColor Yellow
Start-Process -FilePath $surrealExe `
    -ArgumentList "start", "surrealkv:data", "--bind", "127.0.0.1:18000", "--user", "root", "--pass", $surrealPass `
    -WorkingDirectory (Join-Path $backendDir "surrealdb") `
    -WindowStyle Hidden
Start-Sleep -Seconds 5

Write-Host "`n[4/8] Qdrant 由核心运行时管理..." -ForegroundColor Yellow

Write-Host "`n[5/8] 启动后端 Server..." -ForegroundColor Yellow
$env:PATH = "$(Split-Path -Parent $nodeExe);$env:PATH"
Start-Process -FilePath $serverExe -ArgumentList "--runtime-profile=local" -WorkingDirectory $env:AMITIA_RUNTIME_ROOT -WindowStyle Hidden
Start-Sleep -Seconds 20

Write-Host "`n[6/8] 启动前端..." -ForegroundColor Yellow
$viteEntry = Join-Path $desktopDir "node_modules\vite\bin\vite.js"
Start-Process -FilePath $nodeExe `
    -ArgumentList $viteEntry, "--host", "127.0.0.1", "--port", "15178" `
    -WorkingDirectory $desktopDir `
    -WindowStyle Hidden
Start-Sleep -Seconds 10

Write-Host "`n[7/8] 启动更新管理服务..." -ForegroundColor Yellow
$adminSourceFiles = @(
    Get-ChildItem -Path (Join-Path $backendDir "cmd\admin-server") -Recurse -File -Filter "*.go" -ErrorAction SilentlyContinue
    Get-ChildItem -Path (Join-Path $backendDir "internal\adminrelease") -Recurse -File -Filter "*.go" -ErrorAction SilentlyContinue
)
$adminNeedsBuild = -not (Test-Path $adminServerExe)
if (-not $adminNeedsBuild) {
    $adminBinaryTime = (Get-Item $adminServerExe).LastWriteTimeUtc
    $adminNeedsBuild = $null -ne ($adminSourceFiles | Where-Object { $_.LastWriteTimeUtc -gt $adminBinaryTime } | Select-Object -First 1)
}
if ($adminNeedsBuild) {
    & $goExe -C $backendDir build -o $adminServerExe .\cmd\admin-server
    if ($LASTEXITCODE -ne 0) {
        Write-Host "更新管理服务构建失败" -ForegroundColor Red
        exit 1
    }
}
if (-not $env:AMITIA_ADMIN_DATA_DIR) {
    $env:AMITIA_ADMIN_DATA_DIR = Join-Path $root "admin-data"
}
if (-not $env:AMITIA_ADMIN_PUBLISH_ROOT) {
    $env:AMITIA_ADMIN_PUBLISH_ROOT = Join-Path $env:AMITIA_ADMIN_DATA_DIR "publish"
}
if (-not $env:AMITIA_ADMIN_DESKTOP_PUBLISH_DIR) {
    $env:AMITIA_ADMIN_DESKTOP_PUBLISH_DIR = Join-Path $env:AMITIA_ADMIN_PUBLISH_ROOT "amitia"
}
if (-not $env:AMITIA_ADMIN_ANDROID_PUBLISH_DIR) {
    $env:AMITIA_ADMIN_ANDROID_PUBLISH_DIR = Join-Path $env:AMITIA_ADMIN_PUBLISH_ROOT "amitia\android"
}
if (-not $env:AMITIA_ADMIN_WEB_DIR) {
    $env:AMITIA_ADMIN_WEB_DIR = Join-Path $root "admin-system\dist"
}
$adminMySQLConfigPath = Join-Path $root "config\admin-mysql.local.json"
if (Test-Path $adminMySQLConfigPath) {
    $adminMySQLConfig = Get-Content -Raw $adminMySQLConfigPath | ConvertFrom-Json
    if (-not $env:AMITIA_ADMIN_MYSQL_HOST -and $adminMySQLConfig.host) {
        $env:AMITIA_ADMIN_MYSQL_HOST = [string]$adminMySQLConfig.host
    }
    if (-not $env:AMITIA_ADMIN_MYSQL_PORT -and $adminMySQLConfig.port) {
        $env:AMITIA_ADMIN_MYSQL_PORT = [string]$adminMySQLConfig.port
    }
    if (-not $env:AMITIA_ADMIN_MYSQL_USER -and $adminMySQLConfig.username) {
        $env:AMITIA_ADMIN_MYSQL_USER = [string]$adminMySQLConfig.username
    }
    if (-not $env:AMITIA_ADMIN_MYSQL_PASSWORD -and $adminMySQLConfig.password) {
        $env:AMITIA_ADMIN_MYSQL_PASSWORD = [string]$adminMySQLConfig.password
    }
    if (-not $env:AMITIA_ADMIN_MYSQL_DATABASE -and $adminMySQLConfig.database) {
        $env:AMITIA_ADMIN_MYSQL_DATABASE = [string]$adminMySQLConfig.database
    }
    if (-not $env:AMITIA_ADMIN_MYSQL_CHARSET -and $adminMySQLConfig.charset) {
        $env:AMITIA_ADMIN_MYSQL_CHARSET = [string]$adminMySQLConfig.charset
    }
    if (-not $env:AMITIA_ADMIN_MYSQL_LOC -and $adminMySQLConfig.loc) {
        $env:AMITIA_ADMIN_MYSQL_LOC = [string]$adminMySQLConfig.loc
    }
    if (-not $env:AMITIA_ADMIN_MYSQL_MAX_OPEN_CONNS -and $adminMySQLConfig.maxOpenConns) {
        $env:AMITIA_ADMIN_MYSQL_MAX_OPEN_CONNS = [string]$adminMySQLConfig.maxOpenConns
    }
    if (-not $env:AMITIA_ADMIN_MYSQL_MAX_IDLE_CONNS -and $adminMySQLConfig.maxIdleConns) {
        $env:AMITIA_ADMIN_MYSQL_MAX_IDLE_CONNS = [string]$adminMySQLConfig.maxIdleConns
    }
    if (-not $env:AMITIA_ADMIN_MYSQL_CONN_MAX_LIFETIME_MINUTES -and $adminMySQLConfig.connMaxLifetimeMinutes) {
        $env:AMITIA_ADMIN_MYSQL_CONN_MAX_LIFETIME_MINUTES = [string]$adminMySQLConfig.connMaxLifetimeMinutes
    }
}
if (-not $env:AMITIA_ADMIN_MYSQL_PASSWORD) {
    Write-Host "MySQL 管理账号未配置，请设置 AMITIA_ADMIN_MYSQL_PASSWORD 或创建 config/admin-mysql.local.json" -ForegroundColor Red
    exit 1
}
$androidSigningKey = Join-Path $root "mobile_app\.update-signing\manifest-private-key.pem"
$androidSigningPassphrase = Join-Path $root "mobile_app\.update-signing\passphrase.txt"
if (-not $env:AMITIA_ADMIN_ANDROID_MANIFEST_KEY_PATH -and (Test-Path $androidSigningKey)) {
    $env:AMITIA_ADMIN_ANDROID_MANIFEST_KEY_PATH = (Resolve-Path $androidSigningKey).Path
}
if (-not $env:AMITIA_ADMIN_ANDROID_MANIFEST_KEY_PASSPHRASE -and (Test-Path $androidSigningPassphrase)) {
    $env:AMITIA_ADMIN_ANDROID_MANIFEST_KEY_PASSPHRASE = (Get-Content -Raw $androidSigningPassphrase).Trim()
}
New-Item -ItemType Directory -Force -Path $adminLogDir | Out-Null
Start-Process -FilePath $adminServerExe `
    -WorkingDirectory $backendDir `
    -RedirectStandardOutput (Join-Path $adminLogDir "admin-server.out.log") `
    -RedirectStandardError (Join-Path $adminLogDir "admin-server.err.log") `
    -WindowStyle Hidden
Start-Sleep -Seconds 3

Write-Host "`n[8/8] 启动更新管理前端..." -ForegroundColor Yellow
$adminViteEntry = Join-Path $adminSystemDir "node_modules\vite\bin\vite.js"
if (-not (Test-Path $adminViteEntry)) {
    $env:PATH = "$(Split-Path -Parent $nodeExe);$env:PATH"
    pnpm --dir $adminSystemDir install
    if ($LASTEXITCODE -ne 0) {
        Write-Host "更新管理前端依赖安装失败" -ForegroundColor Red
        exit 1
    }
}
Start-Process -FilePath $nodeExe `
    -ArgumentList $adminViteEntry, "--host", "127.0.0.1", "--port", "15179" `
    -WorkingDirectory $adminSystemDir `
    -RedirectStandardOutput (Join-Path $adminLogDir "admin-web.out.log") `
    -RedirectStandardError (Join-Path $adminLogDir "admin-web.err.log") `
    -WindowStyle Hidden
Start-Sleep -Seconds 5

Write-Host "`n=== 验证服务状态 ===" -ForegroundColor Cyan
Get-CimInstance Win32_Process -ErrorAction SilentlyContinue |
    Where-Object {
        $_.Name -and [System.IO.Path]::GetFileNameWithoutExtension($_.Name) -in @("server", "admin-server", "AmitiaCore", "qdrant", "surreal", "node") -and
        $_.ExecutablePath -and $_.ExecutablePath.StartsWith($root, [System.StringComparison]::OrdinalIgnoreCase)
    } |
    Select-Object ProcessId, Name, ExecutablePath |
    Format-Table -AutoSize

Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue |
    Where-Object { $_.LocalPort -in 13306, 18899, 18998, 18000, 19178, 15178, 15179 } |
    Format-Table LocalPort, OwningProcess -AutoSize

$startupHealthy = $true
try {
    $health = Invoke-RestMethod -Uri "http://127.0.0.1:18899/api/public/health" -TimeoutSec 5
    Write-Host "后端健康检查: $($health | ConvertTo-Json -Compress)" -ForegroundColor Green
} catch {
    $startupHealthy = $false
    Write-Host "后端健康检查失败: $($_.Exception.Message)" -ForegroundColor Red
}

try {
    $adminHealth = Invoke-RestMethod -Uri "http://127.0.0.1:18998/readyz" -TimeoutSec 5
    Write-Host "管理服务健康检查: $($adminHealth | ConvertTo-Json -Compress)" -ForegroundColor Green
} catch {
    $startupHealthy = $false
    Write-Host "管理服务健康检查失败: $($_.Exception.Message)" -ForegroundColor Red
}

foreach ($serviceURL in @("http://127.0.0.1:15178", "http://127.0.0.1:15179", "http://127.0.0.1:18000/health", "http://127.0.0.1:19178/healthz")) {
    try {
        $serviceHealth = Invoke-WebRequest -Uri $serviceURL -TimeoutSec 5
        if ($serviceHealth.StatusCode -ne 200) {
            $startupHealthy = $false
        }
    } catch {
        $startupHealthy = $false
        Write-Host "服务健康检查失败: $serviceURL" -ForegroundColor Red
    }
}
if (-not $startupHealthy) {
    exit 1
}
