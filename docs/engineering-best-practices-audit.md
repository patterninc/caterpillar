# Engineering Best Practices Audit — caterpillar

| | |
|---|---|
| **Audit date** | 2026-11-15 |
| **Auditor** | Claude — gauge-repo skill |
| **Rubric version** | `item-credit-v1` — 2026-09-04 (`references/best-practices.md`) |

## Changes since last audit

The last audit report (dated 2026-09-22) was reverted, so no report existed on `main`; this report recreates it. Measured against current `main` (latest release `v2.32.3`), the baseline was 11 Met, 6 Partial, 22 Gap and 10 N/A (35.9%, critical gates RED). Since the 2026-09-22 report, CI started running `go vet` and `go test -race`, which made item 10 Partial. Item 14 is now rated Partial, because the convention was followed but not enforced. `backstage.yaml` was removed.

This branch moves these items:

- Item 2 (AGENTS.md): Gap → **Partial**. `AGENTS.md` now exists (checks, hooks, commit and secrets rules) but has no code-layout guidance.
- Item 10 (linters): Partial → **Met**.
- Item 11 (formatters): Gap → **Met**.
- Item 13 (pre-commit hooks): Gap → **Met**.
- Item 14 (commit conventions): Partial → **Met**.
- Item 16 (required CI): Partial → **Met**.
- Item 17 (dependency allow/deny list): Gap → **Met**.
- Item 18 (license scanning): Gap → **Met**.
- Item 21 (complexity limits): Gap → **Met**.
- Item 40 (scoped secrets): Partial → **Met**.

## Repo profile

`caterpillar` is a public (Apache-2.0) Go 1.27 data-ingestion and pipeline-processing engine owned by Pattern (`patterninc/caterpillar`, verified via `gh repo view`). It uses cgo and librdkafka (through confluent-kafka-go). It ships as a CLI binary in GitHub releases (linux/amd64 and linux/arm64) and as the multi-arch Docker Hub image `patternoss/caterpillar`, both published by `.github/workflows/release.yaml`. The repo deploys no hosted service: there is no Terraform, no ECS/Lambda/Kubernetes config and no owned database.

The layout follows the Go idiom (`cmd/caterpillar`, `internal/pkg/pipeline/task/...`). A per-task `README.md` sits in 19 of the 20 `task/*` directories; the `aws/` wrapper has none. GitHub reports 18 contributors, and `.github/CODEOWNERS` assigns everything to one team, so this is a team-scale repo.

The AWS integrations (SQS, SNS, S3, Parameter Store, Translate) are pipeline tasks that run in the user's own AWS account. The repo has no AWS footprint of its own, so item 49's `aws[]` sub-check does not apply. There is no browser UI, no visual surface and no persistence layer. Because the verified owner is `patterninc`, Pattern's inherited controls apply to items 19, 20, 39 and 47.

## Scorecard

| Metric | Value |
|--------|-------|
| **Critical gates** | **RED** |
| **Adjusted compliance** | **55.1%** |

Critical gates are RED because item 2 (AGENTS.md) is Partial. Every other applicable critical gate (6, 15, 16, 19, 20, 23, 24, 40, 48) is Met. Adjusted compliance is calculated separately:

`(20 Met + 0.5 × 3 Partial) / (49 total - 10 justified N/A) = 21.5 / 39 = 55.1%`

### Status totals

| Status | Items |
|--------|------:|
| Met | 20 |
| Partial | 3 |
| Gap | 16 |
| N/A | 10 |
| **Total** | **49** |

### Per-category breakdown

| Category | Met | Partial | Gap | N/A |
|----------|----:|--------:|----:|----:|
| Documentation & Context | 2 | 2 | 4 | 1 |
| Guardrails & Enforcement | 11 | 0 | 1 | 1 |
| Testing & Feedback Loops | 3 | 0 | 6 | 4 |
| Environment & Tooling | 4 | 1 | 4 | 4 |
| Agent dispatch | 0 | 0 | 1 | 0 |
| **Total** | **20** | **3** | **16** | **10** |

