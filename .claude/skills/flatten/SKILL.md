---
skill: flatten
version: 1.0.0
caterpillar_type: flatten
description: Flatten nested JSON objects into single-level key-value pairs using underscore-joined keys.
role: transform
requires_upstream: true
requires_downstream: true
aws_required: false
---

## Purpose

Converts a deeply nested JSON object into a flat map. Nested keys are joined with `_`.
Optionally preserves the original nested structure under a specified key.

## Schema

```yaml
- name: <string>                # REQUIRED
  type: flatten                 # REQUIRED
  include_original: <string>    # OPTIONAL — key name to store original nested data
  fail_on_error: <bool>         # OPTIONAL (default: false)
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Need flat key-value pairs for CSV / DB | basic `flatten` |
| Need both flat AND original nested | set `include_original: "raw"` (or any key name) |
| Only specific nested object | add `jq` upstream to extract it first, then flatten |
| Arrays in nested data | arrays are indexed: `items_0`, `items_1`, … |

## Flattening Behavior

**Input:**
```json
{
  "user": {
    "id": 42,
    "address": { "city": "Portland", "zip": "97201" }
  },
  "status": "active"
}
```

**Output (no include_original):**
```json
{
  "user_id": 42,
  "user_address_city": "Portland",
  "user_address_zip": "97201",
  "status": "active"
}
```

**Output (include_original: "raw"):**
```json
{
  "user_id": 42,
  "user_address_city": "Portland",
  "user_address_zip": "97201",
  "status": "active",
  "raw": { "user": { "id": 42, ... }, "status": "active" }
}
```

## Array Flattening

Arrays produce indexed keys:
```json
Input:  { "tags": ["news", "tech"] }
Output: { "tags_0": "news", "tags_1": "tech" }
```

## Validation Rules

- `flatten` operates on JSON objects — upstream data must be valid JSON
- Deep nesting produces long key names — review expected output key names
- Array indexing is automatic — warn users if they expect arrays to be preserved
- `include_original` value is any non-empty string (used as the key name in output)

## Examples

### Basic flatten
```yaml
- name: flatten_response
  type: flatten
```

### Flatten preserving original
```yaml
- name: flatten_with_backup
  type: flatten
  include_original: raw
```

### Extract then flatten (specific sub-object)
```yaml
- name: extract_user
  type: jq
  path: .user

- name: flatten_user
  type: flatten
```

### API response → flatten → write CSV
```yaml
tasks:
  - name: fetch
    type: http
    method: GET
    endpoint: https://api.example.com/users

  - name: parse_users
    type: jq
    path: .data[]
    explode: true

  - name: flatten
    type: flatten

  - name: write
    type: file
    path: output/users_flat_{{ macro "timestamp" }}.json
```

### SQS events → flatten → ingest API
```yaml
tasks:
  - name: source
    type: sqs
    queue_url: "{{ env "SQS_QUEUE_URL" }}"
    exit_on_empty: true

  - name: flatten_event
    type: flatten

  - name: post
    type: http
    method: POST
    endpoint: https://ingest.example.com/flat-events
    headers:
      Content-Type: application/json
```

## Anti-patterns

- Flattening without first checking key length — deeply nested objects with array items produce very long keys
- Expecting arrays to be preserved — they become indexed `_0`, `_1`, … keys
- Not using `jq` upstream when only a sub-object needs flattening — whole record is flattened otherwise
- Using `flatten` on non-JSON data — will produce a runtime error
