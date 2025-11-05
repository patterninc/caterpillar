# Mux and Demux Tasks

The Mux and Demux tasks are specialized internal components that handle data flow coordination in DAG (Directed Acyclic Graph) pipelines. They enable complex data routing patterns by managing multiple input and output channels.

## Overview

These tasks are automatically created and managed by the DAG execution engine and are not directly configurable by users. They serve as the plumbing that makes parallel and branching data flows possible in complex pipeline topologies.

## Mux Task (Multiplexer)

The Mux task takes data from a single input channel and distributes copies to multiple output channels.

### Function
- Receives records from one input channel
- Creates copies of each record 
- Sends copies to all connected output channels
- Handles proper channel cleanup on completion

### Use Cases
- **Fan-out patterns**: When one task needs to send data to multiple downstream tasks
- **Data broadcasting**: Distributing the same data to parallel processing branches
- **Pipeline branching**: Creating multiple processing paths from a single data source

### Example DAG Pattern
```yaml
# This creates a Mux internally
dag: source_task >> [process_a, process_b, process_c]
```

## Demux Task (Demultiplexer) 

The Demux task takes data from multiple input channels and merges them into a single output channel.

### Function
- Receives records from multiple input channels concurrently
- Merges records from all inputs into one output stream
- Maintains proper goroutine coordination
- Ensures all input channels are fully consumed

### Use Cases
- **Fan-in patterns**: When multiple tasks need to send data to one downstream task
- **Data aggregation**: Combining results from parallel processing branches
- **Pipeline convergence**: Merging multiple processing paths into a single stream

### Example DAG Pattern
```yaml
# This creates a Demux internally
dag: [source_a, source_b, source_c] >> destination_task
```

## Technical Implementation

### Concurrency Model
- Each Mux/Demux runs in its own goroutine
- Input processing is handled concurrently via additional goroutines
- Proper synchronization using `sync.WaitGroup`
- Channels are closed automatically to signal completion

### Memory Management
- Records are copied (not shared) to prevent data races
- Context is properly propagated through record copies
- Channel buffers prevent blocking in most scenarios

### Error Handling
- Tasks inherit error handling from the overall pipeline configuration
- Channel closure signals normal completion vs error conditions
- Proper cleanup prevents goroutine leaks

## Data Flow Examples

### Simple Fan-out
```
Input: [1, 2, 3]

task1 >> [task2, task3]

Mux created automatically:
task1 → Mux → task2 (receives: [1, 2, 3])
          └─→ task3 (receives: [1, 2, 3])
```

### Simple Fan-in
```
Inputs: task1=[A, B], task2=[X, Y]

[task1, task2] >> task3

Demux created automatically:
task1 ─┐
       └→ Demux → task3 (receives: [A, B, X, Y] in some order)
task2 ─┘
```

### Complex Diamond Pattern
```
task1 >> [task2, task3] >> task4

Creates both Mux and Demux:
task1 → Mux → task2 ─┐
         └─→ task3 ─└→ Demux → task4
```

## Performance Characteristics

### Throughput
- Mux: Limited by the slowest output channel
- Demux: Processes inputs concurrently for optimal throughput
- Channel buffer size affects performance under high load

### Resource Usage
- Minimal CPU overhead for data copying
- Memory usage proportional to channel buffer sizes
- Goroutine count scales with complexity of DAG topology

## Debugging and Monitoring

### Logging
- Mux/Demux tasks appear in pipeline logs with generated names
- Channel operations are logged for debugging complex flows
- Timing information helps identify bottlenecks

## Integration with DAG System

### Automatic Creation
The DAG parser automatically inserts Mux/Demux tasks when analyzing expressions:
- Tuple expressions `(a, b, c)` create connection points
- Binary operators `>>` determine data flow direction
- Complex expressions may create multiple Mux/Demux instances

### Naming Convention
- Mux tasks: `mux_<hash>` where hash identifies the connection point
- Demux tasks: `demux_<hash>` where hash identifies the merge point
- Names are automatically generated and not user-configurable

## Limitations

- **Order preservation**: Demux does not guarantee output order matches input order
- **Backpressure**: Slow downstream tasks can block entire Mux branches
- **Error propagation**: Individual branch failures may affect entire pipeline
- **Resource scaling**: Very wide fan-out patterns may consume significant resources

## Best Practices

### Pipeline Design
- Keep fan-out width reasonable (< 10 branches typically)
- Ensure downstream tasks can handle similar data rates
- Consider data volume when designing complex topologies

## Related Components

- **DAG Parser**: Creates Mux/Demux tasks during expression analysis
- **DAG Execution Engine**: Manages Mux/Demux lifecycle and coordination
- **Channel Manager**: Handles channel creation and cleanup for Mux/Demux tasks
- **Task Registry**: Tracks Mux/Demux instances for proper resource management