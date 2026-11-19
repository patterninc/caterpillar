# Runbooks

Step-by-step fixes for recurring operational problems. Caterpillar is a CLI, so "the service" is whatever runs it (for Data Acquisition, `patterninc/caterpillar-services`, Airflow, or Heimdall jobs). Start from the symptom.

| Symptom | Runbook |
| --- | --- |
| Run exits 0 but data is missing | [silent-task-errors.md](silent-task-errors.md) |
| Run never finishes, no summary printed | [pipeline-hang.md](pipeline-hang.md) |
| SQS or Kafka messages duplicated, redelivered, or lost | [redelivery-and-data-loss.md](redelivery-and-data-loss.md) |
| `config:` error at start, SSM or region errors | [config-and-secret-load.md](config-and-secret-load.md) |
| Cutting a release, rolling one back | [release-and-rollback.md](release-and-rollback.md) |
| Docker Hub image not published, rotating its token | [docker-hub-publishing.md](docker-hub-publishing.md) |
| Build fails on cgo or librdkafka | [build-cgo-librdkafka.md](build-cgo-librdkafka.md) |
