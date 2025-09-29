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
| `concurrency` | int | `10` | Number of concurrent message processors |
| `max_messages` | int | `10` | Maximum number of messages to receive per batch |
| `wait_time` | int | `10` | Long polling wait time in seconds |
| `exit_on_empty` | bool | `false` | Exit when queue is empty |
| `message_group_id` | string | - | Message group ID for FIFO queues |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## Example Configurations

### Reading from SQS queue:
```yaml
tasks:
  - name: read_messages
    type: sqs
    queue_url: https://sqs.us-west-2.amazonaws.com/123456789012/my-queue
    max_messages: 10
    wait_time: 10
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

## Sample Pipelines

- `test/pipelines/sqs_reader.yaml` - SQS message reading example

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