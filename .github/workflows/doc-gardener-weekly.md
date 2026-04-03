---
# After editing this file, run: gh aw compile
# Then commit both this file and .github/workflows/doc-gardener-weekly.lock.yml
name: Doc Gardener (weekly)
description: Runs the Doc Gardener agent weekly to sync documentation with Go source and open a PR if there is drift.
on:
  schedule: weekly on monday
  workflow_dispatch: {}
engine: copilot
imports:
  - ../agents/doc-gardener.agent.md
permissions:
  contents: read
  actions: read
safe-outputs:
  create-pull-request:
    max: 1
---

# Doc Gardener — Weekly run

This workflow runs the [Doc Gardener](.github/agents/doc-gardener.agent.md) agent on a weekly schedule (every Monday). The agent compares Go source (structs, signatures, CLI flags) to README and docs, and opens a pull request if documentation has drifted.

- **Trigger:** Every Monday (scattered time) or manually via **Actions → Doc Gardener (weekly) → Run workflow**.
- **What it does:** The imported agent analyzes the repo, detects doc drift, and requests a PR via safe-outputs. You will see the run in the [Agents tab](https://github.com/patterninc/caterpillar/agents) and any created PR in Pull requests.
