#!/usr/bin/env bash
# Trigger: PostToolUse Write|Edit
# Purpose: When a .yaml file is written or edited, validate:
#          1. Valid YAML syntax
#          2. Pipeline structure (tasks key, task types, required fields)
#          3. Hardcoded credentials
#          4. Non-sandbox resource references (warn, don't block)

set -euo pipefail

INPUT=$(cat)

FILE_PATH=$(echo "$INPUT" | python3 -c "import sys, json; d=json.load(sys.stdin); print(d.get('tool_input', {}).get('file_path', ''))" 2>/dev/null || echo "")

# Only process .yaml or .yml files
if [[ "$FILE_PATH" != *.yaml ]] && [[ "$FILE_PATH" != *.yml ]]; then
  exit 0
fi

# Skip non-pipeline files
if [[ "$FILE_PATH" == *".github"* ]] || [[ "$FILE_PATH" == *"settings"* ]]; then
  exit 0
fi

echo "--- Pipeline Validation: $FILE_PATH ---"

python3 -c "
import sys, yaml, re

# ── Config ───────────────────────────────────────────────────────

SUPPORTED_TYPES = {
    'archive', 'aws_parameter_store', 'compress', 'converter', 'delay',
    'echo', 'file', 'flatten', 'heimdall', 'http_server', 'http',
    'join', 'jq', 'kafka', 'replace', 'sample', 'sns', 'split', 'sqs', 'xpath'
}

SOURCE_TYPES = {'file', 'kafka', 'sqs', 'http', 'http_server', 'aws_parameter_store'}

REQUIRED_FIELDS = {
    'file': ['path'], 'kafka': ['bootstrap_server', 'topic'],
    'sqs': ['queue_url'], 'http': ['endpoint'], 'http_server': ['port'],
    'sns': ['topic_arn'], 'aws_parameter_store': ['path'], 'jq': ['path'],
    'xpath': ['expression'], 'compress': ['format'],
    'archive': ['format', 'mode'], 'sample': ['strategy', 'value'],
    'delay': ['duration'], 'join': ['number'],
}

CREDENTIAL_FIELDS = {'password', 'token', 'api_key', 'consumer_secret', 'token_secret'}

RESOURCE_FIELDS = {'queue_url', 'topic_arn', 'bootstrap_server', 'endpoint'}

SANDBOX_RE = re.compile(r'(sandbox|dev|test|staging|nonprod)', re.IGNORECASE)

# ── Parse ────────────────────────────────────────────────────────

try:
    with open('$FILE_PATH') as f:
        data = yaml.safe_load(f)
    if data is None:
        print('WARN  empty file')
        sys.exit(0)
except yaml.YAMLError as e:
    print(f'ERROR invalid YAML syntax: {e}')
    sys.exit(1)

print('OK    YAML syntax valid')

if not isinstance(data, dict) or 'tasks' not in data:
    print('ERROR missing top-level tasks: key')
    sys.exit(1)

tasks = data.get('tasks', [])
if not tasks:
    print('WARN  tasks list is empty')
    sys.exit(0)

errors = []
warnings = []

# ── Validate tasks ───────────────────────────────────────────────

names = []
for i, task in enumerate(tasks):
    pos = i + 1
    name = task.get('name', f'<unnamed #{pos}>')
    ttype = task.get('type', '')

    # Duplicate names
    if name in names:
        errors.append(f'ERROR task #{pos} \"{name}\": duplicate name')
    names.append(name)

    # Missing type
    if not ttype:
        errors.append(f'ERROR task #{pos} \"{name}\": missing type field')
        continue

    # Hyphen instead of underscore
    if ttype not in SUPPORTED_TYPES:
        if ttype.replace('-', '_') in SUPPORTED_TYPES:
            errors.append(f'ERROR task #{pos} \"{name}\": type \"{ttype}\" uses hyphens — use underscores')
        else:
            errors.append(f'ERROR task #{pos} \"{name}\": type \"{ttype}\" is not supported')
        continue

    # First task must be source
    if i == 0 and ttype not in SOURCE_TYPES:
        errors.append(f'ERROR task #1 \"{name}\": type \"{ttype}\" cannot be first — must be a source')

    # Required fields
    for field in REQUIRED_FIELDS.get(ttype, []):
        if field not in task:
            errors.append(f'ERROR task #{pos} \"{name}\" ({ttype}): missing required field \"{field}\"')

    # Hardcoded credentials
    for field in CREDENTIAL_FIELDS:
        val = task.get(field, '')
        if val and isinstance(val, str) and not val.strip().startswith('{{'):
            errors.append(f'ERROR task #{pos} \"{name}\": \"{field}\" appears hardcoded — use {{{{ secret }}}} or {{{{ env }}}}')

    # SQS max_messages
    if ttype == 'sqs' and task.get('max_messages', 0) > 10:
        errors.append(f'ERROR task #{pos} \"{name}\" (sqs): max_messages cannot exceed 10')

    # echo/sns not first
    if ttype in ('echo', 'sns') and i == 0:
        errors.append(f'ERROR task #1 \"{name}\": {ttype} requires upstream — cannot be first')

    # Kafka batch_flush_interval vs timeout
    if ttype == 'kafka' and 'batch_flush_interval' in task and 'timeout' in task:
        warnings.append(f'WARN  task #{pos} \"{name}\" (kafka): verify batch_flush_interval < timeout')

    # ── Non-sandbox resource check ───────────────────────────────

    for field in RESOURCE_FIELDS:
        val = task.get(field, '')
        if not val or not isinstance(val, str):
            continue
        if val.strip().startswith('{{') and val.strip().endswith('}}'):
            continue
        if not SANDBOX_RE.search(val):
            warnings.append(f'WARN  task #{pos} \"{name}\": {field} does not appear to be sandbox — will require mock testing')

    # S3 path check
    path_val = task.get('path', '')
    if isinstance(path_val, str) and path_val.startswith('s3://'):
        if not path_val.strip().startswith('{{') and not SANDBOX_RE.search(path_val):
            warnings.append(f'WARN  task #{pos} \"{name}\": S3 path does not appear to be sandbox — will require mock testing')

    # Prod SSM secret paths
    for field, val in task.items():
        if isinstance(val, str) and '{{ secret' in val and '/prod/' in val:
            warnings.append(f'WARN  task #{pos} \"{name}\": {field} uses /prod/ SSM path — will require mock testing')

# ── Output ───────────────────────────────────────────────────────

for w in warnings:
    print(w)
for e in errors:
    print(e)

if errors:
    print(f'\n{len(errors)} error(s) found — fix before running')
    sys.exit(1)
else:
    print(f'OK    {len(tasks)} tasks valid')
"

EXIT_CODE=$?
if [ $EXIT_CODE -eq 0 ]; then
  echo "OK    pipeline looks good"
else
  echo ""
  echo "Run pipeline-lint agent for a detailed report."
fi

exit 0  # never block the write — just inform
