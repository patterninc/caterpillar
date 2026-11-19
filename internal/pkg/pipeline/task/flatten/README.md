# Flatten Task

The `flatten` task flattens nested JSON structures into a single level, making complex data structures easier to process and analyze.

## Function

The flatten task takes nested JSON objects and converts them into flat key-value pairs, joining nested keys with an underscore. The separator is not configurable.

## Behavior

The flatten task converts nested JSON structures into flat key-value pairs. It receives records from its input channel, flattens the nested JSON using underscore separators (e.g., `user_address_city`), and sends the flattened records to its output channel. Optionally, it can preserve the original nested structure under a specified key.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `flatten` | Must be "flatten" |
| `include_original` | string | - | Key name to include the original nested structure |
| `task_concurrency` | int | `1` | Number of competing-consumer workers for this task |
| `context` | map | - | JQ expressions whose results are stored on each record for downstream tasks |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## Example Configurations

### Basic flattening:
```yaml
tasks:
  - name: flatten_data
    type: flatten
```

### Flattening with original data preserved:
```yaml
tasks:
  - name: flatten_with_original
    type: flatten
    include_original: "original_data"
```

## Use Cases

- **Data normalization**: Convert nested structures to flat format
- **Database storage**: Prepare data for flat database schemas
- **CSV export**: Convert nested JSON to flat CSV format
- **Data analysis**: Simplify complex data structures for analysis
- **API integration**: Flatten data for APIs that expect flat structures
- **ETL workflows**: Transform data structure for target systems