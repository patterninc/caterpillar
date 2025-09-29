# AWS Parameter Store Task

The `aws_parameter_store` task reads from or writes to AWS Systems Manager Parameter Store, enabling secure configuration management and parameter operations.

## Function

The AWS Parameter Store task operates in two modes depending on whether an input channel is provided:

- **Write mode** (with input channel): Receives records from the input channel and sets parameters in Parameter Store based on the data
- **Read mode** (no input channel): Retrieves parameters from Parameter Store and sends them to the output channel

## Input Channel

In write mode, accepts `*record.Record` objects containing data that can be used to set parameters in Parameter Store.

## Output Channel

In read mode, sends `*record.Record` objects containing parameter values from Parameter Store.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `aws_parameter_store` | Must be "aws_parameter_store" |
| `set` | map[string]string | - | Parameters to set using JQ expressions |
| `get` | map[string]string | - | Parameters to retrieve from Parameter Store |
| `secure` | bool | `true` | Whether to store parameters as SecureString |
| `overwrite` | bool | `true` | Whether to overwrite existing parameters |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## Example Configurations

### Write mode - Set parameters from pipeline data:
```yaml
tasks:
  - name: set_config
    type: aws_parameter_store
    set:
      "/my-app/api_key": ".api_key"
      "/my-app/endpoint": ".endpoint"
    secure: true
    overwrite: true
```

### Read mode - Get parameters from Parameter Store:
```yaml
tasks:
  - name: get_config
    type: aws_parameter_store
    get:
      "api_key": "/prod/api/key"
      "database_url": "/prod/database/url"
```


## Use Cases

- **Configuration management**: Store and retrieve application configuration
- **Secret management**: Securely store and retrieve sensitive data
- **Dynamic configuration**: Update parameters based on pipeline data
- **Environment setup**: Configure different environments with different parameters
- **Data persistence**: Store pipeline results for later use
- **Cross-pipeline sharing**: Share data between different pipeline runs