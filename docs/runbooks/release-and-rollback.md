# Cutting a release, rolling one back

Publishing is the deploy for this repo (ADR 0006). `.github/workflows/release.yaml` runs on pushes to `main` and on `v*` and `latest` tags; `.github/workflows/smoke.yml` runs after it.

## Cut a release

1. Make sure `main` is green and `CHANGELOG.md` has the changes under `## [Unreleased]`. Move them under a new `## [X.Y.Z] - YYYY-MM-DD` heading in a PR, and merge it.
2. Tag the merge commit and push the tag:
   ```bash
   git checkout main && git pull
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```
3. Watch the Release run: both `build-linux-*` jobs, `release`, then `docker-merge`. Then watch Smoke: `smoke-binary` for both assets and `smoke-image` for both platforms.
4. If consumers track `latest`, move it once the smoke run passes:
   ```bash
   git tag -f latest vX.Y.Z
   git push -f origin latest
   ```
   This only retags the Docker image; it builds no binaries.

## Roll back

Releases are immutable; roll back by pointing consumers at the previous version.

1. Image consumers on `latest`: move `latest` back to the previous good tag (step 4 above with the old version). Consumers pinned to `<version>` change their pin.
2. Binary consumers download by release tag; point them at the previous tag. Never delete or rename the assets `caterpillar` and `caterpillar-linux-arm64`.
3. Fix forward on `main` and cut a new patch release. Do not reuse a tag.
4. `dev` always follows `main`; revert the bad commit on `main` to roll `dev` back.
