!ifndef BUILD_UNINSTALLER
  Var AmitiaPendingInstallDir
!endif

!ifdef BUILD_UNINSTALLER
  !include "nsDialogs.nsh"
  Var AmitiaDeleteDataCheckbox
  Var AmitiaDeleteUserData

  Function un.AmitiaDeleteDataPageShow
    ${If} $AmitiaDeleteUserData == ""
      StrCpy $AmitiaDeleteUserData ${BST_UNCHECKED}
    ${EndIf}
    ${NSD_CreateCheckbox} 120u 162u 177u 24u "同时删除使用数据（AmitiaData，不可恢复）"
    Pop $AmitiaDeleteDataCheckbox
    ${NSD_SetState} $AmitiaDeleteDataCheckbox $AmitiaDeleteUserData
  FunctionEnd

  Function un.AmitiaDeleteDataPageLeave
    ${NSD_GetState} $AmitiaDeleteDataCheckbox $AmitiaDeleteUserData
  FunctionEnd
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

!macro customUnWelcomePage
  !define MUI_UNWELCOMEPAGE_TITLE "卸载 Amitia"
  !define MUI_UNWELCOMEPAGE_TEXT "卸载程序将移除 Amitia 应用程序文件。$\r$\n$\r$\n默认保留 AmitiaData 使用数据；如需彻底清除，请勾选下方选项。"
  !define MUI_PAGE_CUSTOMFUNCTION_SHOW un.AmitiaDeleteDataPageShow
  !define MUI_PAGE_CUSTOMFUNCTION_LEAVE un.AmitiaDeleteDataPageLeave
  !insertmacro MUI_UNPAGE_WELCOME
  !undef MUI_UNWELCOMEPAGE_TEXT
  !undef MUI_UNWELCOMEPAGE_TITLE
!macroend

!macro customUnInstall
  ${If} $AmitiaDeleteUserData == ${BST_CHECKED}
  ${AndIf} ${FileExists} "$INSTDIR\..\AmitiaData\*.*"
    ClearErrors
    RMDir /r /REBOOTOK "$INSTDIR\..\AmitiaData"
    ${If} ${Errors}
      System::Call "kernel32::GetLastError() i .R0"
      DetailPrint "删除 AmitiaData 失败，路径：$INSTDIR\..\AmitiaData，Windows 错误代码：$R0"
      MessageBox MB_ICONEXCLAMATION|MB_OK "AmitiaData 未能完全删除。$\r$\n路径：$INSTDIR\..\AmitiaData$\r$\nWindows 错误代码：$R0$\r$\n其余文件将在系统重启后继续删除。"
      SetErrorLevel 1
    ${EndIf}
  ${EndIf}
!macroend

!macro customInstall
  DeleteRegValue HKCU "Software\${APP_GUID}" "AmitiaPendingInstallDir"
!macroend
