@echo off
REM NithronSync Windows Installation Script (Portable)
REM For users who don't want to use the installer

echo NithronSync Installation
echo ========================
echo.

set INSTALL_DIR=%LOCALAPPDATA%\NithronSync
set EXE_NAME=NithronSync.exe

REM Create installation directory
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"

REM Copy executable
if exist "%~dp0NithronSync.exe" (
    copy /Y "%~dp0NithronSync.exe" "%INSTALL_DIR%\%EXE_NAME%"
) else (
    echo Error: NithronSync.exe not found in current directory
    pause
    exit /b 1
)

REM Create desktop shortcut
echo Creating desktop shortcut...
powershell -Command "$ws = New-Object -ComObject WScript.Shell; $s = $ws.CreateShortcut('%USERPROFILE%\Desktop\NithronSync.lnk'); $s.TargetPath = '%INSTALL_DIR%\%EXE_NAME%'; $s.Save()"

REM Create start menu shortcut
if not exist "%APPDATA%\Microsoft\Windows\Start Menu\Programs\NithronSync" (
    mkdir "%APPDATA%\Microsoft\Windows\Start Menu\Programs\NithronSync"
)
powershell -Command "$ws = New-Object -ComObject WScript.Shell; $s = $ws.CreateShortcut('%APPDATA%\Microsoft\Windows\Start Menu\Programs\NithronSync\NithronSync.lnk'); $s.TargetPath = '%INSTALL_DIR%\%EXE_NAME%'; $s.Save()"

REM Add to startup
reg add "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v NithronSync /t REG_SZ /d "\"%INSTALL_DIR%\%EXE_NAME%\" --minimized" /f

echo.
echo Installation complete!
echo NithronSync has been installed to: %INSTALL_DIR%
echo.
echo To start NithronSync, use the desktop shortcut or find it in the Start menu.
echo.
pause

