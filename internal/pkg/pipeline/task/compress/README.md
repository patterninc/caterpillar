# Compress Task

The `compress` task compresses or decompresses data using various compression algorithms, enabling efficient data storage and transmission.

## Function

The compress task can operate in two modes:
- **Compress mode**: Compresses data using specified compression algorithms
- **Decompress mode**: Decompresses previously compressed data

## Behavior

The compress task processes data based on the `operation` field:
- **Compress**: Converts data to compressed format using the specified algorithm
- **Decompress**: Converts compressed data back to its original format

The task receives records from its input channel, applies compression/decompression, and sends the processed records to its output channel.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `compress` | Must be "compress" |
| `format` | string | - | Compression format (gzip, snappy, etc.) |
| `operation` | string | - | Operation type (compress or decompress) |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## Supported Formats

The task supports various compression formats:
- **gzip**: Standard gzip compression
- **snappy**: Fast compression/decompression
- **zlib**: Standard zlib compression
- **deflate**: Deflate compression algorithm

## Example Configurations

### Compress data with gzip:
```yaml
tasks:
  - name: compress_data
    type: compress
    format: gzip
    operation: compress
```

### Decompress gzip data:
```yaml
tasks:
  - name: decompress_data
    type: compress
    format: gzip
    operation: decompress
```

### Compress with snappy:
```yaml
tasks:
  - name: fast_compress
    type: compress
    format: snappy
    operation: compress
```

## Sample Pipelines

- `test/pipelines/compress_test.yaml` - Compression example
- `test/pipelines/decompress_test.yaml` - Decompression example

## Use Cases

- **Data storage**: Compress data before storing to reduce storage requirements
- **Data transmission**: Compress data for efficient network transmission
- **Backup compression**: Compress backup data for storage efficiency
- **Log compression**: Compress log files for archival
- **Performance optimization**: Reduce I/O overhead with compressed data
- **Bandwidth optimization**: Reduce network bandwidth usage