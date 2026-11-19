# Engineering Best Practices Audit — caterpillar

| | |
|---|---|
| **Audit date** | 2026-11-19 |
| **Auditor** | Claude — gauge-repo skill |
| **Rubric version** | `item-credit-v1` — 2026-09-04 (`references/best-practices.md`) |
| **Audited ref** | branch `docs/gauge-repo-markdown-audit` (PR #95), merged with `main` at `f817f42` |

## Changes since last audit

The previous report (2026-08-20, 40% under the older scoring) was re-baselined against `main` at `f817f42` under `item-credit-v1`: **35.4%** (10 Met, 9 Partial, 22 Gap, 8 N/A), critical gates **RED**. Items that moved in this PR:

- **Gap → Met:** 1 skills, 2 AGENTS.md, 3 ADRs, 8 on-call, 10 linters, 13 pre-commit, 17 dependency policy, 18 license scan, 21 complexity, 24 integration tests, 25 golden tests, 29 coverage floor, 30 mutation testing, 31 benchmarks, 33 structured CI output, 35 deploy smoke, 36 devcontainer, 42 hot reload, 49 dispatch manifest.
- **Partial → Met:** 4 runbooks, 6 README, 7 changelog, 11 formatter, 14 commit conventions, 23 unit tests, 34 deterministic fixtures, 37 one-command setup, 40 scoped secrets.
- **N/A → Partial:** 32 flaky test quarantine. The suite is no longer a single test, and one existing test is flaky today (see the item).
- **Inherited under `item-credit-v1`:** 19 and 20 (Pattern Wiz) and 39 (Toolsmith) are Met by org policy; they were Gap or Met by other evidence before.
- **Still Gap:** 16 required CI checks (needs a repo admin; command in the PR body), 43 structured logging, 44 traces and metrics (both need production code changes, out of scope for this PR).

## Repo profile

Caterpillar is a Go 1.27 CLI (`cmd/caterpillar`) that runs YAML-defined pipelines of tasks connected by Go channels: HTTP, S3, SQS, SNS, Kafka, SFTP, SSM, jq, XPath and converters (21 task types in `internal/pkg/pipeline/tasks.go`). **Artifact type:** CLI tool / data-pipeline runner. **Deployment model:** distributed binary and image. `release.yaml` publishes linux amd64/arm64 binaries to GitHub Releases and the multi-arch image `patternoss/caterpillar` to Docker Hub; it is run by other systems (for example `patterninc/caterpillar-services`). **Persistence:** none of its own. **UI:** none. **AWS footprint:** none deployed from this repo; tasks call AWS APIs with the caller's credentials. **Team:** Data Acquisition (`@patterninc/data-acquisition` in `CODEOWNERS`), many contributors. **Visibility:** public. **GitHub owner:** `patterninc`, verified with `gh api repos/patterninc/caterpillar`, so Pattern inherited controls apply.

## Scorecard

| Metric | Value |
|--------|-------|
| **Adjusted compliance** | **91.7%** ((38 Met + 0.5 × 1 Partial) / (49 − 7 N/A) = 38.5 / 42) |
| **Critical gates** | **RED**: item 16 (required CI checks) is Gap. All other gates (2, 6, 15, 19, 20, 23, 24, 40, 48) are Met |
| Status counts | 38 Met · 1 Partial · 3 Gap · 7 Not applicable |
| After the item 16 ruleset is added | 94.0% (39.5 / 42), critical gates GREEN |

### Per-category breakdown

| Category | Met | Partial | Gap | N/A |
|----------|-----|---------|-----|-----|
| Documentation & Context (1–9) | 9 | 0 | 0 | 0 |
| Guardrails & Enforcement (10–22) | 12 | 0 | 1 | 0 |
| Testing & Feedback Loops (23–35) | 9 | 1 | 0 | 3 |
| Environment & Tooling (36–48) | 7 | 0 | 2 | 4 |
| Agent dispatch (49) | 1 | 0 | 0 | 0 |
| **Total** | **38** | **1** | **3** | **7** |

## Documentation & Context

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 1 | Skills / reusable prompt workflows | **Met** | `.claude/skills/add-task-type`, `debug-pipeline`, `cut-release` | — |
| 2 | AGENTS.md | **Met** | `AGENTS.md` (commands, layout, invariants, tests, CI, conventions); `CLAUDE.md` symlinks to it | — |
| 3 | Architecture decision records | **Met** | `docs/adr/` index plus 0001–0006, each citing code and PRs; marked retroactive, with inferred rationale labelled | — |
| 4 | Runbooks | **Met** | `docs/runbooks/`: silent task errors, hangs, redelivery/loss, config and secret load, release and rollback, Docker Hub publishing, cgo/librdkafka builds | — |
| 5 | API contract docs | **Met** | The YAML pipeline schema is the contract: each `internal/pkg/pipeline/task/*/README.md` has a Configuration Fields table | Equivalent credited: no network API surface |
| 6 | README with setup & run instructions | **Met** | `README.md` "How to Run Locally" and "Development" (C compiler and librdkafka requirement, `scripts/dev.sh`) | — |
| 7 | Changelog with migration notes | **Met** | `CHANGELOG.md` (Keep a Changelog) for 2.26.0–2.33.0 with **Migration** notes (#118, #112, #133); earlier releases in GitHub Releases | Keep adding entries under Unreleased |
| 8 | On-call playbooks | **Met** | `docs/oncall/playbook.md`: owner, Slack, triage by incident type, severity, rollback | No monitors exist for a CLI; triage starts from the runner's logs |
| 9 | CODEOWNERS | **Met** | `.github/CODEOWNERS` `* @patterninc/data-acquisition`; `main + releases` ruleset requires code-owner review | — |

## Guardrails & Enforcement

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 10 | Linters | **Met** | `.golangci.yml` (govet, staticcheck, errcheck, ineffassign, unused, gocyclo, depguard); `quality.yml` job `lint` | 16 pre-existing findings are baselined per file |
| 11 | Formatters | **Met** | `quality.yml` job `format` (`gofmt -l`), gofmt in `.golangci.yml` formatters and pre-commit | — |
| 12 | Type checking | **Met** | Go compiler on every build (`ci.yaml` job `build`) | — |
| 13 | Pre-commit hooks | **Met** | `.pre-commit-config.yaml` (gofmt, vet, tests, golangci-lint, gitleaks, dependency policy, conventional commit messages); `scripts/dev.sh` installs it | — |
| 14 | Commit message conventions | **Met** | `pr-title.yml` job `pr-title` enforces Conventional Commits on PR titles (the squash commit subject); `commit-msg` hook | — |
| 15 | Branch protection | **Met** | Rulesets `main + releases` (repo) and `require-pr-review`, `require-copilot-code-review` (org): no deletion or force-push, reviews required | — |
| 16 | Required CI checks | **Gap** | The only required status check on `main` is the org's `agent-pr-approval` (`gh api repos/patterninc/caterpillar/rules/branches/main`). `build` and the new jobs are not required | A repo admin adds the `require-ci` ruleset (command in the PR body) once each check has reported |
| 17 | Dependency allow/deny lists | **Met** | `.github/dependency-policy.txt` plus `scripts/check-deps.sh` (direct-dependency deny list and `go mod verify`); `quality.yml` job `dependencies` | — |
| 18 | License compliance scanning | **Met** | `scripts/check-licenses.sh` (go-licenses allow list); `quality.yml` job `licenses` | Zlib and BSD-2-Clause-NetBSD allowed for vendored librdkafka |
| 19 | Secret scanning | **Met** | Met — inherited Pattern Wiz policy (Wiz Secret Scanner reports on PRs); gitleaks in pre-commit | — |
| 20 | SAST gates | **Met** | Met — inherited Pattern Wiz policy (Wiz SAST Scanner reports on PRs) | — |
| 21 | Max complexity limits | **Met** | `gocyclo` `min-complexity: 30` in `.golangci.yml`; one baselined function (`dag.go` `validateInput`, 36) | Refactor `validateInput` and drop the baseline |
| 22 | Import boundaries | **Met** | `internal/` packages (compiler-enforced); `depguard` rules: no `cmd` imports from `internal/pkg`, task packages may not import `pipeline`, no AWS SDK v1 | — |

## Testing & Feedback Loops

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 23 | Unit tests | **Met** | Tests in 22 packages (jq, config, record, task base, 15 task types, ack, pipeline/DAG); total coverage 41.4% (was 4.3%); `go test -race ./...` in `ci.yaml` `build` and `quality.yml` `coverage` | Network-only tasks (http, heimdall, sftp, kafka I/O) are covered only by integration tests or not at all |
| 24 | Integration tests | **Met** | `//go:build integration` tests for S3 (`file`), SQS, SNS, SSM (`aws_parameter_store`, `{{ secret }}`) against LocalStack 4.14.0; `integration.yml` job `integration` | First run is on this PR's CI; confirm it passes before merge. Kafka, SFTP and HTTP have no integration tests yet |
| 25 | Snapshot / golden-file tests | **Met** | `test/golden/golden_test.go` runs 11 `test/pipelines/*.yaml` and diffs against `test/golden/testdata/*.golden`; `-update` regenerates | — |
| 26 | Contract tests (Pact) | **Not applicable** | — | CLI driving user-defined pipelines against arbitrary systems; no consumer-service contract to verify |
| 27 | End-to-end tests (Playwright) | **Not applicable** | — | No browser UI |
| 28 | Visual regression tests | **Not applicable** | — | No UI surface |
| 29 | Coverage thresholds | **Met** | `scripts/check-coverage.sh` floor 40.0% (measured 41.4%); `quality.yml` job `coverage` | Raise the floor as tests are added |
| 30 | Mutation testing | **Met** | `mutation.yml` (weekly gremlins v0.6.0, report artifact) | Informational |
| 31 | Load / performance benchmarks | **Met** | `*_bench_test.go` for jq, csv converter, `SendRecord`, record marshalling, linear pipeline handoff; `bench.yml` compares PR vs base with benchstat and fails on a significant slowdown above 25% (`scripts/check-bench.sh`) | Informational and noisy on shared runners; not proposed as a required check |
| 32 | Flaky test quarantine | **Partial** | Policy in `AGENTS.md` (Tests → Flaky tests): `t.Skip("flaky: <task>")`, linked ticket, two-week limit | `TestSamplersSettleDroppedRecords/percent_keeps_all` (`internal/pkg/pipeline/task/acking_test.go`) fails about 7% of runs because of an off-by-one in `sample/percent.go` (`rand.Int(..., 101)` with `n < cutoff`). Fix the sampler, or quarantine the test, before `build` becomes required |
| 33 | Structured CI output | **Met** | `go test -json` uploaded as `test-results` (`quality.yml` `coverage`) and `integration.json` (`integration.yml`) | — |
| 34 | Deterministic test fixtures | **Met** | `test/pipelines/*` fixtures are used by the golden tests; unit tests use `t.TempDir()` and in-test fixtures | — |
| 35 | Smoke tests for deploys | **Met** | `smoke.yml` runs after each successful Release run: `smoke-binary` downloads both release assets for `v*` tags, `smoke-image` pulls `patternoss/caterpillar` for each platform; both run `scripts/smoke.sh` (hello_name pipeline, 10 greetings, no task errors) | First run is after the next release; the image job is skipped with a warning while Docker Hub secrets are absent |

## Environment & Tooling

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 36 | Devcontainer config | **Met** | `.devcontainer/devcontainer.json` (Go 1.27 image, AWS CLI, Docker, pre-commit, air) | — |
| 37 | One-command setup | **Met** | `scripts/dev.sh` (toolchain and C compiler check, build, tests, smoke, hooks) | — |
| 38 | Seed scripts for local databases | **Not applicable** | — | No database |
| 39 | MCP servers | **Met** | Met — Toolsmith-managed MCP access | — |
| 40 | Scoped secrets per environment | **Met** | The binary holds no credentials; `{{ secret }}` reads SSM with the caller's role (`config/secret.go`), so scope is the runner's per-environment IAM. CI secrets (`DOCKERHUB_*`) are read only by release jobs that run on pushes to `main` and tags, never on PRs. Documented in `docs/secrets.md` | Runner-side scoping lives in the consumer repos (for caterpillar-services, `configs/task_policy.json`) |
| 41 | Preview environments per PR | **Not applicable** | — | Distributed binary and image; no service with a URL to preview |
| 42 | Hot-reload / watch mode | **Met** | `.air.toml` (rebuild and re-run `PIPELINE` on change) | — |
| 43 | Structured logging (JSON) | **Gap** | `fmt.Println`/`fmt.Printf` to stdout throughout (`pipeline.go`, `task.go`); errors interleaved with data (README "Error Handling") | Move errors to stderr and adopt `log/slog` JSON output; production code change |
| 44 | Observable traces and metrics | **Gap** | No OTel or Prometheus instrumentation (`prometheus/client_golang` is only indirect); `task.go` has `TODO: do prom metrics` | Add per-task record and error counters, exposable by the runner |
| 45 | Feature flags with local overrides | **Not applicable** | — | Versioned CLI releases; behaviour is selected per pipeline in YAML, which already acts as a local override |
| 46 | Database migration tooling | **Not applicable** | — | No database |
| 47 | Dependency update automation | **Met** | Met — inherited Pattern Wiz policy | — |
| 48 | Reproducible builds | **Met** | `go.sum`; `go mod verify` in CI; `build/Dockerfile` pins `golang:1.27-alpine` and `alpine:3.20`; Actions pinned by version or commit | — |

## Documentation & Context (agent dispatch)

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 49 | Agent-dispatch manifest | **Met** | `.agents/pattern-agents.json`: `schema_version`, `github.repo`, `clickup_list_id`, `slack_channel`, `datadog`, `skills.plugins`. No AWS footprint is deployed from this repo, so `aws[]` is omitted | ClickUp list and Datadog service are inferred; confirm with the owners |

## Prioritized recommendations

1. **[S]** (Gap, critical gate 16) Add the `require-ci` repository ruleset on `main` requiring `build`, `lint`, `format`, `coverage`, `dependencies`, `licenses`, `integration` and `pr-title`, bound to GitHub Actions.
2. **[S]** (Partial, item 32) Fix the off-by-one in `internal/pkg/pipeline/task/sample/percent.go` so `percent: 100` keeps every record. This also removes the 7% flake in `build` and `coverage`. Land it before step 1.
3. **[M]** (Gap, item 43) Send task errors to stderr and adopt structured JSON logging (`log/slog`).
4. **[M]** (Gap, item 44) Add per-task record, error and latency metrics.
5. **[M]** (Met, hardening) Add integration tests for Kafka (a Redpanda service container), SFTP and HTTP, and fix the bugs the new tests skip (`t.Skip("known bug: …")`).

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
- New unit tests record known production bugs as `t.Skip("known bug: …")` tests, ready to be unskipped when each bug is fixed.
