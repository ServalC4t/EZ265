[Setup]
AppName=EZ265
AppVersion=1.0.0
AppPublisher=ServalC4t
AppPublisherURL=https://github.com/ServalC4t/EZ265
AppSupportURL=https://github.com/ServalC4t/EZ265/issues
DefaultDirName={autopf}\EZ265
DefaultGroupName=EZ265
UninstallDisplayIcon={app}\EZ265.exe
OutputDir=..\dist
OutputBaseFilename=EZ265_v1.0.0_Setup
Compression=lzma2/ultra64
SolidCompression=yes
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
SetupIconFile=..\assets\icon.ico
WizardStyle=modern
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "japanese"; MessagesFile: "compiler:Languages\Japanese.isl"

[Files]
Source: "..\EZ265 v1.0.0.exe"; DestDir: "{app}"; DestName: "EZ265.exe"; Flags: ignoreversion
Source: "..\bin\ffmpeg.exe"; DestDir: "{app}\bin"; Flags: ignoreversion
Source: "..\bin\ffprobe.exe"; DestDir: "{app}\bin"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\EZ265"; Filename: "{app}\EZ265.exe"
Name: "{group}\Uninstall EZ265"; Filename: "{uninstallexe}"
Name: "{autodesktop}\EZ265"; Filename: "{app}\EZ265.exe"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"
Name: "contextmenu"; Description: "Add Explorer context menu (right-click)"; GroupDescription: "Integration:"

[Registry]
; Context menu - cascading menu
Root: HKCU; Subkey: "Software\Classes\*\shell\H265Convert"; ValueType: string; ValueName: "MUIVerb"; ValueData: "EZ265"; Tasks: contextmenu; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\*\shell\H265Convert"; ValueType: string; ValueName: "Icon"; ValueData: """{app}\EZ265.exe"",0"; Tasks: contextmenu
Root: HKCU; Subkey: "Software\Classes\*\shell\H265Convert"; ValueType: string; ValueName: "SubCommands"; ValueData: ""; Tasks: contextmenu

; Sub-item 1: Add video
Root: HKCU; Subkey: "Software\Classes\*\shell\H265Convert\shell\01add"; ValueType: string; ValueName: ""; ValueData: "Add to EZ265"; Tasks: contextmenu; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\*\shell\H265Convert\shell\01add"; ValueType: string; ValueName: "Icon"; ValueData: """{app}\EZ265.exe"",0"; Tasks: contextmenu
Root: HKCU; Subkey: "Software\Classes\*\shell\H265Convert\shell\01add\command"; ValueType: string; ValueName: ""; ValueData: """{app}\EZ265.exe"" --add ""%1"""; Tasks: contextmenu

; Sub-item 2: Add and start
Root: HKCU; Subkey: "Software\Classes\*\shell\H265Convert\shell\02start"; ValueType: string; ValueName: ""; ValueData: "Add to EZ265 && Start"; Tasks: contextmenu; Flags: uninsdeletekey
Root: HKCU; Subkey: "Software\Classes\*\shell\H265Convert\shell\02start"; ValueType: string; ValueName: "Icon"; ValueData: """{app}\EZ265.exe"",0"; Tasks: contextmenu
Root: HKCU; Subkey: "Software\Classes\*\shell\H265Convert\shell\02start\command"; ValueType: string; ValueName: ""; ValueData: """{app}\EZ265.exe"" --encode ""%1"""; Tasks: contextmenu

[Run]
Filename: "{app}\EZ265.exe"; Description: "Launch EZ265"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "{app}\EZ265.exe"; Parameters: "--uninstall"; Flags: runhidden

[UninstallDelete]
Type: filesandordirs; Name: "{app}"
