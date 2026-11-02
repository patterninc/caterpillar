#!/usr/bin/env bash
# Trigger: PostToolUse Bash
# Purpose: After ./caterpillar or .claude/scripts/run-pipeline.sh runs, report:
#          status, record count, errors, suggestions, JSON output validation.

set -euo pipefail

INPUT=$(cat)

COMMAND=$(echo "$INPUT" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('tool_input', {}).get('command', ''))" 2>/dev/null || echo "")

if [[ "$COMMAND" != *"./caterpillar -conf"* ]] && [[ "$COMMAND" != *"caterpillar -conf"* ]] && [[ "$COMMAND" != *"run-pipeline.sh"* ]]; then
  exit 0
fi

OUTPUT=$(echo "$INPUT" | python3 -c "
import sys, json
d = json.load(sys.stdin)
result = d.get('tool_response', {})
if isinstance(result, str):
    print(result)
elif isinstance(result, dict):
    print(result.get('output', result.get('stdout', '')))
" 2>/dev/null || echo "")

EXIT_CODE=$(echo "$INPUT" | python3 -c "
import sys, json
d = json.load(sys.stdin)
result = d.get('tool_response', {})
if isinstance(result, dict):
    print(result.get('exit_code', result.get('returncode', 0)))
else:
    print(0)
" 2>/dev/null || echo "0")

PIPELINE_FILE=$(echo "$COMMAND" | grep -oE '(\-conf\s+|run-pipeline\.sh\s+)\S+' | awk '{print $NF}')

echo "--- Run Summary: $PIPELINE_FILE ---"

# ── Status ───────────────────────────────────────────────────────

if [ "$EXIT_CODE" = "0" ]; then
  echo "STATUS  success (exit 0)"
else
  echo "STATUS  FAILED (exit $EXIT_CODE)"
fi

# ── Record count ─────────────────────────────────────────────────

if [ -n "$OUTPUT" ]; then
  RECORD_COUNT=$(echo "$OUTPUT" | grep -v "^---" | grep -v "^error" | grep -v "^Task" | grep -v "^pipeline" | grep -v "^$" | grep -v "^Preflight" | grep -v "^OK" | grep -v "^nothing" | grep -v "^BLOCK" | grep -v "^STATUS" | grep -v "^WARN" | wc -l | tr -d ' ')
  if [ "$RECORD_COUNT" -gt "0" ]; then
    echo "RECORDS $RECORD_COUNT record(s) output"
  fi
fi

# ── Errors ───────────────────────────────────────────────────────

NON_FATAL=$(echo "$OUTPUT" | grep -E "^error in " || true)
if [ -n "$NON_FATAL" ]; then
  echo ""
  echo "NON-FATAL ERRORS:"
  echo "$NON_FATAL" | while IFS= read -r line; do
    echo "  $line"
  done
fi

FATAL=$(echo "$OUTPUT" | grep -E "Task '.+' failed with error:" || true)
if [ -n "$FATAL" ]; then
  echo ""
  echo "FATAL ERRORS:"
  echo "$FATAL" | while IFS= read -r line; do
    echo "  $line"
  done
fi

# ── Suggestions ──────────────────────────────────────────────────

echo "$OUTPUT" | python3 -c "
import sys, re

output = sys.stdin.read()
suggestions = []

patterns = {
    'task type is not supported':     'Fix task type — check hyphens vs underscores',
    'failed to initialize task':      'Init failure — check AWS credentials, region, SSM paths',
    'task not found':                 'DAG references a task name not in tasks:',
    'context keys were not set':      'Add context: { key: \".jq\" } to upstream task',
    'malformed context template':     'Fix {{ context \"key\" }} syntax',
    'macro .* is not defined':        'Valid macros: timestamp, uuid, unixtime, microtimestamp',
    'nothing to do':                  'tasks: list is empty',
    'invalid DAG groups':             'Fix DAG syntax',
    'connection refused':             'Cannot reach host — check server/endpoint/queue_url',
    'NoCredentialProviders':          'No AWS credentials — set AWS_ACCESS_KEY_ID or use IAM role',
    'AccessDenied':                   'IAM permissions insufficient — run pipeline-permissions agent',
    'ResourceNotFoundException':      'SSM parameter path not found',
    'batch_flush_interval':           'batch_flush_interval must be < timeout for kafka write',
}

for pattern, suggestion in patterns.items():
    if re.search(pattern, output, re.IGNORECASE):
        suggestions.append(suggestion)

if suggestions:
    print('')
    print('SUGGESTIONS:')
    for s in suggestions:
        print(f'  -> {s}')
" 2>/dev/null || true

# ── JSON output validation ───────────────────────────────────────

if [ "$EXIT_CODE" = "0" ] && [ -f "$PIPELINE_FILE" ]; then
  JSON_SINKS=$(grep -E 'path:.*\.json' "$PIPELINE_FILE" | grep -v 's3://' | grep -oE "'[^']+'" | tr -d "'" | grep -v 'http' || true)

  if [ -n "$JSON_SINKS" ]; then
    echo ""
    echo "JSON OUTPUT:"
    for SINK_PATH in $JSON_SINKS; do
      BASE_DIR=$(dirname "$SINK_PATH")
      BASE_NAME=$(basename "$SINK_PATH" | sed 's/{{ macro "[^"]*" }}/.*/g')
      LATEST_FILE=$(ls -t "${BASE_DIR}/"${BASE_NAME} 2>/dev/null | head -1 || true)

      if [ -n "$LATEST_FILE" ] && [ -f "$LATEST_FILE" ]; then
        python3 -c "
import json
path = '$LATEST_FILE'
try:
    with open(path) as f:
        data = json.load(f)
    with open(path, 'w') as f:
        json.dump(data, f, indent=2)
        f.write('\n')
    if isinstance(data, list):
        print(f'OK    {path} — JSON array ({len(data)} records) — pretty-printed')
    else:
        print(f'OK    {path} — JSON object — pretty-printed')
except json.JSONDecodeError as e:
    print(f'ERROR {path} — invalid JSON: {e}')
    print(f'      Tip: use jq [.items[] | {{...}}] to produce a JSON array')
" 2>/dev/null || true
      fi
    done
  fi
fi

# ── Next step ────────────────────────────────────────────────────

echo ""
if [ "$EXIT_CODE" = "0" ]; then
  echo "Next: run pipeline-review before promoting to production"
else
  echo "Next: run pipeline-debugger for diagnosis"
fi

exit 0
