#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if [ "${GO2NVR_SKIP_WEBUI:-}" != "1" ]; then
  "$ROOT/scripts/build-webui.sh"
fi

cd "$ROOT"
exec go build "$@"
