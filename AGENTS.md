# caterpillar

A Go CLI that reads a YAML pipeline configuration and executes an ordered chain (or DAG) of typed data-processing tasks. Tasks receive `*record.Record` values from an upstream buffered channel, transform or route them, and emit results downstream.

The binary can operate in two modes depending on the pipeline:
- **Batch mode** (most pipelines) — runs, processes all records, and exits.
- **Server mode** — when the pipeline starts with an `http_server` task, the CLI acts as a long-running HTTP server. Incoming requests are converted to `*record.Record` values and emitted downstream to the rest of the pipeline. The binary does not exit until the server shuts down (or `end_after` is configured).

Apply these instructions to `/Users/prasadlohakpure/Desktop/go_projects/src/github.com/patterninc/caterpillar`. Treat paths and commands below as relative to that location unless explicitly stated otherwise.

## Tech Stack

- **Language:** Go 1.22+
- **Config format:** YAML (gopkg.in/yaml.v3)
- **Validation:** go-playground/validator/v10
- **Testing:** standard `go test`
- **CI:** GitHub Actions (`.github/workflows/ci.yaml`) — runs `go build ./cmd/caterpillar/caterpillar.go` on every PR to `main`; does not run tests

## Commands

| Action | Command |
|---|---|
| Build | `go build ./cmd/caterpillar/caterpillar.go` |
| Run a pipeline | `./caterpillar -conf <path/to/pipeline.yaml>` |

**Do NOT run `go build ./...`** — three packages currently fail to build (see Known Broken Packages).

## Directory Map

| Directory | Purpose |
|---|---|
| `cmd/caterpillar/` | Entry point — parses `-conf` flag, loads YAML into `Pipeline`, calls `p.Run()` |
| `internal/pkg/pipeline/` | Core runtime: `Pipeline` struct, `Run()`, channel wiring, DAG execution engine |
| `internal/pkg/pipeline/tasks.go` | Task registry (`supportedTasks` map) — add new task types here |
| `internal/pkg/pipeline/task/` | `Task` interface and `Base` struct shared by all task packages |
| `internal/pkg/pipeline/task/<name>/` | One directory per task type; each exports `New() (task.Task, error)` |
| `internal/pkg/pipeline/task/aws/` | AWS-specific tasks (currently `parameter_store`) |
| `internal/pkg/pipeline/record/` | `Record` type — the unit of data flowing between tasks |
| `internal/pkg/config/` | `config.String` — Go template string that expands `{{ secret }}` and `{{ context }}` at runtime |
| `internal/pkg/jq/` | JQ query wrapper used by `Base.Context` to extract per-record context values |
| `test/pipelines/` | Example YAML pipeline files (most require live AWS/Kafka — not safe to run without dependencies) |
| `docs/` | Onboarding checklist, code-mint framework docs, outcomes, skills-status |
| `.agents/` | code-mint AI infrastructure (skills, rules, reports, status JSON) |

## How to Add a New Task Type

Use `internal/pkg/pipeline/task/echo/echo.go` as the canonical minimal example.

### Step 1 — Create the package

```
internal/pkg/pipeline/task/<taskname>/
    <taskname>.go
    README.md        (document all YAML fields — follow echo/README.md format)
```

### Step 2 — Embed `task.Base` and implement `Run`

```go
package mytask

import (
    "github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
    "github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type myTask struct {
    task.Base `yaml:",inline" json:",inline"`  // required — enables YAML decoding and satisfies interface
    MyOption  string `yaml:"my_option,omitempty" json:"my_option,omitempty"`
}

func New() (task.Task, error) {
    return &myTask{}, nil
}

func (t *myTask) Run(input <-chan *record.Record, output chan<- *record.Record) error {
    for {
        r, ok := t.GetRecord(input)
        if !ok {
            break
        }
        // transform r.Data here
        t.SendRecord(r, output)  // also evaluates context: JQ expressions
    }
    return nil
}
```

Rules:
- `task.Base` **must** be embedded with `yaml:",inline" json:",inline"` — YAML decoding breaks without it.
- `New()` signature is always `func() (task.Task, error)` — keep it consistent with the registry even if init cannot fail.
- `Init()` is called once after unmarshaling, before `Run`. Override it only when you need one-time setup (e.g., creating a client). The `Base.Init()` no-op covers the default case.
- No `input` channel: task is a **source** (generates records). No `output` channel: task is a **sink**. Both present: transformer.
- `t.GetRecord(input)` safely reads from the channel and handles nil and close. `t.SendRecord(r, output)` evaluates `context:` JQ expressions and forwards the record.

