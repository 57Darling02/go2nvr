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

if [ -z "${APP_VERSION:-}" ]; then
  if [ "${GITHUB_REF_TYPE:-}" = "tag" ] && [ -n "${GITHUB_REF_NAME:-}" ]; then
    APP_VERSION="$GITHUB_REF_NAME"
  elif command -v git >/dev/null 2>&1 && git -C "$ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    APP_VERSION="$(git -C "$ROOT" describe --tags --dirty --always 2>/dev/null || git -C "$ROOT" rev-parse --short HEAD)"
  else
    APP_VERSION="dev"
  fi
fi
export VITE_APP_VERSION="$APP_VERSION"

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
