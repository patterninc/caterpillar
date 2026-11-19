# Engineering Best Practices Audit — caterpillar

| | |
|---|---|
| **Audit date** | 2026-11-19 |
| **Auditor** | Claude — gauge-repo skill |
| **Rubric version** | `item-credit-v1` — 2026-09-04 (`references/best-practices.md`) |
| **Audited ref** | branch `docs/gauge-repo-markdown-audit` (PR #95), merged with `main` at `f817f42` |

## Changes since last audit

The previous report (2026-08-20, 40% under the older scoring) was re-baselined against `main` at `f817f42` under `item-credit-v1`: **35.4%** (10 Met, 9 Partial, 22 Gap, 8 N/A), critical gates **RED**. This PR adds documentation only (no code, test, workflow or tooling changes). Items that moved:

- **Gap → Met:** 1 skills, 2 AGENTS.md, 3 ADRs, 8 on-call, 49 dispatch manifest.
- **Partial → Met:** 4 runbooks, 6 README, 7 changelog, 40 scoped secrets.
- **N/A → Partial:** 32 flaky test quarantine. The suite has grown past one test, and one existing test is flaky today (see the item).
- **Inherited under `item-credit-v1`:** 19 and 20 (Pattern Wiz) and 39 (Toolsmith) are Met by org policy.

## Repo profile

Caterpillar is a Go 1.27 CLI (`cmd/caterpillar`) that runs YAML-defined pipelines of tasks connected by Go channels: HTTP, S3, SQS, SNS, Kafka, SFTP, SSM, jq, XPath and converters (21 task types in `internal/pkg/pipeline/tasks.go`). **Artifact type:** CLI tool / data-pipeline runner. **Deployment model:** distributed binary and image. `release.yaml` publishes linux amd64/arm64 binaries to GitHub Releases and the multi-arch image `patternoss/caterpillar` to Docker Hub; it is run by other systems (for example `patterninc/caterpillar-services`). **Persistence:** none of its own. **UI:** none. **AWS footprint:** none deployed from this repo; tasks call AWS APIs with the caller's credentials. **Team:** Data Acquisition (`@patterninc/data-acquisition` in `CODEOWNERS`), many contributors. **Visibility:** public. **GitHub owner:** `patterninc`, verified with `gh api repos/patterninc/caterpillar`, so Pattern inherited controls apply.

## Scorecard

| Metric | Value |
|--------|-------|
| **Adjusted compliance** | **52.4%** ((19 Met + 0.5 × 6 Partial) / (49 − 7 N/A) = 22 / 42) |
| **Critical gates** | **RED**: 16 required CI (Gap), 23 unit tests (Partial), 24 integration tests (Gap). Gates 2, 6, 15, 19, 20, 40 and 48 are Met |
| Status counts | 19 Met · 6 Partial · 17 Gap · 7 Not applicable |

### Per-category breakdown

| Category | Met | Partial | Gap | N/A |
|----------|-----|---------|-----|-----|
| Documentation & Context (1–9) | 9 | 0 | 0 | 0 |
| Guardrails & Enforcement (10–22) | 5 | 2 | 6 | 0 |
| Testing & Feedback Loops (23–35) | 0 | 3 | 7 | 3 |
| Environment & Tooling (36–48) | 4 | 1 | 4 | 4 |
| Agent dispatch (49) | 1 | 0 | 0 | 0 |
| **Total** | **19** | **6** | **17** | **7** |

## Documentation & Context

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 1 | Skills / reusable prompt workflows | **Met** | `.claude/skills/add-task-type`, `debug-pipeline`, `cut-release` | — |
| 2 | AGENTS.md | **Met** | `AGENTS.md` (commands, layout, invariants, tests, CI, conventions); `CLAUDE.md` symlinks to it | — |
| 3 | Architecture decision records | **Met** | `docs/adr/` index plus 0001–0006, each citing code and PRs; marked retroactive, with inferred rationale labelled | — |
| 4 | Runbooks | **Met** | `docs/runbooks/`: silent task errors, hangs, redelivery/loss, config and secret load, release and rollback, Docker Hub publishing, cgo/librdkafka builds | — |
| 5 | API contract docs | **Met** | The YAML pipeline schema is the contract: each `internal/pkg/pipeline/task/*/README.md` has a Configuration Fields table | Equivalent credited: no network API surface |
| 6 | README with setup & run instructions | **Met** | `README.md` "How to Run Locally" and "Development" (C compiler and librdkafka requirement) | — |
| 7 | Changelog with migration notes | **Met** | `CHANGELOG.md` (Keep a Changelog) for 2.26.0–2.33.0 with **Migration** notes (#118, #112, #133); earlier releases in GitHub Releases | Keep adding entries under Unreleased |
| 8 | On-call playbooks | **Met** | `docs/oncall/playbook.md`: owner, Slack, triage by incident type, severity, rollback | No monitors exist for a CLI; triage starts from the runner's logs |
| 9 | CODEOWNERS | **Met** | `.github/CODEOWNERS` `* @patterninc/data-acquisition`; `main + releases` ruleset requires code-owner review | — |

## Guardrails & Enforcement

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 10 | Linters | **Gap** | `ci.yaml` runs `go vet -copylocks=false` only; no `.golangci.yml` | Add golangci-lint (govet, staticcheck, errcheck, unused, gocyclo, depguard) in CI. A local run on 2026-11-19 found 16 findings to baseline |
| 11 | Formatters | **Partial** | `gofmt -l` is clean today | Nothing enforces it; add a CI `gofmt -l` check |
| 12 | Type checking | **Met** | Go compiler on every build (`ci.yaml` job `build`) | — |
| 13 | Pre-commit hooks | **Gap** | None | Add `.pre-commit-config.yaml` mirroring CI |
| 14 | Commit message conventions | **Partial** | Conventional Commits used by recent PRs and documented in `AGENTS.md` | Not enforced; add a PR-title check |
| 15 | Branch protection | **Met** | Rulesets `main + releases` (repo) and `require-pr-review`, `require-copilot-code-review` (org): no deletion or force-push, reviews required | — |
| 16 | Required CI checks | **Gap** | The only required status check on `main` is the org's `agent-pr-approval` (`gh api repos/patterninc/caterpillar/rules/branches/main`) | A repo admin requires `build` (bound to GitHub Actions, app id 15368) in a repo ruleset |
| 17 | Dependency allow/deny lists | **Gap** | Preferred libraries noted in `AGENTS.md` only | Add a deny-list file and a CI check with `go mod verify` |
| 18 | License compliance scanning | **Gap** | None | Add `go-licenses check` in CI; librdkafka needs Zlib and BSD-2-Clause-NetBSD allowed |
| 19 | Secret scanning | **Met** | Met — inherited Pattern Wiz policy (Wiz Secret Scanner reports on PRs) | — |
| 20 | SAST gates | **Met** | Met — inherited Pattern Wiz policy (Wiz SAST Scanner reports on PRs) | — |
| 21 | Max complexity limits | **Gap** | None enforced; `dag.go` `validateInput` measures 36 | Add gocyclo with a limit and baseline that function |
| 22 | Import boundaries | **Met** | `internal/` packages (compiler-enforced); task packages cannot import `pipeline` without a cycle | — |

## Testing & Feedback Loops

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 23 | Unit tests | **Partial** | Three test files (`pipeline/dag_test.go`, `pipeline/ack/ack_test.go`, `pipeline/task/acking_test.go`), run by `ci.yaml` `build`; total coverage 4.3% | Most task packages, `jq` and `config` have no tests |
| 24 | Integration tests | **Gap** | None | LocalStack tests for the S3, SQS, SNS and SSM tasks in a CI job |
| 25 | Snapshot / golden-file tests | **Gap** | `test/pipelines/` fixtures are not compared automatically | Run deterministic pipelines and diff against committed output |
| 26 | Contract tests (Pact) | **Not applicable** | — | CLI driving user-defined pipelines against arbitrary systems; no consumer-service contract to verify |
| 27 | End-to-end tests (Playwright) | **Not applicable** | — | No browser UI |
| 28 | Visual regression tests | **Not applicable** | — | No UI surface |
| 29 | Coverage thresholds | **Gap** | None | Add a floor in CI below the measured value |
| 30 | Mutation testing | **Gap** | None | Scheduled, informational run once unit coverage exists |
| 31 | Load / performance benchmarks | **Gap** | No `Benchmark*` functions | Benchmark jq, converters and channel handoff, compared in CI |
| 32 | Flaky test quarantine | **Partial** | Policy in `AGENTS.md` (Tests → Flaky tests) | `TestSamplersSettleDroppedRecords/percent_keeps_all` (`internal/pkg/pipeline/task/acking_test.go`) fails about 7% of runs (21 of 300) because of an off-by-one in `sample/percent.go`: `rand.Int(..., 101)` with `n < cutoff`. Fix the sampler or quarantine the test |
| 33 | Structured CI output | **Gap** | `go test` text output only | Upload `go test -json` as an artifact |
| 34 | Deterministic test fixtures | **Partial** | `test/pipelines/*` fixtures exist | Not wired into automated tests (see 25) |
| 35 | Smoke tests for deploys | **Gap** | `release.yaml` runs `hello_name.yaml` on each binary before upload, but nothing checks what was published | Run the published release assets and image after each Release run |

## Environment & Tooling

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 36 | Devcontainer config | **Gap** | None | Pin Go 1.27 and a C toolchain |
| 37 | One-command setup | **Partial** | `makefile` `build`; its `test` target is a no-op (`directories=` is empty) | A single bootstrap command (build, test, sample run) |
| 38 | Seed scripts for local databases | **Not applicable** | — | No database |
| 39 | MCP servers | **Met** | Met — Toolsmith-managed MCP access | — |
| 40 | Scoped secrets per environment | **Met** | The binary holds no credentials; `{{ secret }}` reads SSM with the caller's role (`config/secret.go`), so scope is the runner's per-environment IAM. CI secrets (`DOCKERHUB_*`) are read only by release jobs on pushes to `main` and tags, never on PRs. Documented in `docs/secrets.md` | Runner-side scoping lives in the consumer repos |
| 41 | Preview environments per PR | **Not applicable** | — | Distributed binary and image; no service with a URL to preview |
| 42 | Hot-reload / watch mode | **Gap** | None | An `air` config that rebuilds and re-runs a pipeline |
| 43 | Structured logging (JSON) | **Gap** | `fmt.Println`/`fmt.Printf` to stdout throughout; errors interleaved with data (README "Error Handling") | Errors to stderr and `log/slog` JSON output |
| 44 | Observable traces and metrics | **Gap** | No OTel or Prometheus instrumentation; `task.go` has `TODO: do prom metrics` | Per-task record and error counters |
| 45 | Feature flags with local overrides | **Not applicable** | — | Versioned CLI releases; behaviour is selected per pipeline in YAML |
| 46 | Database migration tooling | **Not applicable** | — | No database |
| 47 | Dependency update automation | **Met** | Met — inherited Pattern Wiz policy | — |
| 48 | Reproducible builds | **Met** | `go.sum`; `build/Dockerfile` pins `golang:1.27-alpine` and `alpine:3.20` | — |

## Documentation & Context (agent dispatch)

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 49 | Agent-dispatch manifest | **Met** | `.agents/pattern-agents.json`: `schema_version`, `github.repo`, `clickup_list_id`, `slack_channel`, `datadog`, `skills.plugins`. No AWS footprint is deployed from this repo, so `aws[]` is omitted | ClickUp list and Datadog service are inferred; confirm with the owners |

## Prioritized recommendations

1. **[S]** (Gap, critical gate 16) Require `build` on `main` with a repo ruleset bound to GitHub Actions. First fix item 32's flake, or `build` blocks about 7% of PRs.
2. **[M]** (Gap, critical gate 24) LocalStack integration tests for S3, SQS, SNS and SSM in a CI job.
3. **[M]** (Partial, critical gate 23) Unit tests for `jq`, `config`, `record` and the offline task packages.
4. **[S]** (Gap, 10, 21) golangci-lint with gocyclo and depguard in CI, baselining the 16 existing findings.
5. **[S]** (Gap, 13, 17, 18, 29, 33) Pre-commit, dependency deny list, license check, coverage floor, `go test -json` artifacts.
6. **[M]** (Gap, 25, 31, 35) Golden-file tests, benchmarks with a CI comparison, post-release smoke tests.
7. **[S]** (Gap, 36, 42) Devcontainer and watch mode.
8. **[M]** (Gap, 43, 44) Structured logging and metrics.
9. **[S]** (Partial, 11, 14, 37) gofmt and PR-title checks in CI; a one-command setup script.
10. **[S]** (Partial, 32) Fix the off-by-one in `internal/pkg/pipeline/task/sample/percent.go`.

## Declined practices

| # | Practice | Rationale |
|---|----------|-----------|
| 26 | Contract tests (Pact) | CLI driving user-defined pipelines against arbitrary external systems; no fixed consumer-service contract. |
| 27 | End-to-end tests (Playwright) | No browser UI. |
| 28 | Visual regression tests | No UI surface. |
| 38 | Seed scripts for local databases | No persistence layer. |
| 41 | Preview environments per PR | Distributed binary and image, not a deployed service. |
| 45 | Feature flags with local overrides | Versioned CLI; behaviour is chosen per pipeline in YAML. |
| 46 | Database migration tooling | No database. |

## Beyond the checklist

- Each task package has its own `README.md` with a Configuration Fields table, which serves as the user-facing contract.
- `README.md` documents the error and acknowledgement semantics precisely, including where data can be lost (non-critical errors, backpressure).
- `release.yaml` verifies the architecture of every published binary and image, and pins its one write-capable third-party action to a commit.
