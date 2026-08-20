---
skill: aws-parameter-store
version: 1.0.0
caterpillar_type: aws_parameter_store
description: Read parameters from or write parameters to AWS SSM Parameter Store as pipeline data.
role: source | sink
requires_upstream: false   # read mode
requires_downstream: false # write mode
aws_required: true
---

## Purpose

Dual-mode SSM task:
- **Read mode** (no upstream + `get`): retrieves parameters → emits records with parameter values
- **Write mode** (has upstream + `set`): extracts values from each record using JQ → writes to SSM

Distinct from `{{ secret "/path" }}` template function, which injects a parameter value into task config at pipeline init time. This task treats SSM parameters as **data** that flows through the pipeline.

## Schema

```yaml
- name: <string>                        # REQUIRED
  type: aws_parameter_store             # REQUIRED
  get: <map[string]string>              # CONDITIONAL — read mode: output_key → /ssm/path
  set: <map[string]string>              # CONDITIONAL — write mode: /ssm/path → JQ expression
  secure: <bool>                        # OPTIONAL — store as SecureString (default: true)
  overwrite: <bool>                     # OPTIONAL — overwrite existing params (default: true)
  fail_on_error: <bool>                 # OPTIONAL (default: false)
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Load config values into pipeline | read mode: use `get` |
| Write pipeline results to SSM | write mode: use `set` |
| Store sensitive values | `secure: true` (default) |
| Store non-sensitive config | `secure: false` |
| Don't overwrite if exists | `overwrite: false` |
| SSM paths are environment-specific | use `{{ env "ENV" }}` in path values |
| Values from record fields | `set` values are JQ expressions: `".field_name"` |
| Static config injection into task config | use `{{ secret "/path" }}` template instead |

## Mode Detection

- No upstream task + `get` defined → **Read mode** (source)
- Has upstream task + `set` defined → **Write mode** (sink)

## Key Distinction: `aws_parameter_store` task vs `{{ secret }}` template

| Mechanism | When | Use case |
|-----------|------|---------|
| `{{ secret "/path" }}` | Pipeline init (once) | Inject API keys/tokens into task config fields |
| `aws_parameter_store` task | Runtime per record | SSM params are the pipeline's input or output data |

## Validation Rules

- `get` or `set` must be present — cannot be empty
- `set` values are **JQ expressions** (e.g. `".access_token"`, `".expires | tostring"`) — not literal values
- SSM parameter paths must start with `/`
- `secure: true` requires KMS permissions — warn if KMS may not be available
- `overwrite: false` silently skips existing parameters — confirm this is intended behavior
- Write mode data must be valid JSON — add `jq` upstream to ensure correct format

## IAM Permissions

```
# Read mode
ssm:GetParameter
ssm:GetParameters
ssm:GetParametersByPath

# Write mode
ssm:PutParameter

# Encrypted parameters (read)
kms:Decrypt

# Encrypted parameters (write)
kms:GenerateDataKey
```

## Examples

### Read parameters (source)
```yaml
- name: load_config
  type: aws_parameter_store
  get:
    api_key:      "/prod/api/key"
    db_url:       "/prod/database/url"
    tenant_id:    "/prod/app/tenant"
  fail_on_error: true
```

### Read with env-driven paths
```yaml
- name: load_env_config
  type: aws_parameter_store
  get:
    endpoint: "{{ env "SSM_ENDPOINT_PATH" }}"
    token:    "{{ env "SSM_TOKEN_PATH" }}"
```

### Write record fields to SSM
```yaml
- name: store_tokens
  type: aws_parameter_store
  set:
    "/prod/auth/access_token":  ".access_token"
    "/prod/auth/refresh_token": ".refresh_token"
    "/prod/auth/expires_at":    ".expires_in | tostring"
  secure: true
  overwrite: true
```

### Full pattern: fetch OAuth token → store in SSM
```yaml
tasks:
  - name: fetch_token
    type: http
    method: POST
    endpoint: https://auth.example.com/oauth/token
    body: '{"grant_type":"client_credentials","client_id":"{{ env "CLIENT_ID" }}"}'
    headers:
      Content-Type: application/json
    fail_on_error: true

  - name: parse_token
    type: jq
    path: |
      {
        "access_token": (.data | fromjson | .access_token),
        "expires_in":   (.data | fromjson | .expires_in)
      }

  - name: store_token
    type: aws_parameter_store
    set:
      "/prod/oauth/access_token": ".access_token"
      "/prod/oauth/expires_at":   ".expires_in | tostring"
    secure: true
    overwrite: true
```

### Write with non-secure params (config, not secrets)
```yaml
- name: store_config
  type: aws_parameter_store
  set:
    "/prod/app/last_run_ts":    '"{{ macro "timestamp" }}"'
    "/prod/app/processed_count": ".count | tostring"
  secure: false
  overwrite: true
```

## Anti-patterns

- Using `set` with literal string values instead of JQ expressions — `set` values are always JQ
- SSM parameter paths missing the leading `/` → SSM API error
- `secure: true` without verifying KMS permissions — write will fail silently without `fail_on_error: true`
- `overwrite: false` when the intent is to always update — params silently skipped on subsequent runs
- Using this task when a `{{ secret "/path" }}` template would be simpler (static injection at pipeline init)
