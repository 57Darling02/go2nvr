@ECHO OFF

CALL "%~dp0build-webui.cmd"
IF ERRORLEVEL 1 EXIT /B %ERRORLEVEL%
SET GO2NVR_SKIP_WEBUI=1

@SET GOOS=windows
@SET GOARCH=amd64
@SET FILENAME=go2nvr_win64.zip
CALL "%~dp0build-go2nvr.cmd" -ldflags "-s -w" -trimpath -o go2nvr.exe && 7z a -mx9 -sdel %FILENAME% go2nvr.exe

@SET GOOS=windows
@SET GOARCH=386
@SET FILENAME=go2nvr_win32.zip
CALL "%~dp0build-go2nvr.cmd" -ldflags "-s -w" -trimpath -o go2nvr.exe && 7z a -mx9 -sdel %FILENAME% go2nvr.exe

@SET GOOS=windows
@SET GOARCH=arm64
@SET FILENAME=go2nvr_win_arm64.zip
CALL "%~dp0build-go2nvr.cmd" -ldflags "-s -w" -trimpath -o go2nvr.exe && 7z a -mx9 -sdel %FILENAME% go2nvr.exe

@SET GOOS=linux
@SET GOARCH=amd64
@SET FILENAME=go2nvr_linux_amd64
CALL "%~dp0build-go2nvr.cmd" -ldflags "-s -w" -trimpath -o %FILENAME% && upx --best --lzma %FILENAME%

@SET GOOS=linux
@SET GOARCH=386
@SET FILENAME=go2nvr_linux_i386
CALL "%~dp0build-go2nvr.cmd" -ldflags "-s -w" -trimpath -o %FILENAME% && upx --best --lzma %FILENAME%

@SET GOOS=linux
@SET GOARCH=arm64
@SET FILENAME=go2nvr_linux_arm64
CALL "%~dp0build-go2nvr.cmd" -ldflags "-s -w" -trimpath -o %FILENAME% && upx --best --lzma %FILENAME%

@SET GOOS=linux
@SET GOARCH=arm
@SET GOARM=7
@SET FILENAME=go2nvr_linux_arm
CALL "%~dp0build-go2nvr.cmd" -ldflags "-s -w" -trimpath -o %FILENAME% && upx --best --lzma %FILENAME%

@SET GOOS=linux
@SET GOARCH=arm
@SET GOARM=6
@SET FILENAME=go2nvr_linux_armv6
CALL "%~dp0build-go2nvr.cmd" -ldflags "-s -w" -trimpath -o %FILENAME% && upx --best --lzma %FILENAME%

@SET GOOS=linux
@SET GOARCH=mipsle
@SET FILENAME=go2nvr_linux_mipsel
CALL "%~dp0build-go2nvr.cmd" -ldflags "-s -w" -trimpath -o %FILENAME% && upx --best --lzma %FILENAME%

@SET GOOS=darwin
@SET GOARCH=amd64
@SET FILENAME=go2nvr_mac_amd64.zip
CALL "%~dp0build-go2nvr.cmd" -ldflags "-s -w" -trimpath -o go2nvr && 7z a -mx9 -sdel %FILENAME% go2nvr

@SET GOOS=darwin
@SET GOARCH=arm64
@SET FILENAME=go2nvr_mac_arm64.zip
CALL "%~dp0build-go2nvr.cmd" -ldflags "-s -w" -trimpath -o go2nvr && 7z a -mx9 -sdel %FILENAME% go2nvr

@SET GOOS=freebsd
@SET GOARCH=amd64
@SET FILENAME=go2nvr_freebsd_amd64.zip
CALL "%~dp0build-go2nvr.cmd" -ldflags "-s -w" -trimpath -o go2nvr && 7z a -mx9 -sdel %FILENAME% go2nvr

@SET GOOS=freebsd
@SET GOARCH=arm64
@SET FILENAME=go2nvr_freebsd_arm64.zip
CALL "%~dp0build-go2nvr.cmd" -ldflags "-s -w" -trimpath -o go2nvr && 7z a -mx9 -sdel %FILENAME% go2nvr
