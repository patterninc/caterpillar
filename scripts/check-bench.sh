#!/usr/bin/env bash
# Reads `benchstat -format csv old new` output and fails if any benchmark's
# sec/op is significantly worse (benchstat reports a delta, not ~) by more
# than the threshold percent.
#
#   scripts/check-bench.sh benchstat.csv 25
set -euo pipefail

csv="$1"
threshold="${2:-25}"

awk -F, -v threshold="$threshold" '
  $0 ~ /sec\/op/ { in_time = 1; next }
  $0 ~ /^,/ && $0 !~ /sec\/op/ { in_time = 0 }
  $0 ~ /^$/ { in_time = 0 }
  in_time && $1 != "" && $1 != "geomean" {
    delta = $6
    if (delta ~ /^\+[0-9.]+%$/) {
      gsub(/[+%]/, "", delta)
      if (delta + 0 > threshold + 0) {
        printf "regression: %s is %s%% slower (threshold %s%%, %s)\n", $1, delta, threshold, $7
        bad = 1
      }
    }
  }
  END { exit bad }
' "$csv"
