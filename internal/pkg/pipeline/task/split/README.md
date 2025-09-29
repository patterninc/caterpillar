# Split Task

The `split` task splits data by a specified delimiter, creating multiple records from a single input record.

## Function

The split task takes a single record containing data with a delimiter and creates multiple output records, each containing one piece of the split data. This is useful for processing multi-line files, CSV data, or any delimited content.

## Behavior

The split task divides data into multiple records based on a delimiter. It receives records from its input channel, splits the data by the specified delimiter, and sends each piece as a separate record to its output channel. This is useful for processing multi-line files or delimited data.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `split` | Must be "split" |
| `delimiter` | string | `\n` | Character or string used to split the data |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## Example Configurations

### Split by newlines (default):
```yaml
tasks:
  - name: split_lines
    type: split
```

### Split by custom delimiter:
```yaml
tasks:
  - name: split_csv
    type: split
    delimiter: ","
```

### Split by multiple characters:
```yaml
tasks:
  - name: split_sections
    type: split
    delimiter: "---"
```

## Sample Pipelines

- `test/pipelines/hello_name.yaml` - Split task in a data processing pipeline
- `test/pipelines/file.yaml` - File reading with splitting

## Use Cases

- **Multi-line file processing**: Split log files or text files into individual lines
- **CSV processing**: Split CSV data into individual records
- **Data normalization**: Break down large records into smaller, manageable pieces
- **Text processing**: Split text by custom delimiters for further processing
- **ETL workflows**: Prepare data for downstream processing tasks

## Common Delimiters

- `\n` - Newline (default)
- `,` - Comma (for CSV-like data)
- `|` - Pipe (for pipe-delimited files)
- `\t` - Tab (for tab-delimited files)
- `---` - Custom section separators
- `;` - Semicolon (for semicolon-delimited files)