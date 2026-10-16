---
skill: echo
version: 1.0.0
caterpillar_type: echo
description: Print record data to stdout. Use as a debug probe, pipeline monitor, or terminal sink.
role: sink | pass-through
requires_upstream: true
requires_downstream: false  # terminal when last task; pass-through when not last
aws_required: false
---

## Purpose

Prints each record to stdout. When used as the last task it is a terminal sink.
When placed mid-pipeline it is a pass-through — records continue to the next task after printing.

Two output modes:
- `only_data: true` — prints the record's data field as-is (clean output)
- `only_data: false` — prints the full record envelope as JSON (includes ID, origin, context)

## Schema

```yaml
- name: <string>          # REQUIRED
  type: echo              # REQUIRED
  only_data: <bool>       # OPTIONAL — true = data only, false = full record JSON (default: false)
  fail_on_error: <bool>   # OPTIONAL (default: false)
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| See clean data output | `only_data: true` |
| Inspect record ID, origin, context | `only_data: false` |
| Terminal task (no downstream needed) | last position in task list |
| Mid-pipeline debug checkpoint | any position except last |
| Probe pipeline for task testing | last position, `only_data: true` |
| Production pipeline — no output needed | replace with `file` or other sink |

## Output Format Comparison

`only_data: true`:
```
{"id": 1, "name": "Alice", "status": "active"}
```

`only_data: false`:
```json
{
  "id": "a1b2c3d4-...",
  "origin": "fetch_users",
  "data": "{\"id\": 1, \"name\": \"Alice\"}",
  "context": { "user_id": "1" }
}
```

## Validation Rules

- `echo` must have an upstream task — it is never a source
- When not the last task, records pass through transparently
- `only_data: false` shows data as an escaped JSON string inside the envelope — if output appears double-encoded, switch to `only_data: true`
- For production pipelines, replace `echo` with a proper sink (`file`, `sqs`, `http`, etc.)

## Examples

### Terminal sink (dev/test)
```yaml
- name: output
  type: echo
  only_data: true
```

### Full record inspection (debug)
```yaml
- name: inspect
  type: echo
  only_data: false
```

### Mid-pipeline checkpoint (pass-through)
```yaml
- name: source
  type: file
  path: data/input.json

- name: debug_raw
  type: echo
  only_data: true           # prints, passes record forward

- name: transform
  type: jq
  path: '{ "id": .id }'

- name: debug_transformed
  type: echo
  only_data: true           # prints again, passes forward

- name: write
  type: file
  path: output/result.json
```

### Probe pipeline (isolate one task for testing)
```yaml
# Probe for testing the 'converter' task
tasks:
  - name: source_stub
    type: file
    path: test/pipelines/names.txt

  - name: task_under_test
    type: split

  - name: probe_sink
    type: echo
    only_data: true
```

## Anti-patterns

- Using `echo` as a production sink when data should be saved or forwarded
- Confusing double-encoded output from `only_data: false` — the data field is a JSON-encoded string inside the JSON envelope
- Placing `echo` as the first task — it has no source mode
- Forgetting to replace `echo` with a real sink before deploying to production
