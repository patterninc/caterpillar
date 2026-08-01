---
skill: sample
version: 1.0.0
caterpillar_type: sample
description: Filter records using a sampling strategy — head, tail, nth, random, or percent.
role: transform
requires_upstream: true
requires_downstream: true
aws_required: false
---

## Purpose

Selects a subset of records using one of five strategies. Useful for development (limit data volume), QA (representative sampling), and performance throttling.

**Constraint**: cannot be the first or last task — requires both input and output channels.

## Schema

```yaml
- name: <string>          # REQUIRED
  type: sample            # REQUIRED
  filter: <string>        # OPTIONAL — strategy (default: random)
  limit: <int>            # OPTIONAL — record count (head, tail, nth)
  percent: <int>          # OPTIONAL — percent to keep (random, percent)
  divider: <int>          # OPTIONAL — denominator for random (default: 1000)
  size: <int>             # OPTIONAL — buffer size for random strategy (default: 50000)
  fail_on_error: <bool>   # OPTIONAL (default: false)
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Take first N records | `filter: head`, `limit: N` |
| Take last N records | `filter: tail`, `limit: N` |
| Take every Nth record | `filter: nth`, `limit: N` |
| Random X% of records | `filter: random`, `percent: X`, `divider: 100` |
| Exact percentage | `filter: percent`, `percent: X` |
| Development — limit to small set | `filter: head`, `limit: 100` |
| QA sampling — 10% random | `filter: random`, `percent: 10`, `divider: 100` |
| Sparse sample 0.1% | `filter: random`, `percent: 1`, `divider: 1000` |

## Strategy Reference

| Filter | Keeps | Key fields |
|--------|-------|-----------|
| `random` | `percent/divider` fraction, randomly | `percent`, `divider`, `size` |
| `head` | First `limit` records | `limit` |
| `tail` | Last `limit` records (buffers all) | `limit` |
| `nth` | Records at positions 1, 1+N, 1+2N, … | `limit` |
| `percent` | Exactly `percent`% of records | `percent` |

## Throughput Calculator

```
random: effective_rate = percent / divider
  percent: 10, divider: 100  → 10%   (1 in 10)
  percent: 1,  divider: 100  → 1%    (1 in 100)
  percent: 1,  divider: 1000 → 0.1%  (1 in 1000)
  percent: 5,  divider: 100  → 5%    (1 in 20)
```

## Validation Rules

- `sample` cannot be the first task (no source mode) — flag if at position 0
- `sample` cannot be the last task (no sink mode) — flag if at end of task list
- `tail` strategy buffers all records in memory before emitting — warn for large datasets
- `nth` selects record 1, then every N records after — confirm this matches user's intent vs. random sampling

## Examples

### Dev: first 100 records
```yaml
- name: dev_limit
  type: sample
  filter: head
  limit: 100
```

### QA: random 10% sample
```yaml
- name: qa_sample
  type: sample
  filter: random
  percent: 10
  divider: 100
```

### Every 50th record
```yaml
- name: sparse
  type: sample
  filter: nth
  limit: 50
```

### Last 5 records
```yaml
- name: tail_check
  type: sample
  filter: tail
  limit: 5
```

### Sparse 0.1% sample
```yaml
- name: very_sparse
  type: sample
  filter: random
  percent: 1
  divider: 1000
```

### Development pipeline with head sample
```yaml
tasks:
  - name: read_large
    type: file
    path: s3://my-bucket/huge-dataset.json

  - name: split
    type: split

  - name: dev_sample
    type: sample
    filter: head
    limit: 50

  - name: transform
    type: jq
    path: '{ "id": .id, "value": .v }'

  - name: echo
    type: echo
    only_data: true
```

## Anti-patterns

- Placing `sample` as the first or last task — it requires both upstream and downstream
- Using `tail` on a large stream — buffers everything in memory before emitting
- Confusing `nth` with "every Nth starting at N" — it starts at record 1, then 1+N, 1+2N, …
- Using `filter: random` with `percent: 10` and no `divider` — default `divider: 1000` means 10/1000 = 1% not 10%
