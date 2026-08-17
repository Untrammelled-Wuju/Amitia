$backendDir = "D:\桌面\跟进项目\U-Ai\backend"

Write-Host "Starting Qdrant..."
Start-Process -FilePath "$backendDir\qdrant\qdrant.exe" -WorkingDirectory "$backendDir\qdrant" -WindowStyle Hidden

Start-Sleep -Seconds 3

Write-Host "Starting SurrealDB..."
Start-Process -FilePath "$backendDir\surrealdb\surreal.exe" -WorkingDirectory "$backendDir\surrealdb" -WindowStyle Hidden -ArgumentList "start","--log","trace","--user","root","--pass","root","rocksdb://data/data.sdb","--bind","127.0.0.1:18000"

Start-Sleep -Seconds 3

Write-Host "Starting Backend Server..."
Start-Process -FilePath "$backendDir\server.exe" -WorkingDirectory "$backendDir" -WindowStyle Hidden

Write-Host "All services started."

Get-Process | Where-Object {$_.ProcessName -match "server|qdrant|surreal"} | Select-Object Id,ProcessName,Path
