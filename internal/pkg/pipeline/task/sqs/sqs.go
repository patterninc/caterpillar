package sqs

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	qs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

const (
	defaultConcurrency     = 10
	defaultMaxMessages     = 10
	defaultWaitTimeSeconds = 10
	defaultRegion          = "us-west-2"
)

type deliveryMode string

const (
	deliveryAtMostOnce  deliveryMode = "at-most-once"
	deliveryAtLeastOnce deliveryMode = "at-least-once"
)

var (
	awsRegionRegex = regexp.MustCompile(`^[a-z]{2}-[a-z]+-\d+$`)
	ctx            = context.Background()

	queueDrainAttributes = []types.QueueAttributeName{
		types.QueueAttributeNameApproximateNumberOfMessages,
		types.QueueAttributeNameApproximateNumberOfMessagesNotVisible,
		types.QueueAttributeNameApproximateNumberOfMessagesDelayed,
	}
)

// sqsClient is the subset of *qs.Client this task uses, so tests can fake it.
type sqsClient interface {
	ReceiveMessage(context.Context, *qs.ReceiveMessageInput, ...func(*qs.Options)) (*qs.ReceiveMessageOutput, error)
	DeleteMessage(context.Context, *qs.DeleteMessageInput, ...func(*qs.Options)) (*qs.DeleteMessageOutput, error)
	SendMessage(context.Context, *qs.SendMessageInput, ...func(*qs.Options)) (*qs.SendMessageOutput, error)
	GetQueueAttributes(context.Context, *qs.GetQueueAttributesInput, ...func(*qs.Options)) (*qs.GetQueueAttributesOutput, error)
}

type sqs struct {
	task.ServerBase `yaml:",inline" json:",inline"`
	QueueURL        string       `yaml:"queue_url" json:"queue_url" validate:"required"`
	Concurrency     int          `yaml:"concurrency,omitempty" json:"concurrency,omitempty"`
	MaxMessages     int32        `yaml:"max_messages,omitempty" json:"max_messages,omitempty"`
	MaxRecords      int          `yaml:"max_records,omitempty" json:"max_records,omitempty" validate:"omitempty,gte=0"`
	WaitTimeSeconds int          `yaml:"wait_time_seconds,omitempty" json:"wait_time_seconds,omitempty"`
	ExitOnEmpty     bool         `yaml:"exit_on_empty,omitempty" json:"exit_on_empty,omitempty"`
	MessageGroupId  string       `yaml:"message_group_id,omitempty" json:"message_group_id,omitempty"` // used for FIFO queues
	Delivery        deliveryMode `yaml:"delivery,omitempty" json:"delivery,omitempty"`

	client      sqsClient
	tracker     *ack.Tracker
	outstanding atomic.Int32
}

func New() (task.Task, error) {

	return &sqs{
		Concurrency:     defaultConcurrency,
		MaxMessages:     defaultMaxMessages,
		WaitTimeSeconds: defaultWaitTimeSeconds,
	}, nil

}

// Init initializes the SQS client before pipeline execution
// This is called once during task unmarshaling, before any goroutines are spawned
func (s *sqs) Init() error {
	if s.QueueURL == "" {
		return fmt.Errorf("queue_url is required")
	}

	switch s.Delivery {
	case "", deliveryAtLeastOnce:
		s.Delivery = deliveryAtLeastOnce
	case deliveryAtMostOnce:
	default:
		return fmt.Errorf("invalid delivery mode %q: must be %q or %q", s.Delivery, deliveryAtMostOnce, deliveryAtLeastOnce)
	}

	s.tracker = ack.NewTracker(s.Concurrency)

	region := s.extractRegionFromQueueURL()
	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	s.client = qs.NewFromConfig(awsConfig)

	return nil
}

func (s *sqs) extractRegionFromQueueURL() string {
	// Split the URL by dots to extract the region
	// https://sqs.us-west-2.amazonaws.com/84212345678/test-sqs

	parts := strings.Split(s.QueueURL, ".")

	if len(parts) >= 2 {
		region := parts[1]
		if awsRegionRegex.MatchString(region) {
			return region
		}
	}

	return defaultRegion
}

