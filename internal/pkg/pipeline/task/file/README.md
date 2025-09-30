# File Task

The `file` task reads from or writes to files, supporting both local filesystem and Amazon S3 storage.

## Function

The file task operates in two modes depending on whether an input channel is provided:

- **Write mode** (with input channel): Receives records from the input channel and writes them to the specified file
- **Read mode** (no input channel): Reads data from the specified file and sends it to the output channel

This dual functionality makes the file task useful as both a data source and a data sink in your pipeline.

## Behavior

The file task operates in two modes depending on whether an input channel is provided:

- **Write mode** (with input channel): Receives records from the input channel and writes the data to the specified file
- **Read mode** (no input channel): Reads data from the specified file and sends it to the output channel

The task automatically determines its mode based on the presence of input/output channels.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `file` | Must be "file" |
| `path` | string | `/tmp/caterpillar.txt` | File path or S3 URL (s3://bucket/key) supports glob patterns in reading mode
| `region` | string | `us-west-2` | AWS region for S3 operations |
| `delimiter` | string | `\n` | Delimiter used to separate records when reading |
| `success_file` | bool | `false` | Whether to create a success file after writing |
| `success_file_name` | string | `_SUCCESS` | Name of the success file |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## Path Schemes

The task supports different path schemes:
- **Local files**: `file:///path/to/file.txt` or `/path/to/file.txt` or 
- **S3 files**: `s3://bucket-name/path/to/file.txt`

## Example Configurations

### Reading from a local file:
```yaml
tasks:
  - name: read_data
    type: file
    path: /path/to/input.txt
    delimiter: "\n"
```

### Writing to S3:
```yaml
tasks:
  - name: write_to_s3
    type: file
    path: s3://my-bucket/output/data.txt
    region: us-east-1
    success_file: true
```

### Using macros in path:
```yaml
tasks:
  - name: timestamped_output
    type: file
    path: output/data_{{ macro "timestamp" }}.txt
```

## Sample Pipelines

- `test/pipelines/file.yaml` - Basic file operations
- `test/pipelines/context_test.yaml` - File task with context variables

## Use Cases

- **Data ingestion**: Read data from files for processing
- **Data export**: Write processed data to files
- **Backup**: Store data in S3 for backup purposes
- **ETL workflows**: Part of extract, transform, load processes
- **Log processing**: Read log files for analysis

## AWS Requirements

For S3 operations, ensure:
- AWS credentials are configured
- Appropriate IAM permissions for S3 access
- Correct region configuration