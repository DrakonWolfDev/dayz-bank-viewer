Unicode true
!include "MUI2.nsh"
!include "LogicLib.nsh"
!ifndef VERSION
!error "VERSION is required"
!endif
Name "DayZ Bank Viewer"
OutFile "../../dist/dayz-bank-viewer-${VERSION}-windows-amd64-setup.exe"
InstallDir "$PROGRAMFILES64\DayZ Bank Viewer"
InstallDirRegKey HKLM "Software\WolfForge\DayZBankViewer" "InstallDir"
RequestExecutionLevel admin
Icon "../../web/app.ico"
UninstallIcon "../../web/app.ico"
!define MUI_ABORTWARNING
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "../../LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Function .onInit
 SetRegView 64
FunctionEnd

Section "DayZ Bank Viewer"
 SetRegView 64
 SetShellVarContext all
 InitPluginsDir
 File /oname=$PLUGINSDIR\lifecycle.ps1 "lifecycle.ps1"
 nsExec::ExecToStack '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -ExecutionPolicy Bypass -File "$PLUGINSDIR\lifecycle.ps1" -InstallDir "$INSTDIR"'
 Pop $0
 Pop $1
 StrCpy $2 "0"
 ${If} $0 == 10
  StrCpy $2 "1"
 ${ElseIf} $0 != 0
  MessageBox MB_ICONSTOP "Could not stop the existing viewer. Installation cancelled. $1"
  Abort
 ${EndIf}
 SetOutPath "$INSTDIR"
 File "../../dist/windows-amd64/dayz-bank-viewer.exe"
 File "../../web/app.ico"
 File "service.install"
 File "../../README.md"
 File "../../LICENSING.md"
 File "../../LICENSE"
 File "../../dist/windows-amd64/GO-LICENSE"
 File "../../dist/windows-amd64/GO-PATENTS"
 CreateDirectory "$APPDATA\DayZ Bank Viewer"
 ; Service runs as LocalService, with write access only to its own data directory.
 nsExec::ExecToStack 'icacls "$APPDATA\DayZ Bank Viewer" /inheritance:r /grant:r "*S-1-5-18:(OI)(CI)F" "*S-1-5-32-544:(OI)(CI)F" "*S-1-5-19:(OI)(CI)M" "*S-1-5-32-545:(OI)(CI)RX"'
 Pop $0
 Pop $1
 ${If} $0 != 0
  MessageBox MB_ICONSTOP "Could not secure the viewer data directory. $1"
  Abort
 ${EndIf}
 nsExec::ExecToStack 'sc.exe query DayZBankViewer'
 Pop $0
 Pop $1
 ${If} $0 != 0
  nsExec::ExecToStack 'sc.exe create DayZBankViewer binPath= "\"$INSTDIR\dayz-bank-viewer.exe\" -service -config \"$APPDATA\DayZ Bank Viewer\config.json\"" start= demand obj= "NT AUTHORITY\LocalService" DisplayName= "DayZ Bank Viewer"'
  Pop $0
  Pop $1
  ${If} $0 != 0
   MessageBox MB_ICONSTOP "Could not register the viewer service. $1"
   Abort
  ${EndIf}
 ${EndIf}
 nsExec::ExecToStack 'sc.exe config DayZBankViewer binPath= "\"$INSTDIR\dayz-bank-viewer.exe\" -service -config \"$APPDATA\DayZ Bank Viewer\config.json\"" obj= "NT AUTHORITY\LocalService"'
 Pop $0
 Pop $1
 ${If} $0 != 0
  MessageBox MB_ICONSTOP "Could not update the viewer service registration. $1"
  Abort
 ${EndIf}
 nsExec::ExecToStack 'sc.exe failureflag DayZBankViewer 1'
 Pop $0
 Pop $1
 nsExec::ExecToStack 'sc.exe failure DayZBankViewer reset= 86400 actions= restart/5000/restart/15000/restart/30000'
 Pop $0
 Pop $1
 WriteUninstaller "$INSTDIR\Uninstall.exe"
 WriteRegStr HKLM "Software\WolfForge\DayZBankViewer" "InstallDir" "$INSTDIR"
 WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\DayZBankViewer" "DisplayName" "DayZ Bank Viewer"
 WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\DayZBankViewer" "DisplayVersion" "${VERSION}"
 WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\DayZBankViewer" "UninstallString" '$\"$INSTDIR\Uninstall.exe$\"'
 WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\DayZBankViewer" "DisplayIcon" "$INSTDIR\app.ico"
 CreateDirectory "$SMPROGRAMS\DayZ Bank Viewer"
 CreateShortcut "$SMPROGRAMS\DayZ Bank Viewer\DayZ Bank Viewer.lnk" "$INSTDIR\dayz-bank-viewer.exe" "-desktop" "$INSTDIR\app.ico"
 CreateShortcut "$SMPROGRAMS\DayZ Bank Viewer\Uninstall.lnk" "$INSTDIR\Uninstall.exe"
 ${If} $2 == 1
  nsExec::ExecToStack 'sc.exe start DayZBankViewer'
  Pop $0
  Pop $1
  ${If} $0 != 0
   MessageBox MB_ICONEXCLAMATION "The upgrade was installed but the service could not restart. $1"
  ${EndIf}
 ${EndIf}
SectionEnd

Section "Uninstall"
 SetRegView 64
 SetShellVarContext all
 InitPluginsDir
 File /oname=$PLUGINSDIR\lifecycle.ps1 "lifecycle.ps1"
 nsExec::ExecToStack '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -ExecutionPolicy Bypass -File "$PLUGINSDIR\lifecycle.ps1" -InstallDir "$INSTDIR" -Uninstall'
 Pop $0
 Pop $1
 ${If} $0 != 0
 ${AndIf} $0 != 10
  MessageBox MB_ICONSTOP "Could not remove the viewer service. Uninstall cancelled. $1"
  Abort
 ${EndIf}
 Delete "$INSTDIR\dayz-bank-viewer.exe"
 Delete "$INSTDIR\app.ico"
 Delete "$INSTDIR\service.install"
 Delete "$INSTDIR\README.md"
 Delete "$INSTDIR\LICENSING.md"
 Delete "$INSTDIR\LICENSE"
 Delete "$INSTDIR\GO-LICENSE"
 Delete "$INSTDIR\GO-PATENTS"
 Delete "$INSTDIR\Uninstall.exe"
 RMDir "$INSTDIR"
 Delete "$SMPROGRAMS\DayZ Bank Viewer\DayZ Bank Viewer.lnk"
 Delete "$SMPROGRAMS\DayZ Bank Viewer\Uninstall.lnk"
 RMDir "$SMPROGRAMS\DayZ Bank Viewer"
 DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\DayZBankViewer"
 DeleteRegKey HKLM "Software\WolfForge\DayZBankViewer"
 ; Configuration, logs, exports and banking records are deliberately retained.
SectionEnd
