#!/usr/bin/env bash
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

OUT="repo-dump.md"
EXCLUDE_PATTERN='(\.git|venv|node_modules|__pycache__|\.DS_Store|'"$OUT"')'

echo "Dumping repository context to $OUT..."

{
  echo "# Repository Dump - $(date -u)"
  echo
  echo "## File Listing"
  echo '```text'
  find . -maxdepth 4 -not -path '*/.*' | grep -Ev "$EXCLUDE_PATTERN"
  echo '```'
  echo

  if [ -f "design.md" ]; then
    echo "## design.md"
    echo '```markdown'
    cat design.md
    echo '```'
    echo
  fi

  # Find and include source files
  find . -type f \( -name "*.sh" -o -name "*.go"  \) \
    | grep -Ev "$EXCLUDE_PATTERN" \
    | sort | while read -r file; do
      echo "## File: $file"
      case "$file" in
        *.sh) echo '```bash' ;;
        *.go) echo '```go' ;;
        *.py) echo '```python' ;;
        *)    echo '```text' ;;
      esac
      cat "$file"
      echo '```'
      echo
    done
} > "$OUT"

echo "Done. Run 'cat $OUT' to see the results."
