# AGENTS.md

Guidance for people and coding agents working in this repository. The
[README](README.md) explains what caterpillar is, how to build and run it, and
how pipelines are configured.

## Checks

Every pull request runs `.github/workflows/ci.yaml`. Its `ci-status` job passes
only when every other job passes, and it is the check to require before merge.
The jobs call these makefile targets, so you can run the same checks locally
before you push:

| Command | What it checks |
| --- | --- |
| `make build` | The `caterpillar` binary builds. |
| `make fmt-check` | Every Go file is `gofmt`-formatted. Fix with `gofmt -w .`. |
| `make vet` | `go vet` over every package. |
| `make lint` | golangci-lint with `.golangci.yml`, reporting only issues introduced since `LINT_BASE` (default `origin/main`). Older findings are a backlog, not a blocker; do not add new ones. |
| `make lint-policy` | The dependency allow/deny list (gomodguard_v2) and `go vet`, over the whole tree. |
| `go test -race ./...` | The tests. They need no Kafka, AWS or network. |

`make tools` installs golangci-lint at the version CI uses.

### Adding a dependency

`.golangci.yml` lists every module the code may import directly. A new import
from any other module fails `make lint-policy`. To add one, add the module to
`gomodguard_v2.allowed` in the same pull request, so the addition is reviewed;
say why it is needed.

## Git hooks

`.pre-commit-config.yaml` runs gofmt, go vet, golangci-lint (new issues only)
and a gitleaks secret scan of the staged changes on every commit. The hook
versions are pinned. Install them once:

```bash
pipx install pre-commit   # or: uv tool install pre-commit
make hooks
```

Run every hook over the whole tree with `pre-commit run --all-files`.

## Commit messages and pull request titles

Use [Conventional Commits](https://www.conventionalcommits.org/):
`<type>(<optional scope>): <description>`, for example
`fix(kafka): stop deferred offset stores from queuing behind heartbeat polls`.

- Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`,
  `ci`, `chore`, `revert`.
- The scope is optional; use the task or package name (`kafka`, `sqs`, `http`,
  `file`, `task`, ...).
- Mark a breaking change with `!` after the type or scope (`feat(http)!: ...`)
  and explain it in the body.

Pull requests are usually squash-merged, and the squashed commit takes the pull
request title, so CI checks the **title** against this format on every pull
request (the `pr-title` job).

## Secrets

The tool itself holds no credentials: pipelines resolve secrets at runtime (see
[Secrets](README.md#secrets) in the README). Never commit a secret or a real
credential, even in a test fixture.
