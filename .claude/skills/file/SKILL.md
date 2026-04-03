---
skill: file
version: 1.0.0
caterpillar_type: file
description: Read records from or write records to a local file or S3 object.
role: source | sink
requires_upstream: false   # read mode has no upstream; write mode requires upstream
requires_downstream: false # write mode has no downstream; read mode requires downstream
aws_required: conditional  # only when path starts with s3://
---

## Purpose

Dual-mode task. Automatically detects its role:
- **Read mode** (source): no upstream task → reads file, emits one record per delimiter
- **Write mode** (sink): has upstream task → receives records, writes each to the file

## Schema

```yaml
- name: <string>                    # REQUIRED — unique task name
  type: file                        # REQUIRED — must be exactly "file"
  path: <string>                    # REQUIRED — local path, S3 URL, or glob pattern
  region: <string>                  # OPTIONAL — AWS region (default: us-west-2, S3 only)
  delimiter: <string>               # OPTIONAL — record separator in read mode (default: \n)
  success_file: <bool>              # OPTIONAL — write _SUCCESS marker after write (default: false)
  success_file_name: <string>       # OPTIONAL — success marker filename (default: _SUCCESS)
  fail_on_error: <bool>             # OPTIONAL — stop pipeline on error (default: false)
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| path starts with `s3://` | set `region` |
| path is the first task | read mode (source) |
| path has upstream task | write mode (sink) |
| reading multiple files | use glob pattern (e.g. `s3://bucket/prefix/*.json`) |
| output filename must be unique per run | use `{{ macro "timestamp" }}` or `{{ macro "uuid" }}` in path |
| output path depends on record data | use `{{ context "key" }}` in path |
| writing to S3 and a downstream system needs confirmation | set `success_file: true` |
| credentials come from environment | use `{{ env "VAR" }}` in path |
| credentials come from AWS SSM | use `{{ secret "/path" }}` in path |

## Validation Rules

- `path` is required
- Glob patterns are read-mode only — flag if glob appears in write-mode position
- `success_file` only applies to write mode — flag if set on a source task
- S3 paths must begin with `s3://`
- When `path` contains `{{ context "key" }}`, verify an upstream task sets that key in its `context:` block
- `fail_on_error: true` is recommended for source tasks in production pipelines

## Template functions supported in `path`

```
{{ env "BUCKET" }}            → resolved once at pipeline init
{{ secret "/ssm/path" }}      → resolved once at pipeline init
{{ macro "timestamp" }}       → resolved per record
{{ macro "uuid" }}            → resolved per record
{{ macro "unixtime" }}        → resolved per record
{{ context "key" }}           → resolved per record, set by upstream task
```

## Examples

### Read — local file, split on newlines
```yaml
- name: read_input
  type: file
  path: data/records.txt
  delimiter: "\n"
  fail_on_error: true
```

### Read — S3 glob (multiple files)
```yaml
- name: read_s3_files
  type: file
  path: s3://my-bucket/incoming/2024-03-*.json
  region: us-west-2
  fail_on_error: true
```

### Write — local file with timestamp
```yaml
- name: write_output
  type: file
  path: output/result_{{ macro "timestamp" }}.json
```

### Write — S3 with success marker
```yaml
- name: write_s3
  type: file
  path: s3://my-bucket/processed/data_{{ macro "uuid" }}.json
  region: us-east-1
  success_file: true
```

### Write — per-record dynamic path using context
```yaml
- name: write_per_user
  type: file
  path: output/{{ context "user_id" }}_{{ macro "timestamp" }}.json
```

## Anti-patterns

- Hardcoding bucket names → use `{{ env "BUCKET" }}` or `{{ secret "/path" }}`
- Using glob patterns in write mode → not supported
- Setting `success_file: true` on a source task → only valid for write mode
- Missing `region` for S3 paths → defaults to `us-west-2`; make explicit for cross-region access

## IAM permissions (S3)

```
s3:GetObject         # read
s3:PutObject         # write
s3:ListBucket        # glob patterns
```
