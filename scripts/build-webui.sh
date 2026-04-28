#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEBUI="$ROOT/webui"
WWW="$ROOT/www"

if ! command -v pnpm >/dev/null 2>&1; then
  echo "pnpm is required to build the Web UI." >&2
  exit 1
fi

if [ ! -f "$WEBUI/package.json" ]; then
  echo "webui/package.json was not found." >&2
  exit 1
fi

pnpm --dir "$WEBUI" install --frozen-lockfile
pnpm --dir "$WEBUI" build

case "$WWW" in
  "$ROOT"/www) ;;
  *)
    echo "Refusing to clean unexpected output directory: $WWW" >&2
    exit 1
    ;;
esac

mkdir -p "$WWW"
rm -rf "$WWW/assets" "$WWW/index.html" "$WWW/favicon.ico"
cp -R "$WEBUI/dist/." "$WWW/"

test -f "$WWW/index.html"
test -f "$WWW/favicon.ico"
test -d "$WWW/assets"
