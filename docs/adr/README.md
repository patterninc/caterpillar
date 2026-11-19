# Architecture decision records

Short, dated records of why caterpillar is built the way it is. Records 0001 to 0006 were written on 2026-11-19, **after the fact**, from the code and git history; their rationale is inferred where marked. New decisions get the next number and are written when the decision is made.

An ADR is immutable once accepted. To change a decision, add a new ADR that supersedes the old one and set the old one's status to `Superseded by NNNN`.

| # | Decision | Status |
| --- | --- | --- |
| [0001](0001-channel-pipeline-of-tasks.md) | Pipelines are tasks connected by buffered Go channels | Accepted |
| [0002](0002-task-registry-one-package-per-type.md) | One package per task type, registered in a static map | Accepted |
| [0003](0003-template-rendered-config.md) | Pipeline files are Go templates rendered once at load time | Accepted |
| [0004](0004-dag-execution-experimental.md) | DAG execution alongside the linear runner, marked experimental | Accepted |
| [0005](0005-deferred-acknowledgement.md) | At-least-once sources defer acknowledgement until every branch settles | Accepted |
| [0006](0006-distribution-binary-and-image.md) | Ship as release binaries and a multi-arch Docker image, built with cgo | Accepted |

Template:

```markdown
# NNNN. Title

- Status: Proposed | Accepted | Superseded by NNNN
- Date: YYYY-MM-DD

## Context
## Decision
## Consequences
```