func (s *sqs) Run(input <-chan *record.Record, output chan<- *record.Record) error {

	// Client is already initialized in Init - just use it
	if input != nil {
		return s.sendMessages(input)
	}

	// If input is nil, act as a source: read messages and hand each receipt to
	// the tracker, which deletes it once every downstream task has finished
	// with the record it produced. Finish, not Run, waits for those deletions.
	return s.getMessages(ctx, output)

}

// Finish waits for every deferred deletion before the pipeline treats this
// task as complete, so a shutdown never abandons in-flight acknowledgements.
// It can't happen in Run: downstream tasks that emit only once their input
// closes can't finish with a record until this task's output channel is
// closed, which the pipeline does only after Run returns.
func (s *sqs) Finish() error {

	if s.tracker != nil {
		s.tracker.Wait()
	}

	return nil

}

func (s *sqs) getMessages(ctx context.Context, output chan<- *record.Record) error {

	// do we need to stop pipeline after a while?
	if s.EndAfter > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(s.EndAfter))
		defer cancel()
	}

	recordsRead := 0
	for {
		select {
		case <-ctx.Done():
			// the time has come to stop the pipeline...
			return nil

		default:
			receiveMessageOutput, err := s.client.ReceiveMessage(ctx, &qs.ReceiveMessageInput{
				QueueUrl:            &s.QueueURL,
				MaxNumberOfMessages: s.MaxMessages,
				WaitTimeSeconds:     int32(s.WaitTimeSeconds),
			})

			if err != nil {
				// not a real error, just normal shutdown
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					fmt.Println(`SQS message retrieval cancelled: `, err)
					return nil
				}
				// otherwise, this is a real error
				fmt.Println(`ReceiveMessage:`, err)
				return err
			}

			if receiveMessageOutput == nil || len(receiveMessageOutput.Messages) == 0 {
				if s.shouldExitOnEmpty(ctx) {
					fmt.Println(`Queue is empty, exiting`)
					return nil
				}
				continue
			}

			for _, m := range receiveMessageOutput.Messages {

				if output == nil {
					s.deleteMessage(m.MessageId, m.ReceiptHandle)
				} else if s.Delivery == deliveryAtMostOnce {
					s.SendData(ctx, []byte(*m.Body), output)
					del := ack.New()
					del.AddBranch(1)
					del.Done()
					s.enqueueDelete(m, del)
				} else {
					msgAck := ack.New()
					s.SendData(ack.WithContext(ctx, msgAck), []byte(*m.Body), output)
					s.enqueueDelete(m, msgAck)
				}

				recordsRead++
				if s.MaxRecords > 0 && recordsRead >= s.MaxRecords {
					fmt.Printf("SQS max_records (%d) reached for queue %s, stopping reader\n", s.MaxRecords, s.QueueURL)
					return nil
				}
			}
		}
	}

}

func (s *sqs) enqueueDelete(m types.Message, a *ack.Ack) {
	s.outstanding.Add(1)
	s.tracker.Track(a, &messageAck{
		sqs:           s,
		messageId:     m.MessageId,
		receiptHandle: m.ReceiptHandle,
	})
}

// messageAck acknowledges one received message on behalf of ack.Tracker.
type messageAck struct {
	sqs           *sqs
	messageId     *string
	receiptHandle *string
}

// Ack deletes the message's receipt so SQS doesn't redeliver it. On a
// downstream failure it does nothing: leaving the receipt alone lets SQS
// redeliver the message once the visibility timeout expires.
func (m *messageAck) Ack(failed bool) {

	defer m.sqs.outstanding.Add(-1)

	if failed {
		return
	}

	m.sqs.deleteMessage(m.messageId, m.receiptHandle)

}

// queueDepth is the approximate message count reported by GetQueueAttributes.
type queueDepth struct {
	visible  int64
	inFlight int64
	delayed  int64
}

