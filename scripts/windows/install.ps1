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

.PARAMETER Preserve
    Comma-separated server,gpu,models,context,parallel,cache,keepalive, or all/none.
    The interactive wizard offers independent checkboxes for these choices.

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
    [switch]$Uninstall,
    [ValidateSet('preserve', 'auto', 'external')][string]$Ollama = 'preserve',
    [string]$Models = 'none',
    [string]$Preserve,
    [ValidateSet('auto', 'notify', 'off')][string]$Updates = 'auto',
    [switch]$InstallOllama,
    [switch]$NonInteractive
)

$ErrorActionPreference = 'Stop'

$InstallDir = Join-Path $env:LocalAppData 'Programs\offcode'
$ExePath = Join-Path $InstallDir 'offcode.exe'

function Broadcast-EnvironmentChange() {
    # Tells Explorer to re-read the environment, so a terminal opened from
    # the Start Menu (or Win+R) right after this script runs sees the new
    # PATH immediately. Does NOT refresh an already-open cmd/PowerShell
    # window — consoles only read their environment once, at launch, no
    # matter what. Best-effort: a failure here just means the user needs a
    # fresh window instead of it being picked up a few seconds sooner.
    try {
        $sig = '[DllImport("user32.dll", SetLastError = true, CharSet = CharSet.Auto)]public static extern IntPtr SendMessageTimeout(IntPtr hWnd, uint Msg, UIntPtr wParam, string lParam, uint fuFlags, uint uTimeout, out UIntPtr lpdwResult);'
        $type = Add-Type -MemberDefinition $sig -Name NativeMethods -Namespace OffcodeInstaller -PassThru
        $result = [UIntPtr]::Zero
        $HWND_BROADCAST = [IntPtr]0xffff
        $WM_SETTINGCHANGE = 0x1A
        $SMTO_ABORTIFHUNG = 0x2
        $type::SendMessageTimeout($HWND_BROADCAST, $WM_SETTINGCHANGE, [UIntPtr]::Zero, 'Environment', $SMTO_ABORTIFHUNG, 5000, [ref]$result) | Out-Null
    } catch {
        # Ignored — see comment above.
    }
}

function Add-UserPath([string]$Dir) {
    $current = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not $current) { $current = '' }
    $parts = $current.Split(';') | Where-Object { $_ -ne '' }
    if ($parts -contains $Dir) { return }
    $updated = if ($current -and -not $current.EndsWith(';')) { "$current;$Dir" } else { "$current$Dir" }
    [Environment]::SetEnvironmentVariable('Path', $updated, 'User')
    $env:Path = "$env:Path;$Dir"
    Broadcast-EnvironmentChange
}

function Remove-UserPath([string]$Dir) {
    $current = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not $current) { return }
    $parts = $current.Split(';') | Where-Object { $_ -ne '' -and $_ -ne $Dir }
    [Environment]::SetEnvironmentVariable('Path', ($parts -join ';'), 'User')
    Broadcast-EnvironmentChange
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

$setupArgs = @('setup', "-ollama=$Ollama", "-models=$Models", "-updates=$Updates")
if ($PSBoundParameters.ContainsKey('Preserve')) { $setupArgs += "-preserve=$Preserve" }
if ($InstallOllama) { $setupArgs += '-install-ollama' }
if (-not $NonInteractive -and -not $PSBoundParameters.ContainsKey('Models')) { $setupArgs += '-interactive' }
& $ExePath @setupArgs
if ($LASTEXITCODE -ne 0) { throw 'Offcode was installed, but setup/downloads failed. Rerun offcode setup -interactive to retry.' }

Write-Host "offcode installed to $InstallDir"
Write-Host "Open a new terminal and run 'offcode' to get started."
