---
skill: pagination
version: 1.0.0
caterpillar_type: http
description: Paginate through multi-page HTTP API responses using the next_page JQ field on the http task.
role: modifier (applied to http task)
requires_upstream: false
requires_downstream: true
aws_required: false
---

## Purpose

The `next_page` field on an `http` task enables automatic pagination. After each
HTTP response, caterpillar evaluates the `next_page` JQ expression. If it
produces a URL string or request object, a follow-up request is made. When it
produces `null` or `empty`, pagination stops and the pipeline moves on.

Every page's response body is emitted downstream as a separate record.

## How it works

```
┌─────────────┐     ┌──────────────┐     ┌──────────────────┐
│ HTTP request │────▶│ HTTP response│────▶│ Emit record      │
└─────────────┘     └──────┬───────┘     └──────────────────┘
                           │
                    ┌──────▼───────┐
                    │ Evaluate     │
                    │ next_page JQ │
                    └──────┬───────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
          string        object        null/empty
        (next URL)   (full override)  (stop)
              │            │
              └─────┬──────┘
                    ▼
            Next HTTP request
            (loop continues)
```

## JQ input

The `next_page` JQ expression receives a JSON object with two keys:

```json
{
  "data":    "<raw response body as a string>",
  "headers": {
    "Content-Type": ["application/json"],
    "Link": ["<https://api.example.com/items?page=2>; rel=\"next\""]
  }
}
```

| Key | Type | Description |
|-----|------|-------------|
| `data` | string | Raw HTTP response body. Use `.data \| fromjson` to parse as JSON. |
| `headers` | `map[string][]string` | Response headers. Each value is an array of strings. Header names use Go canonical form (`content-type` becomes `Content-Type`). |

## Built-in variables

| Variable | Access pattern | Description |
|----------|---------------|-------------|
| `page_id` | `[inputs][1].page_id` or `(input \| input \| .page_id)` | Page counter — starts at **2** on the first `next_page` call (page 1 is the initial request) and increments by 1 for each subsequent page. |

Both access patterns are equivalent. `[inputs][1].page_id` is the array form;
`(input | input | .page_id)` is the sequential form — use whichever reads
better in your expression.

## Return values

| JQ result | Behavior |
|-----------|----------|
| `"https://..."` (string) | Makes the next request to this URL. Method, headers, and body remain unchanged from the current request. |
| `{ "endpoint": "...", ... }` (object) | Makes the next request using the fields from this object. Only `endpoint` is required; all other fields are optional overrides. |
| `null` | Stops pagination. |
| `empty` | Stops pagination (JQ produces no output). |

### Object return schema

```yaml
{
  "endpoint": "<string>",      # REQUIRED — URL for the next request
  "method":   "<string>",      # OPTIONAL — override HTTP method (e.g. POST)
  "body":     "<string>",      # OPTIONAL — override request body
  "headers":  { "<k>": "<v>" },# OPTIONAL — merged into existing headers
  "proxy": {                   # OPTIONAL — proxy config for the next request
    "scheme": "<string>",      #   e.g. "http"
    "host":   "<string>",      #   e.g. "proxy.internal:8080"
    "insecure_tls": <bool>     #   skip TLS verification
  }
}
```

When `headers` is provided, new headers are merged with existing ones. If a
header key already exists, the new value wins.

### Partial object return

You can return an object with only some fields — missing fields carry forward
from the current request. For example, returning only `body` keeps the current
endpoint, method, and headers:

```yaml
next_page: |
  .data | fromjson |
  if (.items | length) == 500 then
    { body: { pageNumber: (.currentPage + 1) } | @json }
  else empty end
```

## Setting `next_page` dynamically

There are two ways to set `next_page`:

1. **Static** — defined directly on the `http` task in YAML.
2. **Dynamic** — set as a field in the upstream record's JSON. The HTTP task
   merges upstream record fields into its config via `json.Unmarshal`, so
   `next_page` from the record overrides the YAML value.

