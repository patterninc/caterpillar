# On-call playbook

**Owner:** Data Acquisition, GitHub team `@patterninc/data-acquisition` (`.github/CODEOWNERS`), Backstage owner `dev-data-acquisition`. **Slack:** `#dev-data-acquisition`. This repo names no paging rotation or PagerDuty service; escalate in Slack.

Caterpillar has no long-running service or monitors of its own. Incidents reach this repo in one of three ways; triage by which one.

## 1. A pipeline run failed or produced bad data

Usually raised by the system running it (caterpillar-services, Airflow, a Heimdall job).

1. Get the run's stdout and exit status, the pipeline YAML, and the caterpillar version (image tag or release tag).
2. Classify with the runbooks:
   - exit 0, data missing: [silent-task-errors](../runbooks/silent-task-errors.md)
   - never finished: [pipeline-hang](../runbooks/pipeline-hang.md)
   - duplicates or loss from SQS or Kafka: [redelivery-and-data-loss](../runbooks/redelivery-and-data-loss.md)
   - failed at start (`config:`): [config-and-secret-load](../runbooks/config-and-secret-load.md)
3. Decide: pipeline config problem (fix in the pipeline's repo), upstream data change, or caterpillar bug.
4. For a caterpillar bug, reproduce locally with a trimmed pipeline under `test/pipelines/`, then fix with a regression test.

## 2. A new release broke consumers

1. Check the last Release run in Actions, then run the published artifact yourself: `docker run --rm -v "$PWD/test:/test:ro" -w / patternoss/caterpillar:<tag> /caterpillar -conf /test/pipelines/hello_name.yaml` should print 10 greetings.
2. Roll back with [release-and-rollback](../runbooks/release-and-rollback.md): move `latest` back, or ask consumers to pin the previous version.
3. Post in `#dev-data-acquisition` with the bad version, the good version, and the consumer impact.
4. Fix forward and cut a patch release.

## 3. CI or publishing is broken

- `main` red: find the PR that broke it from the run's commit; revert if the fix is not quick, because every open PR is blocked by required checks.
- Image not published: [docker-hub-publishing](../runbooks/docker-hub-publishing.md).
- Build errors: [build-cgo-librdkafka](../runbooks/build-cgo-librdkafka.md).

## Severity

| Severity | Example | Response |
| --- | --- | --- |
| High | Released version drops or corrupts records in production pipelines | Roll back now, then investigate |
| Medium | A task type fails for one pipeline; a workaround exists | Fix within the sprint |
| Low | CI flake, docs, Docker publish skipped with binaries fine | Ticket in the Data Acquisition ClickUp list |

## After the incident

Add a regression test, add a row to the relevant runbook if the fix was new, and record any architectural change as an ADR in `docs/adr/`.
