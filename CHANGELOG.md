# Changelog

All notable changes to caterpillar. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versions follow [Semantic Versioning](https://semver.org/). Changes that need action from pipeline authors or consumers have a **Migration** note.

Add entries under **Unreleased** in the same PR as the change. When cutting a release, move them under a version heading (see `docs/runbooks/release-and-rollback.md`). Releases before 2.26.0 are listed only in [GitHub Releases](https://github.com/patterninc/caterpillar/releases).

## [Unreleased]

### Added
- Documentation: `AGENTS.md`, `CHANGELOG.md`, ADRs, runbooks, on-call playbook, secrets guide, agent skills and the agent-dispatch manifest. No change to the binary.

## [2.33.0] - 2026-11-17

### Added
- `http`: `auth` fetches a short-lived access token, caches it, and sets it on every request including `next_page` pages. It cannot be combined with `oauth` (#140).

## [2.32.3] - 2026-11-12

### Fixed
- `xpath`: expressions are pre-compiled, and union (`|`) results are returned in DOM document order (#138).

### Removed
- `backstage.yaml` removed from this public repository (#137).

## [2.32.2] - 2026-10-28

### Added
- linux/arm64 release binary `caterpillar-linux-arm64` and a multi-arch `patternoss/caterpillar` image; CI runs tests on pull requests (#134).

### Changed
- Building requires Go 1.27 (#133).
  - **Migration:** build from source with Go 1.27 or later. Prebuilt binaries and images are unaffected.

## [2.32.1] - 2026-10-22

### Fixed
- Concurrent workers of one task no longer serialise on `Base.SendData` (#132).

## [2.32.0] - 2026-09-30

### Added
- `file`: matched files are read concurrently (#128).

## [2.31.0] - 2026-09-23

### Added
- `sqs`: `max_records` stops read mode after that many records (#125).

### Fixed
- `kafka`: deferred offset stores no longer queue behind heartbeat polls (#129).

## [2.30.0] - 2026-09-16

### Added
- `sqs`: `delivery` field for read mode: `at-least-once` (default, delete after downstream finishes) or `at-most-once` (delete on receive) (#124).

## [2.29.0] - 2026-09-11

### Added
- `http`: `ignore_error: true` skips records whose request fails after retries (#121).

### Fixed
- `sqs`: FIFO drain is confirmed with queue attributes before `exit_on_empty` ends the read (#123).

## [2.28.3] - 2026-09-10

### Changed
- `file`: a `file` task used as a source always runs with one worker (#118).
  - **Migration:** `task_concurrency` above 1 on a source `file` task is now ignored; use the concurrent reads added in 2.32.0 instead.

## [2.28.2] - 2026-09-08

### Fixed
- `sqs`: empty FIFO receives are not treated as drained while acks are deferred (#116).
- `kafka`: the group consumer keeps polling while `Finish` waits on deferred acks (#115).

## [2.28.1] - 2026-09-04

### Fixed
- `kafka`: uses the cooperative-sticky assignor and suppresses `ErrState` warnings (#113).

## [2.28.0] - 2026-09-03

### Changed
- `kafka`: group offset commits are deferred until downstream tasks finish with the record (#112).
  - **Migration:** delivery from Kafka group reads is now at-least-once; make sinks idempotent if duplicates after a restart matter.

## [2.27.0] - 2026-08-24

### Added
- Acknowledgement tracking for pipeline records (`internal/pkg/pipeline/ack`), so sources can defer acknowledging a message until every downstream branch has finished (#79).

## [2.26.0] - 2026-08-21

### Added
- `heimdall`: `skip_on_error: true` skips records whose job fails in destination mode (#109).

[Unreleased]: https://github.com/patterninc/caterpillar/compare/v2.33.0...HEAD
[2.33.0]: https://github.com/patterninc/caterpillar/compare/v2.32.3...v2.33.0
[2.32.3]: https://github.com/patterninc/caterpillar/compare/v2.32.2...v2.32.3
[2.32.2]: https://github.com/patterninc/caterpillar/compare/v2.32.1...v2.32.2
[2.32.1]: https://github.com/patterninc/caterpillar/compare/v2.32.0...v2.32.1
[2.32.0]: https://github.com/patterninc/caterpillar/compare/v2.31.0...v2.32.0
[2.31.0]: https://github.com/patterninc/caterpillar/compare/v2.30.0...v2.31.0
[2.30.0]: https://github.com/patterninc/caterpillar/compare/v2.29.0...v2.30.0
[2.29.0]: https://github.com/patterninc/caterpillar/compare/v2.28.3...v2.29.0
[2.28.3]: https://github.com/patterninc/caterpillar/compare/v2.28.2...v2.28.3
[2.28.2]: https://github.com/patterninc/caterpillar/compare/v2.28.1...v2.28.2
[2.28.1]: https://github.com/patterninc/caterpillar/compare/v2.28.0...v2.28.1
[2.28.0]: https://github.com/patterninc/caterpillar/compare/v2.27.0...v2.28.0
[2.27.0]: https://github.com/patterninc/caterpillar/compare/v2.26.0...v2.27.0
[2.26.0]: https://github.com/patterninc/caterpillar/compare/v2.25.0...v2.26.0
