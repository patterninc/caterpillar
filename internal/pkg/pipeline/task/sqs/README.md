# SQS Task

The `sqs` task reads from or writes to Amazon Simple Queue Service (SQS) queues, enabling integration with AWS messaging infrastructure.

## Behavior

The SQS task operates in two modes depending on whether an input channel is provided:

- **Write mode** (with input channel): Receives records from the input channel and sends them as messages to the SQS queue
- **Read mode** (no input channel): Polls messages from the SQS queue and sends them to the output channel

The task automatically determines its mode based on the presence of input/output channels. The AWS region is automatically extracted from the queue URL, so no separate region configuration is needed.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `sqs` | Must be "sqs" |
| `queue_url` | string | - | SQS queue URL (required) |
| `concurrency` | int | `10` | Number of concurrent workers that acknowledge (delete) fully-processed messages |
| `max_messages` | int | `10` | Maximum number of messages to receive per batch |
| `wait_time_seconds` | int | `10` | Long polling wait time in seconds |
| `exit_on_empty` | bool | `false` | Exit when queue is empty |
| `end_after` | duration | - | Stop polling after this much time (read mode); e.g. `5m` |
| `message_group_id` | string | - | Message group ID for FIFO queues |
| `task_concurrency` | int | `1` | Number of competing-consumer workers for this task |
| `context` | map | - | JQ expressions whose results are stored on each record for downstream tasks |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

In read mode the task polls until the queue drains (`exit_on_empty`) or `end_after` elapses;
with neither set it polls indefinitely.

## Example Configurations

### Reading from SQS queue:
```yaml
tasks:
  - name: read_messages
    type: sqs
    queue_url: https://sqs.us-west-2.amazonaws.com/123456789012/my-queue
    max_messages: 10
    wait_time_seconds: 10
    concurrency: 5
```

### Writing to SQS queue:
```yaml
tasks:
  - name: send_messages
    type: sqs
    queue_url: https://sqs.us-west-2.amazonaws.com/123456789012/output-queue
```

### FIFO queue with message group ID:
```yaml
tasks:
  - name: fifo_processor
    type: sqs
    queue_url: https://sqs.us-west-2.amazonaws.com/123456789012/my-queue.fifo
    message_group_id: "batch-1"
    exit_on_empty: true
```

### Using environment variables:
```yaml
tasks:
  - name: sqs_processor
    type: sqs
    queue_url: {{ env "SQS_QUEUE_URL" }}
```

## Message Acknowledgment

When reading from a queue, a message's receipt is deleted only once every downstream task
has finished with the record produced from it. A downstream failure leaves the receipt
alone, so SQS redelivers the message after the visibility timeout rather than losing it.
Delivery is therefore at-least-once: a pipeline may see a message more than once, but never
zero times.

Two consequences worth tuning for:

- **`channel_size` bounds how many messages can sit inside the pipeline at once** (see the
  root README). A message waiting in a deep channel can exceed the queue's visibility
  timeout, at which point SQS redelivers it while the first copy is still in flight and
  the eventual delete fails on a stale receipt handle. Keep `channel_size` in proportion
  to how long a record takes to traverse the pipeline, relative to the queue's visibility
  timeout. Messages that have finished the pipeline but whose delete has not yet returned
  are bounded only by `concurrency` (how many `DeleteMessage` calls run at once), not by
  `channel_size`.
- **SQS caps in-flight messages** at 120,000 per standard queue and 20,000 per FIFO queue.
  A large `channel_size` on a long pipeline can approach that; on a breach `ReceiveMessage`
  returns `OverLimit` and the task stops. FIFO queues are stricter still, since
  unacknowledged messages block their message group.

## Sample Pipelines

- `test/pipelines/sqs_with_context_concurrency.yaml` - SQS read with context variables and concurrency; run `test/pipelines/setup_localstack_sqs.sh` first to create the queue

## Use Cases

- **Message processing**: Process messages from SQS queues
- **Event-driven workflows**: Trigger pipelines based on SQS messages
- **Data distribution**: Send processed data to multiple consumers via SQS
- **Asynchronous processing**: Decouple data producers from consumers
- **Load balancing**: Distribute work across multiple pipeline instances
- **Reliability**: Ensure message delivery with SQS's reliability features

## AWS Requirements

For SQS operations, ensure:
- AWS credentials are configured (IAM user, role, or environment variables)
- Appropriate IAM permissions for SQS access:
  - `sqs:ReceiveMessage`
  - `sqs:DeleteMessage`
  - `sqs:SendMessage`
  - `sqs:GetQueueAttributes`
- Correct region configuration
- Valid queue URL