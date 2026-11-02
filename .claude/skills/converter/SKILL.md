---
skill: converter
version: 1.0.0
caterpillar_type: converter
description: Convert record data between formats — CSV, HTML, XLSX, XLS, EML, or SST.
role: transform
requires_upstream: true
requires_downstream: true
aws_required: false
---

## Purpose

Converts the data field of each incoming record from one format to another.
Output records and shape depend on the target format (see per-format behavior below).

## Schema

```yaml
- name: <string>              # REQUIRED
  type: converter             # REQUIRED
  format: <string>            # REQUIRED — "csv", "html", "xlsx", "xls", "eml", or "sst"
  delimiter: <string>         # OPTIONAL — SST only: key/value separator (default: \t)

  # CSV-specific
  skip_first: <bool>          # OPTIONAL — treat first row as header (default: false)
  columns: <list>             # OPTIONAL — column definitions
    - name: <string>          # column name
      is_numeric: <bool>      # treat as number (default: false)

  # HTML-specific
  container: <string>         # OPTIONAL — XPath to scope extraction

  # XLSX/XLS-specific
  sheets: [<string>, ...]     # OPTIONAL — sheet names to process (default: all)
  skip_rows: <int>            # OPTIONAL — rows to skip on all sheets (default: 0)
  skip_rows_by_sheet:         # OPTIONAL — per-sheet row skip override
    <sheet_name>: <int>

  fail_on_error: <bool>       # OPTIONAL (default: false)
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Input is CSV, first row is headers | `format: csv`, `skip_first: true` |
| Input is CSV, no headers | `format: csv`, `skip_first: false`, provide `columns` |
| Column types matter | set `is_numeric: true` on numeric columns |
| Input is HTML, extract specific section | `format: html`, set `container` XPath |
| Input is `.xlsx` | `format: xlsx` |
| Input is legacy `.xls` | `format: xls` |
| Process only specific sheets | set `sheets` array |
| Each sheet has header rows to skip | set `skip_rows` or `skip_rows_by_sheet` |
| Input is email / `.eml` file | `format: eml` |
| Need sheet name in downstream path | use `{{ context "xlsx_sheet_name" }}` |
| Need filename of EML part downstream | use `{{ context "converter_filename" }}` |
| Input is SSTable key=value | `format: sst`, optionally set `delimiter` |

## Column Naming Matrix (CSV)

| skip_first | columns provided | Result |
|-----------|-----------------|--------|
| `true` | no | use row 1 values as column names |
| `true` | yes | use provided names (override row 1) |
| `false` | no | `Col1`, `Col2`, `Col3`, … |
| `false` | yes | use provided names |

## Per-format Output Behavior

| Format | Emits | Context keys set |
|--------|-------|-----------------|
| `csv` | One JSON record per original record | — |
| `html` | One JSON record per original record | — |
| `xlsx` / `xls` | **One record per sheet** | `xlsx_sheet_name` |
| `eml` | One record per part (body.html, body.txt, headers.json, attachments) | `converter_filename`, `content_type` |
| `sst` | One record per line | — |

## Validation Rules

- `format` is required
- `skip_first` and `columns` only apply to `format: csv`
- `container` only applies to `format: html`
- `sheets`, `skip_rows`, `skip_rows_by_sheet` only apply to `format: xlsx` / `format: xls`
- `delimiter` only applies to `format: sst`
- XLSX emits **one record per sheet** — if user expects per-row records, they need a `split` task after converter

## Examples

### CSV with headers
```yaml
- name: parse_csv
  type: converter
  format: csv
  skip_first: true
```

### CSV with explicit columns
```yaml
- name: parse_csv
  type: converter
  format: csv
  skip_first: true
  columns:
    - name: id
      is_numeric: true
    - name: email
    - name: revenue
      is_numeric: true
```

### HTML table extraction
```yaml
- name: parse_table
  type: converter
  format: html
  container: "//table[@class='results']"
```

### Excel — all sheets, skip header row
```yaml
- name: parse_excel
  type: converter
  format: xlsx
  skip_rows: 1
```

### Excel — specific sheets, per-sheet skip
```yaml
- name: parse_excel
  type: converter
  format: xlsx
  sheets: ["Sales", "Returns"]
  skip_rows: 1
  skip_rows_by_sheet:
    Returns: 3
```

### Write each Excel sheet to its own file
```yaml
- name: parse_excel
  type: converter
  format: xlsx

- name: write_sheet
  type: file
  path: output/{{ context "xlsx_sheet_name" }}_{{ macro "timestamp" }}.csv
```

### EML — extract parts and write each
```yaml
- name: read_email
  type: file
  path: inbox/message.eml

- name: parse_email
  type: converter
  format: eml

- name: write_part
  type: file
  path: output/{{ context "converter_filename" }}
```

## Anti-patterns

- Expecting per-row records from XLSX without a `split` task after converter
- Using `skip_first` on `format: html` or `format: xlsx` — only valid for CSV
- Not using `{{ context "xlsx_sheet_name" }}` when writing each sheet to a separate file
- Forgetting that EML `converter_filename` includes sanitized filenames — downstream paths should use the context key
