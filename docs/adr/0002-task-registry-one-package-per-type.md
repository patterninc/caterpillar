# 0002. One package per task type, registered in a static map

- Status: Accepted
- Date: 2026-11-19 (retroactive)

## Context

The pipeline file names each task's `type`. The runner must turn that string into a configured Go value, and new types are added often (21 today).

## Decision

Each type lives in its own package under `internal/pkg/pipeline/task/<type>/` with its own `README.md`. Its struct embeds `task.Base` (name, type, `fail_on_error`, `task_concurrency`, `context`). `supportedTasks` in `internal/pkg/pipeline/tasks.go` maps the YAML `type` to the package's `New` constructor. Decoding is two-pass: decode `task.Base` to read `type`, then decode the full node into the value `New` returned, run `validator` struct tags, then call `Init()` once.

## Consequences

- Adding a type is local: one package, one registry line, one README. `.claude/skills/add-task-type/SKILL.md` documents the steps.
- An unknown type fails at load with `task type is not supported: <type>`, before any task runs.
- Task packages must not import `internal/pkg/pipeline` (that would be an import cycle); `.golangci.yml` enforces this with `depguard`.
- `Init()` errors (for example, an unreachable AWS endpoint) surface at load time as `failed to initialize task <name>`.
