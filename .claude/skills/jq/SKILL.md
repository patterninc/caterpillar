---
skill: jq
version: 1.0.0
caterpillar_type: jq
description: Transform, filter, reshape, or extract fields from JSON data using JQ queries.
role: transform
requires_upstream: true
requires_downstream: true
aws_required: conditional  # only when using translate() custom function
---

## Purpose

Applies a JQ expression to each record's data. The result replaces the record data.
When `explode: true`, array results are split into individual records.
Custom function `translate(text; src; tgt)` calls AWS Translate.

## How stored JSON is produced (read this if output looks “invalid”)

Caterpillar **always JSON-encodes** the JQ result with Go’s `encoding/json` before the record leaves the jq task (unless `as_raw: true`). Your `path` should return **native** JQ values (objects, arrays, numbers, strings, booleans, null)—not pre-serialized JSON text for whole-record payloads.

| Symptom | Typical cause | Fix |
|--------|----------------|-----|
| Nested fields show as quoted JSON strings (`"{\"a\":1}"`) | Used `tojson` / `tostring` on objects you wanted as nested JSON | Emit the object directly: `"nested": .foo` not `"nested": (.foo \| tojson)` |
| Whole file fails `JSON.parse` / “invalid JSON” in one shot | File has **one JSON value per line** (NDJSON / JSON Lines) or `join` concatenated multiple values | Use an NDJSON reader, or end with a jq that outputs **one** array/object for the whole batch (no `explode`), or write `.jsonl` / document NDJSON in the consumer |
| Downstream sees `null` after jq | `path` used `.data \| fromjson` on body that is already an object | Use `.field` on the body; reserve `.data \| fromjson` for **`context:`** only |
| `explode: true` errors or wrong fan-out | Path returns a single non-array | Use a path that yields multiple outputs (e.g. `.items[]`) or one array and `explode: true` |

**`tojson` in `path`:** Use on purpose when the **next step needs a string** (HTTP `body` that must be a string, cookie blobs, form fields). For sinks that expect structured JSON records, **omit `tojson`** so nested data stays as real JSON objects/arrays after the second encode.

**`as_raw: true`:** Skips JSON marshaling; output is `fmt`’d text. Only for plain-text downstream tasks.

## NDJSON vs one JSON document

- **Default file sink behavior:** each record is written out as its own JSON serialization (often one line per record).
- **`join` with default delimiter `\n`:** merges many records into **one** record whose `data` is **multiple JSON values separated by newlines**—still not a single JSON array unless you built one in jq.
- **If you need one JSON array in a file:** use a jq `path` that produces **one** array value for the whole batch (no `explode`), or keep NDJSON and use tools that read line-by-line. After `join`, the record body is multiple JSON documents concatenated; it is **not** one `json.Unmarshal`-able value unless you built a single array/object in jq **before** join/file.

## Schema

```yaml
- name: <string>              # REQUIRED
  type: jq                    # REQUIRED
  path: <string>              # REQUIRED — JQ expression
  explode: <bool>             # OPTIONAL — split array output into separate records (default: false)
  as_raw: <bool>              # OPTIONAL — emit raw string instead of JSON (default: false)
  fail_on_error: <bool>       # OPTIONAL (default: false)
  context: <map[string]string># OPTIONAL — JQ exprs to store values in record context
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Extract a single field | `path: .field_name` |
| Reshape the object | `path: '{ "new_key": .old_key }'` |
| Array → individual records | add `explode: true`, ensure path returns array |
| Filter array elements | `path: '.items[] \| select(.active == true)'` with `explode: true` |
| Need value in a downstream task | add `context: { key: ".jq_expr" }` |
| Emit plain string not JSON | add `as_raw: true` |
| Translate text via AWS | use `translate(.field; "en"; "es")` in path |
| Input arrives as JSON string | prefix with `fromjson \|` e.g. `path: '. \| fromjson \| .field'` |
| Need to build HTTP request config | reshape to `{ "endpoint": ..., "method": ..., "body": ... }` |
| Nested JSON in output records (file/Kafka) | build objects/arrays in jq **without** `tojson` on those branches |
| HTTP POST body must be a JSON string | use `"body": (.payload \| tojson)` or similar **only** for that string field |
| Consumer expects NDJSON | default pipeline + file sink is fine; use `.jsonl` or document format |
| Consumer expects a single JSON array | avoid per-record file writes; emit one jq result that is `[...]` (no `explode`) |

## JQ Quick Reference

| Goal | Expression |
|------|-----------|
| Extract field | `.field` |
| Nested field | `.a.b.c` |
| Iterate array | `.items[]` |
| Filter | `select(.status == "active")` |
| Build object | `{ "k": .v, "k2": .v2 }` |
| Merge objects | `. + { "extra": .x }` |
| Map over array | `map(. + { "id": .key })` |
| Transform object entries | `with_entries` (see Mirakl Mediamarkt `account_health` DAG) |
| Reusable logic | `def name: …; …` |
| Repeat N outputs | `range(1; 4)` then build an object per index (often with `explode: true`) |
| Concat strings | `(.a + " " + .b)` |
| Interpolate in string | `"prefix\\(.id)/suffix"` |
| Number → string | `tostring` |
| String → number | `tonumber` |
| Decode JSON string | `fromjson` |
| Encode to JSON string | `tojson` |
| Safe parse | `try fromjson catch null` |
| URL-encode | `@uri` |
| Base64 encode / decode | `@base64` / `@base64d` |
| Regex replace / cleanup | `gsub("\n"; " ")`, `test("pattern"; "i")` — edge trim: one `gsub` with `\s` alternation (see SP-API / browse-node DAG jq) |
| Array length | `length` |
| Object keys | `keys` |
| Conditional | `if .x then .y else .z end` |
| Default value | `.field // "default"` |
| Bind variable | `expr as $x` then continue the pipeline |