### Step 3 — Register in `tasks.go`

Add an import and one entry to `supportedTasks` in `internal/pkg/pipeline/tasks.go`:

```go
import (
    "github.com/patterninc/caterpillar/internal/pkg/pipeline/task/mytask"
)

var supportedTasks = map[string]func() (task.Task, error){
    // existing entries ...
    `my_task_name`: mytask.New,
}
```

The map key becomes the `type:` value in pipeline YAML.

### Step 4 — Verify

```bash
go build ./cmd/caterpillar/caterpillar.go
```

## Task Interface

Defined in `internal/pkg/pipeline/task/task.go`:

```go
type Task interface {
    Run(<-chan *record.Record, chan<- *record.Record) error
    GetName() string
    GetFailOnError() bool
    GetTaskConcurrency() int
    Init() error  // called once after YAML unmarshal, before pipeline starts
}
```

`task.Base` satisfies every method except `Run`. Only override a method if you need non-default behavior.

## Pipeline YAML Structure

### Linear pipeline (implicit ordering by declaration)

```yaml
channel_size: 10000       # optional; default 10000; buffer size between tasks

tasks:
  - name: source           # unique within the pipeline; used in DAG expressions
    type: file             # must match a key in supportedTasks
    path: ./input.txt
    fail_on_error: false   # optional; default false
    task_concurrency: 1    # optional; default 1 — runs N concurrent workers for this task
    context:
      user_id: ".data | fromjson | .id"   # JQ expressions stored per-record as context

  - name: transform
    type: jq
    path: '{ id: "{{ context "user_id" }}", raw: .data }'

  - name: sink
    type: file
    path: output/{{ context "user_id" }}.json
```

The last declared task is the sink (no output channel). Execution flows top-to-bottom when no `dag:` key is present.

### DAG pipeline (experimental, v2.1.0+)

Add a `dag:` key. All names in the expression must match `name:` values declared under `tasks:`.

```yaml
tasks:
  - name: read
    type: file
    path: input.txt
  - name: branch_a
    type: jq
    path: '.data | { a: . }'
  - name: branch_b
    type: echo
  - name: sink
    type: file
    path: output/result.json

dag: read >> [branch_a, branch_b] >> sink
```

DAG syntax rules:
- `>>` — sequential step
- `[task1, task2]` — parallel fan-out/fan-in; records are broadcast to all branches and merged at the next `>>`
- Brackets must balance; no single-item groups; valid characters: letters, digits, `_`, `-`, `[`, `]`, `,`, `>`, whitespace
- Full syntax reference: `DAG_README.md`

## Known Broken Packages

Do not run `go test ./...` or `go build ./...`.

### 1. `internal/pkg/pipeline/task/kinesis/` — WIP, do not touch

- **Problem:** Imports `github.com/aws/aws-sdk-go-v2/service/kinesis`, which is not in `go.mod`. Does not build.
- **Status:** Untracked (not committed). Not registered in `tasks.go`.
- **Action:** Leave it alone. Do not add it to `go.mod` or register it in `tasks.go` without explicit instruction from the team.

### 2. `internal/pkg/pipeline/task/file/` — production code is fine; test build fails

- **Problem:** `file_success_path_test.go` (untracked) references `resolveSuccessObjectPath` and `writerSchemeFromPath`, which do not yet exist in the production package. The test was written ahead of the implementation.
- **Status:** `file.go` and `s3.go` build and run correctly. Only the test build is broken.
- **Action:** Do not run `go test ./internal/pkg/pipeline/task/file/`. The production package is safe to import and extend.

### 3. Root package — scratch files with duplicate `main()` declarations

- **Problem:** `push_sqs_localstack.go` and `push_kafka_message.go` both declare `package main` with a `func main()`, causing a duplicate-symbol error if you compile the root package.
- **Status:** Untracked (not committed). Used locally for manual testing.
- **Action:** Do not delete without asking. Do not attempt `go build .` at the repo root.

## Conventions