## Documentation & Context

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 1 | Skills / reusable prompt workflows | **Gap** | No `.claude/skills/`, `.claude/commands/` or equivalent | Add repo-local skills for the common flows: add a pipeline task, write and run a `test/pipelines/*_test.yaml`, cut a release. |
| 2 | AGENTS.md | **Partial** | `AGENTS.md` documents the checks (`make fmt-check`, `vet`, `lint`, `lint-policy`, `licenses`, `go test -race ./...`), git hooks, dependency and complexity rules, Conventional Commits, and release and secret handling. It has no code-layout guidance. Critical gate. | Add how to add a pipeline task (the `task/<name>/` package, registration, per-task `README.md`), the `test/pipelines/*_test.yaml` harness, the pipeline-YAML rules, and the cgo/librdkafka build prerequisite. |
| 3 | Architecture decision records | **Gap** | No `docs/adr/` or equivalent | Record the DAG-vs-linear pipeline model, `fail_on_error` semantics and the channel back-pressure model as dated ADRs in `docs/adr/`. |
| 4 | Runbooks | **Gap** | No `docs/runbooks/` | Add runbooks for cutting a release, moving the `latest` tag, and rotating the Docker Hub token. |
| 5 | API contract docs (OpenAPI / protobuf) | **Not applicable** | CLI and pipeline engine; no network API of its own to publish a wire contract for | The per-task READMEs under `internal/pkg/pipeline/task/*/README.md` document each task's config surface, which is the equivalent contract for a pipeline framework. |
| 6 | README with setup & run instructions | **Met** | `README.md`: purpose, prerequisites (Go 1.27), build, run, Docker image tags, DAG syntax, error handling, dynamic config, supported tasks; `DAG_README.md` covers DAG mode. Critical gate. | — |
| 7 | Changelog with migration notes | **Partial** | `release.yaml` uses `generate_release_notes: true`; no `CHANGELOG.md` and no migration notes | Add a curated `CHANGELOG.md` (or link the generated release notes) and require a migration section for breaking releases. |
| 8 | On-call playbooks | **Gap** | No `docs/oncall/` | Add a triage playbook for pipeline failures reported by consuming teams: what to check and whom to involve. |
| 9 | CODEOWNERS | **Met** | `.github/CODEOWNERS` assigns `*` to one team; the `main + releases` ruleset requires code-owner review | — |