Chain steps with jq’s pipe: `.items[] | select(.ok) | {id}`.

## Custom functions (Caterpillar extensions)

These are registered when the jq task compiles your `path` (see `customFunctionsOptions` in `internal/pkg/jq/jq.go`). They are **not** standard jq.

### Cryptographic hashes (hex digest)

Unary filters: pipe a **string** in; output is lowercase hex.

| Function | Digest |
|----------|--------|
| `md5` | MD5 |
| `sha256` | SHA-256 |
| `sha512` | SHA-512 |

Example (Walmart-style signing string):  
`( $consumerId + "\n" + $path + "\n" + ($method | ascii_upcase) + "\n" + $timestamp + "\n" ) | sha256 as $stringToSign`

### HMAC (hex)

```
hmac_md5(data; key)
hmac_sha256(data; key)
hmac_sha512(data; key)
# Optional third argument: prefix bytes as a string, passed to HMAC sum
hmac_sha256(data; key; pref)
```

`data` and `key` are strings; output is hex.

### RSA PKCS#1 v1.5 sign (base64 signature)

```
rsa_sha256(data; private_key_pem_or_der_string)
rsa_sha512(data; private_key_pem_or_der_string)
```

**Important:** `data` must be a **hex-encoded** digest (the implementation decodes it with `hex.DecodeString` before signing). `private_key` is PEM text or raw DER bytes as a string. Supports PKCS#1 and PKCS#8 RSA keys.

### `uuid`

Generates a new random UUID string (v4 via `google/uuid`). Used in headers/objects as a bare call, e.g. `"WM_QOS.CORRELATION_ID": uuid` in a jq object literal.

### `shuffle`

Shuffles an **array**; input must be an array or jq errors.

Example: `.data | split("\n") | shuffle | .[:10]`

### `sleep`

```
input | sleep("duration")
```

`duration` is a Go `time.ParseDuration` string (`"500ms"`, `"30s"`, `"1m"`, etc.). Sleeps, logs to stdout, then returns **the original input** unchanged. Used in pipelines such as throttling `next_page` expressions (e.g. Keepa token refresh).

### `translate` — AWS Translate

```
translate(text; source_lang; target_lang)
```

Requires AWS credentials and the Translate API. Language codes: `"en"`, `"es"`, `"fr"`, `"de"`, `"ja"`, etc.

## How `path` Receives Data

