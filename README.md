# Caterpillar

Caterpillar is a powerful data ingestion and processing pipeline system written in Go. It's designed to handle complex data workflows by connecting multiple processing tasks in a pipeline, similar to how Unix pipes work in shell scripting. Each task processes data and passes it to the next task, creating a flexible and scalable data processing system.

## Purpose

Caterpillar enables you to:
- **Ingest data** from various sources (HTTP APIs, files, S3, SQS, etc.)
- **Transform data** using JQ queries, XPath, and custom logic
- **Process data** with sampling, filtering, and aggregation
- **Output data** to files, HTTP endpoints, or other destinations
- **Chain operations** in a pipeline for complex workflows

The system is particularly useful for:
- ETL (Extract, Transform, Load) processes
- API data aggregation and transformation
- Log processing and analysis
- Data migration and synchronization
- Real-time data streaming workflows

## How to Run Locally

### Prerequisites

- Go 1.24.5 or later
- AWS CLI configured (for AWS-related tasks)
- Docker (optional, for containerized deployment)

### Building and Running

1. **Clone the repository:**
   ```bash
   git clone <repository-url>
   cd caterpillar
   ```

2. **Build the project:**
   ```bash
   go build -o caterpillar ./cmd/caterpillar/caterpillar.go
   ```

3. **Run a pipeline:**
   ```bash
   ./caterpillar -conf test/pipelines/hello_name.yaml
   ```

### Docker Build

To build a Docker image:
```bash
docker build . -f ./build/Dockerfile
```

## Core Concepts

### Tasks and Records

Caterpillar is built around the concept of **tasks** that process **records** in a pipeline. Here's how it works:

1. **Records**: Each piece of data in the pipeline is wrapped in a `Record` object containing:
   - `Data`: The actual data string
   - `Origin`: The name of the task that created this record
   - `ID`: A unique identifier for the record
   - `Context`: Additional metadata that can be shared between tasks

2. **Task Processing**: Each task:
   - Receives records from its input channel
   - Processes the data according to its specific function
   - Sends processed records to its output channel
   - Closes the output channel when done processing all records

3. **Pipeline Flow**: Records flow through the pipeline like this:
   ```
   Task A → Task B → Task C → Task D
   ```
   When Task A finishes processing all records, it closes its output channel. This triggers Task B to finish processing and close its output channel, creating a chain reaction that propagates to the end of the pipeline.

4. **Architecture Components**:
   - **Tasks** are connected in sequence and process data
   - **Channels** pass data between tasks with configurable buffer sizes
   - **Records** contain data and metadata that flow through the pipeline

### Error Handling

By default, tasks handle errors gracefully:
- **Non-critical errors** are logged but don't stop the pipeline
- **Task failures** are recorded but processing continues
- **Pipeline completion** returns a count of processed records

However, you can configure tasks to fail the entire pipeline on error:
```yaml
tasks:
  - name: critical_task
    type: http
    fail_on_error: true  # Pipeline stops if this task fails
```

When `fail_on_error` is set, the pipeline will return a non-zero exit code if any configured task encounters an error.

### Context Variables

Context variables allow you to store data on a record that can be accessed later in the pipeline. This is especially useful when tasks are separated by one or more intermediate tasks.

**Setting Context**: Tasks can set context variables using JQ expressions:
```yaml
tasks:
  - name: extract_user_data
    type: jq
    path: .user.id
    context:
      user_id: .        # Stores the user ID in context
      user_name: .name  # Stores the user name in context
```

**Using Context**: Later tasks can reference context variables using the `{{ context "key" }}` syntax:
```yaml
tasks:
  - name: use_context
    type: file
    path: users/{{ context "user_id" }}_{{ context "user_name" }}.json
```

**Context Persistence**: Context variables persist throughout the pipeline, so you can set them early and use them much later:
```yaml
tasks:
  - name: extract_data
    type: jq
    context:
      timestamp: .timestamp
      user_id: .user.id
  
  - name: process_data
    type: http
    # Uses context set 2 tasks ago
  
  - name: save_result
    type: file
    path: output/{{ context "user_id" }}_{{ context "timestamp" }}.txt
```

