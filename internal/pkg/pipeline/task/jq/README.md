# JQ Task

The `jq` task transforms JSON data using JQ queries, allowing for complex data manipulation, filtering, and transformation.

## Function

The JQ task applies JQ queries to JSON data, enabling powerful data transformation capabilities including filtering, mapping, aggregation, and restructuring of JSON documents.

## Behavior

The JQ task applies JQ queries to transform JSON data. It receives records from its input channel, executes the specified JQ query on the data, and sends the transformed records to its output channel. When `explode: true` is set, array results are split into individual records.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `jq` | Must be "jq" |
| `path` | string | - | JQ query expression to apply |
| `explode` | bool | `false` | If true, splits array results into individual records |
| `as_raw` | bool | `false` | If true, outputs raw values instead of JSON |
| `ignore_error` | bool | `true` | If false, a query error is critical and ends the task; if true, it is non-critical and only the offending record is skipped (see [Query Errors](#query-errors)) |
| `fail_on_error` | bool | `false` | Whether a critical query error makes the run exit non-zero |

## Query Errors

A query can fail on a single record — an explicit `error("...")`, or a runtime type error such as
adding a number to a string. `ignore_error` decides whether that ends the task, and
`fail_on_error` decides the run's verdict, exactly as it does for any other task's error:

| `ignore_error` | `fail_on_error` | task | run |
|---|---|---|---|
| unset (default) | unset (default) | continues, that record dropped | exits 0 |
| unset (default) | `true` | continues, that record dropped | exits 0 |
| `false` | unset | stops at the bad record | exits 0 |
| `false` | `true` | stops at the bad record | exits 1 |

Each skipped record reports `WARN: <task name>: skipping record <id>: <error>`, which under the
defaults is the only signal that data was dropped. An ignored error is never returned from the
task, so `fail_on_error` has nothing to judge — hence the second row — and failing a run on a
query error takes both fields. Tolerating a record while still reporting the run as failed
cannot be expressed.

A query with no output is not an error: a filter that matches nothing (for example
`select(...)` rejecting the input) simply produces no record, with nothing reported.

## JQ Query Examples

### Basic field extraction:
```yaml
tasks:
  - name: extract_id
    type: jq
    path: .id
```

### Complex transformation:
```yaml
tasks:
  - name: transform_data
    type: jq
    path: |
      {
        "user_id": .user.id,
        "full_name": (.user.first_name + " " + .user.last_name),
        "active": .status == "active"
      }
```

### Array processing with explode:
```yaml
tasks:
  - name: process_items
    type: jq
    path: .items[]
    explode: true
```

### Using context variables:
```yaml
tasks:
  - name: dynamic_query
    type: jq
    path: |
      {
        "endpoint": "https://api.example.com/users/{{ context \"user_id\" }}"
      }
```

## Example Configurations

### Simple field extraction:
```yaml
tasks:
  - name: get_user_name
    type: jq
    path: .user.name
```

### Array filtering and transformation:
```yaml
tasks:
  - name: filter_active_users
    type: jq
    path: |
      .users[] | select(.status == "active") | {
        "id": .id,
        "name": .name,
        "email": .email
      }
    explode: true
```

### Aggregation:
```yaml
tasks:
  - name: count_by_status
    type: jq
    path: |
      {
        "total": length,
        "active": map(select(.status == "active")) | length,
        "inactive": map(select(.status == "inactive")) | length
      }
```

## Custom JQ Functions

In addition to standard JQ functions, Caterpillar provides custom functions to extend JQ
capabilities. `bcrypt` and `translate` are described in detail below; the rest are:

| Function | Signature | Description |
|----------|-----------|-------------|
| `md5` | `<string> \| md5` | Hex-encoded MD5 of the piped string |
| `sha256` | `<string> \| sha256` | Hex-encoded SHA-256 of the piped string |
| `sha512` | `<string> \| sha512` | Hex-encoded SHA-512 of the piped string |
| `hmac_md5` | `hmac_md5(data; key)` or `hmac_md5(data; key; prefix)` | Hex-encoded HMAC-MD5 of `data` under `key`; the optional `prefix` bytes are placed ahead of the digest before hex-encoding |
| `hmac_sha256` | `hmac_sha256(data; key)` or `hmac_sha256(data; key; prefix)` | Hex-encoded HMAC-SHA-256, same argument shape |
| `hmac_sha512` | `hmac_sha512(data; key)` or `hmac_sha512(data; key; prefix)` | Hex-encoded HMAC-SHA-512, same argument shape |
| `rsa_sha256` | `rsa_sha256(hex_digest; private_key)` | Base64 RSA PKCS#1 v1.5 signature. `hex_digest` is an already-hashed, hex-encoded SHA-256 digest, not the raw message; `private_key` is PEM text |
| `rsa_sha512` | `rsa_sha512(hex_digest; private_key)` | As above, over SHA-512 |
| `uuid` | `uuid` | A new random UUID; ignores its input |
| `shuffle` | `<array> \| shuffle` | The piped array in random order |
| `sleep` | `sleep("1s")` | Pauses for a duration string, then passes the input through unchanged |

`test/pipelines/hash_test.yaml` and `test/pipelines/uuid_test.yaml` exercise these.

### bcrypt

Hashes the piped value with bcrypt using a salt you supply. Go's standard bcrypt always generates
its own random salt, so this function exists for schemes that derive a value from a *fixed* salt —
for example an API that signs requests with `bcrypt(client_id + "_" + timestamp)` using the client
secret as the salt.

**Signature:** `bcrypt(data; salt)`

**Parameters:**
- `data` (string): The value to hash. Maximum 72 bytes — bcrypt reads no further, so longer input is rejected rather than silently truncated.
- `salt` (string): Either a bare 22-character salt or a full modular-crypt string such as `$2a$04$abcdefghijklmnopqrstuu`. A complete hash is also accepted, in which case its salt is reused.

To select a version or cost, supply the salt in modular-crypt form — `$2b$10$abcdefghijklmnopqrstuu`
uses version `2b` at cost 10. A bare salt defaults to version `2a` at cost 4. Supported versions are
`2`, `2a`, `2b` and `2y`; `2x` is not supported. Cost ranges from 4 to 31.

The salt uses bcrypt's own base64 alphabet (`./A-Za-z0-9`), which orders characters differently from
standard base64, so a salt produced by `@base64` will not round-trip.

**Returns:** A modular-crypt string, `$<version>$<cost>$<salt><checksum>`

**Example:**
```yaml
tasks:
  - name: sign_token_request
    type: jq
    path: |
      {
        "signature": bcrypt(
          .client_id + "_" + (.timestamp | tostring);
          "{{ env \"CLIENT_SECRET\" }}"
        )
      }
```

Both arguments are evaluated against the record, so a salt carried on the record needs no binding:

```yaml
tasks:
  - name: sign_with_per_record_salt
    type: jq
    path: |
      {
        "signature": bcrypt(.payload; .salt)
      }
```

**Note:** The default cost of 4 is the lowest bcrypt permits. It suits reproducing a signature, but
choose a substantially higher cost when hashing anything that needs to resist offline attack.

### translate

Translates text using AWS Translate service.

**Signature:** `translate(text; source_lang; target_lang)`

**Parameters:**
- `text` (string): The text to translate
- `source_lang` (string): Source language code (e.g., "en", "es", "fr")
- `target_lang` (string): Target language code (e.g., "en", "es", "fr")

**Returns:** Translated text as a string

**Example:**
```yaml
tasks:
  - name: translate_greeting
    type: jq
    path: |
      {
        "original": .message,
        "spanish": translate(.message; "en"; "es"),
        "french": translate(.message; "en"; "fr")
      }
```

**Note:** Requires AWS credentials configured in your environment.

## Sample Pipelines

- `test/pipelines/bcrypt_test.yaml` - bcrypt hashing against known-answer vectors
- `test/pipelines/context_test.yaml` - JQ with context variables
- `test/pipelines/convert_industries.yaml` - Data transformation with JQ
- `test/pipelines/hash_test.yaml` - Hashing with JQ
- `test/pipelines/jq_error_not_ignored_test.yaml` - `ignore_error: false`, so the task stops while the run still exits 0
- `test/pipelines/jq_error_skipped_test.yaml` - a query error skipped under the defaults
- `test/pipelines/jq_error_test.yaml` - `ignore_error: false` with `fail_on_error`, failing the run
- `test/pipelines/translate_test.yaml` - Text translation with JQ
- `test/pipelines/uuid_test.yaml` - UUID generation with JQ

## Use Cases

- **Data transformation**: Convert between different JSON structures
- **Data filtering**: Extract specific fields or filter records
- **Data aggregation**: Calculate summaries and statistics
- **API response processing**: Transform API responses for downstream use
- **Data validation**: Check data structure and content
- **ETL workflows**: Transform data as part of extract, transform, load processes