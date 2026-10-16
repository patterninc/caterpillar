---
skill: sqs
version: 1.0.0
caterpillar_type: sqs
description: Read messages from or write messages to an AWS SQS queue.
role: source | sink
requires_upstream: false   # read mode
requires_downstream: false # write mode
aws_required: true
---

## Purpose

Dual-mode SQS task. Auto-detects role:
- **Read mode** (no upstream): polls queue, emits one record per message
- **Write mode** (has upstream): receives records, sends each as SQS message

AWS region is parsed automatically from the queue URL.

## Schema

```yaml
- name: <string>                  # REQUIRED
  type: sqs                       # REQUIRED
  queue_url: <string>             # REQUIRED — full SQS queue URL
  concurrency: <int>              # OPTIONAL — parallel processors (default: 10)
  max_messages: <int>             # OPTIONAL — messages per poll batch, max 10 (default: 10)
  wait_time: <int>                # OPTIONAL — long-poll seconds (default: 10)
  exit_on_empty: <bool>           # OPTIONAL — stop when queue drains (default: false)
  message_group_id: <string>      # OPTIONAL — required for FIFO queue writes
  fail_on_error: <bool>           # OPTIONAL (default: false)
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Reading from queue | first task in pipeline, no upstream |
| Writing to queue | add upstream task |
| Queue URL is configurable | use `{{ env "SQS_QUEUE_URL" }}` |
| Pipeline should stop when queue is empty | set `exit_on_empty: true` |
| FIFO queue | set `message_group_id`; URL ends in `.fifo` |
| Need variable message group | use `{{ macro "uuid" }}` in `message_group_id` |
| High throughput read | increase `concurrency` |
| Sensitive queue URL | use `{{ secret "/ssm/path" }}` |

## Validation Rules

- `queue_url` is required
- `max_messages` ≤ 10 (SQS API hard limit)
- FIFO queues (URL ends in `.fifo`) require `message_group_id` for writes
- Without `exit_on_empty: true` the pipeline polls indefinitely — confirm for production long-running consumers
- AWS region is **not** a field — it is parsed from the queue URL automatically
- `fail_on_error: true` recommended for source tasks in critical pipelines

## IAM Permissions

```
# Read mode
sqs:ReceiveMessage
sqs:DeleteMessage
sqs:GetQueueAttributes

# Write mode
sqs:SendMessage
```

## Examples

### Read (drain queue, stop when empty)
```yaml
- name: read_queue
  type: sqs
  queue_url: '{{ env "SQS_QUEUE_URL" }}'
  max_messages: 10
  wait_time: 10
  exit_on_empty: true
  concurrency: 5
  fail_on_error: true
```

### Read (continuous consumer)
```yaml
- name: consume_events
  type: sqs
  queue_url: https://sqs.us-west-2.amazonaws.com/123456789012/events
  concurrency: 10
  wait_time: 20
```

### Write to standard queue
```yaml
- name: enqueue_results
  type: sqs
  queue_url: https://sqs.us-east-1.amazonaws.com/123456789012/output-queue
```

### FIFO queue read
```yaml
- name: read_fifo
  type: sqs
  queue_url: https://sqs.us-west-2.amazonaws.com/123456789012/ordered.fifo
  exit_on_empty: true
```

### FIFO queue write
```yaml
- name: write_fifo
  type: sqs
  queue_url: https://sqs.us-west-2.amazonaws.com/123456789012/ordered.fifo
  message_group_id: pipeline-batch-{{ macro "uuid" }}
```

## Anti-patterns

- Setting `max_messages` > 10 → SQS API rejects it
- Omitting `exit_on_empty: true` in batch jobs → pipeline never terminates
- Missing `message_group_id` for FIFO write → SQS returns error
- Hardcoding queue URL → use `{{ env "SQS_QUEUE_URL" }}`
- Confusing `concurrency` (SQS-level goroutines) with `task_concurrency` (pipeline-level workers)
