# Converter Task

The `converter` task converts data between different formats, supporting CSV, HTML, XLSX (Excel), and other data format transformations.

## Function

The converter task transforms data from one format to another, enabling interoperability between different data formats and systems.

## Behavior

The converter task transforms data between different formats. It receives records from its input channel, converts the data from the source format to the target format using the specified options, and sends the converted records to its output channel.

- If skip_first is True and no columns are provided, then the column names in the output will be the values in the first row of the CSV file.
- If skip_first is True and columns are provided, then the column names in the output will be the values provided (i.e., Provided column names supersede names from first row).
- If skip_first is False, and no columns are provided, then the column names in the output will be named Col1, Col2, Col3, etc.
- If skip_first is False, and columns are provided, then the column names in the output will be the values provided.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `converter` | Must be "converter" |
| `format` | string | - | Format to convert to (csv, html, sst, xlsx) |
| `delimiter` | string| \t | Used only in sst converter for spliting key and value| 

### CSV Format Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `skip_first` | bool | `false` | Skip the first record (useful for headers) |
| `columns` | array | - | Array of column definitions |
| `columns[].name` | string | - | Name for the column |
| `columns[].is_numeric` | bool | `false` | Whether the column contains numeric data |

### HTML Format Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `container` | string | - | XPath expression to select specific container elements |

### SST Format Options
Convert a single line to the SSTable which could be stored on s3 or via file. It expects a single line as input

### XLSX Format Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `sheets` | array | all sheets | Optional array of sheet names to process. If not specified, all sheets are processed |

**Important:** The XLSX converter emits **one record per sheet**. Each record contains the sheet's data in CSV format, with the sheet name available in the record context under the key `xlsx_sheet_name`.

## Supported Formats

The converter supports the following formats:
- **CSV**: Converts CSV data to JSON with column mapping and type conversion
- **HTML**: Converts HTML to JSON representation with element structure
- **XLSX**: Converts Excel files to CSV format. **Note:** Each sheet in the Excel file is emitted as a separate record with the sheet name stored in the context (key: `xlsx_sheet_name`)

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

### HTML to JSON conversion:
```yaml
tasks:
  - name: html_to_json
    type: converter
    format: html
    container: "//div[@class='content']"
```

### Excel to CSV conversion (all sheets):
```yaml
tasks:
  - name: read_excel
    type: file
    path: data.xlsx
  - name: convert_excel
    type: converter
    format: xlsx
  - name: echo
    type: echo
    only_data: true
```

### Excel to CSV conversion (specific sheets):
```yaml
tasks:
  - name: read_excel
    type: file
    path: report.xlsx
  - name: convert_excel
    type: converter
    format: xlsx
    sheets: ["Sales", "Inventory"]
  - name: echo_sheet_name
    type: echo
    # Each record will have xlsx_sheet_name in context
```

## Sample Pipelines

- `test/pipelines/convert_file.yaml` - File format conversion
- `test/pipelines/convert_industries.yaml` - Data format transformation
- `test/pipelines/converter/convert_xls.yaml` - Excel to CSV conversion
- `test/pipelines/html2json.yaml` - HTML to JSON conversion

## Use Cases

- **Data format conversion**: Convert between different data formats
- **Excel processing**: Extract and process data from Excel spreadsheets, with separate handling for each sheet
- **API integration**: Transform data for different API requirements
- **Database migration**: Convert data for different database systems
- **Report generation**: Convert data to report-friendly formats
- **Data exchange**: Enable data sharing between different systems
- **ETL workflows**: Transform data as part of extract, transform, load processes
- **Multi-sheet Excel analysis**: Process each sheet of an Excel workbook independently