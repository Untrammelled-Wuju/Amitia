@echo off
setlocal EnableDelayedExpansion

set REPO=D:\桌面\跟进项目\U-Ai
set STAGE=%TEMP%\U-Ai-pkg
set ZIP=%REPO%\U-Ai-source.zip

if exist "%ZIP%" del /f /q "%ZIP%" >nul 2>&1
if exist "%STAGE%" rd /s /q "%STAGE%" >nul 2>&1
mkdir "%STAGE%" >nul 2>&1

set XF=*.exe *.dll *.pdb *.zip *.tar *.tar.gz *.tar.xz *.exe~ *.bak *.bin *.dat *.db *.db-shm *.db-wal *.db-journal *.log *.so *.o *.map *.node *.wasm *.lock *.class *.dex *.apk *.aab *.test *.test.exe .DS_Store Thumbs.db server.exe server backend.exe backend amitia-ext.exe amitiax.exe extension.test.exe worker.test.exe kernel.test.exe legacy-package-migrate.exe server_linux_amd64 server_linux_arm64 server_windows_amd64.exe qdrantprocess.test qdrantprocess.test.exe tmp_check.js tmp_check_db.js .env .env.local .qdrant-initialized desktop-instance-id raft_state.json go.sum .metadata server_old.exe.bak
set NB=/R:0 /W:0 /NFL /NDL /NJH /NJS

echo [1/8] Front...
robocopy "%REPO%\front" "%STAGE%\front" /E /XD node_modules dist /XF %XF% %NB% >nul

echo [2/8] Flutter lib...
robocopy "%REPO%\mobile_app\lib" "%STAGE%\mobile_app\lib" /E /XD node_modules /XF %XF% %NB% >nul

echo [3/8] Flutter test...
robocopy "%REPO%\mobile_app\test" "%STAGE%\mobile_app\test" /E /XD node_modules /XF %XF% %NB% >nul

echo [4/8] Flutter android...
robocopy "%REPO%\mobile_app\android" "%STAGE%\mobile_app\android" /E /XD .gradle .cxx build .kotlin app/src/main/assets/runtime-package /XF %XF% %NB% >nul

echo [5/8] Backend cmd...
robocopy "%REPO%\backend\cmd" "%STAGE%\backend\cmd" /E /XD node_modules /XF %XF% %NB% >nul

echo [6/8] Backend internal + pkg + scripts + root go files...
robocopy "%REPO%\backend\internal" "%STAGE%\backend\internal" /E /XD node_modules /XF %XF% %NB% >nul
robocopy "%REPO%\backend\pkg" "%STAGE%\backend\pkg" /E /XD node_modules gameplugin/sdk/game-plugin/node_modules /XF %XF% %NB% >nul
robocopy "%REPO%\backend\scripts" "%STAGE%\backend\scripts" /E /XD .sidecar-build /XF %XF% %NB% >nul

REM Backend root files
for %%f in (go.mod check_quality_tables.go appsettings.json) do (
    if exist "%REPO%\backend\%%f" copy /y "%REPO%\backend\%%f" "%STAGE%\backend\" >nul 2>&1
)

echo [7/8] Desktop...
robocopy "%REPO%\desktop" "%STAGE%\desktop" /E /XD node_modules dist-types release build /XF %XF% %NB% >nul

echo [8/8] Compressing...
cd /d "%REPO%"
powershell -Command "Compress-Archive -Path '%STAGE%\*' -DestinationPath '%ZIP%' -CompressionLevel Optimal -Force"
rd /s /q "%STAGE%" >nul 2>&1

for %%A in ("%ZIP%") do set SIZE=%%~zA
echo Done: %ZIP% [%SIZE% bytes]

start explorer.exe "%REPO%"
