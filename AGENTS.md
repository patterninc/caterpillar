# AGENTS.md

Instructions for AI coding agents (and humans) working in this repo. `CLAUDE.md` is a symlink to this file.

Caterpillar is a single Go CLI, `caterpillar -conf <pipeline.yaml>`, that runs a YAML-defined pipeline of tasks connected by Go channels. It has no server or database of its own. It ships as GitHub release binaries and the multi-arch Docker image `patternoss/caterpillar` (see `.github/workflows/release.yaml`). The repo is **public**: never commit credentials, tokens, or customer data.

## Commands

| What | Command |
| --- | --- |
| One-command setup (toolchain check, build, tests, smoke, hooks) | `scripts/dev.sh` |
| Build | `go build -o caterpillar ./cmd/caterpillar/caterpillar.go` |
| Run a pipeline | `./caterpillar -conf test/pipelines/hello_name.yaml` |
| Unit and golden tests | `go test -race ./...` |
| Regenerate golden files | `go test ./test/golden/... -run Golden -update` (then review the diff) |
| Integration tests (LocalStack) | see [Integration tests](#integration-tests) |
| Benchmarks | `go test -run='^$' -bench=. -benchmem ./...` |
| Lint (golangci-lint v2.15.0, built with Go 1.27) | `golangci-lint run ./...` |
| Format check | `gofmt -l $(git ls-files '*.go')` must print nothing |
| Vet (as CI runs it) | `go vet -copylocks=false ./...` |
| Coverage floor | `go test -coverprofile=coverage.out ./... && scripts/check-coverage.sh coverage.out` |
| Dependency policy, licenses | `scripts/check-deps.sh`, `scripts/check-licenses.sh` |
| Smoke test a binary or image | `scripts/smoke.sh ./caterpillar`, `scripts/smoke.sh docker:patternoss/caterpillar:dev` |
| Watch mode | `PIPELINE=test/pipelines/<file>.yaml air` (config in `.air.toml`) |

**Toolchain:** Go version from `go.mod` (1.27). A C compiler is required because `confluent-kafka-go` builds with cgo; the module bundles librdkafka for glibc builds, and `build/Dockerfile` links Alpine's librdkafka (`-tags dynamic`), which must be 2.14.0 or newer.

## Layout

| Path | What lives there |
| --- | --- |
| `cmd/caterpillar/caterpillar.go` | Entry point: parses `-conf`, `config.Load`, `Pipeline.Run`, exits via `process.Bail` |
| `internal/pkg/config/` | Renders the pipeline file as a Go `text/template` before YAML decoding: `env`, `secret` (SSM Parameter Store, at load time), `macro`, `context` (per record), `indent` |
| `internal/pkg/pipeline/pipeline.go` | Linear runner (one channel between consecutive tasks) and DAG runner (`dag:` key) |
| `internal/pkg/pipeline/tasks.go` | `supportedTasks`: the registry mapping a YAML `type` to a constructor |
| `internal/pkg/pipeline/dag.go` | Parser and validator for `dag:` expressions (`a >> [b, c] >> d`) |
| `internal/pkg/pipeline/ack/` | Per-source-record acknowledgement tracking for at-least-once sources (SQS, Kafka) |
| `internal/pkg/pipeline/record/` | `Record` (ID, Origin, Data, Context) passed between tasks |
| `internal/pkg/pipeline/task/<type>/` | One package per task type, each with a `README.md` documenting its fields |
| `internal/pkg/jq/` | gojq wrapper plus custom functions (crypto, shuffle, sleep, translate) |
| `test/pipelines/` | Example pipelines and fixture data; several are run by the golden tests |
| `docs/` | ADRs, runbooks, on-call playbook, readiness audit |

## Adding or changing a task type

Follow `.claude/skills/add-task-type/SKILL.md`. In short:

1. New package under `internal/pkg/pipeline/task/<type>/` with a struct that embeds `task.Base` and a `New() (task.Task, error)` that sets defaults.
2. Register it in `supportedTasks` in `internal/pkg/pipeline/tasks.go`.
3. Put client creation in `Init()`; `tasks.go` calls it once after decoding and `validator` checks.
4. In `Run`, for every input record: send results with `SendData`/`SendRecord`, then `ack.Release(r.Context)` on success or `ack.Reject(r.Context)` on failure. Returning an error stops that worker.
5. Write `README.md` (Configuration Fields table, behaviour, example) and add the type to the Supported Tasks list in the top-level `README.md`.
6. Add unit tests next to the code and, for AWS types, a `//go:build integration` test.

## Invariants (break these and pipelines silently lose data)

- **Acks:** each `SendRecord` registers one ack branch before the record is visible downstream (`task.go`). A task that consumes a record must release or reject it exactly once. Never add a fan-out count by hand.
- **Deferred acks settle in `Finish()`**, which the pipeline calls after the task's output channel closes (`task.Finisher`). Waiting inside `Run` deadlocks.
- **Errors go to stdout** as `error in <task>: <err>`, interleaved with data (README "Error Handling"). A run exits non-zero only for tasks with `fail_on_error: true`.
- **A `file` task used as a source always runs with concurrency 1** (`runTaskConcurrently`).
- **`channel_size` defaults to 10,000.** A task that stops early while its upstream still produces blocks the run forever once the channel fills.
- **Secrets are resolved once, at load time,** with the caller's AWS credentials. Never log a rendered config.
- **Release asset names `caterpillar` and `caterpillar-linux-arm64` must not change:** downstream images download them by name.

## Tests

- Unit tests live next to the code (`*_test.go`), use `testify`, and must be offline and deterministic: no network, AWS, Kafka, or wall-clock assertions. Use `t.TempDir()` for files.
- Golden tests run selected `test/pipelines/*.yaml` and compare stdout with `test/golden/testdata/*.golden`. Only use pipelines whose output order is deterministic.
- <a id="integration-tests"></a>Integration tests are `*_integration_test.go` with `//go:build integration` and skip unless `AWS_ENDPOINT_URL` is set. Locally:
  ```bash
  docker run -d --rm -p 4566:4566 -e SERVICES=s3,sqs,sns,ssm localstack/localstack:4.14.0
  AWS_ENDPOINT_URL=http://localhost:4566 AWS_ENDPOINT_URL_S3=http://s3.localhost.localstack.cloud:4566 \
    AWS_REGION=us-east-1 AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
    go test -race -tags=integration -run Integration ./...
  ```
  The S3 client has no path-style option, so S3 needs LocalStack's wildcard hostname.
- Benchmarks are `*_bench_test.go`. `.github/workflows/bench.yml` compares each PR with its base and fails on a significant slowdown of more than 25%.
- **Flaky tests:** do not retry in a loop or add sleeps. Quarantine with `t.Skip("flaky: <ClickUp task id>")` as the first line, link the task in the PR, and fix or delete it within two weeks. `go test -run <Name> -count=50 -race` reproduces most races.
- Coverage may not drop below the floor in `scripts/check-coverage.sh`. Raise the floor when you add tests; never lower it to pass.

## CI (all on every pull request to `main`)

| Check (job name) | Workflow | What it runs |
| --- | --- | --- |
| `build` | `ci.yaml` | build, `go vet -copylocks=false`, `go test -race` |
| `lint`, `format`, `coverage`, `dependencies`, `licenses` | `quality.yml` | golangci-lint, gofmt, coverage floor with `go test -json` artifact, deny list and `go mod verify`, go-licenses |
| `integration` | `integration.yml` | LocalStack S3, SQS, SNS, SSM tests |
| `pr-title` | `pr-title.yml` | Conventional Commits PR title |
| `bench` | `bench.yml` | benchstat comparison (informational) |

After a release run, `smoke.yml` smoke-tests the published binaries and image. `mutation.yml` runs gremlins weekly.

## Conventions

- PR titles and commits: Conventional Commits (`feat(http): ...`, `fix(sqs): ...`). The squash-merge title becomes the release note.
- Update `CHANGELOG.md` under `## [Unreleased]` for any change to a task's fields or behaviour; breaking changes need a **Migration** note.
- Match the existing style: backtick string literals, early returns, blank lines inside function bodies, comments that explain *why*.
- Lint baselines live in `.golangci.yml` under `exclusions.rules`, scoped to one file each. Remove an entry when you fix its finding; never widen one.
- Do not add direct dependencies listed in `.github/dependency-policy.txt`.

## Where to look next

- Architecture decisions: [`docs/adr/`](docs/adr/README.md)
- Operations: [`docs/runbooks/`](docs/runbooks/README.md), [`docs/oncall/playbook.md`](docs/oncall/playbook.md)
- Secret handling: [`docs/secrets.md`](docs/secrets.md)
- DAG syntax: [`DAG_README.md`](DAG_README.md)
- Agent metadata: [`.agents/pattern-agents.json`](.agents/pattern-agents.json)
