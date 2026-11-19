# AGENTS.md

Instructions for AI coding agents (and humans) working in this repo. `CLAUDE.md` is a symlink to this file.

Caterpillar is a single Go CLI, `caterpillar -conf <pipeline.yaml>`, that runs a YAML-defined pipeline of tasks connected by Go channels. It has no server or database of its own. It ships as GitHub release binaries and the multi-arch Docker image `patternoss/caterpillar` (see `.github/workflows/release.yaml`). The repo is **public**: never commit credentials, tokens, or customer data.

## Commands

| What | Command |
| --- | --- |
| Build | `go build -o caterpillar ./cmd/caterpillar/caterpillar.go` |
| Run a pipeline | `./caterpillar -conf test/pipelines/hello_name.yaml` |
| Tests (as CI runs them) | `go test -race ./...` |
| Vet (as CI runs it) | `go vet -copylocks=false ./...` |
| Format check | `gofmt -l $(git ls-files '*.go')` must print nothing |
| Docker image | `docker build -f build/Dockerfile .` |

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
| `test/pipelines/` | Example pipelines and fixture data, used for manual runs and the release check |
| `docs/` | ADRs, runbooks, on-call playbook, readiness audit |

## Adding or changing a task type

Follow `.claude/skills/add-task-type/SKILL.md`. In short:

1. New package under `internal/pkg/pipeline/task/<type>/` with a struct that embeds `task.Base` and a `New() (task.Task, error)` that sets defaults.
2. Register it in `supportedTasks` in `internal/pkg/pipeline/tasks.go`.
3. Put client creation in `Init()`; `tasks.go` calls it once after decoding and `validator` checks.
4. In `Run`, for every input record: send results with `SendData`/`SendRecord`, then `ack.Release(r.Context)` on success or `ack.Reject(r.Context)` on failure. Returning an error stops that worker.
5. Write `README.md` (Configuration Fields table, behaviour, example) and add the type to the Supported Tasks list in the top-level `README.md`.
6. Add unit tests next to the code (`*_test.go`), driving `Run` with channels.

## Invariants (break these and pipelines silently lose data)

- **Acks:** each `SendRecord` registers one ack branch before the record is visible downstream (`task.go`). A task that consumes a record must release or reject it exactly once. Never add a fan-out count by hand.
- **Deferred acks settle in `Finish()`**, which the pipeline calls after the task's output channel closes (`task.Finisher`). Waiting inside `Run` deadlocks.
- **Errors go to stdout** as `error in <task>: <err>`, interleaved with data (README "Error Handling"). A run exits non-zero only for tasks with `fail_on_error: true`.
- **A `file` task used as a source always runs with concurrency 1** (`runTaskConcurrently`).
- **`channel_size` defaults to 10,000.** A task that stops early while its upstream still produces blocks the run forever once the channel fills.
- **Secrets are resolved once, at load time,** with the caller's AWS credentials. Never log a rendered config.
- **Release asset names `caterpillar` and `caterpillar-linux-arm64` must not change:** downstream images download them by name.

## Tests

- Tests live next to the code (`*_test.go`) and must be offline and deterministic: no network, AWS, Kafka, or wall-clock assertions. Use `t.TempDir()` for files. Match the style of `internal/pkg/pipeline/dag_test.go` and `internal/pkg/pipeline/task/acking_test.go` (testify, table-driven).
- Ack behaviour is covered by `internal/pkg/pipeline/task/acking_test.go` and `internal/pkg/pipeline/ack/ack_test.go`; extend them when a task's ack handling changes.
- **Flaky tests:** do not retry in a loop or add sleeps. Quarantine with `t.Skip("flaky: <ClickUp task id>")` as the first line, link the task in the PR, and fix or delete it within two weeks. `go test -run <Name> -count=50 -race` reproduces most races.

## CI

| Check (job name) | Workflow | When | What it runs |
| --- | --- | --- | --- |
| `build` | `.github/workflows/ci.yaml` | Pull requests to `main` | build, `go vet -copylocks=false`, `go test -race ./...` |
| Release jobs | `.github/workflows/release.yaml` | Pushes to `main`, `v*` and `latest` tags | amd64 and arm64 builds, each run on `test/pipelines/hello_name.yaml`; GitHub release; Docker Hub image |

Wiz scanners (secrets, SAST, dependencies, IaC) report on every PR through Pattern's org-wide policy.

## Conventions

- PR titles and commits: Conventional Commits (`feat(http): ...`, `fix(sqs): ...`). The squash-merge title becomes the release note.
- Update `CHANGELOG.md` under `## [Unreleased]` for any change to a task's fields or behaviour; breaking changes need a **Migration** note.
- Match the existing style: backtick string literals, early returns, blank lines inside function bodies, comments that explain *why*.
- Keep code `gofmt`-clean and `go vet -copylocks=false` clean.
- Prefer `aws-sdk-go-v2`, `gopkg.in/yaml.v3` and the standard library's `errors`; do not add AWS SDK v1, `github.com/pkg/errors` or `gopkg.in/yaml.v2` as direct dependencies.

## Where to look next

- Architecture decisions: [`docs/adr/`](docs/adr/README.md)
- Operations: [`docs/runbooks/`](docs/runbooks/README.md), [`docs/oncall/playbook.md`](docs/oncall/playbook.md)
- Secret handling: [`docs/secrets.md`](docs/secrets.md)
- DAG syntax: [`DAG_README.md`](DAG_README.md)
- Agent metadata: [`.agents/pattern-agents.json`](.agents/pattern-agents.json)
