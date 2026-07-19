#!/bin/sh

set -e  # Exit immediately if a command exits with a non-zero status.
set -u  # Treat unset variables as an error when substituting.

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

check_command() {
    if ! command -v "$1" >/dev/null
    then
        echo "Error: $1 could not be found. Please install it." >&2
        return 1
    fi
}

build_zip() {
  GO2NVR_SKIP_WEBUI=1 "$ROOT/scripts/build-go2nvr.sh" -ldflags "-s -w" -trimpath -o "$2"
  7z a -mx9 -sdel "$1" "$2"
}

build_upx() {
  GO2NVR_SKIP_WEBUI=1 "$ROOT/scripts/build-go2nvr.sh" -ldflags "-s -w" -trimpath -o "$1"
  upx --best --lzma "$1"
}

check_command go
check_command 7z
check_command upx

export CGO_ENABLED=0

"$ROOT/scripts/build-webui.sh"

set -x  # Print commands and their arguments as they are executed.

GOOS=windows  GOARCH=amd64        build_zip go2nvr_win64.zip     go2nvr.exe
GOOS=windows  GOARCH=386          build_zip go2nvr_win32.zip     go2nvr.exe
GOOS=windows  GOARCH=arm64        build_zip go2nvr_win_arm64.zip go2nvr.exe

GOOS=linux    GOARCH=amd64        build_upx go2nvr_linux_amd64
GOOS=linux    GOARCH=386          build_upx go2nvr_linux_i386
GOOS=linux    GOARCH=arm64        build_upx go2nvr_linux_arm64
GOOS=linux    GOARCH=mipsle       build_upx go2nvr_linux_mipsel
GOOS=linux    GOARCH=arm GOARM=7  build_upx go2nvr_linux_arm
GOOS=linux    GOARCH=arm GOARM=6  build_upx go2nvr_linux_armv6

GOOS=darwin   GOARCH=amd64        build_zip go2nvr_mac_amd64.zip go2nvr
GOOS=darwin   GOARCH=arm64        build_zip go2nvr_mac_arm64.zip go2nvr

GOOS=freebsd  GOARCH=amd64        build_zip go2nvr_freebsd_amd64.zip go2nvr
GOOS=freebsd  GOARCH=arm64        build_zip go2nvr_freebsd_arm64.zip go2nvr
