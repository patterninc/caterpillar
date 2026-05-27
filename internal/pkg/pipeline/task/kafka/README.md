# Kafka Task

The `kafka` task reads from or writes to Apache Kafka topics.

## Behavior

The Kafka task operates in two modes depending on whether an input channel is provided:
- **Write mode** (with input channel): receives records from the input channel and enqueues them to the Kafka topic with the Confluent producer. `batch_flush_interval` maps to the producer's `linger.ms`, and the task flushes pending deliveries before exiting.
- **Read mode** (no input channel): polls messages from the Kafka topic and sends them to the output channel. The reader's polling is controlled by the configured `timeout` and `retry_limit` behavior (see below). Optionally, `end_after` sets a wall-clock deadline that stops the reader regardless of traffic, and `max_records` stops the reader after a fixed number of messages have been forwarded downstream.

The task automatically determines its mode based on the presence of input/output channels.

### Reading Modes

There are two read modes, controlled by whether `group_id` is set:

- **Standalone** (no `group_id`): assigns all partitions directly at `OffsetBeginning` and reads without committing offsets. Every run re-reads from the start of the topic. Useful for one-shot batch reads or testing.
- **Group consumer** (`group_id` set): subscribes via the Kafka consumer group protocol, reads from committed offsets, and commits new offsets periodically. Multiple instances with the same `group_id` split partitions and each message is delivered once to the group. The `auto_offset_reset` field controls behavior when no committed offset is found or the stored offset is out of range (e.g., aged out by retention) — `latest` (default) skips to the tail, `earliest` starts from the beginning of the available log.

> **Broker ACL requirement for standalone mode**: confluent-kafka-go requires a non-empty `group.id` even for direct-assign reads. Standalone mode uses the group ID `caterpillar-standalone-<topic>`. The Kafka principal used by this task must have `READ` permission on `GROUP` resource `caterpillar-standalone-` with `PREFIXED` pattern type. Without this ACL, standalone reads will fail with a group authorization error.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `kafka` | Must be "kafka" |
| `bootstrap_server` | string | - | Kafka broker's bootstrap address (required) |
| `topic` | string | - | Topic to read from or write to (required) |
| `timeout` | duration string | `15s` | Read polling timeout and producer delivery/flush timeout. Uses Go duration format (e.g. `25s`, `1m`). |
| `batch_flush_interval` | duration string | `2s` | Producer linger interval (`linger.ms`) for batching queued writes |
| `retry_limit` | int | `5` | Read retry threshold; reading stops when consecutive retryable errors or timeouts exceed this value |
| `end_after` | duration string | - | Wall-clock deadline for read mode. When set, the reader stops cleanly after this duration regardless of message traffic. Worst-case overshoot is one `timeout` window. |
| `max_records` | int | `0` (unlimited) | Read-mode cap on records forwarded downstream. The reader stops cleanly once this many messages have been sent. In group mode, offsets up to the last forwarded record are committed on shutdown. Must be `>= 0`; negative values are rejected at validation. |
| `group_id` | string | - | Consumer group id. If omitted, standalone mode is used (reads from beginning, no offset commits). |
| `auto_offset_reset` | string | `latest` | Group-mode reset policy when no committed offset exists or the stored offset is out of range. `latest` skips to the tail; `earliest` reads from the beginning of the available log. Ignored in standalone mode. |
| `server_auth_type` | string | `none` | `none` or `tls` — server certificate verification mode |
| `cert` | string | - | CA certificate PEM/CRT content used when `server_auth_type: tls` (alternatively use `cert_path`) |
| `cert_path` | string | - | Path to CA certificate (PEM/CRT) |
| `user_auth_type` | string | `none` | `none`, `sasl`, `scram`, or `mtls` — `mtls` is currently not implemented and returns an error |
| `username` | string | - | Username for SASL/SCRAM authentication |
| `password` | string | - | Password for SASL/SCRAM authentication |
| `idempotent` | bool | `false` | Enables the idempotent producer (`enable.idempotence=true`, `max.in.flight.requests.per.connection=5`) |
| `format` | string | `json` | Message serialization format. `json` passes raw bytes through unchanged (default, backward compatible). `avro` serializes JSON→Avro on write and Avro→JSON on read using Confluent Schema Registry. |
| `schema_registry_url` | string | - | Schema Registry URL. Required when `format: avro`. Schemas must be pre-registered; auto-registration is disabled. |
| `schema_registry_username` | string | - | Schema Registry basic auth username |
| `schema_registry_password` | string | - | Schema Registry basic auth password |

## Authentication

- `server_auth_type: tls` enables server certificate verification using the CA at `cert` or, if absent, the CA file at `cert_path`.
- `user_auth_type: sasl` uses SASL Plain authentication (requires `username` and `password`).
- `user_auth_type: scram` uses SCRAM-SHA-512 authentication (requires `username` and `password`).
- `user_auth_type: mtls` is reserved for mTLS (client cert) but is not implemented in this task yet and will return an error if configured.

If you choose SASL/SCRAM and `server_auth_type: tls`, the task configures Confluent's `security.protocol` as `SASL_SSL`. Without TLS, SASL uses `SASL_PLAINTEXT`.

