#!/usr/bin/env bash
set -euo pipefail

# Creates/refills the local SQS queue used by sqs_with_context_concurrency.yaml
# against a LocalStack instance running on localhost:4566.
#
# Requires: LocalStack running (`localstack start` or the localstack/localstack
# docker image), plus aws-cli and jq installed locally.

ENDPOINT="http://localhost:4566"
REGION="us-west-2"
QUEUE_NAME="local-sqs-context-concurrency-queue"   # matches queue_url in sqs_with_context_concurrency.yaml
TOTAL_MESSAGES=100
BATCH_SIZE=10           # SQS SendMessageBatch max

export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}"
export AWS_DEFAULT_REGION="$REGION"

awscli() {
  aws --endpoint-url "$ENDPOINT" --region "$REGION" "$@"
}

echo "Creating queue '$QUEUE_NAME'..."
QUEUE_URL=$(awscli sqs create-queue --queue-name "$QUEUE_NAME" --query 'QueueUrl' --output text)
echo "Queue URL: $QUEUE_URL"

echo "Pushing $TOTAL_MESSAGES random messages..."
for ((batch_start=0; batch_start<TOTAL_MESSAGES; batch_start+=BATCH_SIZE)); do
  entries="[]"
  for ((i=0; i<BATCH_SIZE; i++)); do
    idx=$((batch_start + i))
    rand=$(openssl rand -hex 6)
    entries=$(jq -c \
      --arg id "msg-$idx" \
      --arg url "https://example.com/item/$idx?rand=$rand" \
      --argjson idx "$idx" \
      '. + [{Id: $id, MessageBody: ({url: $url, id: $idx} | tojson)}]' \
      <<<"$entries")
  done
  awscli sqs send-message-batch --queue-url "$QUEUE_URL" --entries "$entries" >/dev/null
done

echo "Done. Sent $TOTAL_MESSAGES messages to queue '$QUEUE_NAME'."
echo
echo "Queue URL (LocalStack): $QUEUE_URL"
echo
echo "To point the pipeline at LocalStack, run it with:"
echo "  AWS_ENDPOINT_URL=$ENDPOINT AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_DEFAULT_REGION=$REGION go run cmd/caterpillar/caterpillar.go -conf test/pipelines/sqs_with_context_concurrency.yaml"
