# Engineering Best Practices Audit — caterpillar

| | |
|---|---|
| **Audit date** | 2026-08-20 |
| **Auditor** | Claude — gauge-repo skill |
| **Checklist version** | `references/best-practices.md` (49-item engineering best-practices checklist) |

## Repo profile

Caterpillar is a Go data-ingestion and processing pipeline tool (module `github.com/patterninc/caterpillar`, Go 1.25) — a single CLI binary that runs YAML-defined pipelines chaining task types (HTTP, S3, SQS, SNS, Kafka, SFTP, JQ, XPath, converters, etc.). It has no browser UI and no database of its own; persistence, when it happens, is in the external systems its tasks talk to. It ships two ways: a GitHub Release binary (`.github/workflows/release.yaml`) and a multi-arch Docker image pushed to Docker Hub (`patternoss/caterpillar`) — there is no Terraform, ECS, or Lambda config in this repo, so how/where the binary or image is actually run in AWS could not be verified from this repo alone (it is registered in Backstage as a `service` owned by `dev-data-acquisition`, `Environment: stage`). It integrates heavily with AWS (S3, SQS, SNS, SSM Parameter Store, Kafka via confluent-kafka-go), so AWS-adjacent items are judged against that integration surface even without in-repo deploy config. 14 people have committed (`git shortlog`), so this is a small-team-owned, actively developed repo, not a solo project. No `.agents/pattern-agents.json` or sibling manifest, no `AGENTS.md`/`CLAUDE.md`, no `docs/` directory — documentation lives in a top-level `README.md`, `DAG_README.md`, and 22 per-task-type `README.md` files under `internal/pkg/pipeline/task/*/`.

## Scorecard

| Metric | Value |
|--------|-------|
| Raw score | 7 / 49 |
| Adjusted compliance | 40% ((Met + N/A + 0.5×Partial) / 49 = 19.5 / 49) |

### Per-category breakdown

| Category | Met | Partial | Gap | N/A |
|----------|-----|---------|-----|-----|
| Documentation & Context | 2 | 3 | 4 | 0 |
| Guardrails & Enforcement | 3 | 2 | 8 | 0 |
| Testing & Feedback Loops | 0 | 2 | 7 | 4 |
| Environment & Tooling | 2 | 2 | 6 | 4 |

## Documentation & Context

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 1 | Skills / reusable prompt workflows | **Gap** | No `.claude/skills/`, `.claude/commands/`, or equivalent found | Not a large lift given the repo already has strong per-task README structure to draw from. |
| 2 | AGENTS.md | **Gap** | No `AGENTS.md`, `CLAUDE.md`, or `.cursorrules` found anywhere in the tree | Add a top-level `AGENTS.md` covering build (`go build ./cmd/caterpillar/caterpillar.go`), the (currently inert) test target, and the task-package layout convention. |
| 3 | Architecture decision records (ADRs) | **Gap** | No `docs/adr/`, `docs/decisions/`, or similar | The linear-vs-DAG pipeline execution model (`DAG_README.md`) is exactly the kind of decision an ADR should capture; currently only described as "EXPERIMENTAL" in a feature doc. |
| 4 | Runbooks | **Partial** | `DAG_README.md` §"Troubleshooting"/"Common Issues" (lines 144–166) covers DAG-specific failure modes | Scoped to one feature (DAG syntax/perf issues); no broader operational runbook (e.g., how to redeploy, rotate the Docker Hub credentials used in `release.yaml`, recover a stuck pipeline). |
| 5 | API contract docs (OpenAPI / protobuf) | **Met** | 22 per-task-type `README.md` files (e.g. `internal/pkg/pipeline/task/http/server/README.md`) each carry a "Configuration Fields" table | This tool's real "API" is its YAML pipeline schema, not a REST surface — the per-task config tables are the equivalent contract doc and are treated as satisfying this item. |
| 6 | README with setup & run instructions | **Partial** | `README.md` §"How to Run Locally" (prerequisites, clone, build, run) | Verified the literal `go build -o caterpillar cmd/caterpillar/caterpillar.go` command locally — it succeeds, but only because a C toolchain (clang/gcc) was already present for the cgo-based `confluent-kafka-go` dependency; README never mentions this requirement, and `build/Dockerfile`'s own comment ("Alpine 3.20 ships librdkafka 2.4.0; confluent-kafka-go v2 requires 2.14.0+") flags a real version constraint the README is silent on. |
| 7 | Changelog with migration notes | **Partial** | `.github/workflows/release.yaml` sets `generate_release_notes: true` on tag pushes | GitHub auto-generates release notes from merged PR titles; there is no dedicated `CHANGELOG.md` and no explicit migration-notes convention for breaking changes. |
| 8 | On-call playbooks | **Gap** | No incident-response/paging docs found | Given the tool runs production data pipelines (Backstage-registered, `dev-data-acquisition`-owned), a short "what to check when a pipeline run fails silently" playbook would add real value — note the README's own Error Handling section (see item 43) already describes a footgun worth documenting a response to. |
| 9 | CODEOWNERS | **Met** | `.github/CODEOWNERS` — `* @patterninc/data-acquisition`; enforced via the `main + releases` ruleset's `require_code_owner_review: true` | — |