## Guardrails & Enforcement

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 10 | Linters | **Met** | `.golangci.yml` (golangci-lint v2: errcheck, govet, ineffassign, revive, staticcheck, unused, gocyclo, gomodguard_v2); `make lint` (new issues since the PR base) and `make lint-policy` (whole tree), run by the CI `lint` job. The 151 existing findings are a backlog; `govet` copylocks is disabled for three documented findings. | Burn down the 151 existing findings, then drop `--new-from-rev`. Fix the three `task.Base` copies and re-enable copylocks. |
| 11 | Formatters | **Met** | `make fmt-check` fails on any `gofmt -l` output and runs in the CI `lint` job; the tree is gofmt-clean; gofmt also runs in pre-commit | — |
| 12 | Type checking | **Not applicable** | Go is statically typed; the compiler enforces types on every build | The Go toolchain already covers what this item asks of dynamically typed ecosystems. |
| 13 | Pre-commit hooks | **Met** | `.pre-commit-config.yaml`: gofmt, go vet, golangci-lint v2.15.0 and gitleaks v8.30.1, with hook repos pinned to commit SHAs; `make hooks` installs them; documented in `AGENTS.md` | — |
| 14 | Commit message conventions | **Met** | Conventional Commits are documented in `AGENTS.md`; the CI `pr-title` job (`amannn/action-semantic-pull-request`) checks every PR title, which becomes the squash-merge commit message | — |
| 15 | Branch protection | **Met** | Repo ruleset `main + releases` requires a PR and code-owner review and blocks deletion, non-fast-forward, creation and direct updates on `main` and `release/**`; the org ruleset `require-pr-review` also applies. Critical gate. | — |
| 16 | Required CI checks before merge | **Met** | `.github/workflows/ci.yaml` runs on every `pull_request` to `main` with no `paths:` filter. The aggregate `ci-status` job runs `if: always()`, needs `build`, `lint`, `licenses` and `pr-title`, and fails unless all succeed. Requiring `ci-status` in the `main + releases` ruleset is an admin follow-up. Critical gate. | — |
| 17 | Dependency allow-lists / deny-lists | **Met** | `gomodguard_v2` in `.golangci.yml` allows only the listed direct modules and blocks deprecated ones (`github.com/golang/protobuf`, AWS SDK v1, `yaml.v2`, `dgrijalva/jwt-go`, `satori/go.uuid`, `pkg/errors`); `make lint-policy` runs it in CI. The personal-namespace exception `github.com/babourine/x` carries a reason. | Replace `github.com/babourine/x` (one generic set type) with a local type and drop the exception. |
| 18 | License compliance scanning | **Met** | `make licenses` (`go-licenses check` against `ALLOWED_LICENSES`) runs in the CI `licenses` job and fails on any other license. Zlib and BSD-2-Clause-NetBSD (vendored librdkafka sources) are allowed with a reason. `make licenses-save` produces third-party notices for releases. | — |
| 19 | Secret scanning | **Met** | Inherited Pattern Wiz policy (org-wide coverage for verified `patterninc` repos); gitleaks also runs in pre-commit. Critical gate. | — |
| 20 | SAST / static analysis gates | **Met** | Inherited Pattern Wiz policy (org-wide blocking SAST). Critical gate. | — |
| 21 | Max complexity limits | **Met** | `gocyclo` with `min-complexity: 30` in `.golangci.yml`, enforced over the whole tree by `make lint-policy` in CI. The one existing offender (`validateInput` in `internal/pkg/pipeline/dag.go`, complexity 36) carries a justified exclusion. | Split `validateInput` and remove the exclusion; then lower the ceiling toward 20 (seven functions are between 16 and 25). |
| 22 | Import boundary enforcement | **Gap** | `internal/` is used, but no `depguard` rule limits imports between packages | Add `depguard` rules so `pipeline/task/*` packages do not import each other and only use the shared `pipeline`, `config` and `record` packages. |

## Testing & Feedback Loops

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 23 | Unit tests | **Met** | `internal/pkg/pipeline/dag_test.go`, `internal/pkg/pipeline/ack/ack_test.go`; CI runs `go test -race ./...`. Critical gate. | — |
| 24 | Integration tests | **Met** | `internal/pkg/pipeline/task/acking_test.go` wires several tasks together; `test/pipelines/*_test.yaml` (18 files) are runnable full-pipeline scenarios. Critical gate. | — |
| 25 | Snapshot / golden-file tests | **Gap** | No golden files; `test/pipelines/converter/*_test_results.json` are output targets, not compared references | Store expected pipeline outputs as golden files and diff against them in `go test`. |
| 26 | Contract tests (Pact) | **Not applicable** | No network API of its own; tasks talk to Kafka, SQS, HTTP and SFTP directly | Integration runs and the per-task READMEs cover the equivalent surface. |
| 27 | End-to-end tests (Playwright) | **Not applicable** | No browser UI | The CLI surface is exercised by the `test/pipelines/*_test.yaml` runs. |
| 28 | Visual regression tests | **Not applicable** | No visual surface | — |
| 29 | Test coverage thresholds | **Gap** | No coverage gate; the `makefile` `test` target loops over an empty `directories=` | Publish `go test -coverprofile` from CI and fail below a starting floor. |
| 30 | Mutation testing | **Gap** | No mutation testing | Run a Go mutation tester on the pipeline core packages as an advisory scheduled job. |
| 31 | Load / performance benchmarks | **Gap** | No `Benchmark*` functions | Add benchmarks for the jq, xpath and converter hot paths and record baselines. |
| 32 | Flaky test quarantine | **Gap** | No quarantine convention | Adopt a `t.Skip("quarantine: <ticket>")` convention with a periodic review. |
| 33 | Structured CI output | **Gap** | CI prints plain `go test` output; no `-json` or JUnit report | Run `go test -json` and upload the report as an artifact (or render a job summary). |
| 34 | Deterministic test fixtures | **Met** | Fixed inputs under `test/pipelines/` (`bcrypt_cases.json`, `greetings.json`, `sample.csv`, `planets.tar.gz`, ...) | — |
| 35 | Smoke tests for deploys | **Not applicable** | The repo publishes a binary and an image; there is no deployed service to smoke-test | `release.yaml` already runs each built binary on `test/pipelines/hello_name.yaml` and checks the published image's architectures. |

