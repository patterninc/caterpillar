//go:build integration

package sqs_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	qs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/sqs"
)

const sqsSettleTimeout = 10 * time.Second

// sqsIntegration skips unless an endpoint override is set, so the integration
// tag on its own can never reach real AWS.
func sqsIntegration(t *testing.T) *qs.Client {

	t.Helper()

	if os.Getenv("AWS_ENDPOINT_URL") == "" {
		t.Skip("AWS_ENDPOINT_URL is unset; integration tests run only against LocalStack")
	}

	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-east-1"
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		t.Fatalf("loading AWS config: %v", err)
	}

	return qs.NewFromConfig(cfg)

}

// sqsQueue creates a uniquely named queue and deletes it when the test ends.
func sqsQueue(t *testing.T, client *qs.Client, fifo bool) string {

	t.Helper()

	name := "caterpillar-it-" + uuid.NewString()[:13]
	attrs := map[string]string{}
	if fifo {
		name += ".fifo"
		attrs[string(types.QueueAttributeNameFifoQueue)] = "true"
		attrs[string(types.QueueAttributeNameContentBasedDeduplication)] = "true"
	}

	out, err := client.CreateQueue(context.Background(), &qs.CreateQueueInput{QueueName: aws.String(name), Attributes: attrs})
	if err != nil {
		t.Fatalf("creating queue %s: %v", name, err)
	}
	url := aws.ToString(out.QueueUrl)

	t.Cleanup(func() {
		if _, err := client.DeleteQueue(context.Background(), &qs.DeleteQueueInput{QueueUrl: aws.String(url)}); err != nil {
			t.Errorf("deleting queue %s: %v", url, err)
		}
	})

	return url

}

func newSQSTask(t *testing.T, spec string) task.Task {

	t.Helper()

	tk, err := sqs.New()
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

// sqsReceiveAll polls until want messages arrive or the deadline passes. Each
// received message is deleted so a slow poll never sees it twice.
func sqsReceiveAll(t *testing.T, client *qs.Client, url string, want int) []types.Message {

	t.Helper()

	var got []types.Message
	deadline := time.Now().Add(sqsSettleTimeout)

	for len(got) < want && time.Now().Before(deadline) {
		out, err := client.ReceiveMessage(context.Background(), &qs.ReceiveMessageInput{
			QueueUrl:                    aws.String(url),
			MaxNumberOfMessages:         10,
			WaitTimeSeconds:             1,
			MessageSystemAttributeNames: []types.MessageSystemAttributeName{types.MessageSystemAttributeNameAll},
		})
		if err != nil {
			t.Fatalf("ReceiveMessage: %v", err)
		}
		for _, m := range out.Messages {
			got = append(got, m)
			if _, err := client.DeleteMessage(context.Background(), &qs.DeleteMessageInput{QueueUrl: aws.String(url), ReceiptHandle: m.ReceiptHandle}); err != nil {
				t.Fatalf("DeleteMessage: %v", err)
			}
		}
	}

	return got

}

func sqsBodies(messages []types.Message) []string {
	bodies := make([]string, 0, len(messages))
	for _, m := range messages {
		bodies = append(bodies, aws.ToString(m.Body))
	}
	sort.Strings(bodies)
	return bodies
}

func sqsTracked(n int) ([]*record.Record, []*ack.Ack) {
	records := make([]*record.Record, n)
	acks := make([]*ack.Ack, n)
	for i := range records {
		a := ack.New()
		a.AddBranch(1)
		acks[i] = a
		records[i] = &record.Record{
			ID:      i + 1,
			Data:    []byte(`{"n":` + strconv.Itoa(i) + `}`),
			Context: ack.WithContext(context.Background(), a),
		}
	}
	return records, acks
}

func sqsAssertSettled(t *testing.T, acks []*ack.Ack, wantFailed bool) {

	t.Helper()

	for i, a := range acks {
		select {
		case <-a.Wait():
			if got := a.Failed(); got != wantFailed {
				t.Errorf("record %d: Failed() = %v, want %v", i, got, wantFailed)
			}
		case <-time.After(sqsSettleTimeout):
			t.Fatalf("record %d never settled", i)
		}
	}

}

func sqsQueueDepth(t *testing.T, client *qs.Client, url string) (visible, inFlight int) {

	t.Helper()

	out, err := client.GetQueueAttributes(context.Background(), &qs.GetQueueAttributesInput{
		QueueUrl: aws.String(url),
		AttributeNames: []types.QueueAttributeName{
			types.QueueAttributeNameApproximateNumberOfMessages,
			types.QueueAttributeNameApproximateNumberOfMessagesNotVisible,
		},
	})
	if err != nil {
		t.Fatalf("GetQueueAttributes: %v", err)
	}

	visible, _ = strconv.Atoi(out.Attributes[string(types.QueueAttributeNameApproximateNumberOfMessages)])
	inFlight, _ = strconv.Atoi(out.Attributes[string(types.QueueAttributeNameApproximateNumberOfMessagesNotVisible)])

	return visible, inFlight

}

func TestIntegrationSQSWrite(t *testing.T) {

	tests := []struct {
		name string
		fifo bool
	}{
		{name: "standard queue"},
		{name: "fifo queue", fifo: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			client := sqsIntegration(t)
			url := sqsQueue(t, client, tt.fifo)

			tk := newSQSTask(t, fmt.Sprintf("name: write\ntype: sqs\nqueue_url: %s\nmessage_group_id: it-group\n", url))

			records, acks := sqsTracked(3)
			input := make(chan *record.Record, len(records))
			for _, r := range records {
				input <- r
			}
			close(input)

			if err := tk.Run(input, nil); err != nil {
				t.Fatalf("Run: %v", err)
			}
			sqsAssertSettled(t, acks, false)

			messages := sqsReceiveAll(t, client, url, 3)
			if got, want := sqsBodies(messages), []string{`{"n":0}`, `{"n":1}`, `{"n":2}`}; fmt.Sprint(got) != fmt.Sprint(want) {
				t.Errorf("received bodies = %q, want %q", got, want)
			}

			if tt.fifo {
				for _, m := range messages {
					if g := m.Attributes[string(types.MessageSystemAttributeNameMessageGroupId)]; g != "it-group" {
						t.Errorf("message group id = %q, want it-group", g)
					}
				}
			}

		})
	}

}

