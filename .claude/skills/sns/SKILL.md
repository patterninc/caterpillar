---
skill: sns
version: 1.0.0
caterpillar_type: sns
description: Publish pipeline records to an AWS SNS topic. Terminal sink — does not pass records downstream.
role: sink
requires_upstream: true
requires_downstream: false
aws_required: true
---

## Purpose

Receives records from upstream, publishes each as an SNS message. Record `Data` field = message body.
Does **not** emit records downstream. Use DAG if downstream tasks are needed after publication.

## Schema

```yaml
- name: <string>                      # REQUIRED
  type: sns                           # REQUIRED
  topic_arn: <string>                 # REQUIRED — full SNS topic ARN
  region: <string>                    # OPTIONAL — AWS region (default: us-west-2)
  subject: <string>                   # OPTIONAL — message subject line
  attributes: <list>                  # OPTIONAL — SNS message attributes for filtering
  message_group_id: <string>          # OPTIONAL — FIFO topics; auto-UUID if omitted
  message_deduplication_id: <string>  # OPTIONAL — FIFO deduplication ID
  fail_on_error: <bool>               # OPTIONAL (default: false)
```

### Attributes item schema
```yaml
attributes:
  - name: <string>    # attribute name
    type: <string>    # "String", "Number", or "Binary"
    value: <string>   # attribute value
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Standard topic | provide `topic_arn`, omit FIFO fields |
| FIFO topic (ARN ends in `.fifo`) | set `message_group_id`; all messages with same group ID are ordered |
| FIFO + each message independent group | omit `message_group_id` (auto UUID per message, no ordering guarantee) |
| SNS subscription filtering needed | add `attributes` list |
| Topic ARN is environment-specific | use `{{ env "SNS_TOPIC_ARN" }}` |
| Message needs specific format | add `jq` task upstream to reshape the record |
| Post-SNS processing needed | use DAG syntax: `upstream >> [sns_task, other_task]` |
| Region is not us-west-2 | set `region` explicitly |

## Validation Rules

- `topic_arn` is required
- FIFO topic ARNs end in `.fifo` — verify `message_group_id` is set if ordered delivery is required
- `sns` is a terminal sink — it cannot have a downstream task in sequential mode; use DAG if needed
- Record data is sent as-is as the message body — use a `jq` task upstream to format
- `topic_arn` should use `{{ env "VAR" }}` — never hardcode account IDs

## IAM Permissions

```
sns:Publish
```
For encrypted topics:
```
kms:GenerateDataKey
kms:Decrypt
```

## Examples

### Basic notification
```yaml
- name: notify
  type: sns
  topic_arn: "{{ env "SNS_TOPIC_ARN" }}"
  subject: Pipeline alert
```

### With message attributes (subscription filter)
```yaml
- name: publish_event
  type: sns
  topic_arn: arn:aws:sns:us-west-2:123456789012:events
  attributes:
    - name: EventType
      type: String
      value: UserCreated
    - name: Priority
      type: String
      value: High
```

### FIFO topic with group ID
```yaml
- name: ordered_publish
  type: sns
  topic_arn: arn:aws:sns:us-west-2:123456789012:ordered.fifo
  message_group_id: user-events-group
```

### Shape payload then publish
```yaml
- name: format_event
  type: jq
  path: |
    {
      "event": "record_processed",
      "id": .id,
      "ts": "{{ macro "timestamp" }}"
    }

- name: publish
  type: sns
  topic_arn: "{{ env "SNS_TOPIC_ARN" }}"
  region: us-east-1
```

### DAG: process AND publish in parallel
```yaml
tasks:
  - name: source
    type: file
    path: data/input.json
  - name: transform
    type: jq
    path: '{ "id": .id, "result": .value }'
  - name: publish
    type: sns
    topic_arn: "{{ env "SNS_TOPIC_ARN" }}"
  - name: archive
    type: file
    path: s3://bucket/archive/{{ macro "uuid" }}.json

dag: source >> transform >> [publish, archive]
```

## Anti-patterns

- Using `sns` in the middle of a sequential pipeline and expecting records to flow past it — it is a terminal sink
- Hardcoding `topic_arn` with account ID → use `{{ env "VAR" }}`
- FIFO topic without `message_group_id` when ordered delivery is required
- Sending unformatted data — add a `jq` task upstream to structure the message body
