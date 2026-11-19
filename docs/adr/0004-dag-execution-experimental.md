# 0004. DAG execution alongside the linear runner, marked experimental

- Status: Accepted
- Date: 2026-11-19 (retroactive; DAG support was added in #18 on 2025-11-26)

## Context

Linear pipelines cannot fan a stream out to several sinks or merge parallel branches.

## Decision

A pipeline with a `dag:` key (for example `a >> [b, c] >> d`) is parsed by `internal/pkg/pipeline/dag.go` and run by `executeDag` in `pipeline.go`. Without `dag:`, the linear runner is used unchanged. Parallel branches get a copy of every record (`distributeToChannels`) and their outputs are merged (`mergeChannels`). The feature is documented as **EXPERIMENTAL** in `README.md` and `DAG_README.md`.

Inferred rationale: keeping the linear runner as the default protected existing pipelines while the DAG runner matured; a separate code path rather than expressing linear pipelines as DAGs avoided changing their behaviour.

## Consequences

- Two runners to maintain. Ack handling must hold in both: `distributeToChannels` registers one ack branch per copy itself, because it does not go through `SendRecord` (ADR 0005).
- Branches share the same `*record.Record`; a task that mutates a record in one branch is visible in the others.
- `validateInput` in `dag.go` has a cyclomatic complexity of 36 and is the one baselined `gocyclo` finding in `.golangci.yml`.
