# DAG (Directed Acyclic Graph) Feature - EXPERIMENTAL

The DAG feature enables you to define complex task execution flows with parallel processing, branching, and merging capabilities. This allows for more sophisticated pipeline architectures beyond simple linear task chains.

## Overview

The DAG feature was introduced in **v2.1.0-experimental-dag** and provides a declarative way to specify task execution dependencies and parallel processing patterns.

## Syntax

DAG expressions use a simple, intuitive syntax:

- `>>` - Sequential execution (task dependency)
- `[task1, task2]` - Parallel execution (tasks run concurrently)

### Basic Patterns

#### Sequential Chain
```yaml
dag: task1 >> task2 >> task3
```

#### Parallel Execution
```yaml
dag: [task1, task2, task3]
```

#### Fan-out (one-to-many)
```yaml
dag: task1 >> [task2, task3, task4]
```

#### Fan-in (many-to-one)
```yaml
dag: [task1, task2, task3] >> task4
```

#### Diamond Pattern
```yaml
dag: task1 >> [task2, task3] >> task4
```

### Advanced Patterns

#### Complex Branching
```yaml
dag: source >> [branch1 >> transform1, branch2 >> transform2] >> sink
```

#### Multi-stage Pipeline
```yaml
dag: ingest >> [clean, validate] >> [transform, enrich] >> [aggregate, export]
```

## Example Configuration

```yaml
tasks:
  - name: read_csv_file
    type: file
    path: data/input.csv
  - name: split_to_lines
    type: split
  - name: convert_from_csv
    type: converter
    format: csv
    skip_first: true
    columns:
      - name: name
      - name: age
        is_numeric: true
  - name: echo
    type: echo
    only_data: true
  - name: echo2
    type: echo
    only_data: true

# DAG definition
dag: read_csv_file >> [split_to_lines, echo] >> convert_from_csv >> echo2
```

## Key Features

### 1. **Parallel Processing**
Tasks within brackets `[task1, task2]` execute concurrently, improving pipeline throughput.

### 2. **Automatic Channel Management**
The pipeline automatically creates and manages channels between tasks based on the DAG structure.

### 3. **Error Handling**
- Individual task failures can be configured with `fail_on_error`
- Pipeline continues execution for non-failing tasks when possible

### 4. **Validation**
The DAG parser includes comprehensive validation:
- Syntax checking (balanced brackets, valid operators)
- Invalid character detection
- Structure validation (empty groups, malformed expressions)

## Validation Rules

The DAG parser enforces several rules:

- **No empty expressions**: `""` is invalid
- **Balanced brackets**: Every `[` must have a matching `]`
- **No empty groups**: `[]` is invalid
- **No single-item groups**: `[task1]` is invalid (use `task1` directly)
- **Valid characters only**: Letters, numbers, `_`, `-`, `[`, `]`, `,`, `>`, whitespace
- **Proper arrow usage**: Only `>>` allowed, no single `>` or `>>>+`
- **No leading arrows**: `>>task1` is invalid

## Migration from Linear Pipelines

### Before (Linear)
```yaml
tasks:
  - name: task1
    type: file
  - name: task2  
    type: split
  - name: task3
    type: echo
# Implicit linear execution: task1 >> task2 >> task3
```

### After (DAG)
```yaml
tasks:
  - name: task1
    type: file
  - name: task2  
    type: split
  - name: task3
    type: echo

# Explicit DAG definition (same behavior)
dag: task1 >> task2 >> task3

# Or with parallel processing
dag: task1 >> [task2, task3]
```

## Troubleshooting

### Common Issues

1. **Syntax Errors**
   ```
   Error: invalid DAG groups: error at index X, unmatched closing brace ']' found
   ```
   - Check bracket balancing
   - Ensure proper comma placement

2. **Invalid Characters**
   ```
   Error: invalid DAG groups: invalid characters found
   ```
   - Only use letters, numbers, `_`, `-`, brackets, commas, and arrows
   - Remove special characters like `()`, `{}`, `@`, `$`

3. **Performance Issues**
   - Increase `channel_size` for high-throughput pipelines
   - Monitor task concurrency settings
   - Check for bottlenecks in slow tasks

## Best Practices

1. **Use Meaningful Task Names**
   - Use descriptive names that reflect task purpose
   - Follow consistent naming conventions (snake_case recommended)

2. **Optimize Parallel Sections**
   - Group similar-duration tasks together
   - Avoid mixing fast and slow tasks in parallel groups
   - Consider task dependencies when designing parallel sections

3. **Channel Sizing**
   - Set appropriate `channel_size` based on data volume
   - Monitor memory usage in production
   - Use larger buffers for batch processing

4. **Error Handling**
   - Configure `fail_on_error` appropriately for each task
   - Design graceful degradation paths
   - Log errors for debugging

## Technical Implementation

The DAG feature is implemented through:

- **DAG Parser**: Converts string expressions to internal graph structure
- **Validation Engine**: Ensures syntactic and semantic correctness  
- **Channel Manager**: Creates and manages inter-task communication
- **Execution Engine**: Orchestrates parallel and sequential execution
- **Resource Optimizer**: Optimizes memory usage and channel allocation

For more technical details, see the source code in:
- `internal/pkg/pipeline/dag.go`
- `internal/pkg/pipeline/dag_test.go`
- `internal/pkg/pipeline/pipeline.go`