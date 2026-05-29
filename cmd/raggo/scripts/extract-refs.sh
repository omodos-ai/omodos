#!/usr/bin/env bash

INFILE="${1:-/dev/stdin}"
SOURCE_DIR="${2:-source}"
PATTERN_FILE=".tmp_uuids.txt"

# 1. Extract, sort chronologically, and dedupe timestamps
grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(Z|[+-][0-9]{2}:[0-9]{2})' "$INFILE" | sort | uniq > "$PATTERN_FILE"

# Ensure we actually found something
if [ ! -s "$PATTERN_FILE" ]; then
    echo "Error: No valid timestamps found in input stream." >&2
    rm -f "$PATTERN_FILE"
    exit 1
fi

# 2. Fetch files strictly in the order of the sorted timestamps.
# We append a trailing slash to SOURCE_DIR to ensure rg searches the directory properly
while IFS= read -r ts; do
    rg -l -F "$ts" "$SOURCE_DIR/"
done < "$PATTERN_FILE" | awk '!seen[$0]++' | xargs cat

# 3. Cleanup
rm -f "$PATTERN_FILE"
