---
description: When source connection details are known, the first step is to sample one record and infer schema before designing transforms or jq paths.
globs: "**/*"
---

# Source schema first (mandatory)

As soon as you have **concrete source details** (HTTP endpoint and auth, SQS queue URL, Kafka bootstrap/topic, S3 or local path, SSM path, etc.), your **first** action before proposing transforms, `jq` expressions, `context:` keys, or sink field mappings is:

1. **Pull at least one real record** from that source (or the closest safe peek: e.g. SQS with `visibility-timeout 0`, read-only S3 head/get of first line, `curl` sample, `kcat -c 1`, local `head`).
2. **Infer the schema** from that sample: field names, types, nesting, arrays, wrapper keys (e.g. `.items[]`), and whether the payload is JSON, CSV, or opaque text.

## How to do it

- Run **`.claude/scripts/check-source-schema.sh`** with the matching subcommand (`http`, `s3`, `sqs`, `file`, `ssm`, `ssm-path`, `kafka`, or pipe arbitrary bytes into `stdin`). It fetches one sample and prints pretty JSON plus an inferred field table. Use `--no-schema` for a raw-only preview.
- Or invoke the **`source-schema-detector`** agent; it mirrors the same flows in `.claude/agents/source-schema-detector.md`.
- If live access fails (empty queue, auth, network), **ask the user to paste one representative record** and run `... check-source-schema.sh stdin` on it (or `python3 .claude/scripts/lib/source_schema_report.py` on stdin).

## Do not

- Do **not** invent or assume field names and paths without a sample.
- Do **not** skip this step to “save time” when building or debugging pipelines that depend on payload shape.

This applies in **all** conversations where source details appear — not only when using the interactive pipeline builder.
