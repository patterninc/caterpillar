#!/usr/bin/env bash
set -euo pipefail

# Creates/refills the queue used by sqs_fanout_fanin_dag.yaml against a
# LocalStack instance on localhost:4566. Each message carries an array, so the
# pipeline's jq explode step fans one message out into ITEMS_PER_MESSAGE
# records.
#
# Requires: LocalStack running (community image, e.g.
# `docker run -d -p 4566:4566 localstack/localstack:4.0`), plus aws-cli and jq.

ENDPOINT="http://localhost:4566"
REGION="us-west-2"
QUEUE_NAME="local-sqs-fanout-fanin-queue"   # matches queue_url in sqs_fanout_fanin_dag.yaml
# Override either to change the shape of the run. 20 messages keeps it quick;
# above ~50 it also exercises the case where a source with a bounded
# unacknowledged-message window would stall against the join downstream.
TOTAL_MESSAGES="${TOTAL_MESSAGES:-20}"
ITEMS_PER_MESSAGE="${ITEMS_PER_MESSAGE:-5}"
BATCH_SIZE=10           # SQS SendMessageBatch max

export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}"
export AWS_DEFAULT_REGION="$REGION"
export AWS_PAGER=

awscli() {
  aws --endpoint-url "$ENDPOINT" --region "$REGION" "$@"
}

echo "Creating queue '$QUEUE_NAME'..."
QUEUE_URL=$(awscli sqs create-queue --queue-name "$QUEUE_NAME" --query 'QueueUrl' --output text)
echo "Queue URL: $QUEUE_URL"

echo "Pushing $TOTAL_MESSAGES messages of $ITEMS_PER_MESSAGE items each..."
for ((batch_start=0; batch_start<TOTAL_MESSAGES; batch_start+=BATCH_SIZE)); do
  entries="[]"
  for ((i=0; i<BATCH_SIZE && batch_start+i<TOTAL_MESSAGES; i++)); do
    idx=$((batch_start + i))
    entries=$(jq -c \
      --arg id "msg-$idx" \
      --argjson idx "$idx" \
      --argjson n "$ITEMS_PER_MESSAGE" \
      '. + [{Id: $id, MessageBody: ({id: $idx, items: [range($n)| . + 1]} | tojson)}]' \
      <<<"$entries")
  done
  awscli sqs send-message-batch --queue-url "$QUEUE_URL" --entries "$entries" >/dev/null
done

expected_records=$((TOTAL_MESSAGES * ITEMS_PER_MESSAGE * 2))
echo
echo "Sent $TOTAL_MESSAGES messages."
echo "Expected: $expected_records records into join, $((expected_records / 4)) output files."
echo "A correct run ends with both queue depths at 0."
