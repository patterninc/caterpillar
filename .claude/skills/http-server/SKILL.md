---
skill: http-server
version: 1.0.0
caterpillar_type: http_server
description: Start an HTTP server to receive inbound requests (webhooks, API push) as a pipeline data source.
role: source
requires_upstream: false
requires_downstream: true
aws_required: false
---

## Purpose

Starts an embedded HTTP server. Each incoming request becomes one pipeline record:
- **Record data**: request body
- **Record context**: request headers as `http-header-<Name>`

Runs until `end_after` requests are received, or indefinitely if `end_after` is omitted.

## Schema

```yaml
- name: <string>             # REQUIRED
  type: http_server          # REQUIRED
  port: <int>                # OPTIONAL — listening port (default: 8080)
  end_after: <int>           # OPTIONAL — stop after N requests (omit for indefinite)
  auth: <object>             # OPTIONAL — API key auth config
  fail_on_error: <bool>      # OPTIONAL (default: false)
```

### Auth schema
```yaml
auth:
  behavior: api-key
  headers:
    <header-name>: <expected-value>
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Production deployment | add `auth` block with API key |
| Testing / one-shot intake | set `end_after: <N>` |
| Long-running webhook listener | omit `end_after` |
| Access request headers downstream | use `{{ context "http-header-<Name>" }}` |
| HTTPS required | use a reverse proxy (nginx, ALB) in front |
| Auth token must be configurable | use `{{ env "WEBHOOK_SECRET" }}` in auth header value |

## Validation Rules

- `http_server` must always be the **first task** (source only — no upstream)
- `end_after` omitted = runs indefinitely; confirm this is intentional for production
- Port must be available and not blocked by firewall
- For HTTPS, the task serves plain HTTP — put a TLS-terminating proxy in front
- Auth header value should use `{{ env "VAR" }}` — never hardcoded

## Context auto-populated per request

```
{{ context "http-header-Content-Type" }}
{{ context "http-header-Authorization" }}
{{ context "http-header-X-Request-Id" }}
```

## Examples

### Basic webhook receiver
```yaml
- name: webhook_intake
  type: http_server
  port: 8080
  fail_on_error: true
```

### Authenticated server
```yaml
- name: secure_webhook
  type: http_server
  port: 8080
  auth:
    behavior: api-key
    headers:
      Authorization: Bearer {{ env "WEBHOOK_SECRET" }}
```

### Test server (stop after 5 requests)
```yaml
- name: test_receiver
  type: http_server
  port: 9090
  end_after: 5
```

### Access request metadata downstream
```yaml
# Task following http_server:
- name: tag_request
  type: jq
  path: |
    {
      "payload": .,
      "source_ip": "{{ context "http-header-X-Forwarded-For" }}",
      "content_type": "{{ context "http-header-Content-Type" }}"
    }
```

## Anti-patterns

- Using `http_server` anywhere other than position 1 in the task list
- Omitting `auth` in production deployments
- Hardcoding the API key value — use `{{ env "VAR" }}`
- Expecting HTTPS without a TLS proxy in front
- Omitting `end_after` in tests — the pipeline will run forever
