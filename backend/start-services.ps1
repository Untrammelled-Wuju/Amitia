$backendDir = "D:\桌面\跟进项目\U-Ai\backend"

Write-Host "Starting Backend Server..."
Start-Process -FilePath "$backendDir\server.exe" -WorkingDirectory "$backendDir" -WindowStyle Hidden

Write-Host "All services started."

Get-Process | Where-Object {$_.ProcessName -match "server|qdrant|surreal"} | Select-Object Id,ProcessName,Path
