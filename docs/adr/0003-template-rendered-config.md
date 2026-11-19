# 0003. Pipeline files are Go templates rendered once at load time

- Status: Accepted
- Date: 2026-11-19 (retroactive)

## Context

The same pipeline runs in several environments with different endpoints and credentials, and some values (file names, URLs) depend on each record.

## Decision

`config.Load` (`internal/pkg/config/config.go`) parses the whole file as a `text/template` before YAML decoding, with five functions:

- `env` reads an environment variable at load time.
- `secret` reads an SSM Parameter Store value with decryption at load time (`secret.go`), using the default AWS credential chain.
- `macro` and `context` emit placeholders that `config.String.Get` resolves later: macros at evaluation time, context values per record from upstream tasks' `context:` jq queries.
- `indent` re-indents multi-line values for YAML block scalars.

## Consequences

- Secrets never appear in the pipeline file, and their scope is whatever the caller's IAM role can read (see `docs/secrets.md`).
- A missing parameter or SSM outage fails the whole run at load, before any data moves.
- Because rendering happens before YAML parsing, a value containing `: ` or a newline can break the YAML; `indent` exists for the multi-line case.
- Rendered configs contain plaintext secrets and must never be logged.
