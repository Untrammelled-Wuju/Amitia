@echo off
cd /d "%~dp0"

echo [U-Ai] Starting QQ Lagrange...
start "LagrangeBot" /MIN "%~dp0lagrange\bin\Lagrange.OneBot.exe"

echo [U-Ai] Starting server (SignServer managed by Go backend on Linux)...
set CONFIG_PATH=config
server.exe
