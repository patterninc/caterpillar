#!/usr/bin/env bash
# Fetch one sample from a pipeline source and print an inferred JSON schema.
# Usage: .claude/scripts/check-source-schema.sh <subcommand> [args]
#
# Subcommands:
#   http <url> [--method GET|POST] [--header 'K: V']... [--data 'body'|@file] [--bearer TOKEN] [--max-time SEC]
#   s3 <s3://bucket/key> --region REGION [--lines N]   (first N lines; default 1 for NDJSON)
#   sqs <queue_url> --region REGION
#   file <path> [--csv]
#   ssm <parameter_name> --region REGION
#   ssm-path <path_prefix> --region REGION   (first parameter value, get-parameters-by-path)
#   kafka --broker HOST:PORT --topic TOPIC [-- ...extra kcat -X args]
#   stdin [--label TEXT]   (read payload from pipe; use after curl/aws yourself)
#
# Global options (any position):
#   --no-schema   only print fetched/raw body (no inferred table)
#   --raw-only    same as --no-schema
#   -h, --help    show this header

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPORTER="$SCRIPT_DIR/lib/source_schema_report.py"

usage() {
  sed -n '1,25p' "$0" | tail -n +2
  exit "${1:-1}"
}

run_reporter() {
  local label="$1"
  if [[ "${NO_SCHEMA:-0}" == "1" ]]; then
    cat
    return
  fi
  python3 "$REPORTER" --label "$label"
}

NO_SCHEMA=0
WANT_HELP=0
METHOD="GET"
MAX_TIME="10"
HEADERS=()
DATA=""
BEARER=""
REGION=""
LINES="1"
KAFKA_BROKER=""
KAFKA_TOPIC=()
CSV_MODE=0
LABEL_OVERRIDE=""

# Strip global flags from any position (remaining order preserved)
ARGS=()
for a in "$@"; do
  case "$a" in
    -h|--help) WANT_HELP=1 ;;
    --no-schema|--raw-only) NO_SCHEMA=1 ;;
    *) ARGS+=("$a") ;;
  esac
done
if [[ ${#ARGS[@]} -gt 0 ]]; then
  set -- "${ARGS[@]}"
else
  set --
fi

if [[ $# -lt 1 ]]; then
  [[ "$WANT_HELP" == 1 ]] && usage 0 || usage 1
fi

SUB="$1"
shift

curl_http() {
  local url="$1"
  local -a cmd=(curl -sS --max-time "$MAX_TIME" -X "$METHOD")
  local h
  for h in "${HEADERS[@]}"; do
    cmd+=(-H "$h")
  done
  if [[ -n "$BEARER" ]]; then
    cmd+=(-H "Authorization: Bearer ${BEARER}")
  fi
  if [[ -n "$DATA" ]]; then
    if [[ "$DATA" == @* ]]; then
      cmd+=(-H "Content-Type: application/json" --data-binary "${DATA#@}")
    else
      cmd+=(-H "Content-Type: application/json" --data "$DATA")
    fi
  fi
  cmd+=("$url")
  "${cmd[@]}"
}

case "$SUB" in
  http)
    [[ $# -ge 1 ]] || usage
    URL="$1"
    shift
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --method) METHOD="$2"; shift 2 ;;
        -X) METHOD="$2"; shift 2 ;;
        --header|-H) HEADERS+=("$2"); shift 2 ;;
        --data|-d) DATA="$2"; shift 2 ;;
        --bearer) BEARER="$2"; shift 2 ;;
        --max-time) MAX_TIME="$2"; shift 2 ;;
        *) echo "Unknown http option: $1" >&2; usage ;;
      esac
    done
    echo "Fetching: $METHOD $URL" >&2
    curl_http "$URL" | run_reporter "$URL"
    ;;

  s3)
    [[ $# -ge 1 ]] || usage
    S3_URI="$1"
    shift
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --region) REGION="$2"; shift 2 ;;
        --lines) LINES="$2"; shift 2 ;;
        *) echo "Unknown s3 option: $1" >&2; usage ;;
      esac
    done
    [[ -n "$REGION" ]] || { echo "s3: --region required" >&2; exit 1; }
    echo "Reading first $LINES line(s) from $S3_URI (region $REGION)" >&2
    aws s3 cp "$S3_URI" - --region "$REGION" | head -n "$LINES" | run_reporter "$S3_URI"
    ;;

  sqs)
    [[ $# -ge 1 ]] || usage
    QUEUE="$1"
    shift
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --region) REGION="$2"; shift 2 ;;
        *) echo "Unknown sqs option: $1" >&2; usage ;;
      esac
    done
    [[ -n "$REGION" ]] || { echo "sqs: --region required" >&2; exit 1; }
    echo "Peeking 1 message (visibility 0): $QUEUE" >&2
    RAW=$(mktemp)
    BODY_OUT=$(mktemp)
    trap 'rm -f "$RAW" "$BODY_OUT"' EXIT
    aws sqs receive-message \
      --queue-url "$QUEUE" \
      --max-number-of-messages 1 \
      --visibility-timeout 0 \
      --region "$REGION" \
      --output json >"$RAW" || exit $?
    python3 -c "
