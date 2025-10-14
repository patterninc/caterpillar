# Heimdall Task

The `heimdall` task submits jobs to the [Heimdall](https://github.com/patterninc/heimdall) data orchestration and job execution platform.

## Function

The Heimdall task submits jobs to Heimdall for execution and retrieves the results. Heimdall is a lightweight, pluggable data orchestration platform that can execute various types of jobs including shell commands, Spark queries, Snowflake operations, and more.

## Behavior

The Heimdall task can operate in two modes:

### Source Mode (No Input Channel)
When used without an input channel, the task acts as a data source. It submits the configured job to Heimdall and sends the job results to its output channel. This is useful for initiating workflows or executing standalone jobs.

### Destination Mode (With Input Channel)
When used with an input channel, the task acts as a destination. It processes each incoming record by:
1. Parsing the record data as JSON
2. Using the parsed data as the job context
3. Submitting a job to Heimdall with the dynamic context
4. Sending the job results to the output channel

**Important**: When using heimdall as a destination task, you typically need a `jq` task before it to transform the pipeline data into the proper job context format. The jq task should output a JSON object that will be used as the job's context.

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

### Using Heimdall as a destination task:
```yaml
tasks:
  - name: fetch_data
    type: http
    method: GET
    endpoint: https://api.example.com/data
  - name: create_job_context
    type: jq
    path: |
      {
        "query": "SELECT * FROM " + .table_name + " WHERE id = " + (.id | tostring)
      }
  - name: submit_processing_job
    type: heimdall
    poll_interval: 10s
    timeout: 600s
    job:
      name: data-processor
      command_criteria:
        - type:spark
      cluster_criteria:
        - data:prod
  - name: echo_results
    type: echo
    only_data: true
```

In this example, the `jq` task transforms the HTTP response data into a proper job context object with a query field. The heimdall task then uses this context when submitting the job to Heimdall. Each record processed by the pipeline will trigger a separate Heimdall job with the context derived from the jq transformation.

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