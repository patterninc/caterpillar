#!/usr/bin/env bash
# Fails if go.mod lists a denied module as a direct dependency, or if the
# module cache does not match go.sum.
set -euo pipefail

cd "$(dirname "$0")/.."

policy=.github/dependency-policy.txt
direct="$(go list -m -f '{{if not .Indirect}}{{if not .Main}}{{.Path}}{{end}}{{end}}' all)"

status=0
while read -r line; do
  module="${line%%#*}"
  module="${module// /}"
  [ -z "$module" ] && continue
  reason="${line#*# }"
  if printf '%s\n' "$direct" | grep -qx "$module"; then
    echo "denied direct dependency: $module ($reason)"
    status=1
  fi
done < "$policy"

go mod verify

exit "$status"
