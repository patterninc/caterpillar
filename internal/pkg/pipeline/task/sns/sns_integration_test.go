//go:build integration

package sns_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/sns"
)

const snsSettleTimeout = 10 * time.Second

type snsFixture struct {
	region   string
	topicArn string
	queueURL string
	sqs      *awssqs.Client
}

// snsIntegration skips unless an endpoint override is set, so the integration
// tag on its own can never reach real AWS. It wires a fresh topic to a fresh
// queue with raw delivery, so what the task publishes arrives byte for byte.
func snsIntegration(t *testing.T) snsFixture {

	t.Helper()

	if os.Getenv("AWS_ENDPOINT_URL") == "" {
		t.Skip("AWS_ENDPOINT_URL is unset; integration tests run only against LocalStack")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		t.Fatalf("loading AWS config: %v", err)
	}
	snsClient := awssns.NewFromConfig(cfg)
	sqsClient := awssqs.NewFromConfig(cfg)

	name := "caterpillar-it-" + uuid.NewString()[:13]

	topic, err := snsClient.CreateTopic(ctx, &awssns.CreateTopicInput{Name: aws.String(name)})
	if err != nil {
		t.Fatalf("creating topic %s: %v", name, err)
	}
	topicArn := aws.ToString(topic.TopicArn)
	t.Cleanup(func() {
		if _, err := snsClient.DeleteTopic(ctx, &awssns.DeleteTopicInput{TopicArn: aws.String(topicArn)}); err != nil {
			t.Errorf("deleting topic %s: %v", topicArn, err)
		}
	})

	queue, err := sqsClient.CreateQueue(ctx, &awssqs.CreateQueueInput{QueueName: aws.String(name)})
	if err != nil {
		t.Fatalf("creating queue %s: %v", name, err)
	}
	queueURL := aws.ToString(queue.QueueUrl)
	t.Cleanup(func() {
		if _, err := sqsClient.DeleteQueue(ctx, &awssqs.DeleteQueueInput{QueueUrl: aws.String(queueURL)}); err != nil {
			t.Errorf("deleting queue %s: %v", queueURL, err)
		}
	})

	attrs, err := sqsClient.GetQueueAttributes(ctx, &awssqs.GetQueueAttributesInput{
		QueueUrl:       aws.String(queueURL),
		AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn},
	})
	if err != nil {
		t.Fatalf("getting queue ARN: %v", err)
	}

	sub, err := snsClient.Subscribe(ctx, &awssns.SubscribeInput{
		TopicArn:   aws.String(topicArn),
		Protocol:   aws.String("sqs"),
		Endpoint:   aws.String(attrs.Attributes[string(sqstypes.QueueAttributeNameQueueArn)]),
		Attributes: map[string]string{"RawMessageDelivery": "true"},
	})
	if err != nil {
		t.Fatalf("subscribing queue to topic: %v", err)
	}
	t.Cleanup(func() {
		if _, err := snsClient.Unsubscribe(ctx, &awssns.UnsubscribeInput{SubscriptionArn: sub.SubscriptionArn}); err != nil {
			t.Errorf("unsubscribing: %v", err)
		}
	})

	return snsFixture{region: region, topicArn: topicArn, queueURL: queueURL, sqs: sqsClient}

}

func newSNSTask(t *testing.T, spec string) task.Task {

	t.Helper()

	tk, err := sns.New()
	if err != nil {
		t.Fatalf("constructing task: %v", err)
	}
	if err := yaml.Unmarshal([]byte(spec), tk); err != nil {
		t.Fatalf("unmarshalling %q: %v", spec, err)
	}
	if err := tk.Init(); err != nil {
		t.Fatalf("initialising task: %v", err)
	}

	return tk

}

func TestIntegrationSNSPublish(t *testing.T) {

	fx := snsIntegration(t)

	tk := newSNSTask(t, fmt.Sprintf(`
name: publish
type: sns
topic_arn: %s
region: %s
attributes:
  - name: source
    type: String
    value: caterpillar
`, fx.topicArn, fx.region))

	want := []string{`{"id":1}`, `{"id":2}`}
	acks := make([]*ack.Ack, len(want))
	input := make(chan *record.Record, len(want))
	for i, body := range want {
		a := ack.New()
		a.AddBranch(1)
		acks[i] = a
		input <- &record.Record{ID: i + 1, Data: []byte(body), Context: ack.WithContext(context.Background(), a)}
	}
	close(input)

	if err := tk.Run(input, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}

	for i, a := range acks {
		select {
		case <-a.Wait():
			if a.Failed() {
				t.Errorf("record %d was rejected after a successful publish", i)
			}
		case <-time.After(snsSettleTimeout):
			t.Fatalf("record %d never settled", i)
		}
	}

	var got []string
	deadline := time.Now().Add(snsSettleTimeout)
	for len(got) < len(want) && time.Now().Before(deadline) {
		out, err := fx.sqs.ReceiveMessage(context.Background(), &awssqs.ReceiveMessageInput{
			QueueUrl:              aws.String(fx.queueURL),
			MaxNumberOfMessages:   10,
			WaitTimeSeconds:       1,
			MessageAttributeNames: []string{"All"},
		})
		if err != nil {
			t.Fatalf("ReceiveMessage: %v", err)
		}
		for _, m := range out.Messages {
			got = append(got, aws.ToString(m.Body))
			if v := aws.ToString(m.MessageAttributes["source"].StringValue); v != "caterpillar" {
				t.Errorf("message attribute source = %q, want caterpillar", v)
			}
			if _, err := fx.sqs.DeleteMessage(context.Background(), &awssqs.DeleteMessageInput{QueueUrl: aws.String(fx.queueURL), ReceiptHandle: m.ReceiptHandle}); err != nil {
				t.Fatalf("DeleteMessage: %v", err)
			}
		}
	}

	sort.Strings(got)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("delivered bodies = %q, want %q", got, want)
	}

}

func TestIntegrationSNSPublishToMissingTopicRejects(t *testing.T) {

	fx := snsIntegration(t)

	tk := newSNSTask(t, fmt.Sprintf("name: publish\ntype: sns\ntopic_arn: %s-missing\nregion: %s\n", fx.topicArn, fx.region))

	a := ack.New()
	a.AddBranch(1)
	input := make(chan *record.Record, 1)
	input <- &record.Record{ID: 1, Data: []byte("x"), Context: ack.WithContext(context.Background(), a)}
	close(input)

	if err := tk.Run(input, nil); err == nil {
		t.Fatal("Run publishing to a missing topic returned nil, want an error")
	}

	select {
	case <-a.Wait():
		if !a.Failed() {
			t.Error("record was not rejected after a failed publish")
		}
	case <-time.After(snsSettleTimeout):
		t.Fatal("record never settled")
	}

}