- `task.Base` must be embedded with `yaml:",inline" json:",inline"` in every task struct. Omitting the inline tag breaks YAML decoding.
- `New()` always returns `(task.Task, error)` — even tasks with no initialization that can fail.
- Template strings in YAML (`config.String`) expand `{{ secret "/path" }}` (fetched at startup) and `{{ context "key" }}` (evaluated per record from values set upstream via `context:` JQ expressions).
- Task names must be unique within a pipeline. In DAG mode, the `name:` field is the identifier used in the `dag:` expression.
- `fail_on_error: true` causes the pipeline to collect the task error and return it after all goroutines finish. Without it, errors are logged but execution continues.
- Concurrency per task is controlled with `task_concurrency`; the pipeline runs N goroutines calling the same `Run()` concurrently. The `Base` methods (`GetRecord`, `SendRecord`) are safe for concurrent use.

## Smoke Path

The smoke path is the one end-to-end test an agent can always run without any external dependencies (no AWS, no Kafka, no network).

```bash
# Build
go build -o caterpillar ./cmd/caterpillar/caterpillar.go

# Run
./caterpillar -conf test/pipelines/hello_name.yaml
```

Expected outcome: exits with code 0, prints name greetings to stdout, no external services required. If this fails, something fundamental is broken before any task-specific investigation.

## SRE / Operational Model

Caterpillar is a CLI tool, not a long-running server. There is no process to restart, no service to scale, and no health endpoint to query.

**During an incident:**
1. Check CI logs first: `gh run list --repo patterninc/caterpillar` then `gh run view <run-id>` to inspect a specific run.
2. If CI is passing and runtime behavior is wrong, check the state of the relevant AWS service (S3 bucket access, SQS queue depth, SSM parameter existence) using the AWS Console or CLI.
3. There is no caterpillar daemon to restart — re-running the binary with a corrected YAML is the recovery action.

**Release pipeline status:** Currently broken. The Docker Hub publish step fails because the `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` secrets are not configured in the repository's GitHub Actions secrets. Do not attempt to trigger a release until those secrets are set.

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `AWS_ACCESS_KEY_ID` | Required for AWS tasks | IAM access key for authenticating S3, SQS, SNS, and SSM tasks |
| `AWS_SECRET_ACCESS_KEY` | Required for AWS tasks | IAM secret key paired with `AWS_ACCESS_KEY_ID` |
| `AWS_REGION` | Required for all AWS tasks | AWS region to target (e.g., `us-east-1`) |
| `AWS_ENDPOINT_URL` | Optional | Override the AWS endpoint URL; use for LocalStack (e.g., `http://localhost:4566`) |

No credentials are stored in this repository. Set these variables in your shell or CI environment before running pipelines that use AWS-backed tasks.

## Pull Request Instructions

All changes must go through a PR targeting `main`. Never push directly to `main`.

### Branch naming

```
<type>/<short-description>
```

Common types: `feat`, `fix`, `chore`, `refactor`, `docs`. Example: `feat/add-grpc-task`.

### Before opening a PR

1. Build must pass: `go build ./cmd/caterpillar/caterpillar.go`
2. If you added a new task type, confirm it is registered in `tasks.go` and has a `README.md` in its package directory.

### PR title format

```
<type>: <short description in imperative mood>
```

Examples:
- `feat: add grpc task`
- `fix: handle nil record in jq task`
- `chore: update go.mod dependencies`

Keep titles under 72 characters. Do not include the issue number in the title.

### PR body

```
## What
<1-3 sentences describing what changed>

## Why
<why this change is needed>

## Test plan
- [ ] `go build ./cmd/caterpillar/caterpillar.go` passes
- [ ] <any manual steps taken to verify behavior>
```

### Creating the PR

```bash
git checkout -b feat/my-change
git add <specific files>
git commit -m "feat: my change"
gh pr create --title "feat: my change" --body "..."
```

Scope `git add` to specific files — never use `git add -A` or `git add .` to avoid accidentally staging scratch files or credentials.

## Agent Permissions

- **Autonomous:** Read files, run `go build ./cmd/caterpillar/caterpillar.go`, create branches, add new task packages following the pattern above.
- **Supervised:** Modify `internal/pkg/pipeline/tasks.go`, change `go.mod`/`go.sum`, open PRs, modify CI configuration.
- **Restricted:** Delete untracked scratch files at repo root, add anything to `internal/pkg/pipeline/task/kinesis/` or register it, modify `.github/CODEOWNERS`.
