# SNS Task

The `sns` task publishes messages to an AWS SNS topic.

## Behavior

The SNS task acts as a sink, receiving records from the input channel and publishing them to the configured SNS topic. It does not output any records downstream.

> **Note**: If you need to trigger subsequent tasks after SNS publication, use the DAG functionality to manage dependencies.


-   **Message Content**: The `Data` field of the input record is sent as the `Message` body.


## Configuration Fields

| Field | Type | Default | Description |
|---|---|---|---|
| `name` | string | - | Task name for identification |
| `type` | string | `sns` | Must be "sns" |
| `topic_arn` | string | - | The Amazon Resource Name (ARN) of the SNS topic (required) |
| `region` | string | `us-west-2` | AWS Region Defaults to `us-west-2`. |
| `subject` | string | - | Optional subject line for the published message |
| `attributes` | list | - | Optional list of message attributes (name, type, value) |
| `message_group_id` | string | - | Required for FIFO topics. If not provided for a FIFO topic, a UUID is generated. |
| `message_deduplication_id` | string | - | Optional for FIFO topics. |


## Example Configurations

### Basic Usage

```yaml
tasks:
  - name: send_notification
    type: sns
    topic_arn: "arn:aws:sns:us-west-2:123456789012:my-topic"
    subject: "Alert from Caterpillar"
    attributes:
      - name: "Priority"
        type: "String"
        value: "High"
```

### Sending JSON Messages

To send a JSON payload, use a `jq` task or similar transformation before the SNS task to format the record data as a JSON string.

```yaml
tasks:
  - name: create_json
    type: jq
    path: '{ "id": .id, "status": "processed" }'

  - name: send_json_to_sns
    type: sns
    topic_arn: "arn:aws:sns:us-west-2:123456789012:json-topic"
```

### FIFO Topics

For FIFO topics (ARNs ending in `.fifo`), a `message_group_id` is required. The task will:
1. Use the configured `message_group_id` if provided.
2. If not provided, generate a unique UUID for each message.

```yaml
tasks:
  - name: send_fifo
    type: sns
    topic_arn: "arn:aws:sns:us-west-2:123456789012:my-topic.fifo"
    message_group_id: "group1"
```
