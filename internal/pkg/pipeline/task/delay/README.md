# Delay Task

The `delay` task introduces controlled delays between record processing, useful for rate limiting, testing, and managing processing speed.

## Function

The delay task adds a specified delay between processing each record, allowing you to control the pace of data processing through the pipeline.

## Behavior

The delay task adds a specified delay between processing each record. It receives records from its input channel, waits for the configured duration, then sends the records to its output channel. This is useful for rate limiting and controlling processing speed.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `delay` | Must be "delay" |
| `duration` | string | - | Delay duration (e.g., "1s", "100ms", "2m") |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## Duration Format

The delay duration supports various time units:
- **Milliseconds**: `100ms`, `500ms`
- **Seconds**: `1s`, `5s`, `30s`
- **Minutes**: `1m`, `5m`
- **Hours**: `1h`, `2h`

## Example Configurations

### 1 second delay between records:
```yaml
tasks:
  - name: slow_processing
    type: delay
    duration: 1s
```

### 100 millisecond delay:
```yaml
tasks:
  - name: rate_limit
    type: delay
    duration: 100ms
```

### 5 minute delay:
```yaml
tasks:
  - name: long_delay
    type: delay
    duration: 5m
```

## Sample Pipelines

- `test/pipelines/delay_test.yaml` - Delay task examples

## Use Cases

- **Rate limiting**: Control the rate of API calls or external service requests
- **Testing**: Simulate slow processing conditions
- **Resource management**: Prevent overwhelming downstream systems
- **Debugging**: Add delays to observe pipeline behavior
- **Load testing**: Test system behavior under controlled load
- **Batch processing**: Control the pace of batch operations