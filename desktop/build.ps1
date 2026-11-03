# Builds the desktop app for this computer into desktop\bin\37studio.exe. Needs Go 1.26+, Node 22+ and the WebView2
# runtime, which Windows 10 and 11 already have. The PowerShell twin of build.sh, which builds it on Linux and macOS.
# The program must keep the name 37studio: the agent runs `37studio gen|clip|transcribe` from the folder it sits in.
#   Usage: powershell -ExecutionPolicy Bypass -File desktop\build.ps1
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

Push-Location web
if (-not (Test-Path node_modules)) { npm ci; if ($LASTEXITCODE) { throw "npm ci failed" } }
npm run build
if ($LASTEXITCODE) { throw "npm run build failed" }
Pop-Location

$app = "cmd\37studio-desktop"
Remove-Item -Recurse -Force "$app\web" -ErrorAction SilentlyContinue
Copy-Item -Recurse "web\dist" "$app\web"
Copy-Item "brand\37studio-icon-1024.png" "$app\icon.png" -Force
New-Item -ItemType Directory -Force desktop\bin | Out-Null

Push-Location $app
# -H windowsgui: no console window flashes up behind the app when it opens from the Start menu.
go build -ldflags "-H windowsgui" -o ..\..\desktop\bin\37studio.exe .
$built = $LASTEXITCODE
Pop-Location
if ($built) { throw "go build failed" }
Write-Host "Built desktop\bin\37studio.exe"
