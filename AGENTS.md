# AGENTS.md

Guidance for people and coding agents working in this repository. The
[README](README.md) explains what caterpillar is, how to build and run it, and
how pipelines are configured.

## Checks

Every pull request runs `.github/workflows/ci.yaml`. Its jobs call these
makefile targets, so you can run the same checks locally before you push:

| Command | What it checks |
| --- | --- |
| `make build` | The `caterpillar` binary builds. |
| `make fmt-check` | Every Go file is `gofmt`-formatted. Fix with `gofmt -w .`. |
| `make vet` | `go vet` over every package. |
| `make lint` | golangci-lint with `.golangci.yml`, reporting only issues introduced since `LINT_BASE` (default `origin/main`). Older findings are a backlog, not a blocker; do not add new ones. |
| `go test -race ./...` | The tests. They need no Kafka, AWS or network. |

`make tools` installs golangci-lint at the version CI uses.

## Git hooks

`.pre-commit-config.yaml` runs gofmt, go vet, golangci-lint (new issues only)
and a gitleaks secret scan of the staged changes on every commit. The hook
versions are pinned. Install them once:

```bash
pipx install pre-commit   # or: uv tool install pre-commit
make hooks
```

Run every hook over the whole tree with `pre-commit run --all-files`.

## Secrets

The tool itself holds no credentials: pipelines resolve secrets at runtime (see
[Secrets](README.md#secrets) in the README). Never commit a secret or a real
credential, even in a test fixture.
