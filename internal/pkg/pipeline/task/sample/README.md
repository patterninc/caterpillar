# Sample Task

The `sample` task filters and samples data using various strategies, allowing you to process a subset of records for testing, analysis, or performance optimization.

## Function

The sample task applies different sampling strategies to filter records as they pass through the pipeline. It cannot be the first or last task in a pipeline and requires both input and output channels.

## Behavior

The sample task filters records using various sampling strategies. It receives records from its input channel, applies the specified sampling method (random, head, tail, nth, or percent), and sends only the selected records to its output channel. This task cannot be the first or last in a pipeline.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `sample` | Must be "sample" |
| `filter` | string | `random` | Sampling strategy (random, head, tail, nth, percent) |
| `limit` | int | `10` | Number of records to keep (for head, tail, nth strategies) |
| `percent` | int | `1` | Percentage of records to keep (for percent strategy) |
| `divider` | int | `1000` | Divisor for percentage calculation |
| `size` | int | `50000` | Buffer size for random sampling |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## Sampling Strategies

### Random Sampling (`random`)
Randomly selects records based on a percentage. Uses the `percent` and `divider` fields.

```yaml
tasks:
  - name: random_sample
    type: sample
    filter: random
    percent: 10  # 10% of records
    divider: 100
```

### Head Sampling (`head`)
Keeps the first N records from the input.

```yaml
tasks:
  - name: first_ten
    type: sample
    filter: head
    limit: 10
```

### Tail Sampling (`tail`)
Keeps the last N records from the input.

```yaml
tasks:
  - name: last_five
    type: sample
    filter: tail
    limit: 5
```

### Nth Sampling (`nth`)
Keeps every Nth record from the input.

```yaml
tasks:
  - name: every_tenth
    type: sample
    filter: nth
    limit: 10  # Every 10th record
```

### Percent Sampling (`percent`)
Keeps a percentage of records based on the `percent` field.

```yaml
tasks:
  - name: ten_percent
    type: sample
    filter: percent
    percent: 10
```

## Example Configurations

### Random 5% sampling:
```yaml
tasks:
  - name: random_sample
    type: sample
    filter: random
    percent: 5
    divider: 100
```

### First 100 records:
```yaml
tasks:
  - name: first_hundred
    type: sample
    filter: head
    limit: 100
```

### Every 50th record:
```yaml
tasks:
  - name: sparse_sample
    type: sample
    filter: nth
    limit: 50
```

## Sample Pipelines

- `test/pipelines/random_echo.yaml` - Random sampling example
- `test/pipelines/hello_name.yaml` - Sample task in a data processing pipeline

## Use Cases

- **Testing**: Sample a subset of data for pipeline testing
- **Performance optimization**: Reduce processing load by sampling large datasets
- **Development**: Work with smaller datasets during development
- **Data exploration**: Analyze patterns in a representative sample
- **Quality assurance**: Verify pipeline behavior with sample data
- **Load testing**: Test system performance with controlled data volumes