## Guardrails & Enforcement

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 10 | Linters (ESLint, golangci-lint, ruff) | **Gap** | No `.golangci.yml`/`.golangci.yaml` found; `.github/workflows/ci.yaml` only runs `go build` | Add a `golangci-lint` CI job (staticcheck + govet + revive at minimum). |
| 11 | Formatters (Prettier, gofmt, Black) | **Partial** | `gofmt -l .` on the checked-out tree returns no files (code is already gofmt-clean) | The convention is followed in practice but nothing enforces it going forward — no CI step or pre-commit hook runs `gofmt -l`/`gofmt -s`. |
| 12 | Type checking (TypeScript strict, mypy) | **Met** | Go's compiler performs static type checking on every build; `.github/workflows/ci.yaml`'s `go build` step exercises this on every PR | Equivalent credited per Step 3 rule 1 — Go's own compiler is the type checker here. |
| 13 | Pre-commit hooks | **Gap** | No `.pre-commit-config.yaml` or `.githooks/` found | — |
| 14 | Commit message conventions | **Partial** | `git log --oneline` shows a mix: some Conventional Commits (`fix(jq): ...`, `feat(http task): ...`) alongside many all-caps prefixes (`FEAT:`, `FIX:`, `HOTFIX:`, `REFACTOR:`) and plain messages (`chore: upgrade dependancies`) | No enforcement (no commitlint, no CI check) — recommend standardizing on the Conventional Commits style already used by several recent commits. |
| 15 | Branch protection rules | **Met** | `gh api repos/patterninc/caterpillar/rulesets` returns two active rulesets: org-level `require-pr-review` (all branches) and repo-level `main + releases` (`refs/heads/main`, `refs/heads/release/**/*`) — both block `deletion`/`non_fast_forward` and require ≥1 approving review; the repo-level ruleset also requires code-owner review | Classic branch-protection API 404s, but rulesets are the modern equivalent (Step 3 rule 1) and are confirmed active. |
| 16 | Required CI checks before merge | **Gap** | Neither ruleset's `rules[]` array contains a `required_status_checks` entry (only `deletion`, `non_fast_forward`, `creation`, `update`, `pull_request`, and, on the org ruleset, `copilot_code_review`) | `.github/workflows/ci.yaml`'s build job exists but is not wired as a required check — a PR can merge even if the build fails. Cheapest high-leverage fix in this audit. |
| 17 | Dependency allow-lists / deny-lists | **Gap** | None found | Low priority for a 14-contributor repo; Wiz/Dependabot (item 47) already provide reactive vulnerability coverage. |
| 18 | License compliance scanning | **Gap** | No `go-licenses` or similar job in CI | — |
| 19 | Secret scanning (truffleHog) | **Gap** | No gitleaks/truffleHog in CI or hooks; `gh api repos/.../secret-scanning/alerts` returned 404 (could not verify whether GitHub's native secret scanning is enabled — 404 can mean either disabled or a permissions gap, so this specific sub-check is inconclusive) | Add gitleaks (or confirm/enable GitHub Advanced Security secret scanning) at the CI or pre-commit layer regardless of the native-feature status. |
| 20 | SAST / static analysis gates | **Gap** | No CodeQL/gosec/semgrep workflow found | Wiz (item 47) covers dependency CVEs (SCA), not source-level SAST — this is a distinct gap. |
| 21 | Max complexity limits | **Gap** | No `gocyclo`/complexity config found (no `.golangci.yml` at all) | — |
| 22 | Import boundary enforcement | **Met** | All non-`cmd` code lives under `internal/pkg/`, and Go's compiler enforces that `internal/` packages cannot be imported outside the module — a real, language-level import boundary | Equivalent credited per Step 3 rule 1. |

## Testing & Feedback Loops

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 23 | Unit tests | **Partial** | Exactly one test file in the whole repo, `internal/pkg/pipeline/dag_test.go` (testify-based); `go test ./internal/pkg/pipeline/...` confirms all ~20 task subpackages report `[no test files]`; `makefile`'s `test` target iterates over a `directories=` variable that is currently empty, so `make test` is a silent no-op | Populate `directories` in the makefile and add unit tests for at least the task packages with the most logic (jq, converter, http). |
| 24 | Integration tests | **Gap** | None found | The task types (S3, SQS, SNS, Kafka, SFTP) are exactly the surface that benefits most from LocalStack/dockerized-Kafka integration tests; currently zero coverage. |
| 25 | Snapshot / golden-file tests | **Gap** | `test/pipelines/` holds 49 example pipeline YAMLs plus fixture data (`birds.txt`, `greetings.json`, etc.) that look like golden-file candidates, but nothing runs them automatically or diffs actual vs. expected output | Wire a subset of these into a golden-file test harness. |
| 26 | Contract tests (Pact) | **Not applicable** | CLI/library tool driving user-defined pipelines against arbitrary external systems, not a service with its own consumer clients | No fixed client-service contract of the kind Pact verifies. |
| 27 | End-to-end tests (Playwright) | **Not applicable** | No browser UI in the profile | — |
| 28 | Visual regression tests | **Not applicable** | No UI surface | — |
| 29 | Test coverage thresholds | **Gap** | `makefile`'s `report` target produces an HTML coverage report, but it depends on the empty-`directories` `test` target above and is not run or gated in CI | Fix item 23 first, then add a coverage floor in CI. |
| 30 | Mutation testing | **Gap** | None found | Low priority — sequence after real unit-test coverage exists (item 23); mutation testing against a single test file would add no signal today. |
| 31 | Load / performance benchmarks | **Gap** | No `func Benchmark...` found anywhere in the repo | Throughput matters here (channel-buffered concurrent task pipeline) — worth benchmarking the hot paths (jq, converter, channel handoff). |
| 32 | Flaky test quarantine | **Not applicable** | The test suite is a single deterministic unit test (`TestCleanInput` in `dag_test.go`) with no observed or plausible flakiness | Premature to build quarantine tooling before a broader suite exists; revisit once item 23/24 land. |
| 33 | Structured CI output | **Gap** | `.github/workflows/ci.yaml` only runs `go build`; no test step, so no JUnit/machine-readable output exists | — |
| 34 | Deterministic test fixtures | **Partial** | `test/pipelines/*.yaml` and companion data files (`birds.txt`, `greetings.json`, `bcrypt_cases.json`, etc.) are fixed, deterministic inputs | They exist but aren't wired into any automated test (see item 25) — currently used only for manual/local exercising. |
| 35 | Smoke tests for deploys | **Gap** | `.github/workflows/release.yaml` builds and pushes the binary/image with no post-build "does it run" step | A one-line smoke test (e.g., run `test/pipelines/hello_name.yaml` against the freshly built binary) would be cheap and catch a broken release immediately. |

## Environment & Tooling

| # | Practice | Status | Evidence | Recommendation / rationale |
|---|----------|--------|----------|----------------------------|
| 36 | Devcontainer config | **Gap** | No `.devcontainer/`; `build/Dockerfile` needs Alpine's `edge/community` repository for a current-enough `librdkafka-dev`/`librdkafka`, a detail specific to the production image, not the local build | Plain local `go build` works fine on a machine with a C toolchain (verified), but a devcontainer would still standardize the toolchain/Kafka-support story across contributors and CI runners that may lack gcc/clang preinstalled. |
| 37 | One-command setup (make dev) | **Partial** | `makefile` provides `build`, `test`, `report` targets | No single command bootstraps a fully working dev environment (build + run a sample pipeline); `test` is currently inert (item 23). |
| 38 | Seed scripts for local databases | **Not applicable** | No database in the profile — the tool has no persistence layer of its own | — |
| 39 | MCP servers for external tools | **Gap** | None found | Consistent with items 1–2: no agent-facing tooling exists in this repo yet. Low priority until basic agent docs (AGENTS.md) land. |
| 40 | Scoped secrets per environment | **Partial** | `README.md` §"Secrets" documents `{{ secret "/prod/api/token" }}` — SSM Parameter Store paths that can encode environment (e.g., `/prod/...` vs `/staging/...`) | The convention supports environment scoping, but nothing in the tool enforces or validates that a pipeline's secret paths match its intended environment — purely convention-based today. |
| 41 | Preview environments per PR | **Not applicable** | Deployment model is a published binary/Docker image (`release.yaml`), not a deployed web service with a URL to preview | — |
| 42 | Hot-reload / watch mode | **Gap** | None found | Low priority for a CLI batch tool (`go build && ./caterpillar -conf ...` is already a fast loop), but a `watch`/`entr`-based target for pipeline-config iteration would still help. |
| 43 | Structured logging (JSON) | **Gap** | `internal/pkg/pipeline/pipeline.go` uses `fmt.Println`; README's own "Error Handling" section states errors are "reported as `error in <task name>: <error>` on stdout, interleaved with record output rather than sent to stderr, which is why they are easy to miss" | This is a self-documented anti-pattern in the repo's own README — high-value, medium-effort fix: move errors to stderr at minimum, structured JSON logging as a stretch goal. |
| 44 | Observable traces and metrics | **Gap** | No OTel instrumentation; `prometheus/client_golang` appears only as an indirect (transitive) dependency, not used directly in any `.go` file | — |
| 45 | Feature flags with local overrides | **Not applicable** | Versioned CLI/Docker-image releases, not a long-running service that needs runtime toggles; the "EXPERIMENTAL" DAG feature is gated by documentation/versioning rather than a flag, which is the right mechanism for this deployment model | — |
| 46 | Database migration tooling | **Not applicable** | No database — see item 38 | — |
| 47 | Dependency update automation | **Met** | "Pattern Security Automation" (Wiz-backed) opens CVE auto-remediation PRs — e.g. #78 (`CVE-2026-32287`, antchfx/xpath) and #80 (`CVE-2026-54063`, xuri/excelize/v2); both PRs were closed rather than merged directly, but `go.mod` shows both packages already at or past the fixed versions (`xpath v1.3.8`, `excelize v2.11.0`), and `gh api .../dependabot/alerts` shows 20 alerts (7 critical, 4 high, 9 medium), all in `state: fixed` | The team is actually keeping up with flagged CVEs (via other PRs, e.g. #90 "chore: upgrade dependancies"), even though the automation's own PRs aren't the merge path — worth tightening so the auto-PR is the actual fix path rather than a duplicate signal. |
| 48 | Reproducible builds (lockfiles) | **Met** | `go.sum` committed and up to date; `build/Dockerfile` pins exact base image tags (`golang:1.24.7-alpine`, `alpine:3.20`) rather than `latest` | — |
| 49 | Agent-dispatch manifest (`.agents/pattern-agents.json`) | **Gap** | No `.agents/pattern-agents.json` or sibling `.yml`/`.yaml` found anywhere in the repo | The repo has a real AWS integration surface (S3/SQS/SNS/SSM/Kafka task types) but no Terraform/ECS/Lambda config of its own, so whether it has an actual AWS *deployment* footprint (vs. just AWS-integrated task logic) could not be fully verified from this repo alone — treated as a Gap rather than N/A because the manifest's GitHub/ClickUp/Slack/Datadog fields are valuable regardless of the `aws[]` sub-check's applicability. |

## Prioritized recommendations

1. **[S]** Add a `required_status_checks` rule (the existing `ci.yaml` build job) to the `main + releases` ruleset — right now a failing build can still merge (#16).
2. **[S]** Populate the empty `directories=` variable in `makefile`'s `test` target and add a `test` step to `.github/workflows/ci.yaml` so the one existing unit test — and any future ones — actually run in CI (#23, #29).
3. **[M]** Move error output to stderr (minimum) or adopt structured JSON logging — README's own Error Handling section already documents this as an easy-to-miss footgun (#43).
4. **[M]** Add a `golangci-lint` CI job (govet, staticcheck, gofmt check at minimum) — code is gofmt-clean today by convention only, with nothing enforcing it going forward (#10, #11, #21).
5. **[M]** Add integration tests (LocalStack + dockerized Kafka) for the S3/SQS/SNS/Kafka task types, which make up most of the repo's functional surface and currently have zero automated coverage (#24).
6. **[S]** Add a `.agents/pattern-agents.json` manifest (GitHub repo, ClickUp list, Slack channel, Datadog service) so this Pattern-owned repo is agent-dispatch-ready like its peers (#49).
7. **[S]** Document the cgo/C-toolchain requirement (and the librdkafka version constraint called out in `build/Dockerfile`'s own comment) in README's local-setup section (#6).
8. **[L]** Wire a subset of the 49 example pipeline YAMLs in `test/pipelines/` into an automated golden-file test harness that runs each and diffs output against a committed expected result (#25, #34).
9. **[S]** Add a lightweight smoke test to `release.yaml` — run `test/pipelines/hello_name.yaml` against the freshly built binary before publishing (#35).
10. **[S]** Enforce Conventional Commits (commitlint or a CI check) — several recent commits already use the style; make it universal (#14).

## Declined practices

| # | Practice | Rationale |
|---|----------|-----------|
| 26 | Contract tests (Pact) | CLI/library tool driving user-defined pipelines against arbitrary external systems — no fixed consumer-service contract to verify. |
| 27 | End-to-end tests (Playwright) | No browser UI in the profile. |
| 28 | Visual regression tests | No UI surface. |
| 32 | Flaky test quarantine | Test suite is a single deterministic unit test today; no flakiness problem exists yet to quarantine against. |
| 38 | Seed scripts for local databases | No database — the tool has no persistence layer of its own. |
| 41 | Preview environments per PR | Deployment model is a published binary/Docker image, not a deployed web service with a URL to preview. |
| 45 | Feature flags with local overrides | Versioned CLI/Docker-image releases; the "EXPERIMENTAL" DAG feature is appropriately gated by documentation and versioning rather than a runtime flag. |
| 46 | Database migration tooling | No database — see item 38. |

## Beyond the checklist

- 22 per-task-type `README.md` files (`internal/pkg/pipeline/task/*/README.md`) each document configuration fields, behavior, and example YAML — unusually thorough per-package documentation for a Go repo of this size.
- `DAG_README.md` includes a dedicated "Troubleshooting" section with concrete error-message-to-fix mappings for the DAG feature.
- Org-wide automated CVE scanning and remediation (Wiz / "Pattern Security Automation") is active on this repo and its fixes are being kept current, even where the automation's own PRs aren't the merge path (see item 47).
- The Docker release image pins exact base-image tags and documents *why* it needs Alpine's edge/community repo (a specific `librdkafka` version constraint) directly in a Dockerfile comment — a good, low-ceremony way to preserve tribal knowledge at the point of use.
