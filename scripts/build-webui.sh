#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
WEBUI="$ROOT/webui"
NVRUI="$ROOT/internal/nvrui/dist"

if ! command -v pnpm >/dev/null 2>&1; then
  echo "pnpm is required to build the Web UI." >&2
  exit 1
fi

if [ ! -f "$WEBUI/package.json" ]; then
  echo "webui/package.json was not found." >&2
  exit 1
fi

mkdir -p "$NVRUI"
pnpm --dir "$WEBUI" install --frozen-lockfile
pnpm --dir "$WEBUI" build

# Keep a tracked sentinel so `go:embed` remains valid before the first UI build.
touch "$NVRUI/.gitkeep"

test -f "$NVRUI/index.html"
test -f "$NVRUI/favicon.ico"
test -d "$NVRUI/assets"
