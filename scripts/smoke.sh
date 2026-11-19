#!/usr/bin/env bash
# Runs test/pipelines/hello_name.yaml with a built caterpillar and checks the
# output: 10 "Hello, <name>!" lines from the echo task, exit status zero.
#
#   scripts/smoke.sh ./caterpillar
#   scripts/smoke.sh docker:patternoss/caterpillar:dev
set -euo pipefail

cd "$(dirname "$0")/.."

target="${1:-./caterpillar}"
pipeline=test/pipelines/hello_name.yaml

case "$target" in
  docker:*)
    image="${target#docker:}"
    out="$(docker run --rm -v "$PWD/test:/test:ro" -w / "$image" /caterpillar -conf "/$pipeline")"
    ;;
  *)
    out="$("$target" -conf "$pipeline")"
    ;;
esac

echo "$out"

lines="$(printf '%s\n' "$out" | grep -cE ' - echo - Hello, [^!]+!$' || true)"
if [ "$lines" -ne 10 ]; then
  echo "smoke: expected 10 greeting lines from $target, got $lines" >&2
  exit 1
fi

if printf '%s\n' "$out" | grep -q '^error in '; then
  echo "smoke: $target reported a task error" >&2
  exit 1
fi

echo "smoke: ok ($target)"
