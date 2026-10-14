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
; Tells Inno Setup this installer changes PATH, so it broadcasts
; WM_SETTINGCHANGE itself after install/uninstall — Explorer and anything
; launched from it afterward (a new Start Menu terminal, say) picks up the
; new PATH immediately. Does NOT refresh an already-open cmd/PowerShell
; window — Windows consoles never re-read their environment after launch,
; broadcast or not, so the finish page below still tells the user to open
; a new one.
ChangesEnvironment=yes
; Shown as a scrollable page right before the Finish button — the one
; thing worth telling the user explicitly, since nothing else on the
; wizard says it.
InfoAfterFile=postinstall-note.txt
UninstallDisplayIcon={app}\{#MyAppExeName}
; Older identifier ("x64" rather than "x64compatible") on purpose — works
; across a wider range of Inno Setup versions, including whatever ships
; preinstalled on the windows-latest GitHub Actions image.
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Files]
Source: "..\..\THIRD_PARTY_NOTICES.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\LICENSING.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\docs\licenses\*"; DestDir: "{app}\docs\licenses"; Flags: ignoreversion recursesubdirs createallsubdirs
; Built by the CI job / the developer before invoking iscc — see the build
; command in the header comment. Not checked into the repo.
Source: "..\..\bin\offcode.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\README.md"; DestDir: "{app}"; Flags: ignoreversion

[Tasks]
Name: "keepserver"; Description: "Server address"; GroupDescription: "Preserve selected Ollama settings (unchecked = no saved fallback):"
Name: "keepgpu"; Description: "GPU selection and acceleration"; GroupDescription: "Preserve selected Ollama settings (unchecked = no saved fallback):"
Name: "keepmodels"; Description: "Model storage directory"; GroupDescription: "Preserve selected Ollama settings (unchecked = no saved fallback):"
Name: "keepcontext"; Description: "Context size (unchecked: Offcode uses 8192 tokens)"; GroupDescription: "Preserve selected Ollama settings (unchecked = no saved fallback):"
Name: "keepparallel"; Description: "Parallel requests and loaded-model limits"; GroupDescription: "Preserve selected Ollama settings (unchecked = no saved fallback):"
Name: "keepcache"; Description: "Flash Attention and KV cache"; GroupDescription: "Preserve selected Ollama settings (unchecked = no saved fallback):"
Name: "keepkeepalive"; Description: "Time models stay loaded"; GroupDescription: "Preserve selected Ollama settings (unchecked = no saved fallback):"
Name: "ollamaexternal"; Description: "Use existing server only; never start Ollama automatically"; GroupDescription: "Ollama startup:"; Flags: unchecked
Name: "installollama"; Description: "Install missing Ollama using winget (only needed for model downloads)"; Flags: unchecked
Name: "spark"; Description: "Spark - qwen3:1.7b (smallest)"; GroupDescription: "Download models (optional; large downloads):"; Flags: unchecked
Name: "ember"; Description: "Ember - qwen3.5:4b"; GroupDescription: "Download models (optional; large downloads):"; Flags: unchecked
Name: "forge"; Description: "Forge - qwen3:8b"; GroupDescription: "Download models (optional; large downloads):"; Flags: unchecked
Name: "anvil"; Description: "Anvil - gemma4:12b"; GroupDescription: "Download models (optional; large downloads):"; Flags: unchecked
Name: "foundry"; Description: "Foundry - gpt-oss:20b (largest)"; GroupDescription: "Download models (optional; large downloads):"; Flags: unchecked
Name: "embedding"; Description: "Semantic search - nomic-embed-text"; GroupDescription: "Download models (optional; large downloads):"; Flags: unchecked
Name: "updateauto"; Description: "Check and automatically install new versions at startup"; GroupDescription: "Updates (skipped when offline):"; Flags: exclusive
Name: "updatenotify"; Description: "Check and notify only"; GroupDescription: "Updates (skipped when offline):"; Flags: exclusive unchecked
Name: "updateoff"; Description: "Disable startup update checks"; GroupDescription: "Updates (skipped when offline):"; Flags: exclusive unchecked

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
Type: files; Name: "{app}\offcode.exe.previous"
Type: files; Name: "{app}\offcode.exe.update-lock"

[Code]
function SetupArguments(): string;
var
  Models, Mode, Updates, Preserve: string;
begin
  Mode := 'preserve';
  if WizardIsTaskSelected('ollamaexternal') then Mode := 'external';
  Preserve := '';
  if WizardIsTaskSelected('keepserver') then Preserve := Preserve + 'server,';
  if WizardIsTaskSelected('keepgpu') then Preserve := Preserve + 'gpu,';
  if WizardIsTaskSelected('keepmodels') then Preserve := Preserve + 'models,';
  if WizardIsTaskSelected('keepcontext') then Preserve := Preserve + 'context,';
  if WizardIsTaskSelected('keepparallel') then Preserve := Preserve + 'parallel,';
  if WizardIsTaskSelected('keepcache') then Preserve := Preserve + 'cache,';
  if WizardIsTaskSelected('keepkeepalive') then Preserve := Preserve + 'keepalive,';
  if Preserve = '' then Preserve := 'none'
  else Delete(Preserve, Length(Preserve), 1);
  Updates := 'auto';
  if WizardIsTaskSelected('updatenotify') then Updates := 'notify';
  if WizardIsTaskSelected('updateoff') then Updates := 'off';
  Models := '';
  if WizardIsTaskSelected('spark') then Models := Models + 'spark,';
  if WizardIsTaskSelected('ember') then Models := Models + 'ember,';
  if WizardIsTaskSelected('forge') then Models := Models + 'forge,';
  if WizardIsTaskSelected('anvil') then Models := Models + 'anvil,';
  if WizardIsTaskSelected('foundry') then Models := Models + 'foundry,';
  if WizardIsTaskSelected('embedding') then Models := Models + 'embedding,';
  if Models = '' then Models := 'none'
  else Delete(Models, Length(Models), 1);
  Result := 'setup -ollama=' + Mode + ' -preserve=' + Preserve + ' -updates=' + Updates + ' -models=' + Models;
  if WizardIsTaskSelected('installollama') then Result := Result + ' -install-ollama';
end;

procedure CurStepChanged(CurStep: TSetupStep);
var
  ResultCode: Integer;
begin
  if CurStep = ssPostInstall then
  begin
    if not Exec(ExpandConstant('{app}\{#MyAppExeName}'), SetupArguments(), '', SW_SHOW,
      ewWaitUntilTerminated, ResultCode) then
      MsgBox('Could not start setup. Run offcode setup -interactive to retry.', mbError, MB_OK)
    else if ResultCode <> 0 then
      MsgBox('Offcode installed, but setup or model downloads failed. Run offcode setup -interactive to resume downloads.', mbError, MB_OK);
  end;
end;

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