This lets a JQ task upstream construct both the request and its pagination
logic at runtime.

## Pagination patterns

### Pattern 1: Cursor / token in response body

The API returns a cursor or token in the JSON body. Check for its presence and
construct the next URL. This is the most common pagination pattern.

```yaml
- name: fetch_all_items
  type: http
  method: GET
  endpoint: https://marketplace.example.com/v3/items?limit=1000&nextCursor=*
  expected_statuses: "200,401"
  retry_delay: 70s
  max_retries: 10
  next_page: >-
    .data | fromjson |
    if .nextCursor != null then
      "https://marketplace.example.com/v3/items?limit=1000&nextCursor=\(.nextCursor)"
    else null end
```

Common field names: `.nextCursor`, `.next_page_token`, `.nextToken`,
`.nextContinuationToken`, `.response_metadata.next_cursor`,
`.list.meta.nextCursor`, `.pagination.nextToken`.

When tokens may contain special characters, URL-encode them with `@uri`:

```yaml
next_page: |
  .data | fromjson |
  if (.nextContinuationToken // "") != "" then
    "https://api.example.com/docs?continuationToken=" + (.nextContinuationToken | @uri)
  else empty end
```

**When to use:** Walmart Marketplace (items, orders, listing quality), Slack
(`response_metadata.next_cursor`), Bol.com (orders), Lexion
(`nextContinuationToken`), Amazon SP-API Support Cases (`nextToken`),
Google Drive (`nextPageToken`), and most REST APIs with cursor/token-based
pagination.

### Pattern 2: Offset calculated from `page_id`

The API uses offset-based pagination. Use the built-in `page_id` counter to
compute the offset.

```yaml
- name: fetch_inventory
  type: http
  endpoint: https://api.example.com/offers?limit=100&offset=0
  next_page: |
    .data | fromjson |
    if (.offers | length) == 100 then
      "https://api.example.com/offers?limit=100&offset=" +
        (([inputs][1].page_id - 1) * 100 | tostring)
    else null end
```

**When to use:** Allegro (inventory offers), Rapid7 InsightIDR
(investigations index), Shelf Catalog API (page number), Threepn FNSKU API,
Pattern Inventory Hub (encumbrance states), Mirakl (product offers offset),
and any API that uses `offset` + `limit` without providing a next URL.

### Pattern 3: Total count vs. fetched count

The API returns a total count. Compare it against how many records you've
fetched so far to decide whether to continue.

```yaml
- name: get_returns
  type: http
  next_page: |
    .data | fromjson |
    if .count > (.offset // 0) + (.customerReturns | length) then
      "https://api.example.com/returns?limit=100&offset=" +
        ((.offset // 0) + (.customerReturns | length) | tostring)
    else null end
```

**When to use:** Allegro (returns — `.count` vs fetched), Bol.com (orders —
array length vs `pageSize`), and any API that returns a total count or where
you compare fetched batch size against a known page limit.

### Pattern 4: Link header (RFC 5988)

The API puts the next page URL in the `Link` response header.

```yaml
- name: get_users
  type: http
  endpoint: https://api.example.com/v1/users?limit=30
  headers:
    Authorization: {{ secret "/path/to/token" }}
  max_retries: 100
  next_page: >-
    .headers["Link"][] |
    select(test("rel=\"next\"")) |
    capture("<(?<url>[^>]+)>").url
```

**When to use:** Okta (users API — `Link` header with `rel="next"`), GitHub,
and any API following RFC 5988 link relations where the full next URL is in the
`Link` response header.

### Pattern 5: Link header with field extraction

A variant where the next page token is embedded in the Link header URL and
must be extracted with a regex.

```yaml
- name: get_catalog_items
  type: http
  next_page: >-
    .headers.Link[0] |
    match("after_id=([^&>]+)") |
    .captures[0].string |
    "https://api.example.com/products?per_page=1000&after_id=\(.)"
```

