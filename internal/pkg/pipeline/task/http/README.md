# HTTP Task

The `http` task makes HTTP requests to external APIs and services, with support for OAuth authentication, retry logic, and response processing.

## Function

The HTTP task sends HTTP requests to specified endpoints and processes the responses. It can handle various HTTP methods, authentication schemes, and automatically retry failed requests.

## Behavior

The HTTP task operates in two modes depending on whether an input channel is provided:

- **With input channel**: Receives JSON-formatted HTTP request configurations from the input channel. Each record's data should contain a JSON object with HTTP request parameters (method, endpoint, headers, body, etc.). The task merges these with the base configuration from YAML.

- **Without input channel**: Uses the endpoint and configuration specified in the YAML configuration to make HTTP requests. This mode supports pagination and can make multiple requests automatically.

In both modes, the task sends HTTP response data to its output channel and supports automatic retries, OAuth authentication, and proxy configuration.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `http` | Must be "http" |
| `method` | string | `GET` | HTTP method (GET, POST, PUT, DELETE, etc.) |
| `endpoint` | string | - | Target URL for the request |
| `headers` | map[string]string | - | HTTP headers to include |
| `body` | string | - | Request body for POST/PUT requests |
| `timeout` | int | `90` | Request timeout in seconds |
| `max_retries` | int | `3` | Maximum number of retry attempts |
| `retry_delay` | int | `5` | Delay between retries in seconds |
| `expected_statuses` | string | `200` | Comma-separated list of expected HTTP status codes |
| `oauth` | object | - | OAuth configuration (see OAuth section) |
| `proxy` | object | - | Proxy configuration |
| `next_page` | string | - | JQ expression to extract next page URL |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## OAuth Configuration

The task supports both OAuth 1.0 and OAuth 2.0:

### OAuth 1.0
```yaml
oauth:
  consumer_key: "your_consumer_key"
  consumer_secret: "your_consumer_secret"
  token: "your_token"
  token_secret: "your_token_secret"
  version: "1.0"
  signature_method: "HMAC-SHA256"
```

### OAuth 2.0
```yaml
oauth:
  token_uri: "https://oauth2.googleapis.com/token"
  grant_type: "client_credentials"
  scope: ["https://www.googleapis.com/auth/cloud-platform"]
```

## Example Configurations

### Simple GET request:
```yaml
tasks:
  - name: fetch_data
    type: http
    method: GET
    endpoint: https://api.example.com/data
    headers:
      Accept: application/json
```

### POST request with OAuth:
```yaml
tasks:
  - name: create_resource
    type: http
    method: POST
    endpoint: https://api.example.com/resources
    headers:
      Content-Type: application/json
    body: '{"name": "test", "value": 123}'
    oauth:
      consumer_key: "{{ env 'OAUTH_KEY' }}"
      consumer_secret: "{{ env 'OAUTH_SECRET' }}"
```

### Using context variables:
```yaml
tasks:
  - name: api_call
    type: http
    endpoint: https://api.example.com/users/{{ context "user_id" }}
    headers:
      Authorization: Bearer {{ context "auth_token" }}
```

## Sample Pipelines

- `test/pipelines/convert_industries.yaml` - HTTP GET request to fetch CSV data
- `test/pipelines/context_test.yaml` - JQ task forming HTTP request configuration passed to HTTP task
- `test/pipelines/proxy_test.yaml` - HTTP with proxy configuration

## Use Cases

- **API integration**: Connect to external APIs and services
- **Data aggregation**: Fetch data from multiple sources
- **Web scraping**: Retrieve data from web pages
- **Authentication**: Handle OAuth flows for secure APIs
- **Data synchronization**: Keep data in sync with external systems

## Error Handling

The task includes built-in retry logic:
- Automatically retries failed requests
- Configurable retry count and delay
- Respects HTTP status codes for retry decisions
- Can be configured to fail the pipeline on errors

## Security Considerations

- OAuth credentials should be stored securely (use environment variables or secrets)
- HTTPS endpoints are recommended for production use
- Consider rate limiting for API endpoints