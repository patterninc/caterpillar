# 0001. Pipelines are tasks connected by buffered Go channels

- Status: Accepted
- Date: 2026-11-19 (retroactive; the design predates the git history in this repo)

## Context

Caterpillar moves records from sources (files, HTTP, SQS, Kafka) through transforms (jq, split, converter) to sinks (S3, SNS, HTTP). Pipelines are defined by data engineers in YAML, not in code.

## Decision

Every task implements `task.Task` with `Run(input <-chan *record.Record, output chan<- *record.Record) error` (`internal/pkg/pipeline/task/task.go`). `Pipeline.Run` (`internal/pkg/pipeline/pipeline.go`) creates one buffered channel between consecutive tasks, `channel_size` long (default `10e3`), and starts `task_concurrency` goroutines per task that compete for the same input channel. A source task gets a nil input, a sink a nil output. A task's output channel is closed only after all its workers return, which is how end of stream propagates.

## Consequences

- Throughput scales per task with `task_concurrency`, with no coordination code in tasks.
- Ordering is not preserved once any task runs more than one worker.
- Backpressure is implicit: a stalled task blocks its upstream once the channel fills. A task that stops early while upstream is still producing hangs the run (README, "Backpressure caveat").
- Errors are printed per worker (`error in <task>: <err>`) and only fail the run for tasks with `fail_on_error: true`.
