# Echo Task

The `echo` task prints data to the console for debugging and monitoring purposes. It's useful for inspecting data as it flows through the pipeline.

## Function

The echo task receives records from its input channel and prints them to stdout, then optionally forwards them to the output channel.

## Behavior

The echo task prints data to the console for debugging and monitoring. It receives records from its input channel, prints them to stdout (either as full JSON or just the data field based on `only_data`), and optionally forwards them to its output channel.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `echo` | Must be "echo" |
| `only_data` | bool | `false` | If true, prints only the data field. If false, prints the full record as JSON |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## Example Configuration

```yaml
tasks:
  - name: debug_output
    type: echo
    only_data: true
    fail_on_error: false
```

## Sample Pipeline

See `test/pipelines/hello_name.yaml` for an example of the echo task in action.

## Use Cases

- **Debugging**: Inspect data at various points in the pipeline
- **Monitoring**: Log data flow for operational visibility
- **Development**: Verify data transformations during development
- **Testing**: Validate pipeline behavior with sample data