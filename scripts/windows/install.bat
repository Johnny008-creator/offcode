@echo off
rem Pure cmd/batch installer for offcode — no PowerShell, no GUI.
rem Alternative to the Inno Setup GUI wizard (scripts\windows\offcode.iss)
rem for anyone who prefers a plain double-clickable .bat.
rem
rem Usage:
rem   install.bat                   download the latest release and install
rem   install.bat path\to\offcode.exe   install an already-built exe instead
rem   install.bat /uninstall        remove it again
setlocal enabledelayedexpansion

set "INSTALL_DIR=%LocalAppData%\Programs\offcode"
set "EXE_PATH=%INSTALL_DIR%\offcode.exe"

if /i "%~1"=="/uninstall" goto :uninstall
if /i "%~1"=="-uninstall" goto :uninstall

if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%" >nul 2>nul

if not "%~1"=="" (
    copy /y "%~1" "%EXE_PATH%" >nul
    if errorlevel 1 (
        echo Failed to copy "%~1" to "%EXE_PATH%"
        exit /b 1
    )
) else (
    echo Downloading latest offcode.exe...
    certutil -urlcache -split -f "https://github.com/Johnny008-creator/offcode/releases/latest/download/offcode-windows-amd64.exe" "%EXE_PATH%" >nul
    if errorlevel 1 (
        echo Download failed. Build locally and re-run: install.bat path\to\offcode.exe
        exit /b 1
    )
)

call :addpath
rem Shared setup keeps all installer choices and download behavior consistent.
"%EXE_PATH%" setup -interactive
if errorlevel 1 (
    echo Offcode installed, but setup failed. Run "offcode setup -interactive" to retry.
    exit /b 1
)
echo offcode installed to %INSTALL_DIR%
echo Open a new terminal and run "offcode" to get started.
goto :eof

:addpath
rem Read the current per-user PATH straight from the registry (not %PATH%,
rem which also carries the machine-wide PATH — writing that back would
rem duplicate it into the user key and grow it every run).
set "USERPATH="
for /f "usebackq skip=1 tokens=2,*" %%A in (`reg query "HKCU\Environment" /v Path 2^>nul`) do set "USERPATH=%%B"
echo ";!USERPATH!;" | find /i ";%INSTALL_DIR%;" >nul
if not errorlevel 1 goto :eof
if defined USERPATH (
    reg add "HKCU\Environment" /v Path /t REG_EXPAND_SZ /d "!USERPATH!;%INSTALL_DIR%" /f >nul
) else (
    reg add "HKCU\Environment" /v Path /t REG_EXPAND_SZ /d "%INSTALL_DIR%" /f >nul
)
call :broadcastenv
goto :eof

rem Tells Explorer to re-read the environment (WM_SETTINGCHANGE), so a
rem terminal opened from the Start Menu right after this script runs sees
rem the new PATH immediately. Does NOT refresh an already-open cmd/
rem PowerShell window — consoles only read their environment once, at
rem launch, no matter what; that's why "open a new terminal" below still
rem matters regardless. Needs powershell.exe (present by default since
rem Windows 7); silently skipped if it's missing.
:broadcastenv
where powershell >nul 2>nul
if errorlevel 1 goto :eof
powershell -NoProfile -Command "$sig='[DllImport(\"user32.dll\",SetLastError=true,CharSet=CharSet.Auto)]public static extern IntPtr SendMessageTimeout(IntPtr hWnd,uint Msg,UIntPtr wParam,string lParam,uint fuFlags,uint uTimeout,out UIntPtr lpdwResult);'; $t=Add-Type -MemberDefinition $sig -Name NativeMethods -Namespace OffcodeInstaller -PassThru; $r=[UIntPtr]::Zero; $t::SendMessageTimeout([IntPtr]0xffff,0x1A,[UIntPtr]::Zero,'Environment',2,5000,[ref]$r) | Out-Null" >nul 2>nul
goto :eof

:uninstall
if exist "%INSTALL_DIR%" rmdir /s /q "%INSTALL_DIR%"
set "USERPATH="
for /f "usebackq skip=1 tokens=2,*" %%A in (`reg query "HKCU\Environment" /v Path 2^>nul`) do set "USERPATH=%%B"
if defined USERPATH (
    set "NEWPATH=;!USERPATH!;"
    set "NEWPATH=!NEWPATH:;%INSTALL_DIR%;=;!"
    set "NEWPATH=!NEWPATH:~1,-1!"
    reg add "HKCU\Environment" /v Path /t REG_EXPAND_SZ /d "!NEWPATH!" /f >nul
)
call :broadcastenv
echo offcode uninstalled. Open a new terminal for the PATH change to take effect.
goto :eof