**When to use:** Target Plus (products catalog — `after_id` embedded in Link
header URL) and similar APIs where the next page cursor must be regex-extracted
from a Link header value rather than used as a complete URL.

### Pattern 6: Object return — override endpoint, headers, and body

When the next page request needs different headers, body, or method (e.g.
signed requests, rotating tokens), return a full object.

```yaml
- name: get_orders
  type: http
  method: POST
  next_page: |
    .data | fromjson |
    if .data.next_page_token and (.data.next_page_token != "") then
      (now | floor | tostring) as $timestamp |
      "SECRET_VALUE" as $app_secret |
      ({date_from: "2024-01-01"} | tojson) as $body |
      {
        "endpoint": "https://api.example.com/orders/search?page_token=" + .data.next_page_token + "&timestamp=" + $timestamp,
        "headers": {
          "Authorization": "Bearer {{ context "access_token" }}",
          "Content-Type": "application/json"
        },
        "body": $body
      }
    else null end
```

**When to use:** TikTok Shop (orders, products, prices, returns — UK and US
markets, HMAC-SHA256 signing per request), Coupang (CGF fees, revenue
settlement, product listings — CEA HMAC signing), Walmart Pricing Insights
(body-only override with `pageNumber`), Amazon SP-API Contacts (with proxy
config), and any API where each page request needs independently computed
authentication signatures, different body, or rotating headers.

### Pattern 7: Full request override with context references

Combine `next_page` object return with `{{ context "..." }}` references for
values extracted earlier in the pipeline.

```yaml
- name: collect_listings
  type: http
  method: GET
  expected_statuses: 200..299,400,403
  max_retries: 5
  next_page: |
    .data | fromjson as $body |
    ($body.pagination.nextToken // "") as $token |
    if ($token | tostring) != "" then
      {
        endpoint: ("{{ context "base_endpoint" }}?{{ context "base_query" }}&pageToken=" + ($token | @uri)),
        method: "GET",
        headers: {
          "x-amz-access-token": "{{ context "access_token" }}",
          "Content-Type": "application/json"
        },
        proxy: {
          scheme: "http",
          host: "rate-gate.prod.pattern.aws.internal:8080",
          insecure_tls: true
        }
      }
    else empty end
```

**When to use:** Amazon SP-API `searchListingsItems` (both 3P seller and
1P vendor flows — base endpoint, query string, access token, merchant ID,
and rate-limit scope all stored in context), and any multi-step pipeline
where auth tokens, base URLs, or query parameters from earlier tasks are
needed in pagination via `{{ context "..." }}`.

### Pattern 8: Dynamic `next_page` from upstream JQ

Set `next_page` as a field in the upstream JQ output. The HTTP task picks it
up from the record data automatically.

```yaml
- name: build_request
  type: jq
  path: |
    {
      endpoint: "https://api.example.com/meetings?page_size=150",
      headers: {
        "Authorization": "Bearer {{ context "access_token" }}"
      },
      next_page: ".data | fromjson | if (.next_page_token and (.next_page_token != \"\")) then (\"https://api.example.com/meetings?page_size=150&next_page_token=\" + (.next_page_token | @uri)) else empty end"
    }

- name: get_meetings
  type: http
  method: GET
  fail_on_error: true
```

**When to use:** Zoom Meetings API (next_page_token in upstream JQ), Keepa
token-gate (sellers and products — dynamic `next_page` with `sleep()` for
rate-limiting), and any case where the pagination logic varies per-record,
needs runtime construction, or must embed rate-limiting behavior like
`sleep()` calls for API quota replenishment.

### Pattern 9: Complex multi-field page tracking

Some APIs require tracking multiple pagination fields (page ID, page size,
total pages) across requests. Return an object with extra fields to carry
state.

