---
description: Rules for pipeline testing — environment safety, test file standards, and incremental approach.
globs: "test/pipelines/**/*.yaml,test/pipelines/**/*.yml"
---

# Pipeline Testing Rules

## Environment Check — Always First (MANDATORY)

Before running any pipeline against live AWS resources (SQS, SNS, S3, SSM, Kafka), verify the environment is sandbox:

1. Run `aws sts get-caller-identity` to get the account ID.
2. Run `aws iam list-account-aliases` to get the account alias.
3. The account is sandbox/dev ONLY if the alias or account ID contains: `sandbox`, `dev`, `test`, `staging`, or `nonprod`.
4. **If the account is production or cannot be determined — REFUSE to run the pipeline. Do not proceed even if the user asks.** Tell the user to switch to a sandbox account first.
5. If the account is sandbox — proceed.

Use `/project:check-aws` to run the full environment check.

**Pipelines must only run against sandbox AWS accounts. Production execution is blocked.**

## Non-Sandbox Resource Detection — Mock Before Run

Before running any pipeline, scan the YAML for non-sandbox resources:

1. **Detect non-sandbox references** — a resource is non-sandbox unless its URL, ARN, path, or hostname explicitly contains `sandbox`, `dev`, `test`, `staging`, or `nonprod`. Flag any task field that does NOT match:
   - `queue_url` without a sandbox indicator
   - `topic_arn` without a sandbox indicator
   - `bootstrap_server` without a sandbox indicator
   - `endpoint` without a sandbox indicator
   - `path` with `s3://` without a sandbox indicator in the bucket name
   - `{{ secret "..." }}` SSM paths that are not under `/sandbox/`, `/dev/`, `/test/`, or `/staging/` prefixes

2. **If any non-sandbox resource is found — do NOT run the pipeline.** Instead:
   - Tell the user which fields reference production resources.
   - Ask the user to provide a **mock sample input** (paste JSON, CSV, or text).
   - Save the mock input to `test/pipelines/samples/<pipeline_name>_mock.json`.
   - Generate a **mock test pipeline** that:
     - Replaces the production source with `type: file` reading the mock sample file.
     - Replaces the production sink with `type: echo` (`only_data: true`).
     - Keeps all transforms unchanged so the data flow logic is fully tested.
   - Save the mock pipeline to `test/pipelines/<pipeline_name>_mock_test.yaml`.
   - Run the mock pipeline and show the output to the user for verification.

3. **Only after the mock test passes** — deliver the production pipeline YAML to the user for deployment through their own CI/CD process.

**Only sandbox resources are allowed for local execution. Everything else is validated with mock data only.**

## Test Pipeline Requirements

- Every production pipeline must have a corresponding test pipeline in `test/pipelines/`.
- Test pipelines must use local file sources — not live Kafka, SQS, S3, or external HTTP APIs.
- Test pipelines must use `type: echo` with `only_data: true` as the sink — no real writes.
- Test pipelines must be runnable from the project root: `./caterpillar -conf test/pipelines/<name>.yaml`.

## Test Pipeline Naming

- Name test pipelines after the feature they verify: `kafka_read_test.yaml`, `converter_csv_test.yaml`.
- For converter tests, place sample input and expected output files alongside the test pipeline in `test/pipelines/converter/`.

## What a Good Test Pipeline Covers

- Happy path: valid input produces expected output
- Edge cases: empty file, single record, record with missing fields
- Template functions used in the production pipeline (`{{ macro }}`, `{{ context }}`) should be exercised

## Incremental Testing Approach

Use `/pipeline-tester` to generate a test plan. The standard approach is:

1. **Inspect source** — curl / aws cli / kcat to see real data shape before writing any pipeline.
2. **Capture sample** — save 10 real records to `test/pipelines/samples/` as a local file.
3. **Probe each transform** — test one transform at a time using the sample file as source + `echo` as sink.
4. **Chain forward** — add transforms one by one, verify output at each step.
5. **Verify sink** — write to a local file first, inspect shape before hitting the real sink.
6. **Smoke test** — run against the real sink with `sample: head limit: 3`.

Sample data lives in `test/pipelines/samples/`. Probe pipelines live in `test/pipelines/probes/`.

## Do Not

- Do not use production queue URLs, Kafka topics, S3 buckets, or live API endpoints in test pipelines.
- Do not commit test pipelines that require AWS credentials or network access to run.
- Do not leave test pipelines that fail — a broken test pipeline is worse than no test.