## Example Configurations

### Writing to a Kafka topic
```yaml
tasks:
  - name: send_messages
    type: kafka
    bootstrap_server: kafka.local:9092
    topic: output-topic
    idempotent: true
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

### Standalone read (no group_id — reads from beginning every run)
```yaml
tasks:
  - name: read_messages
    type: kafka
    bootstrap_server: kafka.local:9092
    topic: input-topic
    user_auth_type: none
    timeout: 10s
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

### Writing with Avro serialization (Schema Registry)
```yaml
tasks:
  - name: send_avro_messages
    type: kafka
    bootstrap_server: kafka.local:9092
    topic: avro-topic
    format: avro
    schema_registry_url: http://schema-registry.local:8081
    schema_registry_username: sr-user       # optional
    schema_registry_password: sr-pass       # optional
    timeout: 30s
```

> The schema for `avro-topic-value` must be pre-registered in the Schema Registry before writing. The producer uses the latest registered version and does not auto-register schemas.

### Stop after 10 consecutive empty retry polls
```yaml
tasks:
  - name: read_until_empty
    type: kafka
    bootstrap_server: kafka.local:9092
    topic: input-topic
    retry_limit: 10
    timeout: 5s
```

### Stop after a wall-clock duration (regardless of traffic)
```yaml
tasks:
  - name: read_window
    type: kafka
    bootstrap_server: kafka.local:9092
    topic: input-topic
    group_id: my-consumer-group
    end_after: 5m
    timeout: 10s
```

### Stop after a fixed number of records
```yaml
tasks:
  - name: read_first_100
    type: kafka
    bootstrap_server: kafka.local:9092
    topic: input-topic
    group_id: my-consumer-group
    max_records: 100
    timeout: 5s
```

> Combine `max_records` with `end_after` to stop on whichever condition is hit first (e.g. "up to 100 records or 30s, whichever comes first").

### Group consumer reading from the beginning on first run
```yaml
tasks:
  - name: read_from_start
    type: kafka
    bootstrap_server: kafka.local:9092
    topic: input-topic
    group_id: my-consumer-group
    auto_offset_reset: earliest
```

## Notes and Limitations
 - **Standalone mode** reads all partitions from `OffsetBeginning` on every run and never commits offsets. It requires a broker PREFIXED ACL on group `caterpillar-standalone-` (see Reading Modes above).
 - **Group consumer mode** resumes from committed offsets. `auto_offset_reset` fires only if the group has no prior committed offsets or the stored offset is out of range (e.g., aged out by retention). To re-read from the beginning, reset group offsets via `kafka-consumer-groups.sh --reset-offsets --to-earliest`.
 - **Out-of-range stored offset:** librdkafka logs a `%4|OFFSET ... offset reset` warning when this happens. It's informational — the consumer self-recovers to the position implied by `auto_offset_reset`. The warning persists across restarts until a successful read commits a new valid offset (or until you manually reset the group offsets at the broker).
 - **`end_after`** sets a wall-clock read deadline distinct from `retry_limit` (which is idle-based). Use `end_after` when you want a guaranteed stop time even on a busy topic. Worst-case shutdown latency is one `timeout` window because in-flight `ReadMessage` polls cannot be canceled mid-flight.
 - **`max_records`** is count-based and independent of `end_after`/`retry_limit`. The counter increments after each record is forwarded downstream, so the cap is exact for delivered records. If the topic has fewer than `max_records` available, the reader keeps polling until `retry_limit` or `end_after` fires.
 - **Group commits** use Kafka auto-commit every 5000ms. Auto offset store is disabled, so offsets are stored only after a message is sent downstream.
 - **Read isolation** is set to `read_committed` for both standalone and group consumers — this is the consumer-side complement to `idempotent: true` on the producer and ensures consumers never read uncommitted or aborted messages.
 - The init broker probe always uses the 15s default timeout regardless of the configured `timeout` to allow for SCRAM+TLS handshake round trips.
 - **Message format** defaults to `json` (raw bytes pass through). Set `format: avro` to enable Confluent Avro serialization; this requires `schema_registry_url` and a pre-registered schema. The `schema_registry_url` field alone does **not** activate Avro — `format: avro` must be set explicitly.
 - When `format: avro` is used, producer input must be valid JSON. Go's `json.Unmarshal` decodes all numbers as `float64`; Avro schemas with `int` or `long` fields may reject these — convert to integer types before serialization if needed.


## Troubleshooting

- If TLS connections fail, verify the CA at `cert_path` or `cert` matches the broker's certificate chain. Also check whether the certificate at `cert` is correctly formatted (PEM) in multiline YAML (use `|` and indentation).
- If SASL/SCRAM authentication fails, double-check `username`/`password` and the broker's configured mechanism.

- If standalone reads fail with a group authorization error, ask your Kafka admin to run:
  ```
  kafka-acls.sh --add --allow-principal User:<principal> \
    --operation Read --group caterpillar-standalone- \
    --resource-pattern-type prefixed \
    --bootstrap-server <host>:<port>
  ```

**Thanks to the [Confluent Kafka Go client](https://github.com/confluentinc/confluent-kafka-go) for the Kafka client implementation.**