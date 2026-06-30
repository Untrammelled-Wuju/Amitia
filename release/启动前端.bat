@echo off
cd /d "%~dp0..\front"
echo ========================================
echo   Start Frontend - Port 5178
echo ========================================
echo.
call npm run dev
pause