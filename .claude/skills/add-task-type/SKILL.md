---
name: add-task-type
description: Adds a new caterpillar task type (a YAML `type:` value) end to end - package, registry entry, README, tests. Use when asked to add a task, source, sink, or transform to caterpillar. Do not use for changing an existing task's fields (edit that package and its README directly).
---

# Add a task type

1. **Read two similar tasks first.** A transform: `internal/pkg/pipeline/task/replace/`. A source or sink with a client: `internal/pkg/pipeline/task/sns/` or `sqs/`. Match their layout and style.
2. **Create the package** `internal/pkg/pipeline/task/<type>/<type>.go`:
   - a struct embedding `task.Base` with `yaml:"..." json:"..."` tags and any `validate:"..."` tags;
   - `func New() (task.Task, error)` that returns the struct with defaults set;
   - `Init() error` for client creation (called once, after decode and validation);
   - `Run(input <-chan *record.Record, output chan<- *record.Record) error`.
   Check `input`/`output` nil-ness and return `task.ErrNilInput`, `task.ErrPresentInput` or `task.ErrNilOutput` when the task cannot be a source or sink.
3. **Ack contract** (ADR 0005): for every record read from `input`, send any results with `SendData`/`SendRecord`, then call `ack.Release(r.Context)` once on success or `ack.Reject(r.Context)` once on failure. If the task is a deferred-ack source, implement `task.Finisher` and wait for acks in `Finish()`, never in `Run`.
4. **Register** the constructor in `supportedTasks` in `internal/pkg/pipeline/tasks.go` (keep the map alphabetical).
5. **Document** `internal/pkg/pipeline/task/<type>/README.md` with Function, Behavior, a Configuration Fields table (`name`, `type`, `task_concurrency`, `context`, `fail_on_error` rows included), and an example. Add the type to "Supported Tasks" in the top-level `README.md`. Add a `CHANGELOG.md` entry under Unreleased.
6. **Test:**
   - unit tests in `<type>_test.go` driving `Run` with channels (see `internal/pkg/pipeline/task/acking_test.go` for ack assertions);
   - an example pipeline under `test/pipelines/`.
7. **Verify:** `gofmt -l`, `go vet -copylocks=false ./...`, `go test -race ./...`, `go build ./cmd/caterpillar/...`, then run the example pipeline.
