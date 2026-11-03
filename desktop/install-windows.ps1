# Adds 37Studio to this user's Start menu, with no admin rights: the program goes to %LOCALAPPDATA%\Programs\37Studio.
# Run from a source checkout it builds the app first; run from an unpacked release it installs the program beside it.
# Run it again after pulling changes. To remove: .\install-windows.ps1 -Uninstall
param([switch]$Uninstall)
$ErrorActionPreference = "Stop"

$app      = Join-Path $env:LOCALAPPDATA "Programs\37Studio"
$shortcut = Join-Path $env:APPDATA "Microsoft\Windows\Start Menu\Programs\37Studio.lnk"

if ($Uninstall) {
  Remove-Item -Recurse -Force $app -ErrorAction SilentlyContinue
  Remove-Item -Force $shortcut -ErrorAction SilentlyContinue
  Write-Host "Removed 37Studio from the Start menu. Your brands and videos in $env:APPDATA\37Studio are still there."
  return
}

if (Get-Process 37studio -ErrorAction SilentlyContinue) {
  throw "37Studio is open. Windows won't replace a running program: close it and run this again."
}

# A source checkout has build.ps1 next to this file; an unpacked release has the program itself there instead.
if (Test-Path (Join-Path $PSScriptRoot "build.ps1")) {
  & (Join-Path $PSScriptRoot "build.ps1")
  if ($LASTEXITCODE) { throw "the build failed" }
  $root = Join-Path $PSScriptRoot ".."
  $exe = Join-Path $root "desktop\bin\37studio.exe"
  $ico = Join-Path $root "brand\37studio-icon.ico"
} else {
  $exe = Join-Path $PSScriptRoot "37studio.exe"
  $ico = Join-Path $PSScriptRoot "37studio-icon.ico"
}
if (-not (Test-Path $exe)) { throw "37studio.exe isn't where this script expected it: $exe" }

New-Item -ItemType Directory -Force $app | Out-Null
# Keep the name 37studio.exe: the agent runs `37studio gen|clip|transcribe` from the folder the app sits in.
Copy-Item $exe (Join-Path $app "37studio.exe") -Force
Copy-Item $ico (Join-Path $app "37studio-icon.ico") -Force

$shell = New-Object -ComObject WScript.Shell
$link = $shell.CreateShortcut($shortcut)
$link.TargetPath = Join-Path $app "37studio.exe"
$link.WorkingDirectory = $app
$link.IconLocation = Join-Path $app "37studio-icon.ico"
$link.Description = "AI video studio and social media desk"
$link.Save()

Write-Host "Installed. Press the Windows key, type 37Studio and open it."
