@echo off
cd /d "%~dp0"

echo [U-Ai] Starting server (Qdrant/QQ侧车/微信侧车由后端自动拉起)...
set CONFIG_PATH=config
server.exe