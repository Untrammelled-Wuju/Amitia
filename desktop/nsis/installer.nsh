!ifndef BUILD_UNINSTALLER
  Var AmitiaPendingInstallDir
!endif

!macro AmitiaInstallerFunctions
Function AmitiaRequestSystemDriveElevation
  StrCpy $R0 $INSTDIR 2
  ${If} $R0 == "C:"
  ${OrIf} $R0 == "c:"
    ${IfNot} ${UAC_IsAdmin}
      WriteRegStr HKCU "Software\${APP_GUID}" "AmitiaPendingInstallDir" "$INSTDIR"
      ShowWindow $HWNDPARENT ${SW_HIDE}
      !insertmacro UAC_RunElevated
      ${Switch} $0
        ${Case} 0
          ${If} $1 == 1
            Quit
          ${ElseIf} $1 == 2
            ${Break}
          ${Else}
            DeleteRegValue HKCU "Software\${APP_GUID}" "AmitiaPendingInstallDir"
            MessageBox mb_IconStop|mb_TopMost|mb_SetForeground "安装到 C 盘需要管理员权限，无法完成提权。"
            Quit
          ${EndIf}
        ${Case} 1223
          DeleteRegValue HKCU "Software\${APP_GUID}" "AmitiaPendingInstallDir"
          MessageBox mb_IconStop|mb_TopMost|mb_SetForeground "安装到 C 盘必须授予管理员权限。"
          Quit
        ${Default}
          DeleteRegValue HKCU "Software\${APP_GUID}" "AmitiaPendingInstallDir"
          MessageBox mb_IconStop|mb_TopMost|mb_SetForeground "申请管理员权限失败，错误代码：$0"
          Quit
      ${EndSwitch}
    ${EndIf}
  ${EndIf}
FunctionEnd
!macroend

!macro customInit
  ReadRegStr $AmitiaPendingInstallDir HKCU "Software\${APP_GUID}" "AmitiaPendingInstallDir"
  ${If} $AmitiaPendingInstallDir != ""
    StrCpy $INSTDIR $AmitiaPendingInstallDir
  ${EndIf}
  ${If} ${Silent}
    Call AmitiaRequestSystemDriveElevation
  ${EndIf}
!macroend

!macro customPageAfterChangeDir
  PageEx custom
    PageCallbacks AmitiaElevationPre AmitiaElevationLeave
    Caption " "
  PageExEnd
!macroend

!macro AmitiaInstallerPageFunctions
Function AmitiaElevationPre
  ReadRegStr $AmitiaPendingInstallDir HKCU "Software\${APP_GUID}" "AmitiaPendingInstallDir"
  ${If} $AmitiaPendingInstallDir != ""
    StrCpy $INSTDIR $AmitiaPendingInstallDir
    DeleteRegValue HKCU "Software\${APP_GUID}" "AmitiaPendingInstallDir"
  ${EndIf}
  Call AmitiaRequestSystemDriveElevation
  Abort
FunctionEnd

Function AmitiaElevationLeave
FunctionEnd
!macroend

!macro customHeader
  !ifndef BUILD_UNINSTALLER
    !insertmacro AmitiaInstallerFunctions
    !insertmacro AmitiaInstallerPageFunctions
  !endif
!macroend

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

!macro customInstall
  DeleteRegValue HKCU "Software\${APP_GUID}" "AmitiaPendingInstallDir"
!macroend
