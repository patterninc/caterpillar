#!/usr/bin/env bash
# Fails if total statement coverage in a Go cover profile is below the floor.
# The floor sits just under the coverage measured when it was set; raise it
# as tests are added, never lower it to make a change pass.
#
#   scripts/check-coverage.sh coverage.out
set -euo pipefail

profile="${1:-coverage.out}"
floor="${COVERAGE_FLOOR:-40.0}"

total="$(go tool cover -func="$profile" | awk '/^total:/ { sub(/%/, "", $NF); print $NF }')"

echo "coverage: ${total}% (floor ${floor}%)"

if awk -v t="$total" -v f="$floor" 'BEGIN { exit !(t < f) }'; then
  echo "coverage ${total}% is below the ${floor}% floor" >&2
  exit 1
fi
