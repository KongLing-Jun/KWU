@echo off
setlocal

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0start-integration-20260809.ps1" %*
if errorlevel 1 (
  echo.
  echo Integration startup failed. Review the error above and .codex-run\integration-launcher logs.
  pause
  exit /b 1
)

endlocal
