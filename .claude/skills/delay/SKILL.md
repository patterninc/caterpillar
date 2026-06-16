---
skill: delay
version: 1.0.0
caterpillar_type: delay
description: Insert a fixed pause between each record to rate-limit, throttle, or pace pipeline throughput.
role: transform
requires_upstream: true
requires_downstream: true
aws_required: false
---

## Purpose

Waits for `duration` before passing each record to the next task.
Effective throughput = 1 record / `duration` (per worker).
With `task_concurrency: N`, effective throughput ≈ N / `duration`.

## Schema

```yaml
- name: <string>          # REQUIRED
  type: delay             # REQUIRED
  duration: <string>      # REQUIRED — Go duration string (e.g. "100ms", "1s", "5m")
  fail_on_error: <bool>   # OPTIONAL (default: false)
```

## Duration Format

Go duration strings — **must be quoted strings in YAML**:

| Value | Meaning |
|-------|---------|
| `"100ms"` | 100 milliseconds |
| `"500ms"` | 500 milliseconds |
| `"1s"` | 1 second |
| `"30s"` | 30 seconds |
| `"1m"` | 1 minute |
| `"5m"` | 5 minutes |
| `"1h"` | 1 hour |
| `"1m30s"` | 1 minute 30 seconds |

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Rate limit API calls | place before `http` task; `duration` = 1/desired_rate |
| 2 requests/second max | `duration: "500ms"` |
| 1 request/second max | `duration: "1s"` |
| 1 request per minute | `duration: "1m"` |
| Throttle SQS/SNS writes | place before `sqs` or `sns` task |
| Simulate slow processing in test | `duration: "2s"` |
| Prevent downstream overload | place before the bottleneck task |

## Throughput Math

```
1 worker:   rate = 1 / duration
N workers:  rate ≈ N / duration   (task_concurrency: N on the delay task)

Examples:
  duration: 500ms, concurrency: 1  → ~2 records/sec
  duration: 500ms, concurrency: 5  → ~10 records/sec
  duration: 1s,    concurrency: 1  → ~1 record/sec
  duration: 100ms, concurrency: 10 → ~100 records/sec
```

## Validation Rules

- `duration` is required — flag if missing
- Value must be a **string** in Go duration format, not a number: `"1s"` not `1`
- Impact calculation: N records × duration = total pipeline time — warn for large datasets
- Place `delay` **before** the task being rate-limited, not after

## Examples

### Rate limit to 1 request/second
```yaml
- name: throttle
  type: delay
  duration: "1s"

- name: call_api
  type: http
  method: GET
  endpoint: https://api.example.com/data/{{ context "id" }}
```

### 100ms between SQS messages
```yaml
- name: pace_writes
  type: delay
  duration: "100ms"

- name: send_queue
  type: sqs
  queue_url: "{{ env "SQS_QUEUE_URL" }}"
```

### Rate-limited concurrent HTTP pipeline
```yaml
tasks:
  - name: read_ids
    type: file
    path: ids.txt

  - name: split
    type: split

  - name: throttle
    type: delay
    duration: "200ms"
    task_concurrency: 5   # 5 workers × 1/200ms = 25 req/sec

  - name: fetch
    type: http
    method: GET
    endpoint: https://api.example.com/items/{{ context "id" }}
    fail_on_error: false
```

### Simulate slow processing (testing)
```yaml
- name: slow_step
  type: delay
  duration: "2s"
```

## Anti-patterns

- `duration: 1` (integer) → must be `duration: "1s"` (string)
- Placing `delay` after the rate-limited task — delay fires before the record reaches the next task, so it must precede it
- Using `delay` on every record for very large datasets without calculating total pipeline time
- Not combining `delay` with `task_concurrency` when higher throughput is needed despite rate limiting
