@echo off
title 37Studio
cd /d "%~dp0"
docker info >nul 2>&1
if not errorlevel 1 goto ready
echo Starting Docker Desktop...
start "" "%ProgramFiles%\Docker\Docker\Docker Desktop.exe"
for /l %%i in (1,1,90) do (
  timeout /t 2 /nobreak >nul
  docker info >nul 2>&1 && goto ready
)
echo Docker Desktop did not start. Open it yourself, wait until it says "Engine running", then open this again.
pause
exit /b 1

:ready
echo Checking for updates...
docker compose pull
docker compose up -d --remove-orphans
echo Opening 37Studio...
for /l %%i in (1,1,30) do (
  curl -s -o nul http://localhost:8080/ && goto open
  timeout /t 1 /nobreak >nul
)
:open
rem Edge comes with Windows; --app opens 37Studio in its own window, like an app.
start "" msedge --app=http://localhost:8080 || start "" http://localhost:8080
