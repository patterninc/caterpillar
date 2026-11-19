# SQS or Kafka messages duplicated, redelivered, or lost

Sources in read mode defer acknowledgement until every record produced from a message has been released downstream (`internal/pkg/pipeline/ack`, ADR 0005). Delivery is at-least-once.

## Duplicates or redelivery

1. **Slow pipeline vs. visibility timeout (SQS).** If a message takes longer to traverse the pipeline than the queue's visibility timeout, SQS redelivers it while the first copy is still in flight (`internal/pkg/pipeline/task/sqs/README.md`, "Message Acknowledgment"). Raise the queue's visibility timeout or reduce per-message work (lower `channel_size`, raise downstream `task_concurrency`).
2. **A downstream task rejected the record.** `ack.Reject` leaves the message on the queue; it returns after the visibility timeout. Find the `error in <task>:` line for that task.
3. **The run was killed.** Anything not yet settled is redelivered. That is expected.
4. Make sinks idempotent where duplicates matter (for example, deterministic S3 keys).

## Messages acknowledged but data missing

1. A task skipped the record under an ignore setting (`jq` and `xpath` by default). Skipped records count as processed, so the message is deleted. See [silent-task-errors.md](silent-task-errors.md).
2. The data is not recoverable from the queue. Replay from the producer or the dead-letter queue if one is configured.

## FIFO queues not draining

`exit_on_empty` does not treat an empty poll as drained while the task holds receipts or the queue still has visible, in-flight or delayed messages (`sqs/README.md`). A stuck group head holds its whole message group; check for a record that keeps being rejected.
