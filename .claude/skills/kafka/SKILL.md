---
skill: kafka
version: 1.0.0
caterpillar_type: kafka
description: Read messages from or write messages to a Kafka topic, with TLS and SASL/SCRAM support.
role: source | sink
requires_upstream: false   # read mode
requires_downstream: false # write mode
aws_required: false
---

## Purpose

Dual-mode Kafka task. Auto-detects role:
- **Read mode** (no upstream): polls topic, emits one record per message
- **Write mode** (has upstream): receives records, writes each as Kafka message

Supports standalone reader (no group) and coordinated group consumer.
Write mode buffers messages and flushes per `batch_size` and `batch_flush_interval`.

## Schema

```yaml
- name: <string>                    # REQUIRED
  type: kafka                       # REQUIRED
  bootstrap_server: <string>        # REQUIRED — broker address (host:port)
  topic: <string>                   # REQUIRED — topic name
  timeout: <duration>               # OPTIONAL — dial/read/write/commit timeout (default: 15s)
  batch_size: <int>                 # OPTIONAL — messages to buffer before flush (default: 100)
  batch_flush_interval: <duration>  # OPTIONAL — max wait before flush; must be < timeout (default: 2s)
  retry_limit: <int>                # OPTIONAL — empty-poll retries before stopping (default: 5)
  group_id: <string>                # OPTIONAL — consumer group ID (recommended for production)
  server_auth_type: <string>        # OPTIONAL — "none" or "tls" (default: none)
  cert: <string>                    # OPTIONAL — inline CA cert PEM (use | block scalar)
  cert_path: <string>               # OPTIONAL — path to CA cert file
  user_auth_type: <string>          # OPTIONAL — "none", "sasl", or "scram" (default: none)
  username: <string>                # OPTIONAL — SASL/SCRAM username
  password: <string>                # OPTIONAL — SASL/SCRAM password
  fail_on_error: <bool>             # OPTIONAL (default: false)
```

> `mtls` user_auth_type is reserved but not implemented — do not use.

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Reading from topic | first task (no upstream) |
| Writing to topic | add upstream task |
| Production consumer | set `group_id` for coordinated offset commits |
| Dev/one-off read | omit `group_id` (standalone, no offset commits) |
| Broker uses TLS | set `server_auth_type: tls`, provide `cert` or `cert_path` |
| SASL Plain auth | set `user_auth_type: sasl` + `username` + `password` |
| SCRAM-SHA-512 auth | set `user_auth_type: scram` + `username` + `password` |
| Long-running jobs | increase `timeout` (e.g. `5m`) |
| High-throughput write | tune `batch_size` and `batch_flush_interval` |
| Stop after N empty polls | set `retry_limit: N` |
| Inline cert in YAML | use `cert: \|` block scalar |
| Cert from filesystem | use `cert_path: /path/to/ca.pem` |
| Credentials must be secure | use `{{ env "VAR" }}` or `{{ secret "/path" }}` |

## Constraint: batch_flush_interval < timeout

In write mode `batch_flush_interval` must be strictly less than `timeout`.
Example valid: `timeout: 5m`, `batch_flush_interval: 2s` ✓
Example invalid: `timeout: 2s`, `batch_flush_interval: 5s` ✗

## Validation Rules

- `bootstrap_server` and `topic` are required
- `batch_flush_interval` must be `< timeout` in write mode
- `group_id` omitted → standalone reader, offsets **not** committed
- `group_id` set → coordinated consumer, offsets **are** committed after processing
- `user_auth_type: mtls` → returns error at runtime, do not use
- Credentials must use `{{ env "VAR" }}` or `{{ secret "/path" }}`
- Inline `cert` requires proper YAML block scalar formatting

## Examples

### Read — standalone, no auth
```yaml
- name: read_topic
  type: kafka
  bootstrap_server: kafka.local:9092
  topic: input-events
  timeout: 25s
  fail_on_error: true
```

### Read — group consumer (production)
```yaml
- name: consume_events
  type: kafka
  bootstrap_server: kafka.prod:9092
  topic: user-events
  group_id: caterpillar-consumer-v1
  timeout: 25s
```

### Read — SCRAM + TLS
```yaml
- name: read_secure
  type: kafka
  bootstrap_server: kafka.prod:9093
  topic: secure-events
  group_id: prod-consumer
  user_auth_type: scram
  username: "{{ env "KAFKA_USER" }}"
  password: "{{ secret "/prod/kafka/password" }}"
  server_auth_type: tls
  cert_path: /etc/ssl/certs/kafka-ca.pem
  timeout: 25s
```

### Write — SASL
```yaml
- name: publish_results
  type: kafka
  bootstrap_server: kafka.prod:9092
  topic: output-results
  user_auth_type: sasl
  username: "{{ env "KAFKA_USER" }}"
  password: "{{ env "KAFKA_PASS" }}"
  timeout: 5m
  batch_size: 200
  batch_flush_interval: 3s
```

### Write — inline CA cert
```yaml
- name: publish_tls
  type: kafka
  bootstrap_server: kafka.prod:9093
  topic: events
  server_auth_type: tls
  cert: |
    -----BEGIN CERTIFICATE-----
    MIID...
    -----END CERTIFICATE-----
  timeout: 30s
  batch_flush_interval: 2s
```

### Stop after 10 empty polls
```yaml
- name: drain_topic
  type: kafka
  bootstrap_server: kafka.local:9092
  topic: input-topic
  retry_limit: 10
  timeout: 5s
```

## Anti-patterns

- `batch_flush_interval >= timeout` in write mode → runtime error
- Using `user_auth_type: mtls` → not implemented, returns error
- Omitting `group_id` in production multi-instance deployments → no offset coordination
- Hardcoding `username` / `password` → use `{{ env "VAR" }}` or `{{ secret "/path" }}`
- Malformed inline PEM in `cert` (missing `|` block scalar) → TLS failure
