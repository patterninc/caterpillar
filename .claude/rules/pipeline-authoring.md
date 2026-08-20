---
description: Pipeline authoring rules — structure, naming, constraints, and production safeguards.
globs: "**/*.yaml,**/*.yml"
---

# Pipeline Authoring Rules

## Pipeline Structure

- First task must be a source: `file`, `kafka`, `sqs`, `http`, `http_server`, `aws_parameter_store`.
- Last task must be a sink: `file`, `kafka`, `sqs`, `sns`, `echo`.
- Transforms (`jq`, `split`, `join`, `replace`, `flatten`, `xpath`, `converter`, `compress`, `archive`, `sample`, `delay`) must sit between source and sink — never first.
- Every pipeline must have a natural termination point — avoid infinite-polling pipelines in batch jobs.

## Auto-Detect Role

`file`, `kafka`, `sqs`, `http` auto-detect source vs sink based on position. First task = source (read mode); has upstream = sink (write mode).

## Naming

- Task `name` must be unique within a pipeline.
- Use descriptive snake_case names: `read_from_sqs`, `transform_payload`, `write_to_s3`.
- Avoid generic names like `task1`, `step2`, `process`.
- Pipeline filenames should reflect their purpose: `kafka_to_s3.yaml`, not `pipeline1.yaml`.
- Task `type` values use underscores: `aws_parameter_store`, `http_server` — not hyphens.

## Template Functions

Use these in any string field value:

| Function | When resolved |
|----------|--------------|
| `{{ env "VAR" }}` | once at pipeline init |
| `{{ secret "/ssm/path" }}` | once at pipeline init |
| `{{ macro "timestamp" }}` | per record |
| `{{ macro "uuid" }}` | per record |
| `{{ macro "unixtime" }}` | per record |
| `{{ macro "microtimestamp" }}` | per record |
| `{{ context "key" }}` | per record — value set by upstream task's `context:` block |

- `{{ env }}` and `{{ secret }}` are static — do not use where per-record dynamic values are needed.
- Nested templates are not supported — `{{ secret "{{ env "X" }}" }}` will fail.
- Valid macro names: `timestamp`, `uuid`, `unixtime`, `microtimestamp`.

## Error Handling

- Add `fail_on_error: true` to source tasks — a silent source failure with exit code 0 is a false success.
- Add `fail_on_error: true` to any task that calls external services in critical pipelines.

## Context Variables

- Set context keys in the same task that reads the data, close to the source.
- Every `{{ context "key" }}` reference must have a matching `context: { key: ".jq_expr" }` in an upstream task.
- Do not reference a context key before it is set.

## Source-Specific Rules

Before tuning source fields or writing transforms for a new source, **sample one record and infer schema first** — see `.claude/rules/source-schema-first.md` (and the `source-schema-detector` agent).

**Kafka**
- Always set `group_id` in production — without it, offsets are not committed and messages may be reprocessed.
- `batch_flush_interval` must be less than `timeout` in write mode.
- Do not use `user_auth_type: mtls` — not implemented, will error at runtime.

**SQS**
- Set `exit_on_empty: true` for batch jobs that should terminate when the queue drains.
- FIFO queues (URL ends in `.fifo`) require `message_group_id` in write mode.
- `max_messages` must be ≤ 10.

**File / S3**
- S3 paths must have an explicit `region` field.
- Write-mode paths should use `{{ macro "uuid" }}` or `{{ macro "timestamp" }}` to avoid overwriting existing files.
- Do not use glob patterns in write mode.
- Add `success_file: true` when downstream systems need a completion signal.

**HTTP**
- Set `max_retries` and `retry_delay` for unreliable external APIs.
- Pagination `next_page` expression must eventually return null/empty — verify there is a terminal condition.

## JSON Output Format

- Caterpillar's `jq` task always outputs **compact/minified JSON** (single line). It has no built-in pretty-print option.
- When writing multiple JSON records to a single file as a JSON array, wrap inside `jq` using `[.items[] | {...}]` — do **not** use `explode: true` + `join` + `replace` to reconstruct an array. That pattern produces malformed output.
- For NDJSON (one JSON object per line), use `explode: true` with no `join` and name the file `.ndjson`.
- Never use `join` + string manipulation to build JSON structure — always use `jq` for JSON construction.
- Always run pipelines via `.claude/scripts/run-pipeline.sh <pipeline.yaml>` instead of `./caterpillar -conf` directly — the wrapper auto-detects new JSON output files and pretty-prints them after the run.

## Sink-Specific Rules

- Remove `echo` sinks before promoting a pipeline to production — replace with a real sink.
- `sns` is terminal — do not add tasks after it.

## Readability

- Group related fields together within a task block.
- Align multiline JQ `path:` expressions with consistent indentation using YAML block scalar (`|`).
- Long pipelines (10+ tasks) should have comment headers separating logical stages: `# --- Source ---`, `# --- Transform ---`, `# --- Sink ---`.
- Add a `#` comment on any non-obvious config choice to explain why.

## Production Safeguards

When editing an existing production pipeline, confirm with the user before:
- Changing `type`, `topic`, `queue_url`, `bootstrap_server`, `endpoint`, or `path` — these change what data flows where.
- Reordering tasks or removing `join`/`split` tasks — changes the downstream data shape.
- Changing `group_id` on a Kafka consumer — changes offset tracking.
- Changing `exit_on_empty` from `true` to `false` on SQS — turns a batch job into an infinite consumer.
- Renaming a context key that is referenced downstream with `{{ context "key" }}`.
