Verify that an SNS topic exists and is accessible. The user will provide a topic ARN or topic name.

Run these checks:

1. **Topic exists** — `aws sns get-topic-attributes --topic-arn <arn>`
   - If the user gave a name: list topics and find it: `aws sns list-topics` then match by name

2. **Topic type** — Report whether it's standard or FIFO (ARN ends in `.fifo`).

3. **Key attributes** — Report:
   - `TopicArn`
   - `DisplayName`
   - `SubscriptionsConfirmed` / `SubscriptionsPending`
   - `KmsMasterKeyId` (if encrypted)
   - For FIFO: `FifoTopic`, `ContentBasedDeduplication`

4. **Subscriptions** — `aws sns list-subscriptions-by-topic --topic-arn <arn>`
   - Report protocol and endpoint for each (SQS, Lambda, email, HTTP, etc.)

5. **Pipeline implications** — Based on attributes, suggest:
   - Whether `message_group_id` is needed (FIFO topic)
   - Note that `sns` is a terminal sink — no tasks can follow it

Report a clear summary. If the topic doesn't exist or access is denied, explain the error and what IAM permissions are needed (`sns:GetTopicAttributes`, `sns:Publish`).
