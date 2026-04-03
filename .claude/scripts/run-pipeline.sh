#!/usr/bin/env bash
# Wrapper to run a caterpillar pipeline and pretty-print any JSON output files.
# Usage: .claude/scripts/run-pipeline.sh <pipeline.yaml>

set -euo pipefail

PIPELINE_FILE="${1:-}"

if [ -z "$PIPELINE_FILE" ]; then
  echo "Usage: .claude/scripts/run-pipeline.sh <pipeline.yaml>"
  exit 1
fi

if [ ! -f "$PIPELINE_FILE" ]; then
  echo "ERROR: pipeline file not found: $PIPELINE_FILE"
  exit 1
fi

# Build binary if missing
if [ ! -f "./caterpillar" ]; then
  echo "Building caterpillar..."
  go build -o caterpillar cmd/caterpillar/caterpillar.go
fi

# Snapshot output dir before run to detect new files
OUTPUT_BEFORE=$(find output/ -name "*.json" 2>/dev/null | sort || true)

# Run pipeline
echo "Running: $PIPELINE_FILE"
./caterpillar -conf "$PIPELINE_FILE"

# Find newly written JSON files
OUTPUT_AFTER=$(find output/ -name "*.json" 2>/dev/null | sort || true)
NEW_FILES=$(comm -13 <(echo "$OUTPUT_BEFORE") <(echo "$OUTPUT_AFTER") || true)

# Pretty-print each new JSON output file
if [ -n "$NEW_FILES" ]; then
  for FILE in $NEW_FILES; do
    python3 -c "
import json
with open('$FILE') as f:
    data = json.load(f)
with open('$FILE', 'w') as f:
    json.dump(data, f, indent=2)
    f.write('\n')
if isinstance(data, list):
    print(f'OK  $FILE  —  {len(data)} records  —  pretty-printed')
else:
    print(f'OK  $FILE  —  pretty-printed')
" 2>/dev/null || echo "WARN: $FILE could not be pretty-printed (not valid JSON)"
  done
fi
