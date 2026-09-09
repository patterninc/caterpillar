# Engineering Best Practices Audit — caterpillar

| | |
|---|---|
| **Audit date** | 2026-09-09 |
| **Auditor** | Claude — gauge-repo skill |
| **Rubric version** | `item-credit-v1` — 2026-09-04 (`references/best-practices.md`) |

## Repo profile

`caterpillar` is a Go 1.25 data-ingestion and pipeline-processing engine authored at Pattern (`patterninc/caterpillar`, verified via `gh repo view`, public, not archived). It ships as a distributable CLI binary and a multi-arch Docker image published to Docker Hub (`patternoss/caterpillar`). Pipelines are configured via YAML and executed by the compiled binary; the repo itself does not deploy a hosted service — no Terraform, no ECS/Lambda configs, no Kubernetes manifests, no database schema. Package layout follows the Go idiom (`cmd/caterpillar`, `internal/pkg/pipeline/task/...`), with per-task README docs under each `task/*` directory. Contributors are a team of ~20 (top contributor 55 commits; `.github/CODEOWNERS` assigns `@patterninc/data-acquisition`). AWS SDK integrations exist as pipeline tasks (SQS/SNS/S3/Parameter Store), but the repo has no owned AWS footprint that would trigger item 49's `aws[]` sub-check. There is no browser UI, no visual surface, no owned persistence layer. Because ownership is `patterninc` (API-verified), Pattern inherited controls apply to items 19, 20, 39, and 47.

## Scorecard

| Metric | Value |
|--------|-------|
| **Critical gates** | **RED** |
| **Adjusted compliance** | **35.9%** |

Critical gates are RED because items 2 (AGENTS.md), 16 (required CI), and 40 (scoped secrets) are not Met. Adjusted compliance is calculated independently:

`(12 Met + 0.5 × 4 Partial) / (49 total - 10 justified N/A) = 14 / 39 = 35.9%`

### Status totals

| Status | Items |
|--------|------:|
| Met | 12 |
| Partial | 4 |
| Gap | 23 |
| N/A | 10 |
| **Total** | **49** |

### Per-category breakdown

| Category | Met | Partial | Gap | N/A |
|----------|----:|--------:|----:|----:|
| Documentation & Context | 2 | 1 | 5 | 1 |
| Guardrails & Enforcement | 4 | 1 | 7 | 1 |
| Testing & Feedback Loops | 3 | 0 | 6 | 4 |
| Environment & Tooling | 3 | 2 | 4 | 4 |
| Agent dispatch | 0 | 0 | 1 | 0 |
| **Total** | **12** | **4** | **23** | **10** |

## Documentation & Context

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 1 | Skills / reusable prompt workflows | **Gap** | No `.claude/skills/`, `.claude/commands/`, or equivalent | Add repo-local skills for the common flows (add a new pipeline task, run/write a `*_test.yaml`, cut a release). |
| 2 | AGENTS.md | **Gap** | No `AGENTS.md`, `CLAUDE.md`, or `.cursorrules` at root | Add `AGENTS.md` at root covering build/test commands, per-task directory conventions, and pipeline-YAML rules. Critical gate. |
| 3 | Architecture decision records | **Gap** | No `docs/adr/`, `docs/design/`, or equivalent | Record the DAG-vs-linear pipeline decision, `fail_on_error` semantics, and channel/back-pressure model as dated ADRs under `docs/adr/`. |
| 4 | Runbooks | **Gap** | No `docs/runbooks/` | Add runbooks for cutting a release, publishing a Docker Hub image, and rotating Docker Hub credentials. |
| 5 | API contract docs (OpenAPI / protobuf) | **Not applicable** | CLI + pipeline engine; no owned network API to publish a wire contract for | Per-task READMEs under `internal/pkg/pipeline/task/*/README.md` document each task's config surface, which is the equivalent contract for a pipeline framework. |
| 6 | README with setup & run instructions | **Met** | `README.md` (~390 lines): purpose, prerequisites, build, run, DAG syntax, error handling, dynamic config, supported tasks | — |
| 7 | Changelog with migration notes | **Partial** | Release workflow uses `generate_release_notes: true` in `.github/workflows/release.yaml`; no `CHANGELOG.md`; no migration notes | Add a curated `CHANGELOG.md` (or link auto-generated release notes) and require a `## Migration` section for breaking releases. |
| 8 | On-call playbooks | **Gap** | No `docs/oncall/` | Add a triage playbook for pipeline failures observed in dependent services (what to check, whom to page). |
| 9 | CODEOWNERS | **Met** | `.github/CODEOWNERS` — `* @patterninc/data-acquisition` | — |

