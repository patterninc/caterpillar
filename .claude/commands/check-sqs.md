Verify that an SQS queue exists and is accessible. The user will provide a queue URL or queue name.

Run these checks:

1. **Queue exists** — `aws sqs get-queue-attributes --queue-url <url> --attribute-names All`
   - If the user gave a name instead of URL: `aws sqs get-queue-url --queue-name <name>`

2. **Queue type** — Report whether it's standard or FIFO (URL ends in `.fifo`).

3. **Key attributes** — Report:
   - `ApproximateNumberOfMessages` (current depth)
   - `ApproximateNumberOfMessagesNotVisible` (in-flight)
   - `VisibilityTimeout`
   - `MessageRetentionPeriod`
   - `MaximumMessageSize`
   - For FIFO: `ContentBasedDeduplication`, `FifoQueue`

4. **Dead letter queue** — Check `RedrivePolicy` for a DLQ. If present, report the DLQ ARN.

5. **Pipeline implications** — Based on the queue attributes, suggest:
   - Whether `exit_on_empty: true` makes sense (if queue has messages vs empty)
   - Whether `message_group_id` is needed (FIFO)
   - If visibility timeout is low, warn about reprocessing risk

Report a clear summary. If the queue doesn't exist or access is denied, explain the error and what IAM permissions are needed.
