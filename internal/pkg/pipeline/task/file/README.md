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

In read mode, the sanitized base filename is stored in the record context under the key `CATERPILLAR_FILE_NAME_WRITE`. The stem is lowercased with non-alphanumeric characters replaced by underscores, while the extension is preserved and lowercased (e.g. `"Report 1.CSV"` → `"report_1.csv"`).

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `file` | Must be "file" |
| `path` | string | `/tmp/caterpillar.txt` | File path or S3 URL (`s3://bucket/key`); glob patterns supported in read mode |
| `region` | string | `us-west-2` | AWS region for S3 operations |
| `storage_class` | string | `STANDARD` | S3 **write** only: on `PutObject`. Ignored for local paths. See [S3 storage class](#s3-storage-class). |
| `tags` | map[string]string | - | S3 **write** only: object tags applied on `PutObject`. Ignored for local paths. Values support macros and context templates. See [S3 object tags](#s3-object-tags). |
| `delimiter` | string | `\n` | Delimiter used to separate records when reading |
| `success_file` | bool | `false` | Whether to create a success file after writing |
| `success_file_name` | string | `_SUCCESS` | Name of the success file |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## S3 storage class

When the write `path` is an S3 URI (`s3://...`), each object is uploaded with the configured `storage_class`. The same class applies to the optional `success_file` marker in that task.

Allowed values are the **PutObject storage class** strings known to the AWS SDK in this build (invalid values fail when the task runs). Typical values include:

| Value | Notes |
|-------|--------|
| `STANDARD` | Default |
| `REDUCED_REDUNDANCY` | RRS |
| `STANDARD_IA` | Infrequent access |
| `ONEZONE_IA` | Single AZ IA |
| `INTELLIGENT_TIERING` | Intelligent-Tiering |
| `GLACIER` | Glacier Flexible Retrieval (instantiation rules apply) |
| `DEEP_ARCHIVE` | Lowest-cost archive |
| `GLACIER_IR` | Glacier Instant Retrieval |
| `EXPRESS_ONEZONE` | S3 Express One Zone |
| `OUTPOSTS` | S3 on Outposts |
| `SNOW` | Snowball / Snow Family edge |
| `FSX_OPENZFS` | FSx for OpenZFS–backed directory buckets |

AWS may add or adjust classes in newer SDK releases; if a value is rejected as unknown, compare with the [S3 PutObject storage class documentation](https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html#AmazonS3-PutObject-request-header-StorageClass) or upgrade the SDK in this project.

Read mode does not set storage class (objects are read as-is).

## S3 object tags

When the write `path` is an S3 URI (`s3://...`), each object is uploaded with the configured `tags` applied as the `x-amz-tagging` header on `PutObject`. The same tags are applied to the optional `success_file` marker.

Tag values are evaluated per record, so macros and context templates (e.g. `{{ macro "timestamp" }}`, `{{ context "user_id" }}`) are resolved against the record being written.

### Limits

S3 enforces the following constraints ([docs](https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-tagging.html)):

- At most **10 tags** per object.
- Tag keys must be unique (enforced by the YAML map).
- Tag **keys** up to **128 UTF-16 code units**.
- Tag **values** up to **256 UTF-16 code units**.

Tag count and key length are validated on the first S3 write; resolved value length is validated per write. In UTF-16, most characters take 1 code unit and supplementary characters (e.g. many emoji) take 2. Validation runs only when actually writing to S3 — local or read-mode runs are not affected by tag configuration.

### `success_file` marker

The `_SUCCESS` marker is not tied to any record, so tag values for the success marker must only use static strings or startup-time templates (`env`, `secret`, `macro`). A tag that references `{{ context "..." }}` will fail at the success-marker write with `context keys were not set: ...`, since there is no record context to resolve against.

If you need record-derived tag values, either drop the context reference from the success-marker tags, or disable `success_file`.

Read mode does not apply tags (objects are read as-is).

### Example

```yaml
tasks:
  - name: write_to_s3_tagged
    type: file
    path: s3://my-bucket/events/{{ macro "timestamp" }}.jsonl
    region: us-east-1
    tags:
      env: prod
      pipeline: events
      user_id: '{{ context "user_id" }}'
```

## Path Schemes

The task supports different path schemes:
- **Local files**: `file:///path/to/file.txt` or `/path/to/file.txt`
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

### Writing to S3 with a non-default storage class:
```yaml
tasks:
  - name: write_to_s3_ia
    type: file
    path: s3://my-bucket/logs/{{ macro "timestamp" }}.jsonl
    region: us-east-1
    storage_class: STANDARD_IA
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