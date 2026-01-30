# Archive Task

The `archive` task pack and unpack file data in various archive formats (TAR, ZIP), enabling efficient data packaging and extraction within pipelines.

## Function

The archive task handles two primary operations:
- **Pack**: Creates archives from input data (e.g., create a ZIP or TAR file)
- **Unpack**: Extract archives to retrieve individual files

## Behavior

The archive task operates in two modes depending on the specified action:

- **Pack mode** (`action: pack`): Takes input data records and creates an archive file. Each record's data is packaged into the specified archive format with the configured filename. The task outputs the complete archive data.

- **Unpack mode** (`action: unpack`): Takes archive file data as input and extracts individual files. For each file found in the archive, the task outputs a separate record containing that file's data.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `archive` | Must be "archive" |
| `format` | string | `zip` | Archive format: `zip` or `tar` |
| `action` | string | `pack` | Operation to perform: `pack` or `unpack` |
| `file_name` | string | - | Name of the file within the archive (required for `pack` action) |

### File Name Format

The `file_name` field specifies how files are stored within archives. Different formats have specific requirements:

#### ZIP Archives
- **Paths**: Filenames can include directory paths, represented using forward slashes (/)
  - Example: `docs/readme.txt`
- **Separators**: Only forward slashes (/) are allowed as folder separators, regardless of platform
- **Relative Paths**: Filenames must be relative (no drive letters like C: and no leading slash /)
- **Case Sensitivity**: ZIP stores filenames as is, but whether they are case-sensitive depends on the extraction platform
- **Directories**: End directory names with a trailing slash (/) to indicate a folder
- **Duplicates**: Duplicate names are allowed, but may cause confusion for some zip tools
- **Allowed Characters**: Supports Unicode, but stick to common, portable characters for best compatibility

#### TAR Archives
- **Paths**: Filenames can include paths separated by forward slashes (/)
  - Example: `src/main.c`
- **Relative and Absolute Paths**: Both relative (foo.txt) and absolute paths (/foo.txt) can technically be stored, but using relative paths is strongly recommended for portability and to avoid extraction issues
- **Case Sensitivity**: Tar files store names as is; case sensitivity depends on the underlying filesystem
- **Long Paths**: Traditional tar limits path length to 100 bytes for the filename, but modern tar formats (ustar, pax) allow longer names
- **Directories**: Represented as entries ending in a slash (/)
- **Duplicates**: Duplicate filenames are possible; later entries usually overwrite earlier ones on extraction
- **Allowed Characters**: Generally supports any characters, but best practice is to stick to ASCII (letters, digits, underscores, dashes, periods, slashes) for maximum compatibility

## Supported Formats

### ZIP
- **Extension**: `.zip`
- **Use case**: Cross-platform, widely supported compression format
- **Features**: Individual file pack, preserves file structure
- **Pack**: Creates a ZIP archive with single or multiple files
- **Unpack**: Extracts all regular files from ZIP archive

### TAR
- **Extension**: `.tar` or `.tar.gz`
- **Use case**: Unix/Linux native format, streaming support
- **Features**: Preserves file metadata, supports packing (with gzip)
- **Pack**: Creates a TAR archive with file metadata
- **Unpack**: Extracts all regular files from TAR archive (including gzip-compressed)

## Example Configurations

### Pack a file into ZIP
```yaml
tasks:
  - name: create_zip
    type: archive
    format: zip
    action: pack
    file_name: output.txt
```

### Unpack ZIP archive
```yaml
tasks:
  - name: unpack_zip
    type: archive
    format: zip
    action: unpack
```

### Pack a file into TAR
```yaml
tasks:
  - name: create_tar
    type: archive
    format: tar
    action: pack
    file_name: data.txt
```

### Unpack TAR.GZ archive
```yaml
tasks:
  - name: unpack_tar_gz
    type: archive
    format: tar
    action: unpack
```

## Complete Pipeline Examples

### Read files, pack to ZIP, write to file
```yaml
tasks:
  - name: read_source
    type: file
    path: source/*.txt
  
  - name: pack_to_zip
    type: archive
    format: zip
    action: pack
    file_name: archive.txt
  
  - name: write_archive
    type: file
    path: output/archive.zip
```

### Extract TAR.GZ and write individual files
```yaml
tasks:
  - name: read_archive
    type: file
    path: data.tar.gz

  - name: decompress_file
    type: compress
    format: gzip
    action: decompress 
  
  - name: unpack_files
    type: archive
    format: tar
    action: unpack
  
  - name: write_unpacked
    type: file
    path: /output/data.txt
```

### Multi-step packing pipeline
```yaml
tasks:
  - name: read_data
    type: file
    path: test/pipelines/birds.txt
  
  - name: pack_zip
    type: archive
    format: zip
    action: pack
    file_name: birds.zip
  
  - name: unpack_zip
    type: archive
    format: zip
    action: unpack
  
  - name: write_result
    type: file
    path: unpacked_birds/birds.txt
```

## Data Flow

### Pack Operation
```
Input Records
    ↓
[Record Data] → Archive Creation → [Archive Bytes] → Output
```

### Unpack Operation
```
Input Records
    ↓
[Archive Bytes] → File Extraction → [File 1], [File 2], ... → Output
```

## Use Cases

- **Data packaging**: Bundle multiple files into a single archive
- **Data extraction**: Process archived data within pipelines
- **Archive conversion**: Convert between ZIP and TAR formats
- **Backup workflows**: Create and manage compressed backups
- **Data distribution**: Package files for downstream consumption

## Error Handling

- **Missing file_name**: Throws error if `file_name` is not specified for `pack` action
- **Invalid format**: Throws error if format is not `zip` or `tar`
- **Invalid action**: Throws error if action is not `pack` or `unpack`
- **Corrupt archive**: May throw error when unpacking malformed archives
- **Empty data**: Skips processing of empty records

## Technical Details

### ZIP Format
- Uses Go's `archive/zip` package
- Supports standard ZIP compression
- Preserves file metadata (size, modification time)
- Regular files only (directories not included in unpacking)

### TAR Format
- Uses Go's `archive/tar` package
- Supports raw TAR and gzip-compressed TAR files
- Automatic format detection for gzip compression
- Preserves tar header information
- Regular files only (directories and special files filtered)

## Performance Considerations

- **Memory usage**: Entire archive loaded into memory for processing
- **Compression ratio**: ZIP typically provides better compression than TAR alone
- **Processing speed**: TAR is generally faster than ZIP due to simpler format
- **Large files**: For very large archives, consider chunking or streaming approaches

## Sample Pipelines

- `test/pipelines/zip_pack_test.yaml` - Create ZIP archives
- `test/pipelines/zip_unpack_test.yaml` - Extract from ZIP archives
- `test/pipelines/tar_unpack_multifile_test.yaml` - Extract multiple files from TAR archive

## Security Considerations

- Archives are processed in-memory; ensure sufficient memory for large files
- ZIP bomb protection: Be cautious with untrusted archive sources
- Path traversal: Archive extraction validates file paths to prevent escaping base directory
- File permissions: TAR format supports Unix permissions; ZIP has limited permission support
