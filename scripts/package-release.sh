#!/usr/bin/env bash
set -euo pipefail
ZIP_PATH="${1:?usage: package-release.sh out.zip}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SRC="$ROOT/dist/GetCourseDownloader"
BIN="$SRC/GetCourseDownloader"
[[ -x "$BIN" ]] || { echo "Build first: ./scripts/build-mac.sh" >&2; exit 1; }
cp -f "$ROOT/scripts/RELEASE.txt" "$SRC/RELEASE.txt"
[[ "$ZIP_PATH" = /* ]] || ZIP_PATH="$ROOT/$ZIP_PATH"
rm -f "$ZIP_PATH"
(cd "$ROOT/dist" && zip -rq "$ZIP_PATH" GetCourseDownloader)
echo "Created: $ZIP_PATH"
