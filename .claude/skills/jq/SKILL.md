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

## JQ Quick Reference

| Goal | Expression |
|------|-----------|
| Extract field | `.field` |
| Nested field | `.a.b.c` |
| Iterate array | `.items[]` |
| Filter | `select(.status == "active")` |
| Build object | `{ "k": .v, "k2": .v2 }` |
| Concat strings | `(.a + " " + .b)` |
| Number → string | `(.n \| tostring)` |
| String → number | `(.s \| tonumber)` |
| Decode JSON string | `fromjson` |
| Encode to JSON string | `tojson` |
| Array length | `length` |
| Object keys | `keys` |
| Conditional | `if .x then .y else .z end` |
| Default value | `.field // "default"` |

## Custom Functions

### translate — AWS Translate
```
translate(text; source_lang; target_lang)
```
Requires AWS credentials. Language codes: `"en"`, `"es"`, `"fr"`, `"de"`, `"ja"`, etc.

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
- Forgetting `fromjson` when upstream task outputs a JSON string (not object)
- Using `explode: true` without `[]` or array-producing expression → runtime error
- `{{ context "key" }}` inside a pure JQ array/object — it's string interpolation, not JQ — wrap in quotes
- Inconsistent YAML block scalar indentation for multiline `path`
