# Archive Task

The `archive` task packages or extracts files using various archive formats, enabling efficient file bundling and extraction for data processing pipelines.

## Function

The archive task can operate in two modes:
- **Pack mode**: Combines multiple files into a single archive
- **Unpack mode**: Extracts files from an archive

## Behavior

The archive task processes data based on the `action` field:
- **Pack**: Receives individual files and creates an archive file containing them
- **Unpack**: Receives an archive file and extracts its contents, outputting each file individually

The task receives records from its input channel, applies the archiving operation, and sends the processed records to its output channel.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `archive` | Must be "archive" |
| `format` | string | `zip` | Archive format (zip, tar) |
| `action` | string | `pack` | Action type (pack or unpack) |

## Supported Formats

The task supports the following archive formats:
- **zip**: Standard ZIP format, widely compatible
- **tar**: TAR format, commonly used in Unix/Linux environments

## Example Configurations

### Pack files into a ZIP archive:
```yaml
tasks:
  - name: create_zip
    type: archive
    format: zip
    action: pack
```

### Unpack a ZIP archive:
```yaml
tasks:
  - name: extract_zip
    type: archive
    format: zip
    action: unpack
```

### Pack files into a TAR archive:
```yaml
tasks:
  - name: create_tar
    type: archive
    format: tar
    action: pack
```

### Unpack a TAR archive:
```yaml
tasks:
  - name: extract_tar
    type: archive
    format: tar
    action: unpack
```

## Sample Pipelines

- `test/pipelines/zip_pack_test.yaml` - ZIP packing example
- `test/pipelines/zip_unpack_test.yaml` - ZIP unpacking example
- `test/pipelines/tar_unpack_multifile_test.yaml` - TAR unpacking with multiple files

## Use Cases

- **File bundling**: Package multiple files into a single archive for distribution
- **Data consolidation**: Combine separate data files into archives for storage
- **Archive extraction**: Extract files from archives for processing
- **Backup operations**: Create archives of processed data for backup
- **Format conversion**: Convert between archive formats
- **Multi-file handling**: Process multiple files as a single archive unit
