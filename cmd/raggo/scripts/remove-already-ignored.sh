#!/bin/bash
# ai-tools/scripts/remove-already-ignored.sh
# Navigate to project root
cd "$(dirname "$0")/../.." || exit 1

echo "Sweeping for previously ignored attachments..."

prune_service() {
    local service=$1
    
    # Find all 'ignore' directories for this service
    local ignore_dirs=$(find "ingest/$service" -type d -name "ignore")
    
    if [ -n "$ignore_dirs" ]; then
        echo "[$service] Pruning duplicate attachments based on ignore/ folders..."
        # jdupes explanation:
        # -r : recursive
        # -d : delete duplicates
        # -N : no prompt (automatically keep the first file found)
        # -O : respect parameter order (files in ignore_dirs are kept because they are listed first)
        jdupes -r -d -N -O $ignore_dirs "ingest/$service/"
    else
        echo "[$service] No ignore/ directories found yet. Skipping prune."
    fi
}

prune_service "chatgpt"
prune_service "gemini"

echo "Pruning complete!"