import json, sys
with open(sys.argv[1], encoding='utf-8') as f:
    d = json.load(f)
msgs = d.get('Messages') or []
if not msgs:
    sys.stderr.write('Queue empty or no messages available.\\n')
    sys.exit(2)
with open(sys.argv[2], 'w', encoding='utf-8') as w:
    w.write(msgs[0].get('Body', ''))
" "$RAW" "$BODY_OUT" || exit $?
    run_reporter "$QUEUE" <"$BODY_OUT"
    ;;

  file)
    [[ $# -ge 1 ]] || usage
    FILE_PATH="$1"
    shift
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --csv) CSV_MODE=1; shift ;;
        *) echo "Unknown file option: $1" >&2; usage ;;
      esac
    done
    [[ -f "$FILE_PATH" ]] || { echo "file not found: $FILE_PATH" >&2; exit 1; }
    if [[ "$CSV_MODE" == 1 ]]; then
      if [[ "$NO_SCHEMA" == 1 ]]; then
        head -n 2 "$FILE_PATH"
      else
        python3 "$REPORTER" csv-file "$FILE_PATH"
      fi
    else
      head -n 1 "$FILE_PATH" | run_reporter "$FILE_PATH"
    fi
    ;;

  ssm)
    [[ $# -ge 1 ]] || usage
    PARAM="$1"
    shift
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --region) REGION="$2"; shift 2 ;;
        *) echo "Unknown ssm option: $1" >&2; usage ;;
      esac
    done
    [[ -n "$REGION" ]] || { echo "ssm: --region required" >&2; exit 1; }
    echo "get-parameter: $PARAM" >&2
    aws ssm get-parameter --name "$PARAM" --with-decryption --region "$REGION" --output json |
      python3 -c "
import json, sys
d = json.load(sys.stdin)
v = d['Parameter']['Value']
sys.stdout.write(v)
if not v.endswith('\n'):
    sys.stdout.write('\n')
" | run_reporter "$PARAM"
    ;;

  ssm-path)
    [[ $# -ge 1 ]] || usage
    PREFIX="$1"
    shift
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --region) REGION="$2"; shift 2 ;;
        *) echo "Unknown ssm-path option: $1" >&2; usage ;;
      esac
    done
    [[ -n "$REGION" ]] || { echo "ssm-path: --region required" >&2; exit 1; }
    echo "get-parameters-by-path: $PREFIX (first value)" >&2
    aws ssm get-parameters-by-path --path "$PREFIX" --recursive --with-decryption --region "$REGION" --output json |
      python3 -c "
import json, sys
d = json.load(sys.stdin)
params = d.get('Parameters') or []
if not params:
    sys.stderr.write('No parameters under path.\\n')
    sys.exit(2)
p = params[0]
name, val = p['Name'], p.get('Value') or ''
sys.stderr.write(f'Sample parameter: {name}\\n')
sys.stdout.write(val)
if val and not val.endswith('\n'):
    sys.stdout.write('\n')
" | run_reporter "$PREFIX"
    ;;

  kafka)
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --broker|-b) KAFKA_BROKER="$2"; shift 2 ;;
        --topic|-t) KAFKA_TOPIC=(-t "$2"); shift 2 ;;
        --) shift; break ;;
        *) break ;;
      esac
    done
    [[ -n "$KAFKA_BROKER" && ${#KAFKA_TOPIC[@]} -eq 2 ]] || { echo "kafka: --broker and --topic required" >&2; exit 1; }
    if ! command -v kcat >/dev/null 2>&1; then
      echo "kcat not found. Install kcat or use a caterpillar probe pipeline." >&2
      exit 1
    fi
    echo "Consuming 1 message from ${KAFKA_TOPIC[1]} @ $KAFKA_BROKER" >&2
    # Remaining args passed to kcat (e.g. -X security.protocol=SASL_SSL ...)
    kcat -b "$KAFKA_BROKER" "${KAFKA_TOPIC[@]}" -C -c 1 -e -f '%s\n' "$@" 2>/dev/null | run_reporter "kafka:${KAFKA_TOPIC[1]}"
    ;;

  stdin)
    while [[ $# -gt 0 ]]; do
      case "$1" in
        --label) LABEL_OVERRIDE="$2"; shift 2 ;;
        *) echo "Unknown stdin option: $1" >&2; usage ;;
      esac
    done
    run_reporter "${LABEL_OVERRIDE:-stdin}"
    ;;

  *)
    echo "Unknown subcommand: $SUB" >&2
    usage
    ;;
esac