The `path` expression runs directly against the **raw record body** (the upstream task's output bytes, parsed as JSON). There is no `.data` wrapper at the `path` level.

- **`path`** → operates on raw JSON body. If the HTTP source returns `{"users": [...]}`, use `path: .users` — NOT `.data | fromjson | .users`.
- **`context`** → operates on the **record envelope** `{"data": "<json-string>", "metadata": {...}}`. Context expressions must use `.data | fromjson | .field` to access the body.

**Rule of thumb:** Never use `.data | fromjson` in the `path` field. If you see yourself writing that, you are confusing `path` with `context` expression syntax.

## Validation Rules

- `path` is required
- `path` must NOT start with `.data | fromjson` — that pattern is only valid inside `context` expressions, not in `path`
- `explode: true` requires the JQ expression to return an array — flag if expression won't produce an array
- Multiline JQ uses YAML block scalar `|` — indentation must be consistent
- `{{ context "key" }}` interpolation inside `path` is evaluated before JQ runs — use for dynamic expressions
- `as_raw: true` outputs value without JSON encoding — use only for plain string outputs

## Examples

### Extract single field
```yaml
- name: get_id
  type: jq
  path: .user.id
```

### Reshape record
```yaml
- name: normalize
  type: jq
  path: |
    {
      "id": .user.id,
      "name": (.user.first + " " + .user.last),
      "active": (.status == "active"),
      "created": .timestamps.created_at
    }
```

### Nested objects for file/Kafka (do not use `tojson` on structure)

Wrong — `meta` becomes a JSON **string** (double-encoded after Go marshals the record):

```yaml
- name: bad_nested
  type: jq
  path: '{ "id": .id, "meta": (.details | tojson) }'
```

Right — `meta` stays a nested object:

```yaml
- name: good_nested
  type: jq
  path: '{ "id": .id, "meta": .details }'
```

### Explode array into records
```yaml
- name: expand_items
  type: jq
  path: .items[]
  explode: true
```

### Filter and explode
```yaml
- name: active_users
  type: jq
  path: |
    .users[] | select(.status == "active") | {
      "id": .id,
      "email": .email
    }
  explode: true
```

### Store values in context for downstream tasks
```yaml
- name: extract_ids
  type: jq
  path: .
  context:
    user_id: .user.id
    org_slug: .organization.slug
```

### Build HTTP request config (for http sink)
```yaml
- name: build_request
  type: jq
  path: |
    {
      "method": "POST",
      "endpoint": "https://api.example.com/users/{{ context "user_id" }}",
      "body": (. | tojson),
      "headers": { "Content-Type": "application/json" }
    }
```

### Decode JSON string from upstream
Use `fromjson` ONLY when the upstream record is a JSON-encoded string (e.g., SQS message body where the payload is double-encoded). Do NOT use it when upstream is an HTTP or file source — those already deliver parsed JSON.
```yaml
# Correct: upstream sends a literal string like '"{\"id\":1}"' (double-encoded)
- name: parse_payload
  type: jq
  path: . | fromjson | .id

# WRONG: upstream is HTTP/file source — body is already JSON, no fromjson needed
# - name: parse_payload
#   type: jq
#   path: .data | fromjson | .id   # ← .data does not exist, evaluates to null
```

### Translate field
```yaml
- name: translate_desc
  type: jq
  path: |
    {
      "id": .id,
      "description_en": .description,
      "description_es": translate(.description; "en"; "es")
    }
```

## Anti-patterns

- **Using `.data | fromjson` in `path`** — `path` already receives raw JSON. `.data | fromjson` is only for `context` expressions. Using it in `path` evaluates to `null` and silently drops the record.
- **`tojson` on every nested blob** for file/Kafka sinks — creates **string** fields containing escaped JSON; downstream “invalid” or unexpected shape. Reserve `tojson` for string APIs (bodies, cookies).
- **Renaming output `.json` while content is NDJSON** — valid per line, invalid as one document; rename to `.jsonl` or change pipeline shape.
- Forgetting `fromjson` when upstream task outputs a JSON string (not object)
- Using `explode: true` without `[]` or array-producing expression → runtime error
- `{{ context "key" }}` inside a pure JQ array/object — it's string interpolation, not JQ — wrap in quotes
- Inconsistent YAML block scalar indentation for multiline `path`

## Patterns from `yaml_with_jq_tasks/` (production DAGs)

These pipelines (under `yaml_with_jq_tasks/`) repeat the same jq shapes. Use them as templates.

### Shape HTTP `http` task input

Emit an object the HTTP task understands: `endpoint`, optional `method`, `headers`, optional `body`.

- **GET:** multiline `path: |` building `{ endpoint: "https://…" + $query, headers: { … } }` (often with `@uri` on query parts).
- **POST JSON as a string field:** `"body": (.payload | tojson)` when the client expects a JSON **string** (common for scraper-central style APIs).
- **POST `application/x-www-form-urlencoded`:** `body` is a **plain string**, e.g. `"grant_type=client_credentials"` or space-delimited scopes — not a JSON object.
- **Bearer / Basic in headers:** `"Authorization": "Bearer \\(.access_token)"` or `"Basic \\(.basic_auth)"`.

### OAuth and Basic auth helpers

- **Basic header from id/secret:** merge into the record: `. + {basic_auth: ((.clientId + ":" + .clientSecret) | @base64)}`, then reference `Authorization: "Basic \\(.basic_auth)"`.
- **Decode embedded secret (e.g. Walmart private key):** `("\\(.clientSecret)" | @base64d) as $privateKey` then use `$privateKey` in the rest of the expression.

### After an `http` response: `context` + pass-through

The response body is often a JSON string inside the record envelope. Downstream jq **`path`** still sees parsed JSON from the prior task; for **`context`**, use the envelope:

```yaml
- name: extract_access_token
  type: jq
  path: "."
  context:
    access_token: ".data | fromjson | .access_token"
```

Use the same pattern for tokens, cursor pagination (`next_cursor`), multi-field creds (`vendor_id`, `secret_key`), and SQL-sourced rows (`merchant_id`, `asin`, etc.). Quote context values in YAML when the expression contains `:` or starts with `.` in ambiguous positions.

### `{{ context "key" }}` inside `path`

Caterpillar substitutes `{{ context "…" }}` **before** jq runs. Typical uses:

- URLs: `"https://api…/credentials/{{ context \"account_id\" }}/access"`.
- HTTP headers: `"x-amz-access-token": "{{ context \"access_token\" }}"`.
- Merging prior results into each row: `map(. + {account_id: "{{ context \"account_id\" }}"})`.
- Rehydrating interpolated JSON blobs: `({{ context "orders_data" }} | if type == "array" then . else [.] end) as $orders` then `map(. + {{ context "order_addresses" }})` (see Target orders-style merges in-repo).

Keep interpolated fragments valid jq after substitution (arrays/objects must still be legal jq literals).

### `explode: true` recipes

- **Array of objects:** `path: .items` or `path: .` when the parsed body is already an array (NetSuite-style).
- **Top-level array:** `path: ".[]"` (Bol inventory-style).
- **Nested array:** e.g. `path: ".positionItems[]"` (Otto returns-style).
- **Filter then one object per match:** `.[] | select(.destination_id == N) | {endpoint: "…\(.destination_key)…"}` on one line in YAML (Bol/Otto creds pattern).
- **Cartesian / pages:** `range(1;3) as $page | {endpoint: "…\($page)"}` inside a multiline `path` (Amazon SERP-style).
- **Repeat per scalar in an array:** `.locations[].key | {endpoint: "…\(.)/access"}` (Walmart items-style).

`explode: true` requires the jq program to produce **multiple outputs** or a single **array** (per caterpillar rules). Prefer `[]`, `range`, or an explicit array when in doubt.

### Normalizing “wrapped” tabular cells

When every value is `[ "scalar" ]`, unwrap with `with_entries(.value |= (if type=="array" and length>0 and (.[0]|type)=="string" then .[0] else null end))` or small `def` helpers that branch on `type` / `has("tag")`.

### Defensive `fromjson` (mixed string/object rows)

When one pipeline accepts both stringified and object bodies:

```text
(if .data then .data else . end | if type == "string" then fromjson else . end) as $row
```

For optional parse: `def parse_body: if type == "string" then (try fromjson catch null) elif type == "object" then . else null end;`.

### `tojson` on selected branches (warehouse / wide rows)

Some SP-API style extractions map each item to **string columns** that store nested JSON (`competitive_pricing: (.Product.CompetitivePricing | tojson)`). That is intentional when the sink expects JSON-in-string columns — different from “whole nested object for a JSON record” sinks.

### Binary / CSV payload as file bytes

Decode base64 record fields and skip JSON wrapping on the wire:

```yaml
path: ".[].data | @base64d"
as_raw: true
```

### Strict pipelines

Add `fail_on_error: true` on jq when bad transforms should stop the run (e.g. Okta user splitting).

### Jinja inside `path`

DAGs sometimes wrap `{{ context "…" }}` in `{% raw %}…{% endraw %}` so Jinja does not eat braces. When authoring by hand, prefer caterpillar’s `{{ context }}` unless you are inside a Jinja-templated YAML file.

### Legacy / edge reminders

- **SQS / wrapped bodies:** `.Message | fromjson` when `Message` is a JSON string.
- **Session cookies:** single field containing JSON text → `.cookie_string | fromjson` in **`path`** only if that field is the whole body shape you receive.
