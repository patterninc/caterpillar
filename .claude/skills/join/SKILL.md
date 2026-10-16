---
skill: join
version: 1.0.0
caterpillar_type: join
description: Aggregate multiple records into one by batching on count, byte size, or time duration.
role: transform
requires_upstream: true
requires_downstream: true
aws_required: false
---

## Purpose

Buffers incoming records and emits a combined record when a flush condition is met.
Flush triggers (first condition satisfied wins):
- `number` records accumulated
- Total `size` bytes reached
- `duration` elapsed since last flush

If no conditions are set, flushes once at end-of-stream (joins everything).

## Schema

```yaml
- name: <string>          # REQUIRED
  type: join              # REQUIRED
  number: <int>           # OPTIONAL — max records per batch
  size: <int>             # OPTIONAL — max bytes before flush
  duration: <string>      # OPTIONAL — max wait (Go duration: "30s", "5m", "1h")
  delimiter: <string>     # OPTIONAL — separator between joined records (default: \n)
  fail_on_error: <bool>   # OPTIONAL (default: false)
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Batch by fixed record count | set `number` |
| Batch by payload size (e.g. 1 MB chunks) | set `size: 1048576` |
| Flush on time interval | set `duration: "5m"` |
| Multi-condition (whichever comes first) | combine `number`, `size`, `duration` |
| Collect all records into one | set none of the three (end-of-stream flush) |
| Join with newlines | default `delimiter: "\n"` |
| Join with pipe separator | `delimiter: "\|"` |
| Join for JSON array | use `replace` after to wrap: `^(.*)$` → `[$1]` |

## Flush Behavior

```
Incoming: record1, record2, record3 (number: 3 configured)
Output:   "record1\nrecord2\nrecord3"   ← single record
```

Flush triggers are evaluated after **each record is added**. Flushes immediately when first condition is met.

## Size Reference

| Size value | Bytes |
|-----------|-------|
| 1 KB | 1024 |
| 64 KB | 65536 |
| 512 KB | 524288 |
| 1 MB | 1048576 |
| 5 MB | 5242880 |

## Validation Rules

- At least one of `number`, `size`, `duration` is recommended — otherwise all records accumulate in memory until stream ends
- `duration` uses Go format: `"30s"`, `"5m"`, `"1h30m"` — not plain integers
- Large end-of-stream joins risk out-of-memory for unbounded streams — always recommend a limit
- After `join`, data is a single string — downstream tasks receive one large record per batch

## Examples

### Batch 100 records per output record
```yaml
- name: batch_100
  type: join
  number: 100
  delimiter: "\n"
```

### Batch by 1 MB chunks
```yaml
- name: batch_1mb
  type: join
  size: 1048576
  delimiter: "\n"
```

### Flush every 5 minutes
```yaml
- name: time_window
  type: join
  duration: "5m"
  delimiter: "\n"
```

### Multi-trigger (50 records, 512 KB, or 2 minutes)
```yaml
- name: flexible_batch
  type: join
  number: 50
  size: 524288
  duration: "2m"
  delimiter: "\n"
```

### Join all → write as single file
```yaml
- name: collect_all
  type: join
  delimiter: "\n"

- name: write_file
  type: file
  path: output/full_export_{{ macro "timestamp" }}.txt
```

### Batch → build JSON array → POST
```yaml
- name: batch
  type: join
  number: 10
  delimiter: ","

- name: wrap_array
  type: replace
  expression: "^(.*)$"
  replacement: "[$1]"

- name: post_batch
  type: http
  method: POST
  endpoint: https://api.example.com/batch
  headers:
    Content-Type: application/json
```

### SQS drain → batch → S3
```yaml
tasks:
  - name: read_queue
    type: sqs
    queue_url: "{{ env "SQS_QUEUE_URL" }}"
    exit_on_empty: true

  - name: batch
    type: join
    number: 1000
    delimiter: "\n"

  - name: write_batch
    type: file
    path: s3://{{ env "BUCKET" }}/batch_{{ macro "uuid" }}.txt
    success_file: true
```

## Anti-patterns

- No flush condition on an unbounded stream → unbounded memory growth
- `duration: 300` (integer) → must be `duration: "5m"` (Go duration string)
- Expecting records to retain individual identity after `join` — they are concatenated into one string
- Using `join` without `split` when the downstream consumer expects individual records again
