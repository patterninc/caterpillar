---
skill: heimdall
version: 1.0.0
caterpillar_type: heimdall
description: Submit jobs to the Heimdall data orchestration platform and receive results downstream.
role: source | transform
requires_upstream: false   # source mode: no upstream
requires_downstream: true  # always emits job results downstream
aws_required: false
---

## Purpose

Two modes:
- **Source** (no upstream): submits one static job → emits job results to pipeline
- **Destination** (has upstream): for each record, parses its JSON data as job context → submits a job → emits results

Results from the job execution flow to the next task. Supports sync and async (polled) jobs.

## Schema

```yaml
- name: <string>                     # REQUIRED
  type: heimdall                     # REQUIRED
  endpoint: <string>                 # OPTIONAL — Heimdall API URL (default: http://localhost:9090)
  headers: <map[string]string>       # OPTIONAL — API auth headers
  poll_interval: <int>               # OPTIONAL — polling interval in seconds (default: 5)
  timeout: <int>                     # OPTIONAL — job timeout in seconds (default: 300)
  job: <object>                      # REQUIRED — job specification
  fail_on_error: <bool>              # OPTIONAL (default: false)
```

### Job spec schema
```yaml
job:
  name: <string>                     # OPTIONAL — job name (default: caterpillar)
  version: <string>                  # OPTIONAL — job version (default: 0.0.1)
  context: <map[string]any>          # OPTIONAL — static key-value context for the job
  command_criteria: [<string>, ...]  # OPTIONAL — criteria to select the command
  cluster_criteria: [<string>, ...]  # OPTIONAL — criteria to select the cluster
  tags: [<string>, ...]              # OPTIONAL — job tags
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| One static job, results to pipeline | source mode (no upstream) |
| One job per incoming record | destination mode (add upstream, add `jq` to format context) |
| Long-running job (>300s) | increase `timeout` to expected duration |
| Frequent polling needed | decrease `poll_interval` |
| Heimdall requires auth | add `headers` with token |
| Job context is dynamic per record | add `jq` task before heimdall to build context object |
| Spark job | `command_criteria: ["type:spark"]` |
| Shell job | `command_criteria: ["type:shell"]` |
| Auth token must be secure | use `{{ env "HEIMDALL_TOKEN" }}` in headers |

## Validation Rules

- `job` is required
- In destination mode, record data must be valid JSON — add `jq` upstream to format it as the context object
- `timeout` must be long enough for the job type — default 300s may be too short for Spark/EMR jobs
- `poll_interval` must be less than `timeout` — otherwise the first poll attempt may already exceed timeout
- Heimdall endpoint must be reachable from the pipeline host
- Auth tokens must use `{{ env "VAR" }}` or `{{ secret "/path" }}`

## Examples

### Source: submit one static job
```yaml
- name: run_job
  type: heimdall
  endpoint: http://heimdall.example.com
  timeout: 3600
  poll_interval: 15
  job:
    name: daily-etl
    version: 1.0.0
    command_criteria: ["type:spark"]
    cluster_criteria: ["type:emr-on-eks"]
    context:
      query: "SELECT * FROM events WHERE dt = '2024-03-01'"
      output: "s3://bucket/output/"
```

### Source: ping test job
```yaml
- name: run_ping
  type: heimdall
  endpoint: http://localhost:9090
  job:
    name: ping-test
    command_criteria: ["type:ping"]
    cluster_criteria: ["type:localhost"]
```

### Destination: per-record job submission
```yaml
- name: build_context
  type: jq
  path: |
    {
      "table": .source_table,
      "filter_id": (.record_id | tostring),
      "output_path": "s3://{{ env "OUTPUT_BUCKET" }}/" + .record_id
    }

- name: submit_job
  type: heimdall
  endpoint: http://heimdall.example.com
  timeout: 600
  poll_interval: 10
  job:
    name: record-processor
    command_criteria: ["type:spark"]
    cluster_criteria: ["data:prod"]

- name: show_results
  type: echo
  only_data: true
```

### With API auth header
```yaml
- name: secure_job
  type: heimdall
  endpoint: https://heimdall.prod.example.com
  headers:
    X-Heimdall-Token: "{{ env "HEIMDALL_TOKEN" }}"
    X-Heimdall-User: caterpillar
  timeout: 1800
  poll_interval: 30
  job:
    name: analytics-job
    command_criteria: ["type:trino"]
    cluster_criteria: ["type:prod"]
    context:
      query: "SELECT count(*) FROM events"
```

## Anti-patterns

- Destination mode without a `jq` task before heimdall — record data must be a valid JSON context object
- `timeout` too short for long-running jobs — Spark/EMR jobs may take minutes to hours
- Hardcoded auth tokens in `headers` — use `{{ env "VAR" }}`
- `fail_on_error: false` for critical jobs — silent failures mean the pipeline continues with no results
