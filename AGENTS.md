# AGENTS.md

Vendor-neutral operating guide for AI coding agents working in this repo — including
agents authoring a pipeline YAML for a repo that consumes Caterpillar.

Human onboarding lives in `README.md`; the experimental `dag:` syntax in `DAG_README.md`.

---

## What this repo is

Caterpillar is a Go pipeline engine: it loads a YAML config, resolves template
functions, and runs a chain of tasks that pass records to each other. The YAML configs
themselves live in the repos that consume Caterpillar; this repo defines what those
configs can say.

---

## Schema source of truth

When authoring or reviewing a pipeline YAML, consult these files in this repo — do not
rely on copies of the schema elsewhere; they go stale:

| Location | What it answers |
|---|---|
| `internal/pkg/pipeline/tasks.go` | The `supportedTasks` map — the only authoritative list of task `type:` values. Anything else fails at load: `task type is not supported`. |
| `internal/pkg/pipeline/task/<type>/README.md` | Per-task behavior and configuration-field tables (every task has one). |
| `internal/pkg/pipeline/task/<type>/*.go` | The task's config struct. `yaml:"..."` tags are the real field names; `validate:"required"` marks required fields. The struct wins over the README if they disagree. |
| `internal/pkg/pipeline/task/task.go` | `Base` — fields every task accepts: `name`, `type`, `fail_on_error`, `task_concurrency`, `context`. |
| `internal/pkg/pipeline/pipeline.go` | Top-level config keys: `tasks`, `channel_size`, `dag`. |
| `internal/pkg/config/` | Template functions (`env`, `secret`, `macro`, `context`, `indent`); secrets resolve from AWS SSM Parameter Store at load time. |
| `test/pipelines/*.yaml` | Runnable examples for nearly every task and feature. |

Note: YAML unmarshaling is **not strict** — unknown keys are silently ignored, so a
typo'd field name runs with the default instead of erroring. Check field names against
the struct tags.

---

## Commands

```bash
go build -o caterpillar cmd/caterpillar/caterpillar.go   # build (or: make build)
./caterpillar -conf test/pipelines/hello_name.yaml       # run a pipeline
make test                                                # run tests
```

`test/pipelines/*_test.yaml` files double as the test suite — each exercises one
feature end to end and can be run directly with `-conf`. A config's load-time checks
(unknown task type, missing required fields, template/jq parse errors) fire even when
AWS-touching tasks can't run locally; substitute `echo` or local `file` tasks for
S3/SQS ends to make a chain fully runnable.

---

## Conventions for changes here

- A new task type must be registered in `supportedTasks` (`internal/pkg/pipeline/tasks.go`)
  and ship with a `README.md` in its package documenting its configuration fields.
- If you change a task's config struct, update that task's `README.md` in the same PR.
- Add or update a `test/pipelines/*.yaml` example when behavior changes.
