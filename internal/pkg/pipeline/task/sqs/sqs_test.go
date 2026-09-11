package sqs

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	qs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

type fakeSQS struct {
	mu       sync.Mutex
	receives [][]types.Message
	depths   []map[string]string
	depthErr []error

	receiveN int
	depthN   int
}

func (f *fakeSQS) ReceiveMessage(context.Context, *qs.ReceiveMessageInput, ...func(*qs.Options)) (*qs.ReceiveMessageOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i := f.receiveN
	f.receiveN++
	if i >= len(f.receives) {
		return &qs.ReceiveMessageOutput{}, nil
	}
	return &qs.ReceiveMessageOutput{Messages: f.receives[i]}, nil
}

func (f *fakeSQS) DeleteMessage(context.Context, *qs.DeleteMessageInput, ...func(*qs.Options)) (*qs.DeleteMessageOutput, error) {
	return &qs.DeleteMessageOutput{}, nil
}

func (f *fakeSQS) SendMessage(context.Context, *qs.SendMessageInput, ...func(*qs.Options)) (*qs.SendMessageOutput, error) {
	return &qs.SendMessageOutput{}, nil
}

func (f *fakeSQS) GetQueueAttributes(context.Context, *qs.GetQueueAttributesInput, ...func(*qs.Options)) (*qs.GetQueueAttributesOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	i := f.depthN
	f.depthN++
	if i < len(f.depthErr) && f.depthErr[i] != nil {
		return nil, f.depthErr[i]
	}
	if i >= len(f.depths) {
		return &qs.GetQueueAttributesOutput{Attributes: map[string]string{
			string(types.QueueAttributeNameApproximateNumberOfMessages):           "0",
			string(types.QueueAttributeNameApproximateNumberOfMessagesNotVisible): "0",
			string(types.QueueAttributeNameApproximateNumberOfMessagesDelayed):    "0",
		}}, nil
	}
	return &qs.GetQueueAttributesOutput{Attributes: f.depths[i]}, nil
}

func (f *fakeSQS) receiveCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.receiveN
}

func (f *fakeSQS) attrCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.depthN
}

func fifoReader(client sqsClient) *sqs {
	return &sqs{
		QueueURL:        "https://sqs.us-west-2.amazonaws.com/123/q.fifo",
		ExitOnEmpty:     true,
		MaxMessages:     defaultMaxMessages,
		WaitTimeSeconds: 1,
		client:          client,
		tracker:         ack.NewTracker(1),
	}
}

func depth(visible, notVisible, delayed int) map[string]string {
	return map[string]string{
		string(types.QueueAttributeNameApproximateNumberOfMessages):           strconv.Itoa(visible),
		string(types.QueueAttributeNameApproximateNumberOfMessagesNotVisible): strconv.Itoa(notVisible),
		string(types.QueueAttributeNameApproximateNumberOfMessagesDelayed):    strconv.Itoa(delayed),
	}
}

func TestGetMessagesFIFODoesNotExitWhileQueueHasVisibleMessages(t *testing.T) {
	fake := &fakeSQS{
		receives: [][]types.Message{{}, {}},
		depths:   []map[string]string{depth(37876, 0, 0), depth(0, 0, 0)},
	}

	if err := fifoReader(fake).getMessages(context.Background(), make(chan *record.Record, 1)); err != nil {
		t.Fatalf("getMessages: %v", err)
	}
	if got := fake.receiveCount(); got != 2 {
		t.Fatalf("receives = %d, want 2 (empty poll with visible messages is not drain)", got)
	}
}

func TestGetMessagesFIFODoesNotExitWhileMessagesAreInFlightElsewhere(t *testing.T) {
	fake := &fakeSQS{
		receives: [][]types.Message{{}, {}},
		depths:   []map[string]string{depth(0, 10, 0), depth(0, 0, 0)},
	}

	if err := fifoReader(fake).getMessages(context.Background(), make(chan *record.Record, 1)); err != nil {
		t.Fatalf("getMessages: %v", err)
	}
	if got := fake.receiveCount(); got != 2 {
		t.Fatalf("receives = %d, want 2 (empty poll with in-flight messages is not drain)", got)
	}
}

func TestGetMessagesFIFODoesNotExitWhileMessagesAreDelayed(t *testing.T) {
	fake := &fakeSQS{
		receives: [][]types.Message{{}, {}},
		depths:   []map[string]string{depth(0, 0, 5), depth(0, 0, 0)},
	}

	if err := fifoReader(fake).getMessages(context.Background(), make(chan *record.Record, 1)); err != nil {
		t.Fatalf("getMessages: %v", err)
	}
	if got := fake.receiveCount(); got != 2 {
		t.Fatalf("receives = %d, want 2 (empty poll with delayed messages is not drain)", got)
	}
}

func TestGetMessagesFIFODoesNotExitWhenQueueAttributesFail(t *testing.T) {
	fake := &fakeSQS{
		receives: [][]types.Message{{}, {}},
		depths:   []map[string]string{nil, depth(0, 0, 0)},
		depthErr: []error{errors.New("denied"), nil},
	}

	if err := fifoReader(fake).getMessages(context.Background(), make(chan *record.Record, 1)); err != nil {
		t.Fatalf("getMessages: %v", err)
	}
	if got := fake.receiveCount(); got != 2 {
		t.Fatalf("receives = %d, want 2 (attribute errors must not look like drain)", got)
	}
}

func TestGetMessagesFIFODoesNotExitWhenQueueAttributesAreIncomplete(t *testing.T) {
	fake := &fakeSQS{
		receives: [][]types.Message{{}, {}},
		depths: []map[string]string{
			{string(types.QueueAttributeNameApproximateNumberOfMessages): "0"},
			depth(0, 0, 0),
		},
	}

	if err := fifoReader(fake).getMessages(context.Background(), make(chan *record.Record, 1)); err != nil {
		t.Fatalf("getMessages: %v", err)
	}
	if got := fake.receiveCount(); got != 2 {
		t.Fatalf("receives = %d, want 2 (partial attributes are not drain)", got)
	}
}

func TestGetMessagesStandardQueueExitsOnEmptyWithoutAttributes(t *testing.T) {
	fake := &fakeSQS{
		receives: [][]types.Message{{}},
		depthErr: []error{errors.New("GetQueueAttributes should not be called for standard queues")},
	}
	s := fifoReader(fake)
	s.QueueURL = "https://sqs.us-west-2.amazonaws.com/123/q"

	if err := s.getMessages(context.Background(), make(chan *record.Record, 1)); err != nil {
		t.Fatalf("getMessages: %v", err)
	}
	if got := fake.receiveCount(); got != 1 {
		t.Fatalf("receives = %d, want 1", got)
	}
	if got := fake.attrCount(); got != 0 {
		t.Fatalf("GetQueueAttributes calls = %d, want 0", got)
	}
}

func TestGetMessagesFIFOKeepsPollingWhileThisProcessHoldsReceipts(t *testing.T) {
	id, handle := "m1", "rh1"
	fake := &fakeSQS{
		receives: [][]types.Message{{
			{MessageId: &id, ReceiptHandle: &handle, Body: aws.String("body")},
		}},
		depths: []map[string]string{depth(0, 0, 0)},
	}
	s := fifoReader(fake)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := s.getMessages(ctx, make(chan *record.Record, 1))
	if err != nil {
		t.Fatalf("getMessages: %v", err)
	}
	if fake.receiveCount() < 2 {
		t.Fatalf("receives = %d, want at least 2 (outstanding receipt is not drain)", fake.receiveCount())
	}
}
