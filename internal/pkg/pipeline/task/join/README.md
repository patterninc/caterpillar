# Join Task

The `join` task combines multiple records into a single record, useful for aggregating data or creating batch operations.

## Function

The join task collects multiple input records and combines them into a single output record, enabling data aggregation and batch processing capabilities.

## Behavior

The join task combines multiple records into a single record. It receives records from its input channel, buffers them, and sends joined records to its output channel when a configured limit is reached (size, number, or duration). With no limits set, it buffers everything and emits one record when the input closes. The buffer is always flushed when the input closes, so no records are lost.

## Deferred acknowledgment and FIFO queues

Caterpillar deletes an SQS message only after every downstream task has finished with the record produced from it. A `join` task with no `size:`, `number:`, or `duration:` limit holds its buffered records until the input channel closes, so the source message stays in flight for the whole run.

On a FIFO queue, an in-flight message blocks its entire message group: SQS does not deliver later messages in that group until the earlier one is deleted. An unbounded `join` on a FIFO-sourced pipeline therefore blocks the group for the run, and throughput collapses to roughly `max_messages` messages per group per run. The failure is silent: `ReceiveMessage` returns nothing, `exit_on_empty` fires, and the run reports success while the queue still holds undelivered messages.

The queue visibility timeout is not the fix. The gate is an in-flight message, not an expired timeout. Raising the timeout keeps the message in flight longer; lowering it causes SQS to redeliver the same blocked message instead of advancing to the next one in the group.

Set `duration:` (or `size:` / `number:`) on the join so it flushes periodically. Each flush settles records mid-run, downstream tasks finish, and the deletes unblock the group. Alternatively, the producer can use many distinct message group IDs (caterpillar's SQS writer generates a fresh UUID per message when `message_group_id` is unset on a `.fifo` queue).

When a flush limit is set, the join emits multiple records during a run. Caterpillar file writers overwrite: local files are opened with truncate, and S3 objects are written with `PutObject` on a fixed key. A downstream `file` task that writes every flush to the same path keeps only the last batch. Use a path with a per-write-unique token such as `{{ macro "uuid" }}`, `{{ macro "timestamp" }}`, `{{ macro "unixtime" }}`, or `{{ macro "microtimestamp" }}`. Tokens like `{{ ds }}`, `{{ hour }}`, `{{ ts_nodash }}`, `{{ params.x }}`, and `{{ env "X" }}` are constant for a run and do not make the path unique.

## Configuration Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `name` | string | - | Task name for identification |
| `type` | string | `join` | Must be "join" |
| `size` | int | - | Maximum total size (in bytes) before flushing joined records |
| `number` | int | - | Maximum number of records before flushing joined records |
| `duration` | duration | - | Maximum time duration before flushing joined records |
| `delimiter` | string | `\n` | Delimiter used to separate joined records |
| `task_concurrency` | int | `1` | Number of competing-consumer workers for this task |
| `context` | map | - | JQ expressions whose results are stored on each record for downstream tasks |
| `fail_on_error` | bool | `false` | Whether to stop the pipeline if this task encounters an error |

## Example Configurations

### Join all records (no limits configured):
```yaml
tasks:
  - name: join_all
    type: join
    delimiter: "\n"
```

### Join by number of records:
```yaml
tasks:
  - name: join_by_count
    type: join
    number: 100
    delimiter: "\n"
```

### Join by data size:
```yaml
tasks:
  - name: join_by_size
    type: join
    size: 1024
    delimiter: "|"
```

### Join by time duration:
```yaml
tasks:
  - name: join_by_time
    type: join
    duration: "5m"
    delimiter: "\n"
```

### Join with multiple triggers (flushes when first limit is reached):
```yaml
tasks:
  - name: join_flexible
    type: join
    number: 50
    size: 512
    duration: "2m"
    delimiter: "|"
```

## Sample Pipelines

- `test/pipelines/join.yaml` - Join task examples

## Use Cases

- **Data aggregation**: Combine multiple records into a single record
- **Batch processing**: Create batches of data for processing
- **File creation**: Combine multiple lines into a single file
- **Data consolidation**: Merge related data records
- **API batching**: Prepare data for batch API calls
- **Report generation**: Combine data for report creation