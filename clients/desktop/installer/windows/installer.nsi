; NithronSync Windows Installer
; Requires NSIS (https://nsis.sourceforge.io/)

!include "MUI2.nsh"
!include "FileFunc.nsh"

; General settings
Name "NithronSync"
OutFile "NithronSync-Setup.exe"
InstallDir "$PROGRAMFILES64\NithronSync"
InstallDirRegKey HKLM "Software\NithronSync" "InstallDir"
RequestExecutionLevel admin

; Version info
!define VERSION "1.0.0"
!define PRODUCT_NAME "NithronSync"
!define PRODUCT_PUBLISHER "Nithron"
!define PRODUCT_WEB_SITE "https://nithron.com"

VIProductVersion "${VERSION}.0"
VIAddVersionKey "ProductName" "${PRODUCT_NAME}"
VIAddVersionKey "CompanyName" "${PRODUCT_PUBLISHER}"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "FileDescription" "NithronSync Installer"

; MUI Settings
!define MUI_ABORTWARNING
!define MUI_ICON "..\..\build\appicon.ico"
!define MUI_UNICON "..\..\build\appicon.ico"
!define MUI_WELCOMEFINISHPAGE_BITMAP "welcome.bmp"

; Pages
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_LICENSE "..\..\..\..\LICENSE"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES

; Language
!insertmacro MUI_LANGUAGE "English"

; Installer Section
Section "Install"
    SetOutPath "$INSTDIR"
    
    ; Stop running instance
    nsExec::ExecToLog 'taskkill /F /IM NithronSync.exe'
    
    ; Copy files
    File "..\..\build\bin\NithronSync.exe"
    
    ; Create start menu shortcuts
    CreateDirectory "$SMPROGRAMS\NithronSync"
    CreateShortCut "$SMPROGRAMS\NithronSync\NithronSync.lnk" "$INSTDIR\NithronSync.exe"
    CreateShortCut "$SMPROGRAMS\NithronSync\Uninstall.lnk" "$INSTDIR\Uninstall.exe"
    
    ; Create desktop shortcut
    CreateShortCut "$DESKTOP\NithronSync.lnk" "$INSTDIR\NithronSync.exe"
    
    ; Create uninstaller
    WriteUninstaller "$INSTDIR\Uninstall.exe"
    
    ; Registry entries
    WriteRegStr HKLM "Software\NithronSync" "InstallDir" "$INSTDIR"
    WriteRegStr HKLM "Software\NithronSync" "Version" "${VERSION}"
    
    ; Add/Remove Programs entry
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NithronSync" \
        "DisplayName" "NithronSync"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NithronSync" \
        "UninstallString" "$\"$INSTDIR\Uninstall.exe$\""
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NithronSync" \
        "InstallLocation" "$INSTDIR"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NithronSync" \
        "DisplayIcon" "$INSTDIR\NithronSync.exe"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NithronSync" \
        "Publisher" "${PRODUCT_PUBLISHER}"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NithronSync" \
        "DisplayVersion" "${VERSION}"
    WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NithronSync" \
        "URLInfoAbout" "${PRODUCT_WEB_SITE}"
    WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NithronSync" \
        "NoModify" 1
    WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NithronSync" \
        "NoRepair" 1
    
    ; Get installed size
    ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
    IntFmt $0 "0x%08X" $0
    WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NithronSync" \
        "EstimatedSize" "$0"
    
    ; Add to startup (optional)
    WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" \
        "NithronSync" "$\"$INSTDIR\NithronSync.exe$\" --minimized"
SectionEnd

; Uninstaller Section
Section "Uninstall"
    ; Stop running instance
    nsExec::ExecToLog 'taskkill /F /IM NithronSync.exe'
    
    ; Remove from startup
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "NithronSync"
    
    ; Remove files
    Delete "$INSTDIR\NithronSync.exe"
    Delete "$INSTDIR\Uninstall.exe"
    RMDir "$INSTDIR"
    
    ; Remove shortcuts
    Delete "$SMPROGRAMS\NithronSync\NithronSync.lnk"
    Delete "$SMPROGRAMS\NithronSync\Uninstall.lnk"
    RMDir "$SMPROGRAMS\NithronSync"
    Delete "$DESKTOP\NithronSync.lnk"
    
    ; Remove registry entries
    DeleteRegKey HKLM "Software\NithronSync"
    DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\NithronSync"
    
    ; Note: We don't remove user data
    ; Config: %APPDATA%\NithronSync
    ; Data: %LOCALAPPDATA%\NithronSync
    ; Sync folder: User's choice
SectionEnd

; Functions
Function .onInit
    ; Check if already installed
    ReadRegStr $0 HKLM "Software\NithronSync" "InstallDir"
    StrCmp $0 "" done
    
    MessageBox MB_OKCANCEL|MB_ICONINFORMATION \
        "NithronSync is already installed. Click OK to reinstall or Cancel to abort." \
        IDOK done IDCANCEL abort
    
    abort:
        Abort
    done:
FunctionEnd

