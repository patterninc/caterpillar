---
name: debug-pipeline
description: Diagnoses a failing, hanging, or silently lossy caterpillar pipeline run from its YAML and output. Use when a user shares a pipeline file, a caterpillar log, or says a run hung, exited 0 with missing data, duplicated messages, or failed at config load. Do not use for build or CI failures (see docs/runbooks/build-cgo-librdkafka.md).
---

# Debug a pipeline run

1. **Collect:** the pipeline YAML, the full stdout, the exit status, and the caterpillar version. Never ask for or print a rendered config: it contains secrets.
2. **Classify the symptom** and open the matching runbook under `docs/runbooks/`:
   - failed before any output with `config:` -> `config-and-secret-load.md`
   - exit 0, data missing -> `silent-task-errors.md`
   - never finished -> `pipeline-hang.md`
   - SQS/Kafka duplicates or loss -> `redelivery-and-data-loss.md`
3. **Find the failing task:** `grep -E '^(error in |error finishing |ERROR \()'` the output. Read that task's `internal/pkg/pipeline/task/<type>/README.md` for its ignore and error fields.
4. **Reproduce offline.** Copy the pipeline to a scratch file, replace AWS/HTTP sources with a `file` source over a small fixture, keep the failing task, end with `echo`. Build and run: `go build -o caterpillar ./cmd/caterpillar && ./caterpillar -conf <scratch>.yaml`. Add `fail_on_error: true` to the suspect task to get a non-zero exit.
5. **Decide the fix location:** pipeline config (the pipeline's own repo), upstream data, or a caterpillar bug. For a caterpillar bug, write a failing unit test in the task's package first, then fix.
