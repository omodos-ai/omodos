#!/bin/bash
set -euo pipefail

PROVIDER="$1"
ZIP_PATH="$2"
INGEST_DIR="$3"
DATE="$4"
FORCE_STR="$5"

if [ ! -f "$ZIP_PATH" ]; then
    echo "[ERROR] Zip file not found: $ZIP_PATH"
    exit 1
fi

TARGET="$INGEST_DIR/$PROVIDER/$DATE-export"

if [ "$FORCE_STR" = "true" ]; then
    echo "[WIPE] Removing existing directory: $TARGET"
    rm -rf "$TARGET"
fi

mkdir -p "$TARGET"
echo "[EXTRACT] Unzipping $PROVIDER export..."
echo "          From: $ZIP_PATH"
echo "          To:   $TARGET"

7zz x "$ZIP_PATH" -o"$TARGET" -y > /dev/null 2>&1

echo "[✓] Unzip pipeline complete."
