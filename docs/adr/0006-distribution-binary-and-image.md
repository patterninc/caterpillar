# 0006. Ship as release binaries and a multi-arch Docker image, built with cgo

- Status: Accepted
- Date: 2026-11-19 (retroactive; Docker publishing added in #48 on 2026-03-16, arm64 in #134 on 2026-10-27)

## Context

Caterpillar is run by other systems (for example `patterninc/caterpillar-services`, which builds its image on top of a caterpillar image), not deployed from this repo. The Kafka task depends on `confluent-kafka-go`, which needs librdkafka through cgo.

## Decision

`.github/workflows/release.yaml` builds natively on amd64 and arm64 runners with `CGO_ENABLED=1`. Pushes of `v*` tags attach `caterpillar` and `caterpillar-linux-arm64` to a GitHub release. The same pushes, `main`, and the `latest` tag publish `patternoss/caterpillar` (`<version>`, `dev`, `latest`) to Docker Hub, built from `build/Dockerfile` against Alpine edge librdkafka (2.14.0+ is required). Docker publishing is skipped with a warning when the Docker Hub secrets are absent.

## Consequences

- No cross-compiling; each architecture needs its own runner.
- Asset names are a contract with downstream images and must not change.
- "Deploy" for this repo means publishing; `smoke.yml` checks the published binaries and image after each release run.
- Local builds need a C compiler (`scripts/dev.sh` checks for one).
