---
description: Security rules for caterpillar pipeline YAML configs.
globs: "**/*.yaml,**/*.yml"
---

# Security Rules

## Credentials

- Never hardcode passwords, tokens, API keys, or secrets as literal values in pipeline YAML.
- Always use `{{ secret "/ssm/path" }}` for secrets stored in AWS SSM Parameter Store.
- Use `{{ env "VAR" }}` only for non-sensitive config (e.g. region, topic names). Secrets must use `{{ secret }}`.
- SSM paths must follow the pattern `/<env>/<service>/<key>` — e.g. `/prod/kafka/password`, `/staging/api/token`.

## Sensitive Fields

These fields must always use `{{ secret }}` or `{{ env }}` — never a literal value:
- `password`
- `username` (when paired with a password)
- `token`, `api_key`, `consumer_secret`, `token_secret`
- `queue_url`, `bootstrap_server`, `endpoint`, `topic_arn` if they contain credentials or account-specific identifiers

## HTTP

- Production pipeline `endpoint` values must use `https://`, not `http://`.
- Authorization headers (`Authorization`, `X-Api-Key`) must use `{{ secret }}` or `{{ env }}`.

## Files

- Never commit pipeline YAML files that contain literal secrets — even in `test/` pipelines.
- If a secret is accidentally committed, flag it immediately so it can be rotated.