```yaml
- name: fetch_reviews
  type: http
  expected_statuses: "200,504"
  fail_on_error: true
  next_page: |
    (.data? // .) as $raw |
    ($raw | (fromjson? // .)) as $resp |
    ((($resp.reviews // $resp.reviewList // []) | length)) as $n |
    ($resp.pageId // ([inputs][1].page_id // 0)) as $current |
    ($resp.pageSize // ([inputs][1].page_size // 50)) as $page_size |
    ($resp.totalPageCount // 0) as $total |
    ($current + 1) as $next |
    (20) as $max |
    (if $total > 0 then ([$total, $max] | min) else $max end) as $stop |
    if ($next < $stop) and (($total > 0) or ($n == $page_size)) then
      {
        method: "GET",
        page_id: $next,
        page_size: $page_size,
        endpoint: ("https://api.example.com/reviews?pageId=" + ($next | tostring) + "&pageSize=" + ($page_size | tostring))
      }
    else null end
```

Key techniques in this pattern:
- **Defensive parsing**: `(.data? // .) as $raw | ($raw | (fromjson? // .))` handles both string and pre-parsed input.
- **Multiple fallback fields**: `$resp.reviews // $resp.reviewList // []` tries alternative field names.
- **Carried state**: returning `page_id` and `page_size` in the object makes them available to subsequent `next_page` evaluations via `[inputs][1]`.
- **Max pages safety cap**: `(20) as $max` prevents runaway pagination loops.

**When to use:** Amazon Seller Central Brand Customer Reviews (tracks
`pageId`, `pageSize`, `totalPageCount` with a max-pages safety cap), Seller
Central Voice of Customer (offset + page_id with full header override),
and any scraping or API scenario where multiple pagination fields must be
tracked across requests and a hard page limit prevents runaway loops.

### Pattern 10: GraphQL cursor-based pagination

GraphQL APIs typically paginate using a `pageInfo` object with `hasNextPage`
and `endCursor`. Since the query is sent as a POST body, `next_page` must
return an object that overrides the `body` with the updated cursor variable.

```yaml
- name: fetch_all_products
  type: http
  method: POST
  endpoint: https://api.example.com/graphql
  headers:
    Content-Type: application/json
    Authorization: Bearer {{ context "access_token" }}
  body: |
    {
      "query": "query($first: Int!, $after: String) { products(first: $first, after: $after) { edges { node { id name sku } } pageInfo { hasNextPage endCursor } } }",
      "variables": { "first": 100 }
    }
  next_page: |
    .data | fromjson |
    if .data.products.pageInfo.hasNextPage then
      {
        "endpoint": "https://api.example.com/graphql",
        "body": ({
          "query": "query($first: Int!, $after: String) { products(first: $first, after: $after) { edges { node { id name sku } } pageInfo { hasNextPage endCursor } } }",
          "variables": { "first": 100, "after": .data.products.pageInfo.endCursor }
        } | tojson)
      }
    else null end
```

For large queries, move the GraphQL query string into a context variable
upstream to avoid repeating it in both `body` and `next_page`:

```yaml
- name: prepare_graphql_request
  type: jq
  path: |
    "query($first: Int!, $after: String) { orders(first: $first, after: $after) { edges { node { id total } } pageInfo { hasNextPage endCursor } } }" as $query |
    {
      endpoint: "https://api.example.com/graphql",
      headers: {
        "Content-Type": "application/json",
        "Authorization": "Bearer {{ context "access_token" }}"
      },
      body: ({ query: $query, variables: { first: 50 } } | tojson),
      next_page: (
        ".data | fromjson | if .data.orders.pageInfo.hasNextPage then { endpoint: \"https://api.example.com/graphql\", body: ({ query: " + ($query | tojson) + ", variables: { first: 50, after: .data.orders.pageInfo.endCursor } } | tojson) } else null end"
      )
    }

- name: fetch_orders
  type: http
  method: POST
```

**When to use:** Shopify, GitHub, and any GraphQL API that uses Relay-style
cursor pagination with `pageInfo { hasNextPage endCursor }`.

