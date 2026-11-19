---
name: cut-release
description: Prepares and verifies a caterpillar release - changelog section, version tag, and post-release checks. Use when asked to release, tag, or publish a new caterpillar version, or to roll one back. Do not use for publishing changes to caterpillar-services or other consumers.
---

# Cut a release

Follow `docs/runbooks/release-and-rollback.md`; this is the agent checklist.

1. Confirm `main` is green: `gh run list --branch main --limit 5`.
2. Pick the version by SemVer from the Unreleased entries in `CHANGELOG.md`: any **Migration** note means a major bump, any `feat` a minor bump, otherwise a patch.
3. Open a PR titled `docs: release vX.Y.Z` that moves Unreleased entries under `## [X.Y.Z] - <today>` and updates the compare links. Wait for review and merge.
4. **Ask the user before tagging** (it publishes binaries and a Docker image), then:
   `git tag vX.Y.Z <merge-sha> && git push origin vX.Y.Z`.
5. Watch `release.yaml` for that tag with `gh run watch` and report each job's result. Then pull `patternoss/caterpillar:X.Y.Z` and run `hello_name.yaml` (command in `docs/oncall/playbook.md`). If anything fails, follow the rollback section of the runbook and tell the user; do not move `latest`.
6. Moving `latest` is a separate, user-approved step.
