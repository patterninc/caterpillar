---
skill: xpath
version: 1.0.0
caterpillar_type: xpath
description: Extract structured data from XML or HTML using XPath expressions.
role: transform
requires_upstream: true
requires_downstream: true
aws_required: false
---

## Purpose

Applies XPath expressions to XML/HTML record data. When `container` is set, iterates over matching nodes and emits one record per node. Each extracted field value is an array (even if only one match).

Context key `node_index` is automatically set (1-based) when using `container`.

## Schema

```yaml
- name: <string>                   # REQUIRED
  type: xpath                      # REQUIRED
  container: <string>              # OPTIONAL — XPath for repeating container elements
  fields: <map[string]string>      # REQUIRED — field name → XPath expression
  ignore_missing: <bool>           # OPTIONAL — null for missing fields vs error (default: true)
  fail_on_error: <bool>            # OPTIONAL (default: false)
```

## Decision Rules

| Condition | Choice |
|-----------|--------|
| Document has repeating elements (rows, articles, products) | set `container` |
| Extract page-level metadata | omit `container` |
| Missing elements should not stop pipeline | `ignore_missing: true` (default) |
| Missing elements are a hard error | `ignore_missing: false` |
| Need to track which element a record came from | use `{{ context "node_index" }}` downstream |
| Extract text content | use `/text()` in XPath |
| Extract attribute | use `/@attr` in XPath |
| Scoped to element with class | `[@class='name']` |
| Contains class (partial match) | `[contains(@class,'name')]` |

## Output Shape

Each field value is **always an array**:
```json
{
  "title":  ["Article Title"],
  "author": ["Jane Doe"],
  "tags":   ["tech", "news"]
}
```

To get the first value in downstream `jq`: `.title[0]`

## Context Auto-populated

When `container` is used:
```
{{ context "node_index" }}    → "1", "2", "3", ...
```

## Validation Rules

- `fields` is required — must have at least one field
- Field values are always arrays — downstream `jq` must use `.[0]` to extract scalar
- Without `container`, the entire document is one record
- With `container`, each matching node → one record
- `ignore_missing: false` stops pipeline on first missing field — use only for strict validation

## XPath Cheatsheet

| Goal | Expression |
|------|-----------|
| Text content | `.//element/text()` |
| Attribute value | `.//element/@attr` |
| By ID | `//*[@id='foo']` |
| By class | `//*[@class='foo']` |
| Contains class | `//*[contains(@class,'foo')]` |
| nth child | `.//td[2]/text()` |
| Direct child | `./child/text()` |
| First match | `(.//element)[1]` |
| Following sibling | `following-sibling::td[1]/text()` |
| Ancestor | `ancestor::div[@class='row']` |

## Examples

### Extract article data
```yaml
- name: extract_articles
  type: xpath
  container: "//article"
  fields:
    title: ".//h1/text()"
    author: ".//span[@class='author']/text()"
    published: ".//time/@datetime"
    url: ".//a[@class='permalink']/@href"
  ignore_missing: true
```

### Extract table rows
```yaml
- name: extract_rows
  type: xpath
  container: "//table[@id='data-table']//tr[position()>1]"
  fields:
    name:  ".//td[1]/text()"
    email: ".//td[2]/text()"
    role:  ".//td[3]/text()"
```

### Extract page metadata (no container)
```yaml
- name: page_meta
  type: xpath
  fields:
    title:       "//title/text()"
    description: "//meta[@name='description']/@content"
    canonical:   "//link[@rel='canonical']/@href"
    og_image:    "//meta[@property='og:image']/@content"
```

### Use node_index downstream
```yaml
- name: extract_rows
  type: xpath
  container: "//tr"
  fields:
    col1: ".//td[1]/text()"
    col2: ".//td[2]/text()"

- name: tag_with_index
  type: jq
  path: |
    {
      "row_number": "{{ context "node_index" }}",
      "col1": .col1[0],
      "col2": .col2[0]
    }
```

### Product catalog
```yaml
- name: extract_products
  type: xpath
  container: "//div[contains(@class,'product-item')]"
  fields:
    name:    ".//h2/text()"
    price:   ".//span[@class='price']/text()"
    sku:     ".//data[@name='sku']/@value"
    img_src: ".//img/@src"
  ignore_missing: true
```

## Anti-patterns

- Expecting scalar field values — all fields return arrays; always access with `[0]` in downstream `jq`
- Using `ignore_missing: false` in production with inconsistent HTML — pipeline stops on first missing field
- Omitting `container` when document has repeating elements — all elements processed as one record
- XPath expressions without `.//` prefix inside container — relative paths must start with `.//`
