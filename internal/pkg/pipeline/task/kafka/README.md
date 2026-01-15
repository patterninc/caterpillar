# Kafka Task

The `kafka` task reads from or writes to Apache Kafka topics.

## Behavior

The Kafka task operates in two modes depending on whether an input channel is provided:

- **Write mode** (with input channel): receives records from the input channel and sends them as messages to the Kafka topic. Writes are buffered and flushed in batches (see `batch_size` and `timeout`).
- **Read mode** (no input channel): polls messages from the Kafka topic and sends them to the output channel.

The task automatically determines its mode based on the presence of input/output channels.

### Reading Modes

When reading from a Kafka topic, there are two main modes of operation:

- **Standalone reader** (no consumer group): omit `group_id`; the reader pulls messages directly from partitions. Offsets are not coordinated across instances and are not committed.
- **Group consumer** (recommended for production): set `group_id`. Multiple instances with the same `group_id` split partitions between them and coordinate offsets. When `group_id` is set the task will commit offsets after processing messages.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `kafka` | Must be "kafka" |
| `bootstrap_server` | string | - | Kafka broker's bootstrap address (required) |
| `topic` | string | - | Topic to read from or write to (required) |
| `timeout` | duration string | `15s` | Per-operation timeout (used for dial, read, write, commit by default). Uses Go duration format (e.g. `25s`, `1m`). |
| `batch_size` | int | `100` | Number of messages to buffer/flush for write and reader |
| `group_id` | string | - | Consumer group id for group consumption (optional) |
| `server_auth_type` | string | `none` | `none` or `tls` — server certificate verification mode |
| `cert` | string | - | CA certificate PEM/CRT content used when `server_auth_type: tls` (alternatively use `cert_path`) |
| `cert_path` | string | - | Path to CA certificate (PEM/CRT) |
| `user_auth_type` | string | `none` | `none`, `sasl`, `scram`, or `mtls` — client authentication method |
| `username` | string | - | Username for SASL/SCRAM authentication |
| `password` | string | - | Password for SASL/SCRAM authentication |
| `user_cert` | string | - | Client certificate PEM content for mTLS (alternatively use `user_cert_path`) |
| `user_cert_path` | string | - | Client certificate path for mTLS (not implemented) |

## Authentication

- `server_auth_type: tls` enables server certificate verification using the CA at `cert` or, if absent, the CA file at `cert_path`.
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
 - The task uses a single configured `timeout` (default 15s) for dial, read, write and commit operations. Dial attempts use the same `timeout` value for each connection attempt.
 - When reading, the code treats `context.DeadlineExceeded` as transient and retries; the implementation will stop the reader after a limited number of consecutive deadline-exceeded occurrences (default retry limit = 5). Increase `timeout` or decrease message polling intervals to avoid hitting this limit in normal operation.
 - Writes are buffered up to `batch_size` and flushed either when the buffer reaches `batch_size` or when the configured `timeout` elapses since the last flush attempt.
 - `mtls` is a placeholder in the code and currently returns an error / not implemented; client certificate authentication is not provided yet.

## Troubleshooting

- If TLS connections fail, verify the CA at `cert_path` or `cert` matches the broker's certificate chain. Also check whether the certificate at `cert` is correctly formatted (PEM) in multiline YAML (use `|` and indentation).
- If SASL/SCRAM authentication fails, double-check `username`/`password` and the broker's configured mechanism.

**Thanks to the _[kafka-go](https://github.com/segmentio/kafka-go)_ library for Kafka client functionality.**
