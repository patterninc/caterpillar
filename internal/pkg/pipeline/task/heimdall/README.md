# Heimdall Task

The `heimdall` task submits jobs to the [Heimdall](https://github.com/patterninc/heimdall) data orchestration and job execution platform.

## Function

The Heimdall task submits jobs to Heimdall for execution and retrieves the results. Heimdall is a lightweight, pluggable data orchestration platform that can execute various types of jobs including shell commands, Spark queries, Snowflake operations, and more.

## Behavior

The Heimdall task submits jobs to the Heimdall platform and retrieves results. It operates as a data source (no input channel required), submits the configured job to Heimdall, and sends the job results to its output channel. The task supports both synchronous and asynchronous job execution with polling for completion.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `heimdall` | Must be "heimdall" |
| `endpoint` | string | `http://localhost:9090` | Heimdall API endpoint |
| `headers` | map[string]string | - | HTTP headers for API requests |
| `poll_interval` | int | `5` | Polling interval in seconds for async jobs |
| `timeout` | int | `300` | Job timeout in seconds |
| `job` | object | - | Job configuration (see Job Configuration) |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## Job Configuration

The `job` field contains the job specification:

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | `caterpillar` | Job name |
| `version` | string | `0.0.1` | Job version |
| `context` | map[string]any | - | Job execution context |
| `command_criteria` | []string | - | Criteria to match commands |
| `cluster_criteria` | []string | - | Criteria to match clusters |
| `tags` | []string | - | Job tags |

## Example Configurations

### Basic job submission:
```yaml
tasks:
  - name: submit_ping_job
    type: heimdall
    endpoint: http://localhost:9090
    job:
      name: ping-test
      version: 0.0.1
      command_criteria: ["type:ping"]
      cluster_criteria: ["type:localhost"]
```

### Job with custom headers and context:
```yaml
tasks:
  - name: submit_shell_job
    type: heimdall
    endpoint: http://heimdall.example.com
    headers:
      X-Heimdall-User: caterpillar
    job:
      name: data-processing
      version: 1.0.0
      command_criteria: ["type:shell"]
      cluster_criteria: ["type:emr"]
      context:
        script: "echo 'Hello from caterpillar'"
        environment: production
```

### Long-running job with extended timeout:
```yaml
tasks:
  - name: submit_spark_job
    type: heimdall
    endpoint: http://heimdall.example.com
    timeout: 3600  # 1 hour timeout
    poll_interval: 10  # Check every 10 seconds
    job:
      name: spark-analysis
      command_criteria: ["type:spark"]
      cluster_criteria: ["type:emr-on-eks"]
      context:
        query: "SELECT * FROM large_table LIMIT 1000"
```

## Sample Pipelines

- `test/pipelines/heimdall.yaml` - Heimdall job submission example

## Use Cases

- **Data processing**: Submit Spark, Snowflake, or Trino queries
- **Shell execution**: Run shell commands on remote clusters
- **ETL workflows**: Execute data transformation jobs
- **Infrastructure operations**: Run operational tasks on clusters
- **Batch processing**: Submit long-running batch jobs
- **Data orchestration**: Coordinate complex data workflows

## Job Execution

The task supports both synchronous and asynchronous job execution:

- **Synchronous jobs**: Results are returned immediately
- **Asynchronous jobs**: The task polls for completion and retrieves results

## Integration Considerations

- **Heimdall setup**: Ensure Heimdall is running and accessible
- **Authentication**: Configure appropriate headers for API access
- **Job criteria**: Set correct command and cluster criteria
- **Timeout configuration**: Set appropriate timeouts for job types
- **Error handling**: Handle job failures and timeouts gracefully