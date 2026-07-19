@ECHO OFF
SETLOCAL
SET CI=true

SET ROOT=%~dp0..
SET WEBUI=%ROOT%\webui
SET NVRUI=%ROOT%\internal\nvrui\dist

WHERE pnpm >NUL 2>NUL
IF ERRORLEVEL 1 (
  ECHO pnpm is required to build the Web UI. 1>&2
  EXIT /B 1
)

IF NOT EXIST "%WEBUI%\package.json" (
  ECHO webui\package.json was not found. 1>&2
  EXIT /B 1
)

CALL pnpm --dir "%WEBUI%" install --frozen-lockfile
IF ERRORLEVEL 1 EXIT /B %ERRORLEVEL%
CALL pnpm --dir "%WEBUI%" build
IF ERRORLEVEL 1 EXIT /B %ERRORLEVEL%

IF NOT EXIST "%NVRUI%" MKDIR "%NVRUI%"
TYPE NUL > "%NVRUI%\.gitkeep"
EXIT /B 0
