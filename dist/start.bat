@echo off
rem Haulnet community server for Windows. Settings: haulnet-server.env next to this file.
cd /d "%~dp0"
if not exist haulnet-server.env (
  copy /y haulnet-server.env.example haulnet-server.env >nul
  echo Created haulnet-server.env. Open it, set EXTERNAL_ADDRESS, then run start.bat again.
  pause
  exit /b 1
)
haulnetd.exe -env haulnet-server.env
pause