### Pattern 11: HATEOAS links array in response body

Some REST APIs return a `links` array in the response JSON with objects like
`{ "rel": "next", "href": "/path?offset=100" }`. Filter by `rel == "next"`
and extract the `href`.

When the `href` is a relative path, prefix it with the API host:

```yaml
- name: fetch_receipts
  type: http
  next_page: |
    .data | fromjson |
    (.links[] | select(.rel == "next" and .href != "") |
      "https://api.otto.market\(.href)") // empty
```

When the API returns fully qualified URLs in `.href`, use it directly:

```yaml
- name: get_exchange_rates
  type: http
  method: POST
  endpoint: 'https://example.com/services/rest/query/v1/suiteql?limit=500'
  headers:
    Content-Type: application/json
  body: '{"q": "SELECT * FROM exchange_rates"}'
  next_page: >-
    .data | fromjson | .links[] | select(.rel == "next") | .href
  oauth:
    realm: 12345
    token: {{ secret "/netsuite/token" }}
    token_secret: {{ secret "/netsuite/token_secret" }}
    consumer_key: {{ secret "/netsuite/consumer_key" }}
    consumer_secret: {{ secret "/netsuite/consumer_secret" }}
```

**When to use:** Otto Market (receipts, inventory — relative `href` prefixed
with base URL), NetSuite SuiteQL (exchange rates, currencies — fully
qualified `href`), and any API following HATEOAS conventions where the next
page URL is in a `links` array with `rel: "next"`.

### Pattern 12: Rate-limiting gate with `sleep()`

Use `next_page` to poll a status endpoint repeatedly, sleeping between
checks, until a condition is met (e.g. API tokens are replenished). The
`sleep()` JQ function pauses execution before returning the URL.

```yaml
- name: build_token_check
  type: jq
  path: |
    {
      endpoint: "https://api.example.com/token?key={{ secret "/api/token" }}",
      next_page: "if (.data | fromjson | .tokensLeft | tonumber) < 100 then (\"https://api.example.com/token?key={{ secret "/api/token" }}\" | sleep(\"30s\")) else empty end"
    }

- name: token_gate
  type: http
  method: GET
  timeout: 60s
```

When `tokensLeft` is below the threshold, the JQ returns the same URL
wrapped in `sleep("30s")`, causing a 30-second pause before the next poll.
Once enough tokens are available, it returns `empty` to stop and proceed.

**When to use:** Keepa sellers and products pipelines (polls
`/token?key=...` endpoint, sleeps 30s when `tokensLeft` is below threshold),
and any API with rate-limiting where you must wait for token/quota
replenishment before making further data requests.

### Pattern 13: Nested pagination object in response body

Some APIs return a `next_page` or `paging` object in the response body
containing the next URL directly.

```yaml
- name: get_custom_fields
  type: http
  endpoint: https://app.example.com/api/1.0/custom_fields?limit=100
  headers:
    Authorization: Bearer {{ secret "/api/token" }}
  next_page: |
    .data | fromjson |
    if (.next_page and .next_page.offset != null)
    then .next_page.uri
    else null end
```

**When to use:** Asana (custom fields, tasks — `.next_page.uri` contains the
full next URL when `.next_page.offset` is present), and APIs that return a
structured pagination object
(e.g. `{ "next_page": { "offset": "...", "uri": "https://..." } }`) rather
than a flat cursor field.

### Pattern 14: Per-page HMAC signing

APIs that require a unique cryptographic signature for every request need
the signing logic inside `next_page`. Use `now`, `hmac_sha256`, and
string concatenation to compute the signature per page.

