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
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

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
        "endpoint": "https://api.example.com/users/{{ context 'user_id' }}"
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

In addition to standard JQ functions, Caterpillar provides custom functions to extend JQ capabilities:

### Error handling

Custom functions report bad input as a JQ error. With `fail_on_error: false` (the default) the
offending record is logged and skipped and the task carries on; with `fail_on_error: true` the error
stops the task and fails the pipeline.

Because these are ordinary JQ errors, they can be caught and turned into data. Bind the result of a
single call rather than repeating the call in both branches, since some of these functions are
expensive:

```yaml
tasks:
  - name: hash_or_report
    type: jq
    path: |
      . as $record
      | (try ($record.payload | sha256) catch "error: \(.)") as $outcome
      | {
          "id": $record.id,
          "outcome": $outcome
        }
```

### bcrypt

Hashes the piped value with bcrypt using a salt you supply. Go's standard bcrypt always generates
its own random salt, so this function exists for schemes that derive a value from a *fixed* salt —
for example an API that signs requests with `bcrypt(client_id + "_" + timestamp)` using the client
secret as the salt.

**Signature:** `data | bcrypt(salt)`

**Parameters:**
- `data` (string, piped): The value to hash. Maximum 72 bytes — bcrypt reads no further, so longer input is rejected rather than silently truncated.
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
        "signature": (
          (.client_id + "_" + (.timestamp | tostring))
          | bcrypt("{{ env \"CLIENT_SECRET\" }}")
        )
      }
```

When the salt comes from the record rather than a literal, bind it first. Inside `bcrypt(...)` the
input is the piped string, so a bare `.salt` would be evaluated against that string instead of the
enclosing object:

```yaml
tasks:
  - name: sign_with_per_record_salt
    type: jq
    path: |
      . as $record | {
        "signature": ($record.payload | bcrypt($record.salt))
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
- `test/pipelines/html2json.yaml` - HTML to JSON conversion
- `test/pipelines/translate_test.yaml` - Text translation with JQ
- `test/pipelines/uuid_test.yaml` - UUID generation with JQ

## Use Cases

- **Data transformation**: Convert between different JSON structures
- **Data filtering**: Extract specific fields or filter records
- **Data aggregation**: Calculate summaries and statistics
- **API response processing**: Transform API responses for downstream use
- **Data validation**: Check data structure and content
- **ETL workflows**: Transform data as part of extract, transform, load processes