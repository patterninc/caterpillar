---
skill: split
version: 1.0.0
caterpillar_type: split
description: Split one record into many by a delimiter — turns a multi-line blob into individual records.
role: transform
requires_upstream: true
requires_downstream: true
aws_required: false
---

## Purpose

Takes each incoming record's data string and splits it by `delimiter`, emitting one record per segment.
Most commonly used after a `file` or `http` source that reads entire file/response as one record.

## Schema

```yaml
- name: <string>          # REQUIRED
  type: split             # REQUIRED
  delimiter: <string>     # OPTIONAL — character or string to split on (default: \n)
  fail_on_error: <bool>   # OPTIONAL (default: false)
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Multi-line text file → individual lines | default `delimiter: "\n"` |
| CSV row → individual fields | `delimiter: ","` |
| TSV row → individual fields | `delimiter: "\t"` |
| Pipe-delimited data | `delimiter: "\|"` |
| Custom section separator | `delimiter: "---"` |
| JSON-lines file (one JSON per line) | `split` with default, then `jq` to parse each line |
| Empty segments appear (trailing newline) | add `jq` filter after: `select(. != "")` |

## Behavior

```
Input record data:  "line1\nline2\nline3"
Delimiter:          "\n"

Output records:
  record 1 → "line1"
  record 2 → "line2"
  record 3 → "line3"
```

## Validation Rules

- `split` must have both upstream and downstream tasks — it is not a source or sink
- Empty string segments (e.g. from trailing delimiter) produce empty records — filter with downstream `jq select(. != "")`
- `split` operates on the raw data string — not on JSON fields; use `jq` + `explode: true` for JSON arrays instead

## Common Delimiter Reference

| Format | YAML value |
|--------|-----------|
| Newline (default) | `"\n"` or omit |
| Comma | `","` |
| Tab | `"\t"` |
| Pipe | `"\|"` |
| Semicolon | `";"` |
| Section separator | `"---"` |

## Examples

### Split file into lines (default)
```yaml
- name: read_file
  type: file
  path: data/records.txt

- name: split_lines
  type: split

- name: process
  type: jq
  path: '{ "line": . }'
```

### Split CSV row into fields
```yaml
- name: split_csv
  type: split
  delimiter: ","
```

### Split JSON-lines → parse each
```yaml
- name: split_lines
  type: split

- name: parse_each
  type: jq
  path: . | fromjson
```

### Filter empty lines after split
```yaml
- name: split_lines
  type: split

- name: remove_empty
  type: jq
  path: . | select(. != "")
```

### Full pipeline: HTTP response → split → process
```yaml
tasks:
  - name: fetch
    type: http
    method: GET
    endpoint: https://api.example.com/export/csv

  - name: split_lines
    type: split

  - name: parse_csv
    type: converter
    format: csv
    skip_first: true
```

## Anti-patterns

- Using `split` as the first task — it has no source mode, requires upstream
- Using `split` on JSON arrays — use `jq` with `explode: true` instead
- Not filtering empty segments from trailing delimiters
- Splitting JSON objects with commas — use `jq` not `split` for structured data
