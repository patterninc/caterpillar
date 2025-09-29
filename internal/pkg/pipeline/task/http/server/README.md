# HTTP Server Task

The `http_server` task starts an HTTP server to receive data from external sources, enabling inbound data ingestion and webhook processing.

## Function

The HTTP server task creates a web server that listens for incoming HTTP requests and processes the request data through the pipeline. It's useful for receiving webhooks, API calls, or any HTTP-based data ingestion.

## Behavior

The HTTP server task starts a web server that listens for incoming HTTP requests. It operates as a data source (no input channel required), accepts HTTP requests on the configured port, and sends each request as a record to its output channel. The record contains the request body as data and request metadata (headers, method, URL) as context. The server can be configured to stop after a specified number of requests.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `http_server` | Must be "http_server" |
| `port` | int | `8080` | Port number to listen on |
| `end_after` | int | - | Stop server after specified number of requests |
| `auth` | object | - | Authentication configuration (see Auth section) |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## Authentication Configuration

The server supports API key authentication:

```yaml
auth:
  behavior: api-key
  headers:
    Authorization: your-api-key-here
```

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

### Limited request server:
```yaml
tasks:
  - name: test_server
    type: http_server
    port: 8080
    end_after: 100
```

## Sample Pipelines

- `test/pipelines/http_server.yaml` - Basic HTTP server example
- `test/pipelines/http_server_rest.yaml` - HTTP server with REST API interaction

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
- GET, POST, PUT, DELETE, PATCH
- Request body is included in the record data
- Headers are preserved in the record context

### Response Handling:
- Returns appropriate HTTP status codes
- Can be configured to return custom responses
- Handles errors gracefully

## Security Considerations

- **Authentication**: Use API key authentication for production deployments
- **HTTPS**: Consider using HTTPS for secure data transmission
- **Input validation**: Validate incoming request data
- **Rate limiting**: Consider implementing rate limiting for high-traffic scenarios
- **Network security**: Configure firewall rules appropriately