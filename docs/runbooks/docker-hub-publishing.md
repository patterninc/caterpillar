# Docker Hub image not published, rotating its token

`docker-secrets` in `.github/workflows/release.yaml` checks the repository secrets `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN`. If either is missing it logs a warning and skips `docker-build` and `docker-merge`; the binary release still succeeds.

## Image missing after a release

1. Open the Release run. If `docker-secrets` shows the warning `Skipping the patternoss/caterpillar Docker Hub publish`, the secrets are missing; a repository admin adds them (next section).
2. If `docker-build` failed, read its log: `login` failures mean an expired or revoked token; `apk` failures mean Alpine edge is unavailable or librdkafka changed (see [build-cgo-librdkafka.md](build-cgo-librdkafka.md)).
3. Re-run the failed jobs from the run page once fixed. For a tag, re-running rebuilds the same commit.

## Rotate the Docker Hub token

1. A Docker Hub admin of the `patternoss` organization creates a new access token with push access to `patternoss/caterpillar`.
2. A repository admin updates `DOCKERHUB_TOKEN` under Settings, Secrets and variables, Actions.
3. Re-run the latest Release run on `main` and confirm `docker-merge` passes, including its "Verify published image" step.
4. Revoke the old token in Docker Hub.

Only the `docker-build` and `docker-merge` jobs read these secrets, and they run only on pushes to `main` and tags, never on pull requests.