## Guardrails & Enforcement

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 10 | Linters | **Gap** | No `.golangci.yml`; no lint step in `.github/workflows/ci.yaml` | Add `golangci-lint` with a starting profile (govet, staticcheck, errcheck, ineffassign) and run it in CI. |
| 11 | Formatters | **Gap** | No `gofmt`/`gofumpt` check in CI or hooks | Add a `gofmt -l` (fail-if-nonempty) job or `gofumpt` pre-commit step. |
| 12 | Type checking | **Not applicable** | Go is statically typed; the compiler enforces types on every build | The Go toolchain already covers what this checklist item names for dynamically-typed ecosystems. |
| 13 | Pre-commit hooks | **Gap** | No `.pre-commit-config.yaml`, `.husky/`, or `.githooks/` | Add a `.pre-commit-config.yaml` running gofmt, go vet, and golangci-lint. |
| 14 | Commit message conventions | **Met** | Recent tags use Conventional Commits consistently (`feat(kafka):`, `fix(jq):`, `chore:`) | — |
| 15 | Branch protection | **Met** | Repo ruleset `main + releases` (id 8564244) requires PR + code owner review; blocks non-fast-forward, deletion, direct creation. Org ruleset `require-pr-review` also applies. Critical gate. | — |
| 16 | Required CI checks before merge | **Partial** | `.github/workflows/ci.yaml` runs only `go build` on PRs; `go test`, lint, and security scans are not gated | Add `go test ./...`, `golangci-lint`, and a working coverage report to `ci.yaml`, then mark them required in the ruleset. Critical gate. |
| 17 | Dependency allow-lists / deny-lists | **Gap** | No policy config | Add a lightweight allow-list (`go mod why`/depguard rule) restricting new top-level dependencies. |
| 18 | License compliance scanning | **Gap** | No `go-licenses`, FOSSA, or equivalent job | Add `go-licenses check ./...` in CI with an explicit allow-list. |
| 19 | Secret scanning | **Met** | Inherited Pattern Wiz policy (org-wide coverage for verified `patterninc` repos). Critical gate. | — |
| 20 | SAST / static analysis gates | **Met** | Inherited Pattern Wiz policy (org-wide blocking SAST). Critical gate. | — |
| 21 | Max complexity limits | **Gap** | No cyclomatic-complexity config in golangci or elsewhere | Enable `gocyclo`/`gocognit` under golangci-lint with a starting ceiling. |
| 22 | Import boundary enforcement | **Gap** | `internal/` folder is used, but no `depguard`/`import-boundaries` linter | Enforce package boundaries between `pipeline`, `pipeline/task/*`, and shared helpers via `depguard`. |

## Testing & Feedback Loops

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 23 | Unit tests | **Met** | `internal/pkg/pipeline/dag_test.go`, `internal/pkg/pipeline/ack/ack_test.go` cover parser and ack primitives. Critical gate. | — |
| 24 | Integration tests | **Met** | `internal/pkg/pipeline/task/acking_test.go` wires multiple tasks end-to-end; `test/pipelines/*_test.yaml` (17 files) are runnable full-pipeline scenarios documented in the README. Critical gate. | — |
| 25 | Snapshot / golden-file tests | **Gap** | No `testdata/` golden directory | Capture expected pipeline outputs as `test/pipelines/*.golden.json` and diff on run. |
| 26 | Contract tests (Pact) | **Not applicable** | No owned network API; pipeline engine is a library/CLI, and integrations (Kafka, SQS, HTTP, SFTP) test the wire directly | Integration + per-task READMEs cover the equivalent surface. |
| 27 | End-to-end tests (Playwright) | **Not applicable** | No browser UI in this repo | CLI/pipeline surface is exercised by `test/pipelines/*_test.yaml` integration runs. |
| 28 | Visual regression tests | **Not applicable** | No visual surface | — |
| 29 | Test coverage thresholds | **Gap** | `makefile` scaffolds a `test` target driven by `directories=` — the variable is empty, so `make test` runs nothing; no CI coverage gate | Populate `directories=` (or migrate to `go test ./...`), publish coverage from CI, and gate at a starting minimum. |
| 30 | Mutation testing | **Gap** | No `go-mutesting` or equivalent | Add `go-mutesting` on the pipeline core packages as an advisory nightly job. |
| 31 | Load / performance benchmarks | **Gap** | No `*_bench_test.go` or benchstat report | Add Go benchmarks for jq/xpath/converter hot paths and record baselines. |
| 32 | Flaky test quarantine | **Gap** | No quarantine convention or CI job | Adopt a `t.Skip("quarantine: <ticket>")` convention and a monthly quarantine review. |
| 33 | Structured CI output | **Gap** | CI does not run tests; no `-json` / JUnit output | Add `go test -json ./... \| tee test-report.json` and upload as a workflow artifact. |
| 34 | Deterministic test fixtures | **Met** | `test/pipelines/*.yaml`, `bcrypt_cases.json`, `greetings.json` fixed inputs | — |
| 35 | Smoke tests for deploys | **Not applicable** | Repo produces a distributable CLI binary and Docker image; no deployed service to smoke-test | The release workflow only publishes artifacts. |

