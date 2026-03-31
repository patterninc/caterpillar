---
skill: http
version: 1.0.0
caterpillar_type: http
description: Make HTTP requests to external APIs — fetch data (source) or post pipeline records (sink).
role: source | sink
requires_upstream: false   # source mode: no upstream
requires_downstream: true  # always emits response records downstream
aws_required: false
---

## Purpose

Dual-mode HTTP client task:
- **Source mode** (no upstream): sends requests using static YAML config; supports pagination
- **Sink mode** (has upstream): each record's JSON data is merged with the base config to form the request

Response body is passed downstream. Response headers are automatically stored in context as `http-header-<Name>`.

## Schema

```yaml
- name: <string>                    # REQUIRED
  type: http                        # REQUIRED
  endpoint: <string>                # REQUIRED — target URL
  method: <string>                  # OPTIONAL — HTTP verb (default: GET)
  headers: <map[string]string>      # OPTIONAL — request headers
  body: <string>                    # OPTIONAL — request body (POST/PUT)
  timeout: <int>                    # OPTIONAL — seconds (default: 90)
  max_retries: <int>                # OPTIONAL — retry attempts (default: 3)
  retry_delay: <int>                # OPTIONAL — seconds between retries (default: 5)
  expected_statuses: <string>       # OPTIONAL — comma-separated codes (default: "200")
  next_page: <string|object>        # OPTIONAL — JQ expr for next page URL, or pagination object
  context: <map[string]string>      # OPTIONAL — JQ exprs to extract response values into context
  oauth: <object>                   # OPTIONAL — OAuth 1.0 or 2.0 config
  proxy: <object>                   # OPTIONAL — proxy config
  fail_on_error: <bool>             # OPTIONAL (default: false)
```

### OAuth 1.0 schema
```yaml
oauth:
  consumer_key: <string>
  consumer_secret: <string>
  token: <string>
  token_secret: <string>
  version: "1.0"
  signature_method: "HMAC-SHA256"
```

### OAuth 2.0 schema (client credentials)
```yaml
oauth:
  token_uri: <string>
  grant_type: "client_credentials"
  scope: [<string>, ...]
```

### Pagination object schema
```yaml
next_page:
  endpoint: <string>   # JQ expr for next URL
  method: <string>
  body: <string>
  headers: <map>
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Fetching from an API with no incoming data | source mode (no upstream task) |
| Posting each pipeline record to an API | sink mode (add upstream task) |
| API requires Bearer token | add `Authorization: Bearer {{ env "TOKEN" }}` to `headers` |
| API requires OAuth 1.0 | add `oauth` block with `version: "1.0"` |
| API requires OAuth 2.0 | add `oauth` block with `token_uri` and `grant_type` |
| API is paginated | add `next_page` with JQ expression extracting next URL |
| Need downstream access to a response field | add `context` block with JQ expressions |
| Need downstream access to response header | use `{{ context "http-header-<Name>" }}` — auto-populated |
| Endpoint URL contains record-specific data | use `{{ context "key" }}` in endpoint string |
| Non-200 success codes expected | set `expected_statuses: "200,201,202"` |
| Credentials must be secure | use `{{ env "VAR" }}` or `{{ secret "/ssm/path" }}` |

## Response headers in context

All response headers are automatically available downstream:
```
{{ context "http-header-Content-Type" }}
{{ context "http-header-X-Request-Id" }}
```
Header names use Go canonical form (e.g. `content-type` → `Content-Type`).

## Validation Rules

- `endpoint` is required
- `expected_statuses` is a **string**, not an array: `"200,201"` not `["200","201"]`
- Secrets/tokens must never be hardcoded — always `{{ env "VAR" }}` or `{{ secret "/path" }}`
- In sink mode, record data must be valid JSON — add a `jq` task upstream if needed
- `next_page` as a string is a JQ expression applied to the response body
- `batch_flush_interval` not applicable here — see `kafka` skill

## Examples

### GET request (source)
```yaml
- name: fetch_users
  type: http
  method: GET
  endpoint: https://api.example.com/users
  headers:
    Accept: application/json
    Authorization: Bearer {{ env "API_TOKEN" }}
  fail_on_error: true
```

### POST each record (sink)
```yaml
- name: post_to_api
  type: http
  method: POST
  endpoint: https://ingest.example.com/events
  headers:
    Content-Type: application/json
  max_retries: 5
  retry_delay: 2
  expected_statuses: "200,201"
```

### Paginated GET
```yaml
- name: fetch_all_pages
  type: http
  method: GET
  endpoint: https://api.example.com/items?page=1
  next_page: ".links.next"
```

### Extract context from response
```yaml
- name: get_auth_token
  type: http
  method: POST
  endpoint: https://auth.example.com/token
  body: '{"grant_type":"client_credentials"}'
  headers:
    Content-Type: application/json
  context:
    access_token: ".data | fromjson | .access_token"
    expires_in: ".data | fromjson | .expires_in | tostring"
```

### Dynamic endpoint from context
```yaml
- name: fetch_user_detail
  type: http
  method: GET
  endpoint: https://api.example.com/users/{{ context "user_id" }}
  headers:
    Authorization: Bearer {{ context "access_token" }}
```

### OAuth 2.0
```yaml
- name: call_google_api
  type: http
  method: GET
  endpoint: https://www.googleapis.com/some/resource
  oauth:
    token_uri: https://oauth2.googleapis.com/token
    grant_type: client_credentials
    scope:
      - https://www.googleapis.com/auth/cloud-platform
```

## Anti-patterns

- Hardcoded tokens/passwords in headers → use `{{ env "VAR" }}`
- `expected_statuses` as array `["200"]` → must be string `"200"`
- Omitting `fail_on_error: true` on critical source tasks
- Sink mode without a `jq` task upstream when data is not already a valid HTTP request JSON object
