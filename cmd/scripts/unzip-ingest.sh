#!/bin/bash

DIR="$1"
DATE="$2"
FORCE_STR="$3"

if [ -z "$DIR" ] || [ -z "$DATE" ]; then
    echo "Usage: $0 <dir> <date> <true|false>"
    exit 1
fi

FORCE=0
if [ "$FORCE_STR" = "true" ]; then
    FORCE=1
fi

echo "--- Extracting Takeout and ChatGPT zips ---"

# --- Process Gemini Zips ---
GEMINI_TARGET="$DIR/gemini/$DATE-export"

if [ -d "$GEMINI_TARGET" ] && [ "$FORCE" -eq 0 ]; then
    echo "[SKIP] Gemini directory already exists: $GEMINI_TARGET"
    echo "       (Use -f to wipe and force overwrite)"
else
    if [ "$FORCE" -eq 1 ]; then
        echo "[WIPE] Removing existing Gemini directory..."
        rm -rf "$GEMINI_TARGET"
    fi
    
    for zip in "$DIR"/gemini/*.zip; do
        if [ -f "$zip" ]; then
            mkdir -p "$GEMINI_TARGET"
            echo "[EXTRACT] Gemini zip -> $GEMINI_TARGET"
            7zz x "$zip" -o"$GEMINI_TARGET" -y > /dev/null 2>&1
            
            # Clean up Google's annoying Takeout/ wrapper
            if [ -d "$GEMINI_TARGET/Takeout" ]; then
                mv "$GEMINI_TARGET/Takeout/"* "$GEMINI_TARGET/" 2>/dev/null || true
                rm -rf "$GEMINI_TARGET/Takeout"
            fi
        fi
    done
    
    if [ -d "$GEMINI_TARGET" ]; then
        echo "[CREATED] Gemini export directory: $GEMINI_TARGET"
    fi
fi

echo "-------------------------------------------"

# --- Process ChatGPT Zips ---
CHATGPT_TARGET="$DIR/chatgpt/$DATE-export"

if [ -d "$CHATGPT_TARGET" ] && [ "$FORCE" -eq 0 ]; then
    echo "[SKIP] ChatGPT directory already exists: $CHATGPT_TARGET"
    echo "       (Use -f to wipe and force overwrite)"
else
    if [ "$FORCE" -eq 1 ]; then
        echo "[WIPE] Removing existing ChatGPT directory..."
        rm -rf "$CHATGPT_TARGET"
    fi
    
    for zip in "$DIR"/chatgpt/*.zip; do
        if [ -f "$zip" ]; then
            mkdir -p "$CHATGPT_TARGET"
            echo "[EXTRACT] ChatGPT zip -> $CHATGPT_TARGET"
            7zz x "$zip" -o"$CHATGPT_TARGET" -y > /dev/null 2>&1
        fi
    done
    
    if [ -d "$CHATGPT_TARGET" ]; then
        echo "[CREATED] ChatGPT export directory: $CHATGPT_TARGET"
    fi
fi

echo "[✓] Unzip pipeline complete."