## Environment & Tooling

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 36 | Devcontainer config | **Gap** | No `.devcontainer/` | Add a devcontainer with Go 1.27 and the cgo/librdkafka toolchain used by `build/Dockerfile`. |
| 37 | One-command setup | **Partial** | `makefile` has `all: build test`, `tools` and `hooks`, but `test` loops over an empty `directories=`, so `make` only builds | Point `test` at `go test -race ./...` and add a `make dev` that runs `tools`, `hooks`, `build` and `test`. |
| 38 | Seed scripts for local databases | **Not applicable** | No database of its own | — |
| 39 | MCP servers for external tools | **Met** | Toolsmith-managed MCP access (inherited for verified `patterninc` repos) | — |
| 40 | Scoped secrets per environment | **Met** | Pipeline secrets resolve at runtime from the user's own Parameter Store; none are stored in the repo. The only CI credential (the Docker Hub push token) belongs to the `dockerhub-release` environment, named only by the three Docker jobs in `release.yaml`, with a deployment policy limited to `main` and the `v*` and `latest` tags. No `pull_request` job can reach it. Creating the environment and moving the secrets is an admin follow-up. Critical gate. | — |
| 41 | Preview environments per PR | **Not applicable** | Distributable binary, not a hosted service | — |
| 42 | Hot-reload / watch mode | **Not applicable** | Compile-and-run CLI with batch pipeline runs; no long-running dev server | — |
| 43 | Structured logging (JSON) | **Gap** | Tasks report through `fmt.Print*` and `log.Fatal`/`log.Println` (61 call sites outside tests) | Move to `log/slog` with JSON output on stderr, tagging task name and pipeline run. |
| 44 | Observable traces and metrics | **Gap** | No OTel or Prometheus instrumentation; `internal/pkg/pipeline/task/task.go` has `TODO: do prom metrics` | Emit per-task record counts and durations as metrics, with a span per task. |
| 45 | Feature flags with local overrides | **Gap** | No flag mechanism; DAG mode is marked EXPERIMENTAL in the README | Add an environment-variable toggle convention for experimental features such as DAG mode. |
| 46 | Database migration tooling | **Not applicable** | No database of its own | — |
| 47 | Dependency update automation | **Met** | Inherited Pattern Wiz policy for verified `patterninc` repos | — |
| 48 | Reproducible builds (lockfiles) | **Met** | `go.mod` and `go.sum` pin every dependency; `build/Dockerfile` pins `golang:1.27-alpine` and `alpine:3.20`; CI and release pin Go 1.27 and the release runner image. Critical gate. | — |

## Documentation & Context (agent dispatch)

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 49 | Agent-dispatch manifest | **Gap** | No `.agents/pattern-agents.json` (or `.yml`/`.yaml`) | Add `.agents/pattern-agents.json` with `schema_version`, `github.repo`, ClickUp list, Slack channel, Datadog and skills metadata. `aws[]` can be omitted because the repo has no AWS footprint of its own. |

## Prioritized recommendations