```yaml
- name: coupang_api
  type: http
  next_page: |
    (input | input | .page_id) as $page_id |
    if (.data | fromjson | .data | length) >= 20 then
      (now | todateiso8601 | .[2:19] | gsub(":";"") | gsub("-";"") + "Z") as $datetime |
      "GET" as $method |
      "/v2/providers/openapi/apis/api/v1/vendors/{{ context "vendor_id" }}/settlement/cgf-fee/date-range" as $path |
      ("fromDate={{ macros.ds_add(ds, -1) }}&toDate={{ ds }}&pageNum=" + ($page_id | tostring) + "&pageSize=50") as $query |
      ($datetime + $method + $path + $query) as $message |
      ($message | hmac_sha256($message; "{{ context "secret_key" }}")) as $sign |
      {
        "endpoint": "https://api-gateway.coupang.com" + $path + "?" + $query,
        "headers": {
          "Authorization": "CEA algorithm=HmacSHA256, access-key={{ context "access_key" }}, signed-date=" + $datetime + ", signature=" + $sign
        }
      }
    else null end
```

Key elements:
- `now | todateiso8601` generates a fresh timestamp per page request.
- `hmac_sha256(message; secret)` computes the HMAC signature.
- Secrets are injected via `{{ context "..." }}` or `{{ secret "..." }}` — never hardcoded.

For TikTok-style signing, the pattern is similar but concatenates path +
query parameters + body into the HMAC input:

```yaml
next_page: |
  .data | fromjson |
  if .data.next_page_token and (.data.next_page_token != "") then
    (now | floor | tostring) as $timestamp |
    "{{ secret "/app_secret" }}" as $app_secret |
    ("/order/202309/orders/search"
      + "app_key" + "{{ secret "/app_key" }}"
      + "page_size" + "100"
      + "page_token" + .data.next_page_token
      + "shop_cipher" + "{{ context "cipher" }}"
      + "timestamp" + $timestamp
      + $body) as $concat |
    ($app_secret + $concat + $app_secret) as $input_string |
    hmac_sha256($input_string; $app_secret) as $signed |
    {
      "endpoint": "https://open-api.tiktokglobalshop.com/order/202309/orders/search?app_key={{ secret "/app_key" }}&page_size=100&page_token=" + .data.next_page_token + "&timestamp=" + $timestamp + "&sign=" + $signed,
      "headers": { "x-tts-access-token": "{{ context "access_token" }}" },
      "body": $body
    }
  else null end
```

**When to use:** Coupang (CGF fees, revenue settlement — CEA HMAC signing),
TikTok Shop (orders, products, prices, returns — HMAC-SHA256 per page),
and any API that requires a unique cryptographic signature for every request.

### Pattern 15: Batch-size comparison (count == limit)

When the API doesn't return a cursor or total count, detect more pages by
comparing the current batch size to the page limit. If the batch is full,
request the next page; if it's smaller, you've reached the end.

```yaml
- name: get_inventory
  type: http
  fail_on_error: true
  next_page: |
    .data | fromjson |
    if .count == 100 then
      "https://api.example.com/offers?limit=100&offset=" +
        (([inputs][1].page_id - 1) * 100 | tostring)
    else null end
```

This pattern often combines with `page_id`-based offset calculation
(Pattern 2). The stop condition is `batch_size < limit`.

**When to use:** Allegro inventory (`.count == limit`), Goborderless FNSKU
(`length == per_page`), Shelf catalog (`length == per_page`), Rapid7
InsightIDR (`length == 100`), and any API where a full batch implies more
data and a short batch means done.

### Pattern 16: Offset from `page_id` with session headers

Some scraping-style endpoints (Seller Central, internal APIs) require
session cookies or browser-like headers on every request. Combine
`page_id`-based offset with full header override in the returned object.

