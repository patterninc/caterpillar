# HTTP Server Task

The `http_server` task starts an HTTP server to receive data from external sources, enabling inbound data ingestion and webhook processing.

## Function

The HTTP server task creates a web server that listens for incoming HTTP requests and processes the request data through the pipeline. It's useful for receiving webhooks, API calls, or any HTTP-based data ingestion.

## Behavior

The HTTP server task starts a web server that listens for incoming HTTP requests. It operates as a data source (no input channel required), accepts HTTP requests on the configured port, and sends each request as a record to its output channel.

Each record's **data** is a JSON object carrying the request's `method`, `path`, `query`, `body`
and `headers` — the metadata travels in the payload, not in the record context. Reach a field
with a downstream `jq` task (e.g. `path: .body`).

The server serves only the method/path pairs listed in `paths`, defaulting to a single
`GET /`. A request whose method does not match the configured method for that path is
rejected with `405 Method not allowed`.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `http_server` | Must be "http_server" |
| `port` | int | `8080` | Port number to listen on |
| `paths` | list | `GET /` | Method/path pairs to serve; each item takes `method` and `path` |
| `end_after` | duration | - | Shut the server down after this much time, e.g. `10s` |
| `read_timeout` | duration | `15s` | Request read timeout |
| `write_timeout` | duration | `15s` | Response write timeout |
| `idle_timeout` | duration | `5s` | Keep-alive idle timeout |
| `auth` | object | - | Authentication configuration (see Auth section) |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

Duration fields take a string with a unit (`10s`, `5m`); a bare number fails to parse.
Without `end_after` the server runs until the process is stopped.
`task_concurrency` is accepted but forced to 1 — only one server instance runs.

## Authentication Configuration

Set `auth.behavior` to one of `api-key`, `basic-auth`, or `ip-whitelist`. Each behavior reads
its own fields; an unrecognized value fails the task.

```yaml
# api-key: every header listed must be present on the request with a matching value
auth:
  behavior: api-key
  headers:
    Authorization: your-api-key-here

# basic-auth: HTTP Basic credentials
auth:
  behavior: basic-auth
  username: pipeline
  password: {{ env "SERVER_PASSWORD" }}

# ip-whitelist: only these source addresses are served
auth:
  behavior: ip-whitelist
  whitelist_ips:
    - 10.0.0.1
    - 10.0.0.2
```

`ip-whitelist` matches the first address in `X-Forwarded-For` when that header is present,
falling back to the peer address — so it trusts the header, and is only meaningful behind a
proxy that overwrites it.

## Example Configurations

### Basic HTTP server:
```yaml
tasks:
  - name: webhook_receiver
    type: http_server
    port: 8080
```

### Server with authentication:
```yaml
tasks:
  - name: secure_server
    type: http_server
    port: 8443
    auth:
      behavior: api-key
      headers:
        Authorization: Bearer {{ env "API_KEY" }}
```

### Server that shuts itself down:
```yaml
tasks:
  - name: test_server
    type: http_server
    port: 8080
    end_after: 30s
```

### Receiving a POST webhook on a custom path:
```yaml
tasks:
  - name: webhook_receiver
    type: http_server
    port: 8080
    paths:
      - method: POST
        path: /events
```

## Sample Pipelines

- `test/pipelines/http_server.yaml` - Basic HTTP server example
- `test/pipelines/http_server_rest.yaml` - HTTP server with REST API interaction
- `test/pipelines/basic_auth_test.yaml` - HTTP server with basic authentication

## Use Cases

- **Webhook processing**: Receive webhooks from external services
- **API endpoints**: Create custom API endpoints for data ingestion
- **Data collection**: Collect data from web forms or applications
- **Testing**: Create test endpoints for pipeline validation
- **Integration**: Enable HTTP-based integration with external systems
- **Real-time data**: Receive real-time data streams via HTTP

## Server Behavior

### Request Processing:
- Accepts HTTP requests on the configured port
- Processes request body and headers
- Creates records for each incoming request
- Sends records to the output channel for further processing
- Returns HTTP response to the client

### Supported HTTP Methods:
- Any method may be configured, but each `paths` entry serves exactly one; anything else gets `405`
- Request body, method, path, query and headers are all included in the record data

### Response Handling:
- A served request returns `{"ok":true}`
- A rejected request returns `401` with `{"ok":false, "error":"access denied"}`
- An unreadable body returns `400`

## Security Considerations

- **Authentication**: Configure `auth` for production deployments
- **HTTPS**: Consider using HTTPS for secure data transmission
- **Input validation**: Validate incoming request data
- **Rate limiting**: Consider implementing rate limiting for high-traffic scenarios
- **Network security**: Configure firewall rules appropriately