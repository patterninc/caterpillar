---
skill: replace
version: 1.0.0
caterpillar_type: replace
description: Apply a Go RE2 regex find-and-replace to each record's data string.
role: transform
requires_upstream: true
requires_downstream: true
aws_required: false
---

## Purpose

Applies a regular expression to the entire record data string and replaces matches.
Operates on raw string data — not JSON fields. Use a `jq` task upstream to extract a specific field first.

## Schema

```yaml
- name: <string>          # REQUIRED
  type: replace           # REQUIRED
  expression: <string>    # REQUIRED — Go RE2 regex pattern
  replacement: <string>   # REQUIRED — replacement string ($1, $2 for capture groups)
  fail_on_error: <bool>   # OPTIONAL (default: false)
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Clean whitespace | `expression: "\\s+"`, `replacement: " "` |
| Remove characters | `replacement: ""` |
| Capture and reorder groups | `expression: "(a)(b)"`, `replacement: "$2$1"` |
| Add prefix/suffix | `expression: "^(.*)$"`, `replacement: "PREFIX: $1"` |
| Extract pattern from text | `expression: ".*(<pattern>).*"`, `replacement: "$1"` |
| Operate on a specific JSON field | add `jq` task upstream to extract the field first |
| Need lookahead/lookbehind | **not supported** (RE2) — restructure logic |

## Capture Group Reference

Go regex uses `$N` for group references (not `\N`):
```
$0   → entire match
$1   → first capture group
$2   → second capture group
```

## YAML Escaping

Backslashes must be doubled inside YAML quoted strings:

| Regex intent | YAML value |
|-------------|------------|
| `\d` | `"\\d"` |
| `\s` | `"\\s"` |
| `\w` | `"\\w"` |
| `\.` | `"\\."` |
| `\n` | `"\\n"` |
| `\t` | `"\\t"` |
| `\\` | `"\\\\"` |

## Validation Rules

- Both `expression` and `replacement` are required
- Go uses RE2 syntax — no lookaheads `(?=...)` or lookbehinds `(?<=...)`
- `expression` applies to the entire record data string, not a single JSON field
- Capture group references use `$1` not `\1`
- Backslashes must be doubled in YAML string values

## RE2 Quick Reference

| Pattern | Matches |
|---------|---------|
| `.` | any character except `\n` |
| `\d` | digit |
| `\w` | word char `[a-zA-Z0-9_]` |
| `\s` | whitespace |
| `^` | start of string |
| `$` | end of string |
| `*` | 0 or more |
| `+` | 1 or more |
| `?` | 0 or 1 |
| `[abc]` | character class |
| `[^abc]` | negated class |
| `(a\|b)` | alternation |
| `(...)` | capture group |
| `(?:...)` | non-capture group |

## Examples

### Normalize whitespace
```yaml
- name: clean_spaces
  type: replace
  expression: "\\s+"
  replacement: " "
```

### Add greeting prefix
```yaml
- name: greet
  type: replace
  expression: "^(.*)$"
  replacement: "Hello, $1!"
```

### Reformat date YYYY-MM-DD → MM/DD/YYYY
```yaml
- name: reformat_date
  type: replace
  expression: "(\\d{4})-(\\d{2})-(\\d{2})"
  replacement: "$2/$3/$1"
```

### Format phone number
```yaml
- name: format_phone
  type: replace
  expression: "(\\d{3})(\\d{3})(\\d{4})"
  replacement: "($1) $2-$3"
```

### Strip HTML tags
```yaml
- name: strip_html
  type: replace
  expression: "<[^>]*>"
  replacement: ""
```

### Remove non-alphanumeric characters
```yaml
- name: alphanumeric_only
  type: replace
  expression: "[^a-zA-Z0-9\\s]"
  replacement: ""
```

### Extract domain from URL
```yaml
- name: extract_domain
  type: replace
  expression: "https?://([^/]+).*"
  replacement: "$1"
```

### Extract email from text
```yaml
- name: extract_email
  type: replace
  expression: ".*([a-zA-Z0-9._%+\\-]+@[a-zA-Z0-9.\\-]+\\.[a-zA-Z]{2,}).*"
  replacement: "$1"
```

## Anti-patterns

- Using `\1` for capture groups instead of `$1` — Go uses `$` notation
- Single backslash in YAML: `\d` — must be `"\\d"`
- Using lookaheads `(?=...)` — not supported in RE2; restructure with capture groups
- Applying `replace` to a JSON object without first extracting the target field with `jq`
- Using `replace` when a `jq` transform would be cleaner for structured data
