# Converter Task

The `converter` task converts data between different formats, supporting CSV, HTML, and other data format transformations.

## Function

The converter task transforms data from one format to another, enabling interoperability between different data formats and systems.

## Behavior

The converter task transforms data between different formats. It receives records from its input channel, converts the data from the source format to the target format using the specified options, and sends the converted records to its output channel.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `converter` | Must be "converter" |
| `format` | string | - | Format to convert to (csv, html, sst) |
| `delimiter` | string| \t | Used only in sst converter for spliting key and value| 

### CSV Format Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `skip_first` | bool | `false` | Skip the first record (useful for headers) |
| `take_column_names_from_first_row` | bool | `false` | Name columns using values in first row of CSV.  If setting to true, then skip_first is not needed.
| `columns` | array | - | Array of column definitions |
| `columns[].name` | string | - | Name for the column |
| `columns[].is_numeric` | bool | `false` | Whether the column contains numeric data |

### HTML Format Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `container` | string | - | XPath expression to select specific container elements |

### SST Format Options
Convert a single line to the SSTable which could be stored on s3 or via file. It expects a single line as input

## Supported Formats

The converter supports two formats:
- **CSV**: Converts CSV data to JSON with column mapping and type conversion
- **HTML**: Converts HTML to JSON representation with element structure

## Example Configurations

### CSV to JSON conversion:
```yaml
tasks:
  - name: csv_to_json
    type: converter
    format: csv
    skip_first: true
    columns:
      - name: id
        is_numeric: true
      - name: name
      - name: email
      - name: age
        is_numeric: true
```
or
```yaml
tasks:
  - name: csv_to_json_use_column_headers
    type: converter
    format: csv
    skip_first: true
    take_column_names_from_first_row: true
```

### HTML to JSON conversion:
```yaml
tasks:
  - name: html_to_json
    type: converter
    format: html
    container: "//div[@class='content']"
```

## Sample Pipelines

- `test/pipelines/convert_file.yaml` - File format conversion
- `test/pipelines/convert_industries.yaml` - Data format transformation
- `test/pipelines/html2json.yaml` - HTML to JSON conversion

## Use Cases

- **Data format conversion**: Convert between different data formats
- **API integration**: Transform data for different API requirements
- **Database migration**: Convert data for different database systems
- **Report generation**: Convert data to report-friendly formats
- **Data exchange**: Enable data sharing between different systems
- **ETL workflows**: Transform data as part of extract, transform, load processes