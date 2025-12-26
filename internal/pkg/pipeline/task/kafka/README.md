# Kafka Task

The `kafka` task reads from or writes to Apache Kafka topics.

## Behavior

The Kafka task operates in two modes depending on whether an input channel is provided:

- **Write mode** (with input channel): Receives records from the input channel and sends them as messages to the Kafka topic.
- **Read mode** (no input channel): Polls messages from the Kafka topic and sends them to the output channel.

The task automatically determines its mode based on the presence of input/output channels.

### Reading Modes

When reading from a Kafka topic, there are two main modes of operation:

- **Run a standalone reader** (no consumer group): use a `kafka` task without `group_id`; it will pull messages directly from the topic. This is useful for quick debugging but not recommended for production because offsets are not coordinated across consumers.

- **Run a group consumer** (recommended for production): set `group_id` in the task. Multiple instances with the same `group_id` will split partitions between them and coordinate offsets automatically. Use `start_from_beginning: true` only when creating a new group and you want to process all historical messages in the group at every use.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `kafka` | Must be "kafka" |
| `bootstrap_server` | string | - | Kafka broker's bootstrap address (required) |
| `topic` | string | - | Topic to read from or write to (required) |
| `timeout` | duration string | `25s` | Connection timeout (Go duration format, e.g. `25s`, `1m`, `2m30s`) |
| `group_id` | string | - | Consumer group id for group consumption (optional) |
| `start_from_beginning` | bool | false | When using a `group_id` for a non-existent group, set the property for the group to consume from the beginning of topic |
| `server_auth_type` | string | `none` | `none` or `tls` — server certificate verification mode |
| `cert` | string | - | CA certificate PEM/CRT content used when `server_auth_type: tls` (alternatively use `cert_path`) |
| `cert_path` | string | - | Path to CA certificate (PEM/CRT) |
| `user_auth_type` | string | `none` | `none`, `sasl`, `scram`, or `mtls` — client authentication method |
| `username` | string | - | Username for SASL/SCRAM authentication |
| `password` | string | - | Password for SASL/SCRAM authentication |
| `user_cert` | string | - | Client certificate PEM content for mTLS (alternatively use `user_cert_path`) |
| `user_cert_path` | string | - | Client certificate path for mTLS (not implemented) |

## Authentication

- `server_auth_type: tls` enables server certificate verification using the CA at `cert` or if absent, then using CA file at `cert_path`. 
- `user_auth_type: sasl` uses SASL Plain authentication (requires `username` and `password`).
- `user_auth_type: scram` uses SCRAM-SHA-512 authentication (requires `username` and `password`).
- `user_auth_type: mtls` is reserved for mTLS (client cert) but is not implemented in this task yet.

If you choose SASL/SCRAM and `server_auth_type: tls`, both TLS and the SASL mechanism will be configured on the dialer.

## Example Configurations

### Writing to a Kafka topic
```yaml
tasks:
  - name: send_messages
    type: kafka
    bootstrap_server: kafka.local:9092
    topic: output-topic
    user_auth_type: sasl
    username: my-user
    password: my-pass
    server_auth_type: tls
    cert_path: /etc/ssl/certs/kafka-ca.pem
    timeout: 5m
```

### Writing with inline CA (multiline PEM)
```yaml
tasks:
  - name: send_messages_inline_cert
    type: kafka
    bootstrap_server: kafka.local:9092
    topic: output-topic
    server_auth_type: tls
    cert: |
      -----BEGIN CERTIFICATE-----
      MIID... (your PEM here)
      -----END CERTIFICATE-----
    timeout: 2s
```

### Reading from a Kafka topic
```yaml
tasks:
  - name: read_messages
    type: kafka
    bootstrap_server: kafka.local:9092
    topic: input-topic
    user_auth_type: none
    timeout: 10s
```

### Standalone reader (no consumer group)
```yaml
tasks:
  - name: read_standalone
    type: kafka
    bootstrap_server: kafka.local:9092
    topic: input-topic
    timeout: 25s
```

### Group consumer
```yaml
tasks:
  - name: read_group
    type: kafka
    bootstrap_server: kafka.local:9092
    topic: input-topic
    group_id: my-consumer-group
    start_from_beginning: true
    timeout: 25s
```

### Using SCRAM (SCRAM-SHA-512)
```yaml
tasks:
  - name: read_secure
    type: kafka
    bootstrap_server: kafka.local:9093
    topic: secure-topic
    user_auth_type: scram
    username: scram-user
    password: scram-secret
    server_auth_type: tls
    cert_path: /etc/ssl/certs/kafka-ca.pem
```

### SASL Plain (no TLS)
```yaml
tasks:
  - name: send_plain
    type: kafka
    bootstrap_server: kafka.local:9092
    topic: output-topic
    user_auth_type: sasl
    username: plain-user
    password: plain-pass
    timeout: 25s
```

## Notes and Limitations
 - Standalone reader reads partitions directly and does not perform coordinated offset commits across multiple readers. When `group_id` is empty the task will not commit offsets.
 - Group consumers enable scaling: Kafka will assign partitions across group members so each message is delivered only once to the group. When `group_id` is set, the task will commit offsets after processing messages.
 - `timeout` uses Go duration strings (type `duration.Duration`) and defaults to `25s` in code; use values like `25s`, `1m`, or `2m30s` in your YAML.
 - `mtls` is a placeholder in the code and currently returns an error / not implemented; client certificate authentication is not provided yet.
 - The code sets a default `timeout` of 1 minute if not provided.

## Troubleshooting

- If TLS connections fail, verify the CA at `cert_path` or `cert` matches the broker's certificate chain. Also check whether the certificate at `cert` is correctly formatted (PEM) in multiline, use `|` and indentation in YAML.
- If SASL/SCRAM authentication fails, double-check `username`/`password` and the broker's configured mechanism.

**Thanks to the _[kafka-go](https://github.com/segmentio/kafka-go)_ library for Kafka client functionality.**
