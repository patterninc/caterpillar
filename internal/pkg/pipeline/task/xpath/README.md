# XPath Task

The `xpath` task extracts data from XML and HTML documents using XPath expressions, enabling structured data extraction from web pages and XML documents.

## Function

The XPath task applies XPath queries to XML/HTML data to extract specific elements, attributes, or text content. It's particularly useful for web scraping and XML document processing.

## Behavior

The XPath task extracts structured data from XML and HTML documents using XPath expressions. It receives records from its input channel, optionally selects container elements using a container XPath, then extracts multiple fields using field-specific XPath expressions. The extracted data is returned as JSON with field names as keys.

**Important**: Each field value is returned as an **array**, even if there's only one matching node. This allows the task to handle cases where multiple nodes match the same XPath expression. If no nodes match a field's XPath expression:
- When `ignore_missing` is `true` (default): the field is set to `null`
- When `ignore_missing` is `false`: the task returns an error

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `xpath` | Must be "xpath" |
| `container` | string | - | XPath expression to select container elements |
| `fields` | map[string]string | - | Map of field names to XPath expressions |
| `ignore_missing` | bool | `true` | Whether to ignore missing fields |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## Context Variables

When using a container XPath that matches one or more nodes, the task sets the following context variable for each output record:

| Variable | Type | Description |
|----------|------|-------------|
| `node_index` | string | 1-based index of the container node (e.g., "1", "2", "3") |

This allows downstream tasks to track which container element the data was extracted from, useful for maintaining order or grouping related data.

## Example Configurations

### Extract multiple fields from HTML:
```yaml
tasks:
  - name: extract_article_data
    type: xpath
    container: "//article"
    fields:
      title: ".//h1/text()"
      author: ".//span[@class='author']/text()"
      content: ".//div[@class='content']/text()"
      publish_date: ".//time/@datetime"
```

### Extract product information:
```yaml
tasks:
  - name: extract_products
    type: xpath
    container: "//div[@class='product']"
    fields:
      name: ".//h2[@class='product-name']/text()"
      price: ".//span[@class='price']/text()"
      description: ".//p[@class='description']/text()"
      image_url: ".//img/@src"
    ignore_missing: true
```

### Extract table data:
```yaml
tasks:
  - name: extract_table_data
    type: xpath
    container: "//table[@id='data-table']//tr"
    fields:
      name: ".//td[1]/text()"
      email: ".//td[2]/text()"
      phone: ".//td[3]/text()"
```

### Extract without container (from entire document):
```yaml
tasks:
  - name: extract_page_info
    type: xpath
    fields:
      title: "//title/text()"
      meta_description: "//meta[@name='description']/@content"
      canonical_url: "//link[@rel='canonical']/@href"
```

## Sample Pipelines

- `test/pipelines/xpath.yaml` - XPath extraction examples
- `test/pipelines/xpath_with_index.yaml` - Xpath extraction with node index example
- `test/pipelines/html2json.yaml` - HTML to JSON conversion

## Use Cases

- **Web scraping**: Extract data from HTML web pages with node indexing for tracking
- **XML processing**: Parse and extract data from XML documents
- **Data extraction**: Extract structured data from semi-structured sources
- **Content analysis**: Analyze web page content and structure
- **API response processing**: Extract data from XML API responses
- **Document processing**: Parse XML-based document formats
- **Multi-record extraction**: Process multiple similar elements while preserving their order