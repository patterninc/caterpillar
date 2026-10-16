#!/usr/bin/env bash
# Trigger: PreToolUse Bash
# Purpose: Before running ./caterpillar or .claude/scripts/run-pipeline.sh, check:
#          1. Binary is built
#          2. Pipeline file exists
#          3. AWS account is sandbox (BLOCK if not)
#          4. Pipeline has no non-sandbox resources (BLOCK if found)
#          5. All {{ env "VAR" }} references are set
#          6. AWS credentials present if pipeline uses AWS tasks
# Exit 2 to BLOCK the run.

set -euo pipefail

INPUT=$(cat)

# Only intercept caterpillar/run-pipeline commands
COMMAND=$(echo "$INPUT" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('tool_input', {}).get('command', ''))" 2>/dev/null || echo "")

if [[ "$COMMAND" != *"./caterpillar -conf"* ]] && [[ "$COMMAND" != *"caterpillar -conf"* ]] && [[ "$COMMAND" != *"run-pipeline.sh"* ]]; then
  exit 0
fi

# Extract pipeline file path
PIPELINE_FILE=$(echo "$COMMAND" | grep -oE '(\-conf\s+|run-pipeline\.sh\s+)\S+' | awk '{print $NF}')

echo "--- Preflight Check: $PIPELINE_FILE ---"

ERRORS=0
BLOCKED=false

# ── 1. Binary check ──────────────────────────────────────────────

if [ ! -f "./caterpillar" ]; then
  echo "ERROR binary ./caterpillar not found — run: go build -o caterpillar cmd/caterpillar/caterpillar.go"
  ERRORS=$((ERRORS + 1))
else
  echo "OK    binary exists"
fi

# ── 2. Pipeline file check ───────────────────────────────────────

if [ -z "$PIPELINE_FILE" ]; then
  echo "ERROR could not parse pipeline file from command: $COMMAND"
  exit 2
fi

if [ ! -f "$PIPELINE_FILE" ]; then
  echo "ERROR pipeline file not found: $PIPELINE_FILE"
  ERRORS=$((ERRORS + 1))
else
  echo "OK    pipeline file exists: $PIPELINE_FILE"
fi

# ── 3. Sandbox account check (BLOCKING) ─────────────────────────

if command -v aws &>/dev/null; then
  ACCOUNT_ALIAS=$(aws iam list-account-aliases --query 'AccountAliases[0]' --output text 2>/dev/null || echo "NONE")
  ACCOUNT_ID=$(aws sts get-caller-identity --query 'Account' --output text 2>/dev/null || echo "UNKNOWN")

  SANDBOX_PATTERNS="sandbox|dev|test|staging|nonprod"

  ALIAS_OK=false
  ID_OK=false

  if echo "$ACCOUNT_ALIAS" | grep -qiE "$SANDBOX_PATTERNS"; then
    ALIAS_OK=true
  fi
  if echo "$ACCOUNT_ID" | grep -qiE "$SANDBOX_PATTERNS"; then
    ID_OK=true
  fi

  if [ "$ALIAS_OK" = true ]; then
    echo "OK    sandbox account: $ACCOUNT_ALIAS ($ACCOUNT_ID)"
  elif [ "$ACCOUNT_ALIAS" = "NONE" ] && [ "$ACCOUNT_ID" = "UNKNOWN" ]; then
    echo "WARN  could not determine AWS account — no credentials or no access to IAM"
  else
    echo "BLOCK account '$ACCOUNT_ALIAS' ($ACCOUNT_ID) is NOT sandbox"
    echo "      Only sandbox/dev/test/staging accounts are allowed for pipeline execution"
    echo "      Switch account: export AWS_PROFILE=<sandbox-profile>"
    BLOCKED=true
  fi
else
  echo "WARN  aws CLI not installed — cannot verify sandbox account"
fi

# ── 4. Non-sandbox resource check (BLOCKING) ────────────────────

if [ -f "$PIPELINE_FILE" ]; then
  NON_SANDBOX_RESOURCES=()

  python3 -c "
import sys, yaml, re

SANDBOX_RE = re.compile(r'(sandbox|dev|test|staging|nonprod)', re.IGNORECASE)

RESOURCE_FIELDS = {
    'queue_url', 'topic_arn', 'bootstrap_server', 'endpoint',
}

# Fields where s3:// paths live
PATH_FIELDS = {'path'}

with open('$PIPELINE_FILE') as f:
    data = yaml.safe_load(f)

if not isinstance(data, dict) or 'tasks' not in data:
    sys.exit(0)