1. **[S] Partial — AGENTS.md (item 2, critical gate):** Add code-layout guidance to `AGENTS.md`: how to add a pipeline task (package, registration, per-task README), the `test/pipelines/*_test.yaml` harness, the pipeline-YAML rules, and the cgo/librdkafka build prerequisite.
2. **[S] Gap — agent-dispatch manifest (item 49):** Add `.agents/pattern-agents.json` with the core fields (`aws[]` optional).
3. **[S] Gap — skills (item 1):** Add `.claude/skills/` entries for "add a pipeline task" and "cut a release".
4. **[S] Gap — import boundaries (item 22):** Add `depguard` rules that stop `pipeline/task/*` packages from importing each other.
5. **[S] Gap — coverage thresholds (item 29):** Publish coverage from CI and fail below a starting floor.
6. **[S] Gap — structured CI output (item 33):** Run `go test -json` and upload the report as an artifact.
7. **[S] Gap — runbooks (item 4):** Add release, `latest`-tag and Docker Hub token-rotation runbooks under `docs/runbooks/`.
8. **[S] Gap — on-call playbook (item 8):** Add a triage playbook for pipeline failures reported by consuming teams.
9. **[S] Gap — ADRs (item 3):** Record the DAG model, `fail_on_error` semantics and back-pressure design in `docs/adr/`.
10. **[S] Gap — devcontainer (item 36):** Add `.devcontainer/` with Go 1.27 and the librdkafka toolchain.
11. **[S] Gap — flaky quarantine (item 32):** Adopt a `t.Skip("quarantine: <ticket>")` convention with a periodic review.
12. **[M] Gap — structured logging (item 43):** Move to `log/slog` JSON output tagged with task name and pipeline run.
13. **[M] Gap — traces and metrics (item 44):** Instrument per-task counters, durations and spans.
14. **[M] Gap — feature flags (item 45):** Add an environment-variable toggle convention for experimental features such as DAG mode.
15. **[M] Gap — snapshot tests (item 25):** Store golden outputs for pipeline runs and diff against them.
16. **[M] Gap — load benchmarks (item 31):** Add benchmarks for the jq, xpath and converter hot paths.
17. **[L] Gap — mutation testing (item 30):** Run mutation testing on the core packages as an advisory scheduled job.
18. **[S] Partial — one-command setup (item 37):** Point `make test` at `go test -race ./...` and add `make dev`.
19. **[S] Partial — changelog (item 7):** Add `CHANGELOG.md` (or link the generated release notes) and require migration notes for breaking releases.

## Declined practices

| # | Practice | Rationale |
|---|----------|-----------|
| 5 | API contract docs | CLI and pipeline engine with no network API of its own; the per-task READMEs cover the equivalent config contract. |
| 12 | Type checking | Go is statically typed; the compiler enforces this on every build. |
| 26 | Contract tests | No network API of its own; pipeline tasks test the wire (Kafka, SQS, HTTP) directly. |
| 27 | End-to-end tests | No browser UI. |
| 28 | Visual regression tests | No visual surface. |
| 35 | Smoke tests for deploys | Publishes a binary and an image; there is no deployed service. The release workflow already runs each binary and checks the image. |
| 38 | Seed scripts for local DBs | No database of its own. |
| 41 | Preview environments per PR | Distributable binary, not a hosted service. |
| 42 | Hot-reload / watch mode | Compile-and-run CLI; pipeline runs are batch. |
| 46 | Database migration tooling | No database of its own. |

## Beyond the checklist

- Every quality gate is a `makefile` target (`fmt-check`, `vet`, `lint`, `lint-policy`, `licenses`), and CI only calls those targets, so local and CI results match. Tool versions are pinned in one place (`GOLANGCI_LINT_VERSION`, `GO_LICENSES_VERSION`).
- Each lint exemption (the copylocks findings, the `validateInput` complexity exclusion, the `github.com/babourine/x` dependency exception, the two extra licenses) is written down next to the config with its reason.
- A per-task `README.md` sits in 19 of the 20 `internal/pkg/pipeline/task/*/` directories, which gives agents and humans a predictable place to find each task's config surface.
- `test/pipelines/*_test.yaml` doubles as documentation and regression scenarios: each YAML is a README example and a runnable pipeline.
- The release workflow builds each architecture natively (no QEMU), runs each binary before publishing, and checks that the published multi-arch image contains a correct binary for every platform.
- Third-party actions that run with write tokens are pinned to commit SHAs, and only the release job has write access.