func TestIntegrationSQSRead(t *testing.T) {

	client := sqsIntegration(t)
	url := sqsQueue(t, client, false)

	want := []string{"first", "second", "third"}
	for _, body := range want {
		if _, err := client.SendMessage(context.Background(), &qs.SendMessageInput{QueueUrl: aws.String(url), MessageBody: aws.String(body)}); err != nil {
			t.Fatalf("SendMessage: %v", err)
		}
	}

	tk := newSQSTask(t, fmt.Sprintf("name: read\ntype: sqs\nqueue_url: %s\nexit_on_empty: true\nwait_time_seconds: 1\n", url))

	output := make(chan *record.Record, 16)
	if err := tk.Run(nil, output); err != nil {
		t.Fatalf("Run: %v", err)
	}
	close(output)

	var got []string
	for r := range output {
		got = append(got, string(r.Data))
		ack.Release(r.Context)
	}
	sort.Strings(got)
	sort.Strings(want)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("emitted bodies = %q, want %q", got, want)
	}

	finisher, ok := tk.(task.Finisher)
	if !ok {
		t.Fatal("sqs task does not implement task.Finisher")
	}
	done := make(chan error, 1)
	go func() { done <- finisher.Finish() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Finish: %v", err)
		}
	case <-time.After(sqsSettleTimeout):
		t.Fatal("Finish never returned: deferred deletions are stuck")
	}

	if visible, inFlight := sqsQueueDepth(t, client, url); visible != 0 || inFlight != 0 {
		t.Errorf("queue depth after acked read = %d visible, %d in flight, want 0 and 0", visible, inFlight)
	}

}

func TestIntegrationSQSReadRejectedIsNotDeleted(t *testing.T) {

	client := sqsIntegration(t)
	url := sqsQueue(t, client, false)

	if _, err := client.SendMessage(context.Background(), &qs.SendMessageInput{QueueUrl: aws.String(url), MessageBody: aws.String("poison")}); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	tk := newSQSTask(t, fmt.Sprintf("name: read\ntype: sqs\nqueue_url: %s\nexit_on_empty: true\nwait_time_seconds: 1\n", url))

	output := make(chan *record.Record, 4)
	if err := tk.Run(nil, output); err != nil {
		t.Fatalf("Run: %v", err)
	}
	close(output)

	n := 0
	for r := range output {
		n++
		ack.Reject(r.Context)
	}
	if n != 1 {
		t.Fatalf("emitted %d records, want 1", n)
	}

	if err := tk.(task.Finisher).Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	if visible, inFlight := sqsQueueDepth(t, client, url); visible+inFlight != 1 {
		t.Errorf("queue depth after rejected read = %d visible, %d in flight, want the message kept", visible, inFlight)
	}

}