// shouldExitOnEmpty decides whether an empty ReceiveMessage means the queue is
// drained. An empty receive alone proves nothing: messages another consumer
// (or an earlier failed attempt) received but did not delete stay invisible
// until the visibility timeout ends and then reappear with no reader left.
// So the reader exits only when GetQueueAttributes shows no visible, in-flight
// or delayed messages, ignoring the receipts this process itself still holds
// on standard queues.
//
// FIFO queues are stricter: any receipt held by this process blocks the exit,
// because an in-flight message also blocks its whole message group.
func (s *sqs) shouldExitOnEmpty(ctx context.Context) bool {
	if !s.ExitOnEmpty {
		return false
	}

	held := int64(s.outstanding.Load())
	fifo := s.isFifo()
	if fifo && held > 0 {
		return false
	}

	depth, err := s.queueDepth(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		if !fifo && isAccessDenied(err) {
			// Legacy behaviour for standard queues whose role lacks
			// sqs:GetQueueAttributes: exit on the first empty receive.
			fmt.Printf("WARN: GetQueueAttributes denied (%v); sqs:GetQueueAttributes is required to confirm the queue is drained, exiting on empty receive without confirmation\n", err)
			return true
		}
		fmt.Println(`GetQueueAttributes:`, err)
		return false
	}

	// Receipts held by this process are counted as in flight by SQS; they are
	// not a reason to keep polling on a standard queue, where a downstream task
	// (e.g. an unflushed join) may hold them until the output channel closes.
	if !fifo {
		depth.inFlight -= held
	}

	if depth.visible > 0 || depth.inFlight > 0 || depth.delayed > 0 {
		fmt.Printf("Queue still has messages (visible=%d, in_flight_elsewhere=%d, delayed=%d), continuing\n", depth.visible, max(depth.inFlight, 0), depth.delayed)
		return false
	}

	return true
}

// queueDepth reads the approximate visible, in-flight and delayed counts. A
// missing or non-numeric attribute is reported as an error or "not drained"
// rather than as zero.
func (s *sqs) queueDepth(ctx context.Context) (queueDepth, error) {
	var depth queueDepth

	out, err := s.client.GetQueueAttributes(ctx, &qs.GetQueueAttributesInput{
		QueueUrl:       &s.QueueURL,
		AttributeNames: queueDrainAttributes,
	})
	if err != nil {
		return depth, err
	}
	if out == nil {
		return depth, errors.New("empty GetQueueAttributes response")
	}

	values := make(map[types.QueueAttributeName]int64, len(queueDrainAttributes))
	for _, name := range queueDrainAttributes {
		raw, ok := out.Attributes[string(name)]
		if !ok {
			return depth, fmt.Errorf("%s: attribute missing from response", name)
		}
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return depth, fmt.Errorf("%s: %w", name, err)
		}
		values[name] = n
	}

	depth.visible = values[types.QueueAttributeNameApproximateNumberOfMessages]
	depth.inFlight = values[types.QueueAttributeNameApproximateNumberOfMessagesNotVisible]
	depth.delayed = values[types.QueueAttributeNameApproximateNumberOfMessagesDelayed]

	return depth, nil
}

func isAccessDenied(err error) bool {
	var apiErr interface{ ErrorCode() string }
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.ErrorCode() {
	case "AccessDenied", "AccessDeniedException", "AWS.SimpleQueueService.AccessDenied":
		return true
	}
	return false
}

func (s *sqs) isFifo() bool {
	return strings.HasSuffix(s.QueueURL, ".fifo")
}

// deleteMessage acknowledges a message by deleting its receipt. A failure is
// logged rather than returned: the message has already been processed, so the
// worst case is a redelivery after the visibility timeout.
func (s *sqs) deleteMessage(messageId, receiptHandle *string) {

	if _, err := s.client.DeleteMessage(ctx, &qs.DeleteMessageInput{
		QueueUrl:      &s.QueueURL,
		ReceiptHandle: receiptHandle,
	}); err != nil {
		fmt.Printf("failed to delete message %s from queue %s: %v\n", aws.ToString(messageId), s.QueueURL, err)
	}

}

func (s *sqs) sendMessages(input <-chan *record.Record) error {
	if input == nil {
		return nil
	}

	for {
		r, ok := s.GetRecord(input)
		if !ok {
			break
		}
		_, err := s.client.SendMessage(ctx, &qs.SendMessageInput{
			QueueUrl:       &s.QueueURL,
			MessageBody:    aws.String(string(r.Data)),
			MessageGroupId: s.getMessageGroupID(),
		})
		if err != nil {
			return ack.Rejected(r.Context, err)
		}

		ack.Release(r.Context)
	}
	return nil
}

func (s *sqs) getMessageGroupID() *string {
	if s.isFifo() {

		if s.MessageGroupId != "" {
			return &s.MessageGroupId
		}

		id := uuid.New().String()
		return &id
	}
	return nil
}