**Important Notes**:
- Context variables are tied to individual records
- When using `explode: true` in JQ tasks, new records inherit context from the original record
- Context variables are evaluated at runtime, not at pipeline startup
- Each task can set its own context variables that will be available to downstream tasks

## Dynamic Configuration

Caterpillar supports several template functions for dynamic configuration. These are evaluated at different times:

- **Pipeline initialization**: `env` and `secret` functions are evaluated once when the pipeline starts
- **Per record**: `macro` and `context` functions are evaluated for each record as it's processed

### Macro Functions
Macros are evaluated for each record and produce dynamic values (evaluated per record):

- **`{{ macro "unixtime" }}`** - Current Unix timestamp
- **`{{ macro "timestamp" }}`** - Current ISO timestamp (2006-01-02T15:04:05Z07:00)
- **`{{ macro "microtimestamp" }}`** - Current microsecond timestamp
- **`{{ macro "uuid" }}`** - Generate a new UUID

Example:
```yaml
tasks:
  - name: write_file
    type: file
    path: output/data_{{ macro "timestamp" }}.txt
```

### Environment Variables
Access environment variables using the `env` function (evaluated at pipeline initialization):

```yaml
tasks:
  - name: api_call
    type: http
    endpoint: {{ env "API_ENDPOINT" }}
    headers:
      Authorization: Bearer {{ env "API_TOKEN" }}
```

### Secrets
Retrieve secrets from AWS Parameter Store (evaluated at pipeline initialization):

```yaml
tasks:
  - name: secure_api
    type: http
    headers:
      Authorization: Bearer {{ secret "/prod/api/token" }}
```

### Context Variables
Reference context variables set by upstream tasks (evaluated per record):

```yaml
tasks:
  - name: use_context
    type: file
    path: users/{{ context "user_id" }}_{{ context "user_name" }}.json
```

## Supported Tasks

Caterpillar supports the following tasks, each of which can serve different roles depending on their configuration:

- **`aws_parameter_store`** - Read parameters from AWS Systems Manager Parameter Store
- **`compress`** - Compress or decompress data using various algorithms
- **`converter`** - Convert data between different formats (CSV, HTML, JSON, XML, SST)
- **`delay`** - Add controlled delays between record processing
- **`echo`** - Print data to console for debugging and monitoring
- **`file`** - Read from or write to local files and S3 (acts as source or sink)
- **`flatten`** - Flatten nested JSON structures into single-level key-value pairs
- **`heimdall`** - Submit jobs to Heimdall data orchestration platform and return results
- **`http`** - Make HTTP requests with OAuth support and retry logic
- **`http_server`** - Start an HTTP server to receive incoming data
- **`join`** - Combine multiple records into a single record
- **`jq`** - Transform JSON data using JQ queries
- **`replace`** - Perform regex-based text replacement and transformation
- **`sample`** - Sample data using various strategies (random, head, tail, nth, percent)
- **`split`** - Split data by specified delimiters
- **`sqs`** - Read from or write to AWS SQS queues (acts as source or sink)
- **`xpath`** - Extract data from XML/HTML using XPath expressions



## Pipeline Configuration

### Channel Size
Control the buffer size between tasks:

```yaml
channel_size: 10000  # Default is 10,000
tasks:
  - name: task1
    type: echo
```

### Task Configuration
Each task supports common configuration options:

```yaml
tasks:
  - name: my_task
    type: http
    fail_on_error: true  # Stop pipeline on error
    context:
      extracted_value: .data.value  # Set context for downstream tasks
```

### Error Handling
Tasks can be configured to fail the entire pipeline on error:

```yaml
tasks:
  - name: critical_task
    type: http
    fail_on_error: true  # Pipeline stops if this task fails
```

## Examples

See the `test/pipelines/` directory for comprehensive examples of different pipeline configurations and task combinations.

## Credits

**Caterpillar** was created at **Pattern, Inc** by Stan Babourine, with contributions from Will Graham, Prasad Lohakpure, Mahesh Kamble, Shubham Khanna, Ivan Hladush, Amol Udage, Divyanshu Tiwari, Dnyaneshwar Mane and Narayan Attarde.
