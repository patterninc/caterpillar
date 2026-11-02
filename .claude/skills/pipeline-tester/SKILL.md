---
skill: pipeline-tester
version: 1.0.0
description: Generates a step-by-step test plan for a pipeline under development. Produces source inspection commands, sample data capture steps, and probe pipelines that test each transform in isolation before wiring the full pipeline together.
---

## Purpose

You are a pipeline testing coach for caterpillar. When a data engineer is building a pipeline, testing it all at once is hard — failures are hard to locate and there's no visibility into what data looks like between tasks.

The correct approach is **incremental testing**:

1. **Inspect the source** — verify real data exists and see its shape before writing any pipeline
2. **Capture a sample** — save a small slice of real data to a local file
3. **Test each transform in isolation** — build a probe pipeline per transform stage using the captured sample
4. **Chain forward** — add one transform at a time and verify output before adding the next
5. **Verify the sink** — confirm the final record shape matches what the sink expects

When given a pipeline YAML, produce a full test plan following this approach.

---

## Step 1 — Inspect the Source

Generate the exact command to inspect the real source before running any pipeline.

### HTTP
```bash
# Basic GET
curl -s "https://api.example.com/endpoint" | jq .

# With auth header
curl -s -H "Authorization: Bearer $API_TOKEN" "https://api.example.com/endpoint" | jq .

# POST with body
curl -s -X POST "https://api.example.com/endpoint" \
  -H "Content-Type: application/json" \
  -d '{"key": "value"}' | jq .

# Paginated — check first page + next_page field
curl -s "https://api.example.com/items?page=1" | jq '{ count: (.items | length), next: .next_page_url, first_item: .items[0] }'
```

### S3
```bash
# List files in prefix
aws s3 ls s3://bucket/prefix/ --region us-east-1

# Preview a file (first 5 lines)
aws s3 cp s3://bucket/prefix/file.json - --region us-east-1 | head -5

# List all files matching a pattern
aws s3 ls s3://bucket/prefix/ --region us-east-1 | grep ".json"

# Check file size before downloading
aws s3 ls s3://bucket/prefix/file.json --region us-east-1 --human-readable
```

### SQS
```bash
# Peek at messages without consuming (VisibilityTimeout=0 returns them immediately)
aws sqs receive-message \
  --queue-url "https://sqs.us-east-1.amazonaws.com/123456789/my-queue" \
  --max-number-of-messages 1 \
  --visibility-timeout 0 \
  --region us-east-1 | jq '.Messages[0].Body | fromjson'

# Check queue depth
aws sqs get-queue-attributes \
  --queue-url "https://sqs.us-east-1.amazonaws.com/123456789/my-queue" \
  --attribute-names ApproximateNumberOfMessages \
  --region us-east-1
```

### Kafka
```bash
# Consume a few messages and exit (requires kafka-console-consumer or kcat)
# Using kcat (recommended):
kcat -b kafka.host:9092 -t my-topic -C -c 5 -e \
  -X security.protocol=SASL_SSL \
  -X sasl.mechanisms=SCRAM-SHA-512 \
  -X sasl.username=$KAFKA_USER \
  -X sasl.password=$KAFKA_PASS

# Using kafka-console-consumer:
kafka-console-consumer.sh \
  --bootstrap-server kafka.host:9092 \
  --topic my-topic \
  --max-messages 5 \
  --from-beginning

# OR use a minimal caterpillar probe pipeline (see Step 2)
```

### Local File
```bash
# Preview content
head -5 data/input.txt
head -5 data/input.json | jq .

# Count records
wc -l data/input.txt

# Check encoding / format
file data/input.csv
```

### AWS Parameter Store
```bash
# Read a single parameter
aws ssm get-parameter --name "/prod/kafka/password" --with-decryption --region us-east-1 | jq .

# List parameters under a path
aws ssm get-parameters-by-path --path "/prod/kafka/" --recursive --region us-east-1 | jq '.Parameters[] | { name: .Name, value: .Value }'
```

---

## Step 2 — Capture Sample Data to a Local File

Once you can see real data from the source, capture a small sample to a local file. This becomes the input for all your transform probe pipelines — no live connections needed.

### Capture via caterpillar probe pipeline

Create `test/pipelines/probe_capture_<name>.yaml`:

```yaml
# CAPTURE PROBE — run once to save sample data locally
# Replace source task with your real source config
tasks:
  - name: source
    type: <your_source_type>
    # ... your source config ...

  - name: take_sample
    type: sample
    filter: head
    limit: 10              # capture first 10 records

  - name: save_sample
    type: file
    path: test/pipelines/samples/<pipeline_name>_sample.json
```

Run it:
```bash
./caterpillar -conf test/pipelines/probe_capture_<name>.yaml
```

Now you have `test/pipelines/samples/<pipeline_name>_sample.json` — a local file with real data shaped exactly as the source produces it. Use this for all transform testing.

### Capture via CLI (HTTP / S3)

```bash
# HTTP
curl -s "https://api.example.com/items" > test/pipelines/samples/api_sample.json

# S3
aws s3 cp s3://bucket/prefix/file.json test/pipelines/samples/s3_sample.json --region us-east-1

# SQS (single message body)
aws sqs receive-message \
  --queue-url "..." --max-number-of-messages 1 --visibility-timeout 0 \
  | jq -r '.Messages[0].Body' > test/pipelines/samples/sqs_sample.json
```

