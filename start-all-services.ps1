# 完整的 U-Ai 项目启动脚本
# 根据修复后的配置启动所有服务

$backendDir = "D:\桌面\跟进项目\U-Ai\backend"
$frontDir = "D:\桌面\跟进项目\U-Ai\front"

Write-Host "=== U-Ai 完整启动脚本 ===" -ForegroundColor Cyan

# 第一步：清理旧进程
Write-Host "`n[1/5] 清理旧进程..." -ForegroundColor Yellow
$processes = @("server", "qdrant", "surreal", "node", "vite", "electron")
foreach ($proc in $processes) {
    Get-Process -Name $proc -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
}
Start-Sleep -Seconds 3

# 第二步：启动 SurrealDB
Write-Host "`n[2/5] 启动 SurrealDB..." -ForegroundColor Yellow
$surrealPass = "AmitiaSurrealDBRootPassword20260831Securex"
Set-Location "$backendDir\surrealdb"
if (Test-Path "data\data.sdb") {
    Remove-Item -Path "data\data.sdb" -Recurse -Force -ErrorAction SilentlyContinue
}
Start-Process -FilePath "surreal.exe" -ArgumentList "start","surrealkv:data","--bind","127.0.0.1:18000","--user","root","--pass",$surrealPass -WindowStyle Hidden
Start-Sleep -Seconds 5

# 第三步：启动 Qdrant
Write-Host "`n[3/5] 启动 Qdrant..." -ForegroundColor Yellow
Set-Location "$backendDir\qdrant"
Start-Process -FilePath "qdrant.exe" -ArgumentList "--config-path","config\config.yaml" -WindowStyle Hidden
Start-Sleep -Seconds 5

# 第四步：启动后端 Server
Write-Host "`n[4/5] 启动后端 Server..." -ForegroundColor Yellow
Set-Location $backendDir
Start-Process -FilePath "server.exe" -WindowStyle Hidden
Start-Sleep -Seconds 20

# 第五步：启动前端
Write-Host "`n[5/5] 启动前端..." -ForegroundColor Yellow
Set-Location $frontDir
Start-Process -FilePath "cmd.exe" -ArgumentList "/c","pnpm dev" -WindowStyle Hidden
Start-Sleep -Seconds 10

# 验证
Write-Host "`n=== 验证服务状态 ===" -ForegroundColor Cyan
Get-Process | Where-Object {$_.ProcessName -match "server|qdrant|surreal|node"} | Select-Object Id, ProcessName | Format-Table -AutoSize

Write-Host "`n端口监听状态:" -ForegroundColor Yellow
Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue | Where-Object {$_.LocalPort -in 18899, 18000, 19178, 5178} | Format-Table LocalPort, OwningProcess -AutoSize
