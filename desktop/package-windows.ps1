# Packages the desktop app for this machine's architecture into desktop\dist: a setup .exe (installer.nsi), a .zip
# holding the program, its icon and install-windows.ps1 for people who'd rather not run an installer, and a SHA-256
# file. CI runs it on every pull request and attaches the files to each GitHub release (.github/workflows/desktop.yml). The PowerShell twin of package-linux.sh.
#   Usage: powershell -ExecutionPolicy Bypass -File desktop\package-windows.ps1 [version]
param([string]$Version)
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

if (-not $Version) {
  $Version = (Select-String -Path internal\update\update.go -Pattern '^const Version = "(.*)"').Matches[0].Groups[1].Value
}
$Version = $Version -replace '^v', ''
$arch = (go env GOARCH) # amd64 or arm64

& "$PSScriptRoot\build.ps1"
if ($LASTEXITCODE) { throw "the build failed" }

$out = "desktop\dist"
Remove-Item -Recurse -Force $out -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $out | Out-Null

# Staged in a folder of its own so the zip holds exactly these three files, whatever else is in desktop\.
$stage = Join-Path ([System.IO.Path]::GetTempPath()) "37studio-package-$([guid]::NewGuid())"
New-Item -ItemType Directory -Force $stage | Out-Null
try {
  Copy-Item desktop\bin\37studio.exe $stage       # the name matters: the agent runs `37studio gen` from this folder
  Copy-Item brand\37studio-icon.ico $stage
  Copy-Item desktop\install-windows.ps1 $stage    # unpack and run this to get a Start-menu entry
  Compress-Archive -Path "$stage\*" -DestinationPath "$out\37studio-$Version-windows-$arch.zip"
} finally {
  Remove-Item -Recurse -Force $stage -ErrorAction SilentlyContinue
}

# The installer most people download: the same program, installed with Start-menu and desktop shortcuts and an
# entry under Settings → Apps. NSIS is preinstalled on GitHub's Windows runners; elsewhere: winget install NSIS.NSIS
$makensis = (Get-Command makensis -ErrorAction SilentlyContinue).Source
if (-not $makensis) { $makensis = Join-Path ${env:ProgramFiles(x86)} "NSIS\makensis.exe" }
if (-not (Test-Path $makensis)) { throw "NSIS isn't installed, so the installer can't be built: winget install NSIS.NSIS" }
& $makensis /V2 "/DVERSION=$Version" "/DARCH=$arch" "/DOUT=dist\37studio-$Version-windows-$arch-setup.exe" desktop\installer.nsi
if ($LASTEXITCODE) { throw "makensis failed" }

# sha256sum's own format, so `sha256sum -c` checks it the way the Linux files are checked.
$sums = Get-ChildItem $out -Filter "37studio-*" | ForEach-Object {
  "$((Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLower())  $($_.Name)"
}
$sums | Set-Content "$out\37studio-$Version-windows-$arch.sha256" -Encoding ascii
Get-ChildItem $out
