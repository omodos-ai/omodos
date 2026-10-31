#!/bin/bash
# Copyright 2026 Peter J. Braam
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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
