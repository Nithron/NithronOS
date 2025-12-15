@echo off
REM NithronSync Windows Uninstallation Script

echo NithronSync Uninstallation
echo ==========================
echo.

set INSTALL_DIR=%LOCALAPPDATA%\NithronSync

REM Stop running process
taskkill /F /IM NithronSync.exe 2>nul

REM Remove from startup
reg delete "HKCU\Software\Microsoft\Windows\CurrentVersion\Run" /v NithronSync /f 2>nul

REM Remove executable
if exist "%INSTALL_DIR%\NithronSync.exe" (
    del /F "%INSTALL_DIR%\NithronSync.exe"
)
rmdir "%INSTALL_DIR%" 2>nul

REM Remove shortcuts
del /F "%USERPROFILE%\Desktop\NithronSync.lnk" 2>nul
del /F "%APPDATA%\Microsoft\Windows\Start Menu\Programs\NithronSync\NithronSync.lnk" 2>nul
rmdir "%APPDATA%\Microsoft\Windows\Start Menu\Programs\NithronSync" 2>nul

echo.
echo NithronSync has been uninstalled.
echo.
echo Note: Your configuration and sync data were NOT removed.
echo To remove them manually:
echo   Config: %APPDATA%\NithronSync
echo   Data: %LOCALAPPDATA%\NithronSync
echo.
pause

