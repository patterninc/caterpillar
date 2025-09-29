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

## Sample Pipelines

- `test/pipelines/context_test.yaml` - JQ with context variables
- `test/pipelines/convert_industries.yaml` - Data transformation with JQ
- `test/pipelines/html2json.yaml` - HTML to JSON conversion

## Use Cases

- **Data transformation**: Convert between different JSON structures
- **Data filtering**: Extract specific fields or filter records
- **Data aggregation**: Calculate summaries and statistics
- **API response processing**: Transform API responses for downstream use
- **Data validation**: Check data structure and content
- **ETL workflows**: Transform data as part of extract, transform, load processes