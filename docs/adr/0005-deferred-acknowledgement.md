# 0005. At-least-once sources defer acknowledgement until every branch settles

- Status: Accepted
- Date: 2026-11-19 (retroactive; implemented in #79 on 2026-08-24)

## Context

SQS and Kafka sources must not delete or commit a message until everything produced from it has been processed, or a crash loses data. A single source record can fan out into many downstream records (split, DAG branches).

## Decision

Package `internal/pkg/pipeline/ack` attaches an `*ack.Ack` to the source record's context. Each `SendRecord` registers a branch before the record becomes visible downstream (`task.go`), and each consuming task calls `ack.Release` or `ack.Reject` once per record it consumes. The source waits in `Finish()` (the `task.Finisher` interface), which the pipeline calls after that task's output channel is closed, so downstream can drain; waiting in `Run` would deadlock. When a worker exits early, the pipeline rejects whatever is left in its input so sources redeliver instead of waiting forever.

## Consequences

- Tasks never declare fan-out counts; correctness depends on every task releasing or rejecting exactly once. `acking_test.go` and `ack_test.go` cover this.
- Errors a task is configured to ignore (`jq` and `xpath` by default) count as processed: the source message is acknowledged and the record is dropped for good (README, "Non-critical errors").
