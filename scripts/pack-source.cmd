@echo off
set STAGE=%TEMP%\U-Ai-pkg
set REPO=D:\桌面\跟进项目\U-Ai
set ZIP=%REPO%\U-Ai-source.zip
if exist "%ZIP%" del /f /q "%ZIP%" 2>nul
if exist "%STAGE%" rd /s /q "%STAGE%" 2>nul
mkdir "%STAGE%" 2>nul

set XF=*.exe *.dll *.pdb *.zip *.tar *.tar.xz *.exe~ *.bak *.bin *.dat *.db *.log *.so *.map *.node *.wasm *.lock
set NB=/R:0 /W:0 /NFL /NDL /NJH /NJS /XJ

echo [1/4] Front
robocopy "%REPO%\front" "%STAGE%\front" /E /XD node_modules dist /XF %XF% %NB% >nul

echo [2/4] Flutter
robocopy "%REPO%\mobile_app\lib" "%STAGE%\mobile_app\lib" /E /XD node_modules /XF %XF% %NB% >nul
robocopy "%REPO%\mobile_app\test" "%STAGE%\mobile_app\test" /E /XD node_modules /XF %XF% %NB% >nul
robocopy "%REPO%\mobile_app\android" "%STAGE%\mobile_app\android" /E /XD .gradle .cxx build .kotlin app/src/main/assets/runtime-package /XF %XF% %NB% >nul

echo [3/4] Backend
robocopy "%REPO%\backend\cmd" "%STAGE%\backend\cmd" /E /XD node_modules /XF %XF% %NB% >nul
robocopy "%REPO%\backend\internal" "%STAGE%\backend\internal" /E /XD node_modules /XF %XF% %NB% >nul
robocopy "%REPO%\backend\pkg" "%STAGE%\backend\pkg" /E /XD node_modules gameplugin/sdk/game-plugin/node_modules /XF %XF% %NB% >nul
robocopy "%REPO%\backend\scripts" "%STAGE%\backend\scripts" /E /XD .sidecar-build /XF %XF% %NB% >nul

copy /y "%REPO%\backend\go.mod" "%STAGE%\backend\" >nul 2>&1
copy /y "%REPO%\backend\appsettings.json" "%STAGE%\backend\" >nul 2>&1
copy /y "%REPO%\backend\check_quality_tables.go" "%STAGE%\backend\" >nul 2>&1

echo [4/4] Desktop
robocopy "%REPO%\desktop" "%STAGE%\desktop" /E /XD node_modules dist-types release build /XF %XF% %NB% >nul

echo Zipping with 7z...
if exist "C:\Program Files\7-Zip\7z.exe" (
  "C:\Program Files\7-Zip\7z.exe" a -tzip -mx5 "%ZIP%" "%STAGE%\*" >nul
) else (
  powershell -Command "Compress-Archive -Path ('%STAGE%'+'\*') -DestinationPath '%ZIP%' -CompressionLevel Optimal -Force"
)

rd /s /q "%STAGE%" 2>nul
for %%F in ("%ZIP%") do @echo Done: %%~zF bytes
start explorer "%REPO%"
