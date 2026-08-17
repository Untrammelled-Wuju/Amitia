@echo off
cd /d "D:\桌面\跟进项目\U-Ai\backend"

echo Starting Qdrant...
start "" /MIN "qdrant\qdrant.exe"

timeout /t 3 /nobreak > nul

echo Starting SurrealDB...
start "" /MIN "surrealdb\surreal.exe" start --log trace --user root --pass root file://data/data.sdb

timeout /t 3 /nobreak > nul

echo Starting Backend Server...
start "" /MIN "server.exe"

echo All services started.