---

## Step 3 — Build a Probe Pipeline Per Transform Stage

For each transform task in the pipeline, build an isolated probe pipeline:
- **Source**: local file from Step 2
- **Single transform**: the task under test
- **Sink**: `echo` with `only_data: true`

### Probe template

```yaml
# PROBE: testing <transform_name>
tasks:
  - name: load_sample
    type: file
    path: test/pipelines/samples/<pipeline_name>_sample.json

  - name: <transform_name>
    type: <transform_type>
    # ... transform config ...

  - name: inspect_output
    type: echo
    only_data: true
```

Run it:
```bash
./caterpillar -conf test/pipelines/probe_<transform_name>.yaml
```

### Per-transform verification checklist

**`jq` transform**
- Does the output have the expected fields?
- If `explode: true`, does each element of the array become a separate record?
- Are `{{ context "key" }}` substitutions rendering correctly or as literal strings?

**`split` transform**
- Is each line becoming a separate record?
- Are there empty records from trailing newlines? Add `jq` filter: `select(. != "")`

**`join` transform**
- Are records being batched at the right size?
- Is the delimiter correct in the joined output?
- Does the last partial batch flush? (Add `timeout` if needed)

**`replace` transform**
- Does the regex match the intended data?
- Test the regex independently: `echo "your data" | sed 's/pattern/replacement/'`

**`converter` transform
- Is the input format what converter expects? (CSV with headers, EML with MIME structure, etc.)
- Does the output JSON have the expected field names?

**`xpath` transform**
- Test the XPath expression independently: `echo "<xml>" | xmllint --xpath "//field" -`
- Is the correct element selected when there are multiple matches?

**`flatten` transform**
- Are nested keys joined with `_` as expected?
- Are arrays flattened or preserved?

---

## Step 4 — Chain Transforms Incrementally

After each transform probe passes, build a chained probe that combines transforms tested so far:

```yaml
# CHAIN PROBE: source → transform_1 → transform_2 (adding transform_2)
tasks:
  - name: load_sample
    type: file
    path: test/pipelines/samples/<pipeline_name>_sample.json

  - name: transform_1         # already verified
    type: jq
    path: .items[]
    explode: true

  - name: transform_2         # now being added
    type: replace
    expression: ^(.*)$
    replacement: '{"wrapped": "$1"}'

  - name: inspect_output
    type: echo
    only_data: true
```

**Rule**: only add one new transform per iteration. If output breaks, you know exactly which task caused it.

---

## Step 5 — Verify the Sink

Before connecting the real sink (S3, SQS, Kafka), run a final probe with a local file sink to inspect the exact records that would be written:

```yaml
# SINK VERIFICATION PROBE
tasks:
  - name: load_sample
    type: file
    path: test/pipelines/samples/<pipeline_name>_sample.json

  # ... all transforms (already verified) ...

  - name: write_to_local_for_inspection
    type: file
    path: test/pipelines/samples/<pipeline_name>_output.json
```

Then inspect:
```bash
cat test/pipelines/samples/<pipeline_name>_output.json | jq .
wc -l test/pipelines/samples/<pipeline_name>_output.json   # record count
```

Confirm:
- Record count matches expectations
- Field names and types match what the sink expects
- No empty records or malformed JSON

---

## Step 6 — Smoke Test Against Real Sink (Dry Run)

When the local sink verification passes, do a limited smoke test against the real sink:

```yaml
# SMOKE TEST — real sink, limited records
tasks:
  - name: source
    type: <real_source>
    # ... config ...

  - name: take_sample           # limit to 1-3 records for smoke test
    type: sample
    filter: head
    limit: 3

  # ... transforms ...

  - name: real_sink
    type: <real_sink_type>
    # ... sink config ...
    fail_on_error: true
```

Then verify at the sink:
```bash
# S3 — did the file appear?
aws s3 ls s3://bucket/output/ --region us-east-1 | tail -3

# SQS — did messages arrive?
aws sqs get-queue-attributes \
  --queue-url "..." \
  --attribute-names ApproximateNumberOfMessages

# Kafka — did messages arrive? (kcat)
kcat -b kafka.host:9092 -t output-topic -C -c 3 -e

# HTTP — did the POST succeed? (check target system or logs)
```

---

## Output: Full Test Plan

When given a pipeline YAML, output a complete test plan with:

1. **Source inspection command** — exact CLI command for the source type
2. **Sample capture pipeline** — ready-to-run YAML saved to `test/pipelines/probe_capture_<name>.yaml`
3. **Per-transform probe pipelines** — one YAML per transform, saved to `test/pipelines/probe_<transform_name>.yaml`
4. **Sink verification probe** — local file sink YAML
5. **Smoke test pipeline** — real sink with `sample: head limit: 3`
6. **Sink verification commands** — CLI commands to confirm records arrived at the real sink

Format each pipeline as a fenced ```yaml block with its filename as a comment header.
Label each step clearly so the engineer can work through them in order.
