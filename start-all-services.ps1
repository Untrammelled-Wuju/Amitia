$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $MyInvocation.MyCommand.Path
$backendDir = Join-Path $root "backend"
$frontDir = Join-Path $root "front"
$nodeExe = Join-Path $root "desktop\resources\core\node\node.exe"
$surrealExe = Join-Path $backendDir "surrealdb\surreal.exe"
$qdrantExe = Join-Path $backendDir "qdrant\qdrant.exe"
$serverExe = Join-Path $backendDir "server.exe"
$surrealPass = "AmitiaSurrealDBRootPassword20260831Securex"

$env:AMITIA_RUNTIME_ROOT = $root
$env:AMITIA_WORKSPACE_DIR = $root

function Stop-ProjectProcess {
    param([int]$ProcessId)
    $process = Get-CimInstance Win32_Process -Filter "ProcessId = $ProcessId" -ErrorAction SilentlyContinue
    if ($null -eq $process -or [string]::IsNullOrWhiteSpace($process.ExecutablePath)) {
        return
    }
    if (-not $process.ExecutablePath.StartsWith($root, [System.StringComparison]::OrdinalIgnoreCase)) {
        return
    }
    Stop-Process -Id $ProcessId -Force -ErrorAction SilentlyContinue
}

Write-Host "=== U-Ai 完整启动脚本 ===" -ForegroundColor Cyan

Write-Host "`n[1/5] 清理项目旧进程..." -ForegroundColor Yellow
$projectNames = @("server", "AmitiaCore", "qdrant", "surreal", "node", "electron")
Get-CimInstance Win32_Process -ErrorAction SilentlyContinue |
    Where-Object {
        $_.Name -and [System.IO.Path]::GetFileNameWithoutExtension($_.Name) -in $projectNames -and
        $_.ExecutablePath -and $_.ExecutablePath.StartsWith($root, [System.StringComparison]::OrdinalIgnoreCase)
    } |
    ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }

foreach ($port in @(18899, 18000, 19178, 5178)) {
    Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue |
        ForEach-Object { Stop-ProjectProcess -ProcessId $_.OwningProcess }
}
Start-Sleep -Seconds 3

Write-Host "`n[2/5] 启动 SurrealDB..." -ForegroundColor Yellow
Start-Process -FilePath $surrealExe `
    -ArgumentList "start", "surrealkv:data", "--bind", "127.0.0.1:18000", "--user", "root", "--pass", $surrealPass `
    -WorkingDirectory (Join-Path $backendDir "surrealdb") `
    -WindowStyle Hidden
Start-Sleep -Seconds 5

Write-Host "`n[3/5] 启动 Qdrant..." -ForegroundColor Yellow
Start-Process -FilePath $qdrantExe `
    -ArgumentList "--config-path", "config\config.yaml" `
    -WorkingDirectory (Join-Path $backendDir "qdrant") `
    -WindowStyle Hidden
Start-Sleep -Seconds 5

Write-Host "`n[4/5] 启动后端 Server..." -ForegroundColor Yellow
$env:PATH = "$(Split-Path -Parent $nodeExe);$env:PATH"
Start-Process -FilePath $serverExe -WorkingDirectory $backendDir -WindowStyle Hidden
Start-Sleep -Seconds 20

Write-Host "`n[5/5] 启动前端..." -ForegroundColor Yellow
$viteEntry = Join-Path $frontDir "node_modules\vite\bin\vite.js"
Start-Process -FilePath $nodeExe `
    -ArgumentList $viteEntry, "--host", "127.0.0.1", "--port", "5178" `
    -WorkingDirectory $frontDir `
    -WindowStyle Hidden
Start-Sleep -Seconds 10

Write-Host "`n=== 验证服务状态 ===" -ForegroundColor Cyan
Get-CimInstance Win32_Process -ErrorAction SilentlyContinue |
    Where-Object {
        $_.Name -and [System.IO.Path]::GetFileNameWithoutExtension($_.Name) -in @("server", "AmitiaCore", "qdrant", "surreal", "node") -and
        $_.ExecutablePath -and $_.ExecutablePath.StartsWith($root, [System.StringComparison]::OrdinalIgnoreCase)
    } |
    Select-Object ProcessId, Name, ExecutablePath |
    Format-Table -AutoSize

Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue |
    Where-Object { $_.LocalPort -in 18899, 18000, 19178, 5178 } |
    Format-Table LocalPort, OwningProcess -AutoSize

try {
    $health = Invoke-RestMethod -Uri "http://127.0.0.1:18899/api/health" -TimeoutSec 5
    Write-Host "后端健康检查: $($health | ConvertTo-Json -Compress)" -ForegroundColor Green
} catch {
    Write-Host "后端健康检查失败: $($_.Exception.Message)" -ForegroundColor Red
}
