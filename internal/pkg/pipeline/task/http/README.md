# HTTP Task

The `http` task makes HTTP requests to external APIs and services, with support for OAuth authentication, retry logic, and response processing.

## Function

The HTTP task sends HTTP requests to specified endpoints and processes the responses. It can handle various HTTP methods, authentication schemes, and automatically retry failed requests.

## Behavior

The HTTP task operates in two modes depending on whether an input channel is provided:

- **With input channel**: Receives JSON-formatted HTTP request configurations from the input channel. Each record's data should contain a JSON object with HTTP request parameters (method, endpoint, headers, body, etc.). The task merges these with the base configuration from YAML.

- **Without input channel**: Uses the endpoint and configuration specified in the YAML configuration to make HTTP requests. This mode supports pagination and can make multiple requests automatically.

In both modes, the task sends HTTP response data to its output channel and supports automatic retries, OAuth authentication, and proxy configuration.

### Response Format and Headers

The HTTP task outputs the response body as-is (maintains backward compatibility). Response headers are automatically stored in the record's context with the prefix `http-header-`, making them accessible to downstream tasks via context variables.

For example, if the HTTP response includes a `Content-Type` header, it will be available as `{{ context "http-header-Content-Type" }}` in subsequent tasks. Note that HTTP header names are case-sensitive when used as context keys. Go's HTTP library canonicalizes header names (for example, `content-type` becomes `Content-Type`), so you must use the canonical form when accessing headers via context (for example, `http-header-Content-Type`, not `http-header-content-type`).

### Pagination

`next_page` is a JQ expression evaluated after every response. Returning `empty` ends pagination, returning a string sets the next endpoint, and returning an object sets any of `endpoint`, `method`, `body`, `headers`, and `context`. A bare string reuses the current method and headers.

The expression receives two JQ inputs. The first is the response envelope:

| Field | Description |
|-------|-------------|
| `.data` | Response body as a string |
| `.headers` | Response headers |

The second holds the page counter:

| Field | Description |
|-------|-------------|
| `.page_id` | 1-indexed number of the page about to be requested |

Because the initial request is page 1, `page_id` is `2` on the first evaluation. It covers APIs that page by number or offset:

```yaml
next_page: |
  [inputs] as $input |
  ($input[0].data | fromjson) as $body |
  ($input[1].page_id) as $page |
  if ($body.results | length) == 100 then
    "https://api.example.com/things?per_page=100&page=" + ($page | tostring)
  else empty end
```

`inputs` is a one-shot iterator, so capture it once and index into it. A second `[inputs]` yields an empty list, and the resulting `null` compares as less than every number — which silently turns a bound like `$page <= 50` into an infinite loop.

Piping also rebinds `.`, so `.data | fromjson as $body | ...` leaves `.` as the body string rather than the envelope.

#### Carrying state across pages

`context` writes values onto the record before the next iteration renders its templates. Use it for state the response and the counter can't reconstruct — a cursor, a query the next request must reproduce, a running budget. Values are readable via `{{ context "key" }}` in later iterations and by downstream tasks.

Some APIs scope a page token to the query that issued it, and cap how many items one query can walk, so exhausting a token means re-querying with a new filter and then paging that. The token loop has to reproduce the current filter, not the one the task started with:

```yaml
next_page: |
  [inputs] as $input |
  ($input[0].data | fromjson) as $body |
  ($body.next_token // "") as $token |
  ($body.items | length) as $count |
  ($body.items | .[-1].id // "") as $last_id |
  "{{ context "current_query" }}" as $query |
  if $token != "" then
    { endpoint: ("https://api.example.com/things?" + $query + "&page_token=" + ($token | @uri)) }
  elif $count >= 100 and $last_id != "" then
    (($query | sub("&after_id=[^&]*"; "")) + "&after_id=" + ($last_id | @uri)) as $next_query |
    {
      endpoint: ("https://api.example.com/things?" + $next_query),
      context: { current_query: $next_query }
    }
  else empty end
```

The token branch replays `current_query` as-is; the branch that moves the filter rewrites and republishes it, so new tokens are replayed against the query that issued them.

These are values, not expressions — the task-level `context:` block takes JQ that caterpillar evaluates for you, but `next_page` is itself the JQ. A string is stored verbatim, so `context: { cursor: ".data | fromjson | .next" }` stores that text instead of the extracted value. Keep values scalar; objects render as inline JSON.

#### Reading a context key vs writing one

Writing creates the key — `context: { anything: $value }` works whether or not it existed.

Reading requires the key to be set already. Templates are substituted as plain text before JQ parses the expression, so even an unreachable branch counts:

```yaml
# fails: context keys were not set: cursor
next_page: 'if false then "{{ context "cursor" }}" else empty end'
```

Seed anything the expression reads in the upstream task's `context` block, since on page 1 the read happens before any write.

There is no cap on iterations: an expression that never returns `empty` loops forever. Make sure every branch advances toward a terminal condition, since a cursor that repeats or a boundary that stops moving will not stop on its own.

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
| `next_page` | string/object | - | JQ expression to extract next page URL (string) or pagination config (object with `endpoint`, `method`, `body`, `headers`, `context`) — see [Pagination](#pagination) |
| `context` | map[string]string | - | JQ expressions to extract values from the response and store in record context |
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

### Setting context values from response and accessing headers:
```yaml
tasks:
  - name: fetch_user
    type: http
    method: GET
    endpoint: https://api.example.com/user/123
    context:
      user_name: ".data | fromjson | .name"
      user_email: ".data | fromjson | .email"
  
  - name: use_context
    type: jq
    path: |
      {
        "greeting": "Hello {{ context "user_name" }}",
        "email": "{{ context "user_email" }}",
        "content_type": "{{ context "http-header-Content-Type" }}"
      }
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