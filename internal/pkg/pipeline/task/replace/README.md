# Replace Task

The `replace` task performs text replacement using regular expressions, allowing for pattern-based data transformation and cleaning.

## Function

The replace task applies regex-based find and replace operations to the data field of each record, enabling text manipulation, data cleaning, and format standardization.

## Behavior

The replace task performs text replacement using regular expressions. It receives records from its input channel, applies the regex pattern to find and replace text, and sends the modified records to its output channel. The task supports capture groups and various regex features.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `replace` | Must be "replace" |
| `expression` | string | - | Regular expression pattern to match |
| `replacement` | string | - | Replacement string (supports capture group references) |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## Regex Features

The replace task supports standard regex features including:
- **Capture groups**: Use `$1`, `$2`, etc. in replacement to reference captured groups
- **Character classes**: `[a-z]`, `[0-9]`, etc.
- **Quantifiers**: `*`, `+`, `?`, `{n}`, `{n,m}`
- **Anchors**: `^`, `$`, `\b`, `\B`
- **Special characters**: `\d`, `\w`, `\s`, etc.

## Example Configurations

### Simple text replacement:
```yaml
tasks:
  - name: clean_data
    type: replace
    expression: "\\s+"
    replacement: " "
```

### Using capture groups:
```yaml
tasks:
  - name: reformat_date
    type: replace
    expression: "(\\d{4})-(\\d{2})-(\\d{2})"
    replacement: "$2/$3/$1"
```

### Remove unwanted characters:
```yaml
tasks:
  - name: remove_special_chars
    type: replace
    expression: "[^a-zA-Z0-9\\s]"
    replacement: ""
```

### Add prefix/suffix:
```yaml
tasks:
  - name: add_prefix
    type: replace
    expression: "^(.*)$"
    replacement: "PREFIX: $1"
```

### Extract and transform:
```yaml
tasks:
  - name: extract_email
    type: replace
    expression: ".*email:\\s*([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}).*"
    replacement: "$1"
```

## Sample Pipelines

- `test/pipelines/hello_name.yaml` - Replace task with greeting format
- `test/pipelines/convert_file.yaml` - Data format conversion

## Use Cases

- **Data cleaning**: Remove unwanted characters, normalize whitespace
- **Format standardization**: Convert dates, phone numbers, or other formats
- **Text extraction**: Extract specific patterns from text
- **Data transformation**: Transform data formats for downstream processing
- **Log processing**: Clean and standardize log entries
- **ETL workflows**: Prepare data for loading into target systems

## Common Patterns

### Date format conversion:
```yaml
expression: "(\\d{4})-(\\d{2})-(\\d{2})"
replacement: "$2/$3/$1"
```

### Phone number formatting:
```yaml
expression: "(\\d{3})(\\d{3})(\\d{4})"
replacement: "($1) $2-$3"
```

### Remove HTML tags:
```yaml
expression: "<[^>]*>"
replacement: ""
```

### Normalize whitespace:
```yaml
expression: "\\s+"
replacement: " "
```

### Extract domain from URL:
```yaml
expression: "https?://([^/]+).*"
replacement: "$1"
```