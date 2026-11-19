#!/usr/bin/env bash
# One-command local setup: checks the toolchain, downloads modules, builds
# ./caterpillar, runs the unit tests and a sample pipeline, and installs the
# pre-commit hooks when pre-commit is available.
set -euo pipefail

cd "$(dirname "$0")/.."

want="$(awk '/^go /{print $2}' go.mod)"
if ! command -v go >/dev/null; then
  echo "dev: Go $want or later is required (https://go.dev/dl/)" >&2
  exit 1
fi

if ! command -v cc >/dev/null && ! command -v gcc >/dev/null && ! command -v clang >/dev/null; then
  echo "dev: a C compiler is required; confluent-kafka-go builds with cgo" >&2
  echo "     macOS: xcode-select --install   Debian/Ubuntu: apt-get install build-essential" >&2
  exit 1
fi

go mod download
go build -o caterpillar ./cmd/caterpillar/caterpillar.go
go test ./...
scripts/smoke.sh ./caterpillar

if command -v pre-commit >/dev/null; then
  pre-commit install --install-hooks
  pre-commit install --hook-type commit-msg
else
  echo "dev: pre-commit not found; install it (pipx install pre-commit) to run the hooks on commit"
fi

echo "dev: ready. Run a pipeline with ./caterpillar -conf test/pipelines/<file>.yaml"
