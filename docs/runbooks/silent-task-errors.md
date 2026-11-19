# Run exits 0 but data is missing

**Why it happens:** a task's `Run` returned an error, which stops that worker, but the task does not set `fail_on_error: true`, so `Pipeline.Run` records nothing and the process exits 0 (`runTaskConcurrently` in `internal/pkg/pipeline/pipeline.go`). The error is printed to **stdout**, interleaved with data. Separately, `jq` and `xpath` skip failing records by default, and `http` (`ignore_error: true`), `heimdall` (`skip_on_error: true`) and `parameter_store` (`on_missing: skip`) can be configured to.

## Steps

1. Find the error lines in the run's stdout:
   ```bash
   grep -E '^(error in |error finishing |ERROR \()' run.log
   ```
   `error in <task>: <err>` is a worker that stopped. `ERROR (marshal|query|result)` comes from a task's `context:` jq query in `task.Base.SendRecord`; the record is still sent, without that context value.
2. Re-run the pipeline locally against the same input with the failing task's `fail_on_error: true` to confirm it is the cause.
3. If records are being skipped rather than erroring, check the task's ignore field in its `internal/pkg/pipeline/task/<type>/README.md` and turn the skip off while debugging (for `jq`, `ignore_error: false`).
4. Fix the pipeline (bad jq, changed upstream schema, expired credential) and re-run.
5. If the pipeline must never pass silently, set `fail_on_error: true` on its critical tasks in the pipeline YAML.

Long-term fix (tracked as a follow-up in `docs/engineering-best-practices-audit.md`, item 43): send errors to stderr as structured logs.