flagged = []
for i, task in enumerate(data.get('tasks', [])):
    name = task.get('name', f'task#{i+1}')
    ttype = task.get('type', '')

    for field in RESOURCE_FIELDS:
        val = task.get(field, '')
        if not val or not isinstance(val, str):
            continue
        # Skip template-only values — can't evaluate at scan time
        if val.strip().startswith('{{') and val.strip().endswith('}}'):
            continue
        if not SANDBOX_RE.search(val):
            flagged.append(f'  task \"{name}\" → {field}: {val}')

    # Check s3:// paths
    for field in PATH_FIELDS:
        val = task.get(field, '')
        if not val or not isinstance(val, str):
            continue
        if val.startswith('s3://'):
            if val.strip().startswith('{{'):
                continue
            if not SANDBOX_RE.search(val):
                flagged.append(f'  task \"{name}\" → {field}: {val}')

    # Check secret paths for /prod/ prefix
    for field, val in task.items():
        if isinstance(val, str) and '{{ secret' in val:
            if '/prod/' in val and not SANDBOX_RE.search(val):
                flagged.append(f'  task \"{name}\" → {field}: {val} (prod SSM path)')

if flagged:
    print('NON_SANDBOX_FOUND')
    for f in flagged:
        print(f)
else:
    print('ALL_SANDBOX')
" 2>/dev/null | {
    FIRST_LINE=true
    while IFS= read -r line; do
      if [ "$FIRST_LINE" = true ]; then
        FIRST_LINE=false
        if [ "$line" = "NON_SANDBOX_FOUND" ]; then
          echo "BLOCK non-sandbox resources detected in pipeline:"
          BLOCKED=true
        else
          echo "OK    all resources appear to be sandbox"
        fi
      else
        echo "$line"
      fi
    done
  }

  if [ "$BLOCKED" = true ]; then
    echo ""
    echo "      Use mock data instead: ask user for sample input, replace source with local file, sink with echo"
  fi
fi

# ── 5. Env var check ─────────────────────────────────────────────

if [ -f "$PIPELINE_FILE" ] && [ "$BLOCKED" = false ]; then
  MISSING_VARS=()
  ENV_VARS=$(grep -oE '\{\{ env "([^"]+)" \}\}' "$PIPELINE_FILE" | grep -oE '"[^"]+"' | tr -d '"' | sort -u 2>/dev/null || true)

  for VAR in $ENV_VARS; do
    if [ -z "${!VAR:-}" ]; then
      MISSING_VARS+=("$VAR")
    fi
  done

  if [ ${#MISSING_VARS[@]} -gt 0 ]; then
    echo "WARN  env vars referenced but not set:"
    for VAR in "${MISSING_VARS[@]}"; do
      echo "        export $VAR=<value>"
    done
  elif [ -n "$ENV_VARS" ]; then
    echo "OK    all env vars set"
  fi
fi

# ── 6. AWS credentials check ─────────────────────────────────────

if [ -f "$PIPELINE_FILE" ] && [ "$BLOCKED" = false ]; then
  AWS_TASKS=$(grep -E 'type:\s*(sqs|sns|aws_parameter_store|file)' "$PIPELINE_FILE" || true)
  S3_PATHS=$(grep -E 'path:\s*.*s3://' "$PIPELINE_FILE" || true)
  SECRET_REFS=$(grep -oE '\{\{ secret "[^"]+" \}\}' "$PIPELINE_FILE" || true)

  if [ -n "$AWS_TASKS" ] || [ -n "$S3_PATHS" ] || [ -n "$SECRET_REFS" ]; then
    if [ -z "${AWS_ACCESS_KEY_ID:-}" ] && [ -z "${AWS_PROFILE:-}" ]; then
      if [ ! -f "$HOME/.aws/credentials" ] && [ ! -f "$HOME/.aws/config" ]; then
        echo "WARN  pipeline uses AWS but no credentials found"
      fi
    else
      echo "OK    AWS credentials present"
    fi

    if [ -z "${AWS_REGION:-}" ] && [ -z "${AWS_DEFAULT_REGION:-}" ]; then
      echo "WARN  AWS_REGION not set — defaults to us-west-2"
    fi
  fi
fi

# ── Verdict ──────────────────────────────────────────────────────

echo ""
if [ "$BLOCKED" = true ]; then
  echo "BLOCKED — non-sandbox environment or resources detected. Pipeline will not run."
  echo "         Generate a mock test pipeline instead."
  exit 2
elif [ $ERRORS -gt 0 ]; then
  echo "BLOCKED — $ERRORS preflight error(s). Fix before running."
  exit 2
else
  echo "Preflight passed — running pipeline..."
fi

exit 0
