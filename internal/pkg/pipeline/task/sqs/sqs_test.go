package sqs

import (
	"context"
	"errors"
	"testing"

	qs "github.com/aws/aws-sdk-go-v2/service/sqs"
)

const (
	standardQueueURL = "https://sqs.us-west-2.amazonaws.com/123456789012/test-queue"
	fifoQueueURL     = "https://sqs.us-west-2.amazonaws.com/123456789012/test-queue.fifo"
)

type fakeClient struct {
	attrs map[string]string
	err   error
	calls int
}

func (f *fakeClient) ReceiveMessage(context.Context, *qs.ReceiveMessageInput, ...func(*qs.Options)) (*qs.ReceiveMessageOutput, error) {
	return &qs.ReceiveMessageOutput{}, nil
}

func (f *fakeClient) DeleteMessage(context.Context, *qs.DeleteMessageInput, ...func(*qs.Options)) (*qs.DeleteMessageOutput, error) {
	return &qs.DeleteMessageOutput{}, nil
}

func (f *fakeClient) SendMessage(context.Context, *qs.SendMessageInput, ...func(*qs.Options)) (*qs.SendMessageOutput, error) {
	return &qs.SendMessageOutput{}, nil
}

func (f *fakeClient) GetQueueAttributes(context.Context, *qs.GetQueueAttributesInput, ...func(*qs.Options)) (*qs.GetQueueAttributesOutput, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &qs.GetQueueAttributesOutput{Attributes: f.attrs}, nil
}

type apiError struct{ code string }

func (e apiError) Error() string     { return e.code }
func (e apiError) ErrorCode() string { return e.code }

func attrs(visible, notVisible, delayed string) map[string]string {
	return map[string]string{
		"ApproximateNumberOfMessages":           visible,
		"ApproximateNumberOfMessagesNotVisible": notVisible,
		"ApproximateNumberOfMessagesDelayed":    delayed,
	}
}

func newTestSQS(url string, exitOnEmpty bool, client *fakeClient, outstanding int32) *sqs {
	s := &sqs{QueueURL: url, ExitOnEmpty: exitOnEmpty, client: client}
	s.outstanding.Store(outstanding)
	return s
}

func TestShouldExitOnEmpty(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		exitOnEmpty bool
		client      *fakeClient
		outstanding int32
		want        bool
		wantCalls   int
	}{
		{"standard: all zero exits", standardQueueURL, true, &fakeClient{attrs: attrs("0", "0", "0")}, 0, true, 1},
		{"standard: messages in flight elsewhere keeps polling", standardQueueURL, true, &fakeClient{attrs: attrs("0", "84", "0")}, 0, false, 1},
		{"standard: visible messages keep polling", standardQueueURL, true, &fakeClient{attrs: attrs("3", "0", "0")}, 0, false, 1},
		{"standard: delayed messages keep polling", standardQueueURL, true, &fakeClient{attrs: attrs("0", "0", "2")}, 0, false, 1},
		{"standard: own in-flight receipts alone do not block exit", standardQueueURL, true, &fakeClient{attrs: attrs("0", "5", "0")}, 5, true, 1},
		{"standard: in flight beyond own receipts keeps polling", standardQueueURL, true, &fakeClient{attrs: attrs("0", "6", "0")}, 5, false, 1},
		{"standard: GetQueueAttributes error keeps polling", standardQueueURL, true, &fakeClient{err: errors.New("boom")}, 0, false, 1},
		{"standard: access denied falls back to legacy exit", standardQueueURL, true, &fakeClient{err: apiError{"AccessDenied"}}, 0, true, 1},
		{"standard: AccessDeniedException falls back to legacy exit", standardQueueURL, true, &fakeClient{err: apiError{"AccessDeniedException"}}, 0, true, 1},
		{"standard: query-compat access denied falls back to legacy exit", standardQueueURL, true, &fakeClient{err: apiError{"AWS.SimpleQueueService.AccessDenied"}}, 0, true, 1},
		{"standard: wrapped access denied falls back to legacy exit", standardQueueURL, true, &fakeClient{err: errors.Join(errors.New("op"), apiError{"AccessDenied"})}, 0, true, 1},
		{"standard: missing attribute keeps polling", standardQueueURL, true, &fakeClient{attrs: map[string]string{"ApproximateNumberOfMessages": "0"}}, 0, false, 1},
		{"standard: non-numeric attribute keeps polling", standardQueueURL, true, &fakeClient{attrs: attrs("0", "x", "0")}, 0, false, 1},
		{"standard: exit_on_empty false never exits", standardQueueURL, false, &fakeClient{attrs: attrs("0", "0", "0")}, 0, false, 0},
		{"fifo: all zero exits", fifoQueueURL, true, &fakeClient{attrs: attrs("0", "0", "0")}, 0, true, 1},
		{"fifo: in flight keeps polling", fifoQueueURL, true, &fakeClient{attrs: attrs("0", "1", "0")}, 0, false, 1},
		{"fifo: own outstanding receipts block exit without asking", fifoQueueURL, true, &fakeClient{attrs: attrs("0", "0", "0")}, 2, false, 0},
		{"fifo: own receipts are not subtracted", fifoQueueURL, true, &fakeClient{attrs: attrs("0", "2", "0")}, 0, false, 1},
		{"fifo: GetQueueAttributes error keeps polling", fifoQueueURL, true, &fakeClient{err: errors.New("boom")}, 0, false, 1},
		{"fifo: access denied keeps polling", fifoQueueURL, true, &fakeClient{err: apiError{"AccessDenied"}}, 0, false, 1},
		{"fifo: exit_on_empty false never exits", fifoQueueURL, false, &fakeClient{attrs: attrs("0", "0", "0")}, 0, false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestSQS(tt.url, tt.exitOnEmpty, tt.client, tt.outstanding)
			if got := s.shouldExitOnEmpty(context.Background()); got != tt.want {
				t.Errorf("shouldExitOnEmpty() = %v, want %v", got, tt.want)
			}
			if tt.client.calls != tt.wantCalls {
				t.Errorf("GetQueueAttributes calls = %d, want %d", tt.client.calls, tt.wantCalls)
			}
		})
	}
}

func TestShouldExitOnEmptyContextCancelled(t *testing.T) {
	for _, url := range []string{standardQueueURL, fifoQueueURL} {
		s := newTestSQS(url, true, &fakeClient{err: context.Canceled}, 0)
		if s.shouldExitOnEmpty(context.Background()) {
			t.Errorf("%s: shouldExitOnEmpty() = true on cancelled context, want false", url)
		}
	}
}
