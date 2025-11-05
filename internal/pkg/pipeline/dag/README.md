# DAG (Directed Acyclic Graph) Pipeline Execution

The DAG system transforms Caterpillar from a simple sequential pipeline processor into a powerful dataflow engine. In Caterpillar, tasks process data as it streams through the pipeline—tasks do not wait for upstream tasks to fully complete before starting. Instead, each task begins processing as soon as it receives data from its upstream channels, enabling true streaming and pipelined parallelism.

- **Stream data through tasks** as soon as records are available, without waiting for upstream completion
- **Enable parallel processing** where independent tasks operate concurrently on incoming data
- **Support complex data flows** including fan-out (one-to-many) and fan-in (many-to-one) patterns

## Architecture

The DAG system consists of several key components:

### 1. DAG Parser (`/parser`)
- **Tokenizer**: Breaks down DAG expressions into tokens
- **AST (Abstract Syntax Tree)**: Represents the DAG structure in memory
- **Parser**: Builds the AST from tokenized input using recursive descent parsing

### 2. Task Coordination (`/task`)
- **Mux**: Multiplexes data from one input channel to multiple output channels
- **Demux**: Demultiplexes data from multiple input channels to one output channel

### 3. DAG Execution Engine (`dag.go`)
- **Dependency Resolution**: Determines task execution order
- **Channel Management**: Creates and manages communication channels between tasks
- **Goroutine Orchestration**: Spawns and coordinates concurrent task execution

## DAG Expression Syntax

DAG expressions use a simple, intuitive syntax inspired by airflow DAG syntax but places some constraints to remove ambiguity.
### Basic Operations

| Operator | Description | Example |
|----------|-------------|---------|
| `>>` | Sequential execution | `task1 >> task2` |
| `[]` | Grouping/Tuple | `task1 >> [task2, task3]` |
| `,` | Parallel branches | `[task2, task3]` |

### Expression Examples

```yaml
# Sequential execution
dag: task1 >> task2 >> task3

# Fan-out: task1 feeds both task2 and task3
dag: task1 >> [task2, task3]

# Fan-in: both task1 and task2 feed task3
dag: [task1, task2] >> task3

# Complex mixed patterns
dag: task1 >> [task2, task3] >> task4 >> [task5, task6]

# Diamond pattern with convergence
dag: task1 >> [task2, task3] >> task4
```

## Configuration

DAG execution is enabled by adding a `dag` field to your pipeline configuration:

```yaml
tasks:
  - name: extract_data
    type: http
    endpoint: "https://api.example.com/data"
  
  - name: transform_json
    type: jq
    query: '.results[]'
  
  - name: transform_csv
    type: converter
    format: csv
  
  - name: save_results
    type: file
    path: "output.txt"

# DAG expression defining the workflow
dag: |
  extract_data >> [transform_json, transform_csv] >> save_results
```


## Execution Model

### Dataflow and Streaming Semantics

- **Streaming execution**: Each task starts processing as soon as it receives data, without waiting for all upstream data to be produced.
- **No global barriers**: Downstream tasks do not wait for upstream tasks to finish; data flows continuously through the DAG.
- **Goroutine per task**: Each task runs in its own goroutine, processing records as they arrive.
- **Channel-based communication**: Tasks communicate via Go channels, supporting backpressure and pipelined execution.
- **Mux/Demux for complex flows**: Mux and Demux tasks manage data distribution and merging for fan-out/fan-in patterns.
- **Graceful shutdown**: Proper channel closing ensures all data is processed and resources are released.

### Sequential vs DAG Execution

- **Sequential**: When no `dag` field is present, tasks run in the order defined, passing data from one to the next.
- **DAG**: When a `dag` field is present, tasks run according to the dependency graph, with data streaming through all branches as soon as possible.

## Limitations and Considerations

- **Circular Dependencies**: DAGs must be acyclic - circular references will cause errors
- **Resource Management**: Complex DAGs may require tuning channel buffer sizes
- **Debugging**: Parallel execution can make debugging more complex than sequential flows
- **Memory Usage**: Fan-out patterns may increase memory usage due to data duplication

## Error Scenarios

Common DAG configuration errors and their solutions:

### Missing Task Reference
```yaml
dag: task1 >> nonexistent_task  # Error: task not found
```

### Circular Dependency
```yaml
dag: task1 >> task2 >> task1     # Error: circular dependency
```

### Malformed Expression
```yaml
dag: task1 >> (task2,           # Error: unclosed parenthesis
```

## Migration from Sequential Pipelines

Existing sequential pipelines can be easily converted to DAG format:

### Before (Sequential)
```yaml
tasks:
  - name: step1
    type: http
  - name: step2  
    type: jq
  - name: step3
    type: file
```

### After (DAG)
```yaml
tasks:
  - name: step1
    type: http
  - name: step2  
    type: jq
  - name: step3
    type: file

dag: step1 >> step2 >> step3
```
