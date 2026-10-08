#define AppName "CleanPause"
#define AppVersion "1.0.0"

[Setup]
AppId={{6EAD64D5-5DDC-4B7D-94FB-0C2A0361292B}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher=CleanPause
DefaultDirName={localappdata}\CleanPause
DisableDirPage=yes
DefaultGroupName=CleanPause
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
AppMutex=Local\CleanPause.UI.v1
SetupMutex=Local\CleanPause.Setup
OutputDir=..\dist
OutputBaseFilename=CleanPause-Setup
SetupIconFile=..\assets\icons\cleanpause.ico
UninstallDisplayIcon={app}\CleanPause.exe
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
LanguageDetectionMethod=uilanguage
SignTool=cleanpause
SignedUninstaller=yes
SignToolRunMinimized=yes
CloseApplications=no
RestartApplications=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "russian"; MessagesFile: "compiler:Languages\Russian.isl"

[CustomMessages]
english.DesktopShortcut=Create a desktop shortcut
russian.DesktopShortcut=Создать ярлык на рабочем столе
english.LaunchApp=Launch CleanPause
russian.LaunchApp=Запустить CleanPause

[Tasks]
Name: "desktopicon"; Description: "{cm:DesktopShortcut}"; Flags: unchecked

[Files]
Source: "..\dist\CleanPause.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; DestName: "LICENSE.txt"; Flags: ignoreversion

[Icons]
Name: "{userprograms}\CleanPause"; Filename: "{app}\CleanPause.exe"; WorkingDir: "{app}"
Name: "{userdesktop}\CleanPause"; Filename: "{app}\CleanPause.exe"; WorkingDir: "{app}"; Tasks: desktopicon

[Run]
Filename: "{app}\CleanPause.exe"; Description: "{cm:LaunchApp}"; Flags: nowait postinstall skipifsilent

[Code]
procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Value: String;
begin
  if CurUninstallStep = usUninstall then begin
    if RegQueryStringValue(HKCU, 'Software\Microsoft\Windows\CurrentVersion\Run', 'CleanPause', Value) then begin
      if CompareText(Value, '"' + ExpandConstant('{app}\CleanPause.exe') + '"') = 0 then
        RegDeleteValue(HKCU, 'Software\Microsoft\Windows\CurrentVersion\Run', 'CleanPause');
    end;
  end;
end;
