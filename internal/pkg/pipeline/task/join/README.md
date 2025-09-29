# Join Task

The `join` task combines multiple records into a single record, useful for aggregating data or creating batch operations.

## Function

The join task collects multiple input records and combines them into a single output record, enabling data aggregation and batch processing capabilities.

## Behavior

The join task combines multiple records into a single record. It receives records from its input channel, buffers them, and sends joined records to its output channel when any of the configured limits are reached (size, number, or duration). The task flushes immediately when the first limit is satisfied, making it useful for flexible batch processing scenarios.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `join` | Must be "join" |
| `size` | int | - | Maximum total size (in bytes) before flushing joined records |
| `number` | int | - | Maximum number of records before flushing joined records |
| `duration` | string | - | Maximum time duration before flushing joined records |
| `delimiter` | string | `\n` | Delimiter used to separate joined records |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## Example Configurations

### Join all records (no limits configured):
```yaml
tasks:
  - name: join_all
    type: join
    delimiter: "\n"
```

### Join by number of records:
```yaml
tasks:
  - name: join_by_count
    type: join
    number: 100
    delimiter: "\n"
```

### Join by data size:
```yaml
tasks:
  - name: join_by_size
    type: join
    size: 1024
    delimiter: "|"
```

### Join by time duration:
```yaml
tasks:
  - name: join_by_time
    type: join
    duration: "5m"
    delimiter: "\n"
```

### Join with multiple triggers (flushes when first limit is reached):
```yaml
tasks:
  - name: join_flexible
    type: join
    number: 50
    size: 512
    duration: "2m"
    delimiter: "|"
```

## Sample Pipelines

- `test/pipelines/join.yaml` - Join task examples

## Use Cases

- **Data aggregation**: Combine multiple records into a single record
- **Batch processing**: Create batches of data for processing
- **File creation**: Combine multiple lines into a single file
- **Data consolidation**: Merge related data records
- **API batching**: Prepare data for batch API calls
- **Report generation**: Combine data for report creation