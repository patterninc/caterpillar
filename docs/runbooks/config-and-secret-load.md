# `config:` errors at start, SSM or region errors

`config.Load` renders the pipeline file as a Go template, then decodes YAML, then each task's `Init()` runs (`internal/pkg/config/config.go`, `internal/pkg/pipeline/tasks.go`). Any failure exits through `process.Bail("config", err)` before data moves.

| Error contains | Cause | Fix |
| --- | --- | --- |
| `template: render:` | Template syntax error, or a `{{ secret }}` lookup failed | Fix the template; for secrets, see below |
| `ParameterNotFound` | The SSM path does not exist in the account and region used | Check the path; check `AWS_REGION` |
| `AccessDeniedException` on `ssm:GetParameter` or `kms:Decrypt` | The caller's IAM role cannot read that parameter or its KMS key | Grant the specific parameter ARN to the runtime role (for caterpillar-services: `configs/task_policy.json`) |
| `Missing Region` | `config/secret.go` and `aws_parameter_store` take the region only from the environment | Set `AWS_REGION` |
| `yaml:` | Rendered output is not valid YAML, often a multi-line value | Use `{{ secret "/path" \| indent N }}` inside a block scalar |
| `task type is not supported` | Typo in `type`, or a binary older than the task | Check `supportedTasks` in `tasks.go`; upgrade the binary |
| `failed to initialize task <name>` | A task's `Init()` failed, often client creation | Read the wrapped error |
| `Key: '...' Error:Field validation` | A `validate:` struct tag failed | Set the required field from the task's README |

Region defaults differ by task: `file`, `sns` and `sqs` default to `us-west-2` and pass it explicitly, which overrides `AWS_REGION`; set `region:` on those tasks for other regions. `sqs` takes the region from the queue URL when it can.

Never paste a rendered config into a ticket or Slack: it contains the secret values.
