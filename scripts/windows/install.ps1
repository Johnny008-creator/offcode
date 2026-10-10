#Requires -Version 5.1
<#
.SYNOPSIS
    Installs or uninstalls offcode for the current Windows user, no admin
    rights or GUI installer required.

.DESCRIPTION
    Downloads the latest offcode.exe release asset (or copies one already
    built locally, via -LocalExe), places it under
    %LocalAppData%\Programs\offcode, and adds that directory to the
    current user's PATH. This is the scriptable alternative to the Inno
    Setup GUI installer built by scripts\windows\offcode.iss /
    .github\workflows\release.yml — same install location convention,
    no registry uninstall entry.

.PARAMETER LocalExe
    Path to an already-built offcode.exe to install instead of downloading
    one. Useful right after `go build -o bin\offcode.exe .\cmd\offcode`.

.PARAMETER Version
    Release tag to download when -LocalExe is not given, e.g. "v0.1.0".
    Defaults to the latest release.

.PARAMETER Uninstall
    Remove the installed exe and its PATH entry.

.EXAMPLE
    irm https://raw.githubusercontent.com/Johnny008-creator/offcode/master/scripts/windows/install.ps1 | iex

.EXAMPLE
    .\install.ps1 -LocalExe ..\..\bin\offcode.exe

.EXAMPLE
    .\install.ps1 -Uninstall
#>
[CmdletBinding()]
param(
    [string]$LocalExe,
    [string]$Version,
    [switch]$Uninstall
)

$ErrorActionPreference = 'Stop'

$InstallDir = Join-Path $env:LocalAppData 'Programs\offcode'
$ExePath = Join-Path $InstallDir 'offcode.exe'

function Add-UserPath([string]$Dir) {
    $current = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not $current) { $current = '' }
    $parts = $current.Split(';') | Where-Object { $_ -ne '' }
    if ($parts -contains $Dir) { return }
    $updated = if ($current -and -not $current.EndsWith(';')) { "$current;$Dir" } else { "$current$Dir" }
    [Environment]::SetEnvironmentVariable('Path', $updated, 'User')
    $env:Path = "$env:Path;$Dir"
}

function Remove-UserPath([string]$Dir) {
    $current = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not $current) { return }
    $parts = $current.Split(';') | Where-Object { $_ -ne '' -and $_ -ne $Dir }
    [Environment]::SetEnvironmentVariable('Path', ($parts -join ';'), 'User')
}

if ($Uninstall) {
    if (Test-Path $InstallDir) {
        Remove-Item -Recurse -Force $InstallDir
        Write-Host "Removed $InstallDir"
    }
    Remove-UserPath $InstallDir
    Write-Host "offcode uninstalled. Open a new terminal for the PATH change to take effect."
    return
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

if ($LocalExe) {
    if (-not (Test-Path $LocalExe)) {
        throw "No file at -LocalExe path: $LocalExe"
    }
    Copy-Item -Force $LocalExe $ExePath
} else {
    $releaseUrl = if ($Version) {
        "https://github.com/Johnny008-creator/offcode/releases/download/$Version/offcode-windows-amd64.exe"
    } else {
        "https://github.com/Johnny008-creator/offcode/releases/latest/download/offcode-windows-amd64.exe"
    }
    Write-Host "Downloading $releaseUrl"
    Invoke-WebRequest -Uri $releaseUrl -OutFile $ExePath
}

Add-UserPath $InstallDir

Write-Host "offcode installed to $InstallDir"
Write-Host "Open a new terminal and run 'offcode' to get started."
