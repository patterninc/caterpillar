---
skill: compress
version: 1.0.0
caterpillar_type: compress
description: Compress or decompress record data using gzip, snappy, zlib, or deflate.
role: transform
requires_upstream: true
requires_downstream: true
aws_required: false
---

## Purpose

Applies a compression or decompression algorithm to each record's data.
Typically placed immediately before a `file` write (compress) or immediately after a `file` read (decompress).

## Schema

```yaml
- name: <string>          # REQUIRED
  type: compress          # REQUIRED
  format: <string>        # REQUIRED — "gzip", "snappy", "zlib", or "deflate"
  action: <string>        # REQUIRED — "compress" or "decompress"
  fail_on_error: <bool>   # OPTIONAL (default: false)
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| General purpose, wide compatibility | `format: gzip` |
| Fastest compress/decompress | `format: snappy` |
| Standard deflate with header | `format: zlib` |
| Raw deflate, no header | `format: deflate` |
| Writing compressed output | `action: compress`, place before `file` write task |
| Reading compressed input | `action: decompress`, place after `file` read task |
| Output file extension | append `.gz`, `.snappy`, `.zlib` in the downstream `file` path |

## Format Comparison

| Format | Speed | Ratio | Compatibility |
|--------|-------|-------|--------------|
| `gzip` | Medium | Good | Universal |
| `snappy` | Fast | Moderate | Kafka, Parquet, Hadoop |
| `zlib` | Medium | Good | Wide |
| `deflate` | Medium | Good | Wide (no header) |

## Validation Rules

- Both `format` and `action` are required — flag if either is missing
- Do not compress already-compressed data — warn if the upstream task is also `compress`
- Output format should match the downstream consumer's expected format
- Use matching file extension in `file` task path for clarity

## Examples

### Compress with gzip → write to S3
```yaml
- name: compress_output
  type: compress
  format: gzip
  action: compress

- name: write_s3
  type: file
  path: s3://my-bucket/data/output_{{ macro "timestamp" }}.gz
```

### Read from S3 → decompress gzip → process
```yaml
- name: read_compressed
  type: file
  path: s3://my-bucket/archive/data.gz

- name: decompress
  type: compress
  format: gzip
  action: decompress

- name: parse_json
  type: jq
  path: .records[]
  explode: true
```

### Compress with snappy (Kafka / Hadoop pipelines)
```yaml
- name: compress_snappy
  type: compress
  format: snappy
  action: compress
```

### Full pipeline: transform → compress → archive
```yaml
tasks:
  - name: source
    type: sqs
    queue_url: "{{ env "SQS_QUEUE_URL" }}"
    exit_on_empty: true

  - name: transform
    type: jq
    path: '{ "id": .id, "ts": "{{ macro "timestamp" }}", "data": .payload }'

  - name: compress
    type: compress
    format: gzip
    action: compress

  - name: write
    type: file
    path: s3://{{ env "OUTPUT_BUCKET" }}/batch_{{ macro "uuid" }}.gz
    success_file: true
```

## Anti-patterns

- Missing `format` or `action` — both are required
- Compressing already-compressed data — results in larger output and wasted CPU
- Using `snappy` when the downstream consumer expects `gzip` — formats are not interchangeable
- Not matching file extension in `path` (e.g. writing `.json` but data is gzip) — use `.gz`, `.snappy`
