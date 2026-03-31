# Caterpillar

Caterpillar is a data pipeline tool. Pipelines are defined as YAML files with a `tasks:` list. Each task runs sequentially — output of one task feeds the next.

## Pipeline Structure

```yaml
tasks:
  - name: <unique_name>
    type: <task_type>
    # task-specific fields
```

## Available Task Types

| type | role |
|------|------|
| `file` | source (read) or sink (write) — local path, S3, or glob |
| `kafka` | source or sink — supports TLS + SASL/SCRAM |
| `sqs` | source or sink — AWS SQS |
| `http` | source (fetch URL) or sink (POST records) |
| `http_server` | source — listens for inbound HTTP requests |
| `aws_parameter_store` | source or sink — AWS SSM parameters |
| `sns` | sink only — publish to AWS SNS |
| `echo` | sink or pass-through — print to stdout |
| `jq` | transform — JQ expression on JSON records |
| `split` | transform — split record data into multiple records |
| `join` | transform — batch N records into one |
| `replace` | transform — regex find-and-replace |
| `flatten` | transform — flatten nested JSON with `_` separator |
| `xpath` | transform — extract from XML/HTML via XPath |
| `converter` | transform — convert CSV/Excel/HTML/EML formats |
| `compress` | transform — gzip/snappy/zlib/deflate |
| `archive` | transform — zip/tar pack or unpack |
| `sample` | filter — head/tail/nth/random/percent |
| `delay` | rate-limit — pause between records |
| `heimdall` | transform — submit jobs to Heimdall |

## Generating Pipelines

When a user asks to build, create, or write a pipeline — use the `pipeline-builder-interactive` agent. It asks targeted questions about source, transforms, sink, and auth before writing the file. The validation hook runs automatically after the file is written.

Use the `pipeline-builder` skill only as a schema reference when you already have all the details and just need to generate YAML directly.

## Pipeline Review Agents

Use these sub-agents to validate, debug, and optimize pipelines:

| Agent | Purpose | When to use |
|-------|---------|-------------|
| `pipeline-review` | Full review: lint + validate + permissions + optimize | Before shipping any pipeline |
| `pipeline-lint` | Structure, types, required fields, credential security | First check on a new pipeline |
| `pipeline-validate` | Context keys, JQ expressions, inter-task data flow | After lint passes |
| `pipeline-permissions` | AWS IAM policy generation, region checks | When deploying to AWS |
| `pipeline-debugger` | Error diagnosis, echo probe insertion, fix suggestions | When a pipeline fails |
| `pipeline-runner` | Build binary and run pipeline, interpret output | Smoke tests and end-to-end testing |
| `pipeline-optimizer` | Concurrency, batching, error handling, production-readiness | Before production deploy |

Invoke via the Agent tool or ask Claude to "review my pipeline", "debug this error", "check permissions for", etc.

## Example Pipelines

**Before writing any pipeline**, read the matching example from `test/pipelines/examples/`:

```
test/pipelines/
├── examples/
│   ├── basic/          ← file-to-file, NDJSON, CSV, echo
│   ├── transforms/     ← jq, flatten, split/join, replace, context
│   ├── integrations/   ← kafka, sqs, http combos
│   └── production/     ← OAuth, auth chains, webhooks, SNS, compression
├── probes/             ← isolated single-task test pipelines
└── samples/            ← sample data files (JSON, NDJSON, CSV, text)
```

Use examples as templates. Match the user's request to the closest pattern, read that file, then adapt it.