## Environment & Tooling

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 36 | Devcontainer config | **Gap** | No `.devcontainer/` | Add a `.devcontainer/` pinned to the Go version in `go.mod` and the CGO/librdkafka toolchain from `build/Dockerfile`. |
| 37 | One-command setup | **Partial** | `makefile` has `all: build test`, but `test` iterates `directories=` which is empty — running `make` builds only | Wire `test` to `go test ./...` (with coverage), and add a `make dev` alias documenting the CGO/librdkafka prerequisite. |
| 38 | Seed scripts for local databases | **Not applicable** | No owned database | — |
| 39 | MCP servers for external tools | **Met** | Toolsmith-managed MCP access (inherited for verified `patterninc` repos) | — |
| 40 | Scoped secrets per environment | **Partial** | Release workflow uses GitHub `secrets.DOCKERHUB_USERNAME`/`DOCKERHUB_TOKEN`; runtime pipelines pull via AWS Parameter Store (`{{ secret "/prod/api/token" }}`), but there is no documented dev/staging/prod scoping convention | Document the dev/stage/prod SSM prefix layout in README/AGENTS and validate the prefix at pipeline load. Critical gate. |
| 41 | Preview environments per PR | **Not applicable** | Distributable CLI + library, not a deployed hosted service | — |
| 42 | Hot-reload / watch mode | **Not applicable** | Go compile-and-run CLI; pipeline runs are batch — no long-running dev-server surface for hot-reload | — |
| 43 | Structured logging (JSON) | **Gap** | `internal/pkg/pipeline/pipeline.go` and `pipeline/task/task.go` use `fmt.Println` for status/errors; no `log/slog` or structured logger | Switch to `log/slog` with JSON output and include task name, record ID, and pipeline run ID. |
| 44 | Observable traces and metrics | **Gap** | No OTel, Datadog, or Prometheus instrumentation | Emit per-task record counts and durations as OTel metrics; add span-per-task tracing. |
| 45 | Feature flags with local overrides | **Gap** | No `patterninc/toggles-go` or LaunchDarkly integration | Add a lightweight flag hook (env-var overrides in dev) for opt-in features like DAG mode, once it exits EXPERIMENTAL. |
| 46 | Database migration tooling | **Not applicable** | Repo owns no database | — |
| 47 | Dependency update automation | **Met** | Inherited Pattern Wiz policy for verified `patterninc` repos | — |
| 48 | Reproducible builds (lockfiles) | **Met** | `go.mod` + `go.sum` pinned; Dockerfile pins Alpine 3.20 + Go 1.24.7; release workflow uses matching Go version. Critical gate. | — |

## Documentation & Context (agent dispatch)

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 49 | Agent-dispatch manifest | **Gap** | No `.agents/pattern-agents.json` | Add `.agents/pattern-agents.json` with `schema_version`, `github.repo: patterninc/caterpillar`, ClickUp list, Slack channel, and Datadog service metadata. `aws[]` may be omitted (no owned AWS footprint). |

## Prioritized recommendations