```yaml
- name: fetch_voice_of_customer
  type: http
  next_page: |
    .data | fromjson |
    if (.pcrListings | length) == 25 then
      {
        method: "GET",
        endpoint: ("https://sellercentral.amazon.com/pcrHealth/pcrListingSummary?pageSize=25&pageOffset=" +
          ((([inputs][1].page_id // 0) + 1) * 25 | tostring) +
          "&sortColumn=ORDERS_COUNT&sortDirection=DESCENDING"),
        headers: {
          "accept": "application/json",
          "Cookie": "{{ context "cookie_header" }}",
          "user-agent": "Mozilla/5.0 ..."
        }
      }
    else null end
  expected_statuses: "200,504"
```

**When to use:** Seller Central Voice of Customer (session cookies from
headless browser), and any endpoint that requires browser-like session
headers to be carried through pagination.

## Choosing the right pattern

| API behavior | Pattern |
|-------------|---------|
| Returns `nextCursor`, `next_page_token`, or similar | Pattern 1 (cursor) |
| Uses `offset` + `limit`, no next URL provided | Pattern 2 (page_id offset) |
| Returns `total` / `count` alongside results | Pattern 3 (total count) |
| Next URL in `Link` response header | Pattern 4 (Link header) |
| Cursor embedded in Link header URL | Pattern 5 (Link field extraction) |
| Each page request needs unique auth/signing | Pattern 6 (object return) |
| Auth tokens from earlier pipeline steps via context | Pattern 7 (context refs) |
| Pagination logic varies per-record or needs runtime construction | Pattern 8 (dynamic from upstream) |
| Multiple pagination fields to track (pageId, totalPages, etc.) | Pattern 9 (multi-field) |
| GraphQL API with `pageInfo { hasNextPage endCursor }` | Pattern 10 (GraphQL cursor) |
| Response body has `links: [{rel: "next", href: "..."}]` | Pattern 11 (HATEOAS links) |
| Must wait for API rate-limit / token replenishment | Pattern 12 (sleep gate) |
| Response body has nested pagination object (e.g. `.next_page.uri`) | Pattern 13 (nested paging object) |
| Each page needs a unique HMAC/signature computed in JQ | Pattern 14 (per-page HMAC) |
| No cursor or total — detect more pages by `batch_size == limit` | Pattern 15 (batch-size comparison) |
| Scraping endpoint requiring session cookies / browser headers | Pattern 16 (session headers offset) |

## Common JQ idioms

### URL-encoding tokens with `@uri`

Many APIs return tokens that contain characters like `=`, `+`, or `/`. Use
`@uri` to URL-encode them before embedding in URLs:

```jq
"https://api.example.com/items?nextToken=" + (.nextToken | @uri)
```

Some APIs (e.g. Slack) return cursors that are already partially encoded but
missing trailing `=` signs. Append them manually:

```jq
.response_metadata.next_cursor + "%3D"
```

### Defensive JSON parsing

When the response format may vary (string vs. pre-parsed JSON), use a
defensive chain:

```jq
(.data? // .) as $raw |
($raw | (fromjson? // .)) as $resp |
```

This handles: raw string body (`.data | fromjson`), already-parsed JSON
(falls through to `.`), and missing `.data` key (falls back to `.`).

### Safe defaults with `//`

Use `//` to provide fallback values when fields may be absent:

```jq
($resp.pageId // ([inputs][1].page_id // 0)) as $current |
($resp.pageSize // 50) as $page_size |
($resp.totalPageCount // 0) as $total |
(.offset // 0) + (.items | length)
```

### Multiple fallback field names

When the API uses different field names across versions or endpoints:

```jq
(($resp.reviews // $resp.reviewList // []) | length) as $n |
```

### Timestamp generation for signing

For APIs requiring per-request timestamps:

```jq
(now | floor | tostring) as $timestamp |
(now | todateiso8601 | .[2:19] | gsub(":";"") | gsub("-";"") + "Z") as $datetime |
```

### HMAC signing

```jq
($message | hmac_sha256($message; $secret_key)) as $signature |
```

### Object construction with `tojson` / `@json`

Convert objects to JSON strings for request bodies:

```jq
{ body: { pageNumber: (.currentPage + 1), sort: { field: "date" } } | @json }
```

