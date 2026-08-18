# AWS Parameter Store Task

The `aws_parameter_store` task writes to or reads from AWS Systems Manager Parameter Store.

## Function

The task operates in two modes, selected by which field is configured:

- **Write mode**: Receives records from the input channel and sets parameters in Parameter Store based on the data. Requires `set`.
- **Lookup mode**: Fetches parameters per record and writes the decrypted values into record context. Requires `lookup`.

`set` and `lookup` are mutually exclusive. The task always requires an input channel.

| Mode | Field | Behaviour |
|------|-------|-----------|
| Write | `set` | Sets the `set` parameters from each record, then forwards that record downstream unchanged |
| Lookup | `lookup` | Fetches parameters whose paths may vary per record, writes values to record context, then forwards the record |

Startup-time reads use the `{{ secret "/ssm/path" }}` template in any task field. `lookup` is the per-record path.

## Input Channel

Accepts `*record.Record` objects. Lookup paths may reference record context via `{{ context "..." }}`.

## Output Channel

In write mode, each input record is forwarded downstream once, regardless of how many parameters `set` writes.

In lookup mode, each input record is forwarded after its context is populated. `SecureString` values are decrypted.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `aws_parameter_store` | Must be "aws_parameter_store" |
| `set` | map[string]string | - | Parameters to set, keyed by parameter name, valued by a JQ expression over the record |
| `lookup` | map[string]string | - | Parameters to retrieve per record, keyed by context field name, valued by parameter path |
| `cache_ttl` | duration | `5m` | How long successful lookups are cached. Omitted or `0` uses `5m`. |
| `on_missing` | string | `error` | Lookup-mode behaviour when a parameter does not exist: `error` stops the task; `skip` logs and drops the record |
| `secure` | bool | `true` | Whether to store parameters as SecureString (write mode only) |
| `overwrite` | bool | `true` | Whether to overwrite existing parameters (write mode only) |
| `context` | map | - | JQ expressions whose results are stored on each record for downstream tasks |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

Only successful lookups are cached. Missing parameters are never cached and always follow `on_missing`.

`task_concurrency` is accepted but forced to 1, with a warning — a single SSM client runs.

## Example Configurations

### Write mode - Set parameters from pipeline data:
```yaml
tasks:
  - name: source
    type: file
    path: credentials.json
  - name: set_config
    type: aws_parameter_store
    set:
      "/my-app/api_key": ".api_key"
      "/my-app/endpoint": ".endpoint"
    secure: true
    overwrite: true
```

### Lookup mode - Fetch per-record credentials into context:
```yaml
tasks:
  - name: extract_slug
    type: jq
    path: .
    context:
      slug: .tenant_id
  - name: fetch_credentials
    type: aws_parameter_store
    lookup:
      api_token: /prod/tenants/{{ context "slug" }}/api_token
      api_secret: /prod/tenants/{{ context "slug" }}/api_secret
    cache_ttl: 5m
    on_missing: skip
  - name: call_api
    type: http
    endpoint: https://api.example.com/data
    headers:
      Authorization: Bearer {{ context "api_token" }}
```

### Startup-time read (config template, not this task):
```yaml
tasks:
  - name: call_api
    type: http
    endpoint: https://api.example.com/data
    headers:
      Authorization: Bearer {{ secret "/prod/api/token" }}
```

## Use Cases

- **Configuration management**: Store and retrieve application configuration
- **Secret management**: Securely store and retrieve sensitive data
- **Dynamic configuration**: Update parameters based on pipeline data
- **Multi-tenant credentials**: Resolve per-tenant secrets from SSM at runtime
- **Environment setup**: Configure different environments with different parameters
- **Data persistence**: Store pipeline results for later use
- **Cross-pipeline sharing**: Hand values to a later pipeline run, which reads them with `{{ secret }}`