1. **[S] Gap — AGENTS.md (item 2, critical gate):** Author `AGENTS.md` at repo root covering build (`go build ./cmd/caterpillar`), test (`go test ./...`), the pipeline-YAML rules, and the per-task README convention.
2. **[S] Partial — required CI (item 16, critical gate):** Extend `.github/workflows/ci.yaml` to run `go test ./...` and `golangci-lint`, then mark both required in the `main + releases` ruleset.
3. **[M] Partial — scoped secrets (item 40, critical gate):** Document the SSM Parameter Store dev/stage/prod prefix layout in README/AGENTS and reject unrecognized prefixes at pipeline load.
4. **[S] Gap — skills (item 1):** Add `.claude/skills/` entries for "add a pipeline task" and "cut a release".
5. **[S] Gap — ADRs (item 3):** Record the DAG execution model, `fail_on_error` semantics, and back-pressure design in `docs/adr/`.
6. **[S] Gap — runbooks (item 4):** Add `docs/runbooks/release.md` and `docs/runbooks/rotate-dockerhub-credentials.md`.
7. **[S] Gap — on-call playbook (item 8):** Add a triage playbook for downstream failures caused by pipeline runs.
8. **[S] Gap — linters (item 10):** Add `.golangci.yml` with govet/staticcheck/errcheck/ineffassign and run it in CI.
9. **[S] Gap — formatters (item 11):** Add a `gofmt -l` (fail-if-nonempty) job to CI.
10. **[S] Gap — pre-commit (item 13):** Add `.pre-commit-config.yaml` running gofmt, go vet, and golangci-lint.
11. **[S] Gap — max complexity (item 21):** Enable `gocyclo`/`gocognit` in golangci-lint.
12. **[S] Gap — import boundaries (item 22):** Enforce cross-package rules with `depguard`.
13. **[S] Gap — dependency allow-list (item 17):** Add depguard `allow` rules for top-level imports.
14. **[S] Gap — license compliance (item 18):** Add `go-licenses check ./...` to CI with an allow-list.
15. **[S] Gap — coverage thresholds (item 29):** Populate the makefile `directories=` list (or migrate to `go test ./...`) and publish coverage.
16. **[S] Gap — structured CI output (item 33):** Emit `go test -json` output and upload as an artifact.
17. **[M] Gap — structured logging (item 43):** Move to `log/slog` JSON, tagging task name, record ID, run ID.
18. **[M] Gap — traces and metrics (item 44):** Instrument OTel counters and spans per task.
19. **[M] Gap — feature flags (item 45):** Wire `patterninc/toggles-go` (or env-var fallback) for opt-in features like DAG mode.
20. **[S] Partial — changelog (item 7):** Add `CHANGELOG.md` (or link the auto-generated release notes) and require a `## Migration` block for breaking changes.
21. **[S] Partial — one-command setup (item 37):** Fix the empty `directories=` bug in `makefile` so `make test` runs the suite.
22. **[S] Gap — devcontainer (item 36):** Add `.devcontainer/` pinned to Go 1.24.7 + Alpine 3.20 + librdkafka to match `build/Dockerfile`.
23. **[M] Gap — snapshot tests (item 25):** Add `test/pipelines/*.golden.json` outputs and diff on run.
24. **[M] Gap — load benchmarks (item 31):** Add Go benchmarks for jq, xpath, and converter hot paths.
25. **[S] Gap — flaky quarantine (item 32):** Adopt a `t.Skip("quarantine: <ticket>")` convention with a monthly review.
26. **[L] Gap — mutation testing (item 30):** Add `go-mutesting` on core packages as a nightly advisory job.
27. **[S] Gap — agent-dispatch manifest (item 49):** Add `.agents/pattern-agents.json` (`aws[]` optional).

## Declined practices

| # | Practice | Rationale |
|---|----------|-----------|
| 5 | API contract docs | CLI + pipeline engine with no owned network API; per-task READMEs cover the equivalent config contract. |
| 12 | Type checking | Go is statically typed; the compiler enforces this on every build. |
| 26 | Contract tests | No owned network API; pipeline tasks test the wire (Kafka, SQS, HTTP) directly. |
| 27 | End-to-end tests | No browser UI in this repo. |
| 28 | Visual regression tests | No visual surface. |
| 35 | Smoke tests for deploys | Repo publishes a CLI binary and Docker image; no deployed service to smoke-test. |
| 38 | Seed scripts for local DBs | Repo owns no database. |
| 41 | Preview environments per PR | Distributable binary/library, not a deployed hosted service. |
| 42 | Hot-reload / watch mode | Compile-and-run CLI; pipeline runs are batch. |
| 46 | Database migration tooling | Repo owns no database. |

## Beyond the checklist

- Per-task `README.md` under `internal/pkg/pipeline/task/*/` documents each task's config surface — a repeatable convention that gives agents (and humans) a predictable place to look, well beyond what item 6 rewards.
- The `test/pipelines/*_test.yaml` suite is a self-documenting integration harness: each YAML is both an example in the README and a runnable regression test, so contributors adding a new task write both the docs and the test in the same file.
- The release pipeline builds and pushes multi-arch (`linux/amd64,linux/arm64`) Docker images plus versioned GitHub Releases from a single workflow, keeping distribution surfaces in sync with source tags.
- Conventional-commit discipline is unusually consistent across ~90 tags, which makes auto-generated release notes usable without further curation.
