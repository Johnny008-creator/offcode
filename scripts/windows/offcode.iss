; Inno Setup script for the offcode Windows installer.
;
; Builds a single installer exe that copies offcode.exe into the current
; user's per-user Programs folder (no admin rights needed), adds that
; directory to the user's PATH, and creates a Start Menu shortcut that
; opens a console with offcode ready to run.
;
; Build locally (Windows, with Inno Setup 6 installed):
;   go build -o bin\offcode.exe .\cmd\offcode
;   iscc /DMyAppVersion=0.1.0 scripts\windows\offcode.iss
;
; CI builds this automatically on tagged releases — see
; .github/workflows/release.yml — passing /DMyAppVersion from the tag, so
; this hardcoded default only matters for ad-hoc local builds.
#ifndef MyAppVersion
#define MyAppVersion "0.1.0"
#endif

#define MyAppName "offcode"
#define MyAppPublisher "offcode"
#define MyAppURL "https://github.com/Johnny008-creator/offcode"
#define MyAppExeName "offcode.exe"

[Setup]
AppId={{B6E2E6B6-6F2A-4E1A-9B9C-0FFC0DE00001}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
; {userpf} = "%LocalAppData%\Programs" — a per-user location that installs
; and uninstalls cleanly without admin rights, matching the other two
; installers (install.bat / install.ps1) and PrivilegesRequired=lowest below.
DefaultDirName={userpf}\offcode
DefaultGroupName=offcode
DisableProgramGroupPage=yes
; Per-user install by default (no admin prompt); user can still pick an
; all-users location if they relaunch the installer elevated.
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
OutputDir=..\..\dist
OutputBaseFilename=offcode-setup-{#MyAppVersion}
Compression=lzma
SolidCompression=yes
WizardStyle=modern
UninstallDisplayIcon={app}\{#MyAppExeName}
; Older identifier ("x64" rather than "x64compatible") on purpose — works
; across a wider range of Inno Setup versions, including whatever ships
; preinstalled on the windows-latest GitHub Actions image.
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Files]
; Built by the CI job / the developer before invoking iscc — see the build
; command in the header comment. Not checked into the repo.
Source: "..\..\bin\offcode.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\README.md"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
; A plain shortcut to the exe would flash a console and close immediately
; since offcode is a CLI tool, not a GUI app — launch it inside a kept-open
; cmd window instead, starting in the user's home directory.
Name: "{group}\offcode"; Filename: "{cmd}"; Parameters: "/k ""{app}\{#MyAppExeName}"""; WorkingDir: "{%USERPROFILE}"; IconFilename: "{app}\{#MyAppExeName}"
Name: "{group}\Uninstall offcode"; Filename: "{uninstallexe}"

[Registry]
; Append {app} to the current user's PATH so "offcode" works from any
; terminal without relaunching one from the Start Menu shortcut.
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; \
    ValueData: "{olddata};{app}"; Flags: preservestringtype; Check: NeedsAddPath(ExpandConstant('{app}'))

[UninstallDelete]
Type: files; Name: "{app}\offcode.exe"

[Code]
function NeedsAddPath(Param: string): boolean;
var
  OrigPath: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', OrigPath) then
  begin
    Result := True;
    exit;
  end;
  { Avoid duplicate entries on repeat installs/upgrades. }
  Result := Pos(';' + Param + ';', ';' + OrigPath + ';') = 0;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Path: string;
  P: Integer;
  AppDir: string;
begin
  if CurUninstallStep = usPostUninstall then
  begin
    AppDir := ExpandConstant('{app}');
    if RegQueryStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', Path) then
    begin
      P := Pos(';' + AppDir + ';', ';' + Path + ';');
      if P > 0 then
      begin
        { Pos was computed against the ';'-padded string; rebuild without
          the padding to edit the real value safely. }
        Path := ';' + Path + ';';
        Delete(Path, P, Length(AppDir) + 1);
        Delete(Path, 1, 1);
        Delete(Path, Length(Path), 1);
        RegWriteStringValue(HKEY_CURRENT_USER, 'Environment', 'Path', Path);
      end;
    end;
  end;
end;