## Resilience settings for paginated sources

Paginated HTTP tasks should include resilience settings appropriate to the
API. These are set on the `http` task alongside `next_page`:

| Field | Default | Description |
|-------|---------|-------------|
| `expected_statuses` | `"200"` | Comma-separated or range. E.g. `"200,401"`, `"200..299,400,403"`, `"200,504"`. |
| `max_retries` | `3` | Number of retry attempts per page. Set higher for flaky APIs (e.g. `10`, `100`). |
| `retry_delay` | `5` | Seconds between retries. Use longer delays for rate-limited APIs (e.g. `70s`). |
| `retry_backoff_factor` | `1` | Multiplier for exponential backoff. Set `2` for doubling delay. |
| `timeout` | `90` | Request timeout in seconds. Increase for slow APIs. |
| `fail_on_error` | `false` | When `true`, a page failure stops the pipeline. Recommended for source tasks. |

Example with full resilience config:

```yaml
- name: collect_listings
  type: http
  expected_statuses: 200..299,400,403,500,503
  max_retries: 5
  retry_backoff_factor: 2
  timeout: 120
  fail_on_error: true
  next_page: ...
```

## Validation rules

- `next_page` input is `{"data": "...", "headers": {...}}` — NOT the parsed body. Always `.data | fromjson` first.
- Response headers are accessed via `.headers["Header-Name"]` — values are arrays of strings.
- Return `null` or `empty` to stop. Returning an empty string `""` will be treated as an endpoint URL and cause an error.
- `page_id` starts at **2** (the initial request is page 1). The first `next_page` evaluation sees `page_id = 2`.
- When returning an object, `endpoint` is **required** unless you are doing a partial override (e.g. body-only). Missing `endpoint` with no carry-forward will silently stop pagination.
- `headers` in the returned object are **merged** — they don't replace all existing headers, they add/override individual keys.
- `{{ context "..." }}` and `{{ secret "..." }}` templates are resolved **before** the JQ expression is parsed, so they work inside `next_page`.
- When setting `next_page` dynamically from upstream, the value must be a JQ expression **string**, not a pre-evaluated object.
- `proxy` in the returned object is applied to the next request only — it does not persist across subsequent pages unless returned each time.

## Anti-patterns

- **Accessing response fields directly** (`.nextCursor`) without `.data | fromjson` — the response body is a raw string inside `{"data": "...", "headers": {...}}`.
- **Using `""` to stop pagination** — use `null` or `empty`. An empty string is treated as a URL and causes an error.
- **Hardcoding secrets** in `next_page` JQ — use `{{ secret "/path" }}` or `{{ context "key" }}`.
- **Off-by-one errors with `page_id`** — remember it starts at 2, not 1. The first `next_page` call has `page_id = 2` because page 1 is the initial request.
- **Infinite pagination loops** — always include a condition that eventually produces `null`/`empty`:
  - Check cursor presence: `if .nextCursor != null then ... else null end`
  - Check batch size: `if (.items | length) == limit then ... else null end`
  - Set a max page cap: `(20) as $max | if $next < $max then ... else null end`
- **Forgetting `fail_on_error: true`** on paginated sources — a single page failure will silently stop pagination without it.
- **Mismatched offset multiplier and limit** — when using `page_id` offset, ensure `(page_id - 1) * N` uses the same `N` as the `limit` parameter in the URL.
- **Not URL-encoding tokens** — tokens with special characters (`=`, `+`, `/`, `&`) will break the URL. Use `| @uri` to encode them.
- **Forgetting `proxy` on subsequent pages** — if the initial request uses a proxy, the `next_page` object must include `proxy` on every page. Proxy does not carry forward automatically.
- **Recomputing timestamps outside `next_page`** — for signed APIs, the timestamp must be generated inside the `next_page` JQ (via `now | floor`) so each page gets a fresh signature. Using a static timestamp will cause signature mismatches.
