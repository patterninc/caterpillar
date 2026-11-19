# Secrets and environment scoping

Caterpillar holds no runtime credentials. It reads them, at load time, with whatever AWS identity the process runs as.

## Runtime

- `{{ secret "/path" }}` in a pipeline file calls SSM `GetParameter` with decryption (`internal/pkg/config/secret.go`), using the default AWS credential chain. The `aws_parameter_store` task does the same per record.
- **Scope is set by the caller's IAM role**, not by caterpillar. Each environment's runner should use its own role that can read only that environment's parameter paths (for example `/prod/...` from prod, `/stage/...` from stage). Use environment-prefixed paths in pipelines so a wrong-environment read fails with `AccessDenied` instead of returning another environment's value.
- `{{ env "NAME" }}` reads the runner's environment; treat it like a secret if it carries one.
- Rendered configs contain plaintext values. Never log or print them.

## CI

| Secret | Used by | Exposed to pull requests |
| --- | --- | --- |
| `DOCKERHUB_USERNAME`, `DOCKERHUB_TOKEN` | `docker-secrets`, `docker-build`, `docker-merge` in `release.yaml` | No: the workflow runs only on pushes to `main` and tags |
| `GITHUB_TOKEN` | `release` (contents: write, tags only), `pr-title` (pull-requests: read), `smoke` (read) | Read-only on pull requests |

Integration tests use LocalStack with the dummy credentials `test` / `test`; no real AWS account is reachable from CI.

## Leaks

Pattern's org-wide Wiz scanning covers secret detection on every PR, and `.pre-commit-config.yaml` runs `gitleaks` locally. If a secret is committed: rotate it first, then remove it from the code; history rewrites do not help in a public repo.
