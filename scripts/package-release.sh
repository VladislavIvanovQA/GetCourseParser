#!/usr/bin/env bash
# usage: package-release.sh out.zip [path/to/GetCourseDownloader-folder]
set -euo pipefail
ZIP_PATH="${1:?usage: package-release.sh out.zip [bundle-dir]}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SRC="${2:-$ROOT/dist/GetCourseDownloader}"
BIN="$SRC/GetCourseDownloader"
[[ -x "$BIN" ]] || { echo "Missing executable: $BIN" >&2; exit 1; }
cp -f "$ROOT/scripts/RELEASE.txt" "$SRC/RELEASE.txt"
[[ "$ZIP_PATH" = /* ]] || ZIP_PATH="$ROOT/$ZIP_PATH"
rm -f "$ZIP_PATH"
FOLDER_NAME="$(basename "$SRC")"
PARENT="$(dirname "$SRC")"
(cd "$PARENT" && zip -rq "$ZIP_PATH" "$FOLDER_NAME")
echo "Created: $ZIP_PATH"
