@echo off
cd /d "%~dp0"
echo ========================================
echo   Amitia 后端启动
echo   时间: %date% %time%
echo ========================================
echo.
server.exe
echo.
echo ========================================
echo   退出码: %errorlevel%
echo ========================================
pause
