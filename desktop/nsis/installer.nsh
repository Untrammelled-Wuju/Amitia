!macro customCheckAppRunning
  ${if} $INSTDIR != ""
    nsExec::Exec `"$PowerShellPath" -NoProfile -NonInteractive -Command "Get-CimInstance Win32_Process | Where-Object { $$_.Path -and $$_.Path.StartsWith('$INSTDIR\', [System.StringComparison]::OrdinalIgnoreCase) } | ForEach-Object { Stop-Process -Id $$_.ProcessId -Force -ErrorAction SilentlyContinue; Wait-Process -Id $$_.ProcessId -Timeout 8 -ErrorAction SilentlyContinue }"`
    Pop $R0
    Sleep 400
  ${endIf}
!macroend

!macro customUnInstallCheck
!macroend

!macro customUnInstallCheckCurrentUser
!macroend
