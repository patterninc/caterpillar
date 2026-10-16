---
skill: archive
version: 1.0.0
caterpillar_type: archive
description: Pack multiple file records into a zip/tar archive, or unpack an archive into individual file records.
role: transform
requires_upstream: true
requires_downstream: true
aws_required: false
---

## Purpose

Two modes:
- **Pack**: buffers all incoming records → emits one archive record containing them all
- **Unpack**: receives one archive record → emits one record per file inside the archive

## Schema

```yaml
- name: <string>          # REQUIRED
  type: archive           # REQUIRED
  format: <string>        # OPTIONAL — "zip" or "tar" (default: zip)
  action: <string>        # OPTIONAL — "pack" or "unpack" (default: pack)
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Bundle files for delivery | `action: pack` |
| Extract files for processing | `action: unpack` |
| Target system expects ZIP | `format: zip` |
| Unix/Linux environment | `format: tar` |
| Compressed TAR (`.tar.gz`) needed | `format: tar` + `compress` task after with `format: gzip` |
| Multiple files in, one archive out | `action: pack` |
| One archive in, multiple files out | `action: unpack` |

## Behavior Details

| Action | Input | Output |
|--------|-------|--------|
| `pack` | N records (file contents) | 1 archive record |
| `unpack` | 1 archive record | N records (one per file) |

**Note**: `pack` buffers all upstream records in memory before emitting — be cautious with large datasets.

## Validation Rules

- `action: pack` collects everything in memory before emitting — warn for large input streams
- TAR format has no built-in compression — combine with `compress` task for `.tar.gz`
- ZIP is more widely compatible across OS environments
- After `unpack`, each record contains one file's content — downstream tasks process individual files

## Examples

### Pack files into ZIP → write
```yaml
- name: pack_files
  type: archive
  format: zip
  action: pack

- name: write_archive
  type: file
  path: output/bundle_{{ macro "timestamp" }}.zip
```

### Unpack ZIP → process each file
```yaml
- name: read_archive
  type: file
  path: incoming/bundle.zip

- name: unpack
  type: archive
  format: zip
  action: unpack

- name: process
  type: converter
  format: csv
  skip_first: true
```

### Pack → TAR → gzip compress → S3
```yaml
- name: pack_tar
  type: archive
  format: tar
  action: pack

- name: compress
  type: compress
  format: gzip
  action: compress

- name: upload
  type: file
  path: s3://{{ env "BUCKET" }}/archive_{{ macro "timestamp" }}.tar.gz
```

### Unpack TAR with multiple files
```yaml
- name: read_tar
  type: file
  path: s3://my-bucket/incoming/data.tar

- name: extract
  type: archive
  format: tar
  action: unpack

- name: inspect
  type: echo
  only_data: false
```

### Full pipeline: SQS → collect → pack → S3
```yaml
tasks:
  - name: read_queue
    type: sqs
    queue_url: "{{ env "SQS_QUEUE_URL" }}"
    exit_on_empty: true

  - name: transform
    type: jq
    path: '{ "id": .id, "content": .body }'

  - name: pack
    type: archive
    format: zip
    action: pack

  - name: upload
    type: file
    path: s3://{{ env "BUCKET" }}/batches/{{ macro "uuid" }}.zip
    success_file: true
```

## Anti-patterns

- `action: pack` on large unbounded streams — buffers all records in memory; set upstream `join` or `sample` limits first
- Expecting `.tar.gz` from `archive` alone — combine with `compress` task
- Using `unpack` on a non-archive file — produces runtime error
- Placing `archive` as source (first task) — it requires an upstream task
