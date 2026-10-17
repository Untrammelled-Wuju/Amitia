!macro customCheckAppRunning
  ${if} $INSTDIR != ""
    nsExec::Exec `"$PowerShellPath" -NoProfile -Command "Get-CimInstance Win32_Process | Where-Object { $$_.Path -and $$_.Path.StartsWith('$INSTDIR\') } | ForEach-Object { Stop-Process -Id $$_.ProcessId -Force }"`
    Pop $R0
    Sleep 400
  ${endIf}
!macroend
