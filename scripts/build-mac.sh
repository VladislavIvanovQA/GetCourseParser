#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/dist/GetCourseDownloader"
mkdir -p "$OUT"
cd "$ROOT/desktop"
go build -ldflags "-s -w" -o "$OUT/GetCourseDownloader" .
rm -rf "$OUT/extension"
cp -R "$ROOT/extension" "$OUT/extension"
mkdir -p "$OUT/downloads"
echo ""
echo "Built: $OUT/GetCourseDownloader"
echo "Run:   cd $OUT && ./GetCourseDownloader"
echo "Ext:   chrome://extensions → Load unpacked → $ROOT/extension"
