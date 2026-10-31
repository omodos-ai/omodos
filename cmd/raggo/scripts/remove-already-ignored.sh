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
