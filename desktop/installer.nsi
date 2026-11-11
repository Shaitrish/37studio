; The Windows installer: 37studio-<version>-windows-<arch>-setup.exe. package-windows.ps1 builds it with NSIS
; (makensis /DVERSION=… /DARCH=… /DOUT=…). It installs for this user only, with no admin rights, into the same
; folder install-windows.ps1 uses, so either one updates the other. Running a newer installer over an old one is
; the update. Your brands and videos live in %AppData%\37Studio and are never touched by install or uninstall.
Unicode true
SetCompressor /SOLID lzma
RequestExecutionLevel user

!ifndef VERSION
  !error "pass /DVERSION=x.y.z"
!endif
!ifndef ARCH
  !define ARCH amd64
!endif
!ifndef OUT
  !define OUT "37studio-${VERSION}-windows-${ARCH}-setup.exe"
!endif

!define UNINST "Software\Microsoft\Windows\CurrentVersion\Uninstall\37Studio"

Name "37Studio"
OutFile "${OUT}"
InstallDir "$LOCALAPPDATA\Programs\37Studio"
InstallDirRegKey HKCU "${UNINST}" "InstallLocation"
BrandingText "37Studio ${VERSION}"

VIProductVersion "${VERSION}.0"
VIAddVersionKey "ProductName" "37Studio"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "FileDescription" "37Studio installer"
VIAddVersionKey "CompanyName" "Shaitrish"
VIAddVersionKey "LegalCopyright" "MIT License"

!include "MUI2.nsh"
!define MUI_ICON "..\brand\37studio-icon.ico"
!define MUI_UNICON "..\brand\37studio-icon.ico"
!define MUI_ABORTWARNING
!define MUI_WELCOMEPAGE_TEXT "37Studio ${VERSION} will be installed for you only, with no admin rights needed.$\r$\n$\r$\nIf an older version is installed, this updates it. Your brands, projects and videos are kept."
!define MUI_FINISHPAGE_RUN "$INSTDIR\37studio.exe"
!define MUI_FINISHPAGE_RUN_TEXT "Open 37Studio"
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

; Windows won't replace a running program, so ask for it to be closed first instead of failing halfway.
!macro WaitClosed
  check:
    nsExec::ExecToStack 'cmd /c tasklist /FI "IMAGENAME eq 37studio.exe" /NH | find /I "37studio.exe"'
    Pop $0 ; find's exit code: 0 when 37studio.exe is running
    Pop $1 ; its output
    StrCmp $0 "0" 0 done
    MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "37Studio is open. Close it (any running render stops with it), then click Retry." IDRETRY check
    Abort
  done:
!macroend

Section "37Studio"
  !insertmacro WaitClosed
  SetShellVarContext current
  SetOutPath "$INSTDIR"
  ; Keep the name 37studio.exe: the agent runs `37studio gen|clip|transcribe` from the folder the app sits in.
  File "bin\37studio.exe"
  File "..\brand\37studio-icon.ico"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  CreateShortcut "$SMPROGRAMS\37Studio.lnk" "$INSTDIR\37studio.exe" "" "$INSTDIR\37studio-icon.ico" 0 SW_SHOWNORMAL "" "AI video studio and social media desk"
  CreateShortcut "$DESKTOP\37Studio.lnk" "$INSTDIR\37studio.exe" "" "$INSTDIR\37studio-icon.ico"

  ; Settings → Apps → Installed apps lists it, with Uninstall.
  WriteRegStr HKCU "${UNINST}" "DisplayName" "37Studio"
  WriteRegStr HKCU "${UNINST}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINST}" "Publisher" "Shaitrish"
  WriteRegStr HKCU "${UNINST}" "URLInfoAbout" "https://studio.shaitrish.com"
  WriteRegStr HKCU "${UNINST}" "DisplayIcon" "$INSTDIR\37studio-icon.ico"
  WriteRegStr HKCU "${UNINST}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINST}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegStr HKCU "${UNINST}" "QuietUninstallString" '"$INSTDIR\uninstall.exe" /S'
  WriteRegDWORD HKCU "${UNINST}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINST}" "NoRepair" 1
  SectionGetSize 0 $0
  WriteRegDWORD HKCU "${UNINST}" "EstimatedSize" $0
SectionEnd

Section "Uninstall"
  !insertmacro WaitClosed
  SetShellVarContext current
  Delete "$INSTDIR\37studio.exe"
  Delete "$INSTDIR\37studio-icon.ico"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\37Studio.lnk"
  Delete "$DESKTOP\37Studio.lnk"
  DeleteRegKey HKCU "${UNINST}"
SectionEnd
