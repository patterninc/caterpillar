#!/usr/bin/env bash
# Fails if any dependency linked into the binary has a license outside the
# allow list. Zlib and BSD-2-Clause-NetBSD come from librdkafka, which
# confluent-kafka-go vendors; both are permissive.
set -euo pipefail

cd "$(dirname "$0")/.."

allowed=Apache-2.0,MIT,BSD-2-Clause,BSD-3-Clause,ISC,MPL-2.0,Zlib,BSD-2-Clause-NetBSD

go-licenses check ./... --allowed_licenses="$allowed" 2> >(grep -v '^W' >&2)
