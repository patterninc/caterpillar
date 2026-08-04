package sqs

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	qs "github.com/aws/aws-sdk-go-v2/service/sqs"
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

var (
	awsRegionRegex = regexp.MustCompile(`^[a-z]{2}-[a-z]+-\d+$`)
	ctx            = context.Background()
)

type sqs struct {
	task.ServerBase `yaml:",inline" json:",inline"`
	QueueURL        string `yaml:"queue_url" json:"queue_url" validate:"required"`
	Concurrency     int    `yaml:"concurrency,omitempty" json:"concurrency,omitempty"`
	MaxMessages     int32  `yaml:"max_messages,omitempty" json:"max_messages,omitempty"`
	WaitTimeSeconds int    `yaml:"wait_time_seconds,omitempty" json:"wait_time_seconds,omitempty"`
	ExitOnEmpty     bool   `yaml:"exit_on_empty,omitempty" json:"exit_on_empty,omitempty"`
	MessageGroupId  string `yaml:"message_group_id,omitempty" json:"message_group_id,omitempty"` // used for FIFO queues

	client  *qs.Client
	tracker *ack.Tracker
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

	region := s.extractRegionFromQueueURL()
	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	s.client = qs.NewFromConfig(awsConfig)
	s.tracker = ack.NewTracker(s.Concurrency)

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

	// If input is nil, act as a source: read messages and hand each one's
	// receipt to the tracker, which deletes it once every downstream task
	// has finished with the record it produced. Finish - not Run - waits for
	// those deletions; see Finish.
	return s.getMessages(ctx, output)

}

// Finish waits for every deferred deletion to run (and each receipt to be
// deleted or left alone) before the pipeline treats this task as complete, so
// a shutdown never abandons in-flight acknowledgements. It can't happen in
// Run: downstream tasks that only emit once their input closes can't finish
// with a record until this task's output channel is closed, which the
// pipeline does only after Run returns.
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
				if s.ExitOnEmpty {
					fmt.Println(`Queue is empty, exiting`)
					return nil
				}
				continue
			}

			for _, m := range receiveMessageOutput.Messages {

				// nothing to forward to, so there's no downstream ack to
				// wait for: delete the receipt right away, same as when
				// there's no consumer at all.
				if output == nil {
					s.deleteMessage(m.MessageId, m.ReceiptHandle)
					continue
				}

				msgAck := ack.New()
				s.SendData(ack.WithContext(ctx, msgAck), []byte(*m.Body), output)

				s.tracker.Track(msgAck, &messageAck{
					sqs:           s,
					messageId:     m.MessageId,
					receiptHandle: m.ReceiptHandle,
				})
			}
		}
	}

}

// messageAck acknowledges one received message on behalf of ack.Tracker.
type messageAck struct {
	sqs           *sqs
	messageId     *string
	receiptHandle *string
}

// Ack deletes the message's receipt so SQS doesn't redeliver it. On a
// downstream failure it does nothing: the message wasn't fully processed, so
// leaving the receipt alone lets SQS redeliver it once the visibility
// timeout expires instead of losing it.
func (m *messageAck) Ack(failed bool) {

	if failed {
		return
	}

	m.sqs.deleteMessage(m.messageId, m.receiptHandle)

}

// deleteMessage acknowledges a message by deleting its receipt so it isn't
// redelivered. A failure to delete is logged rather than returned: the
// message has already been processed, and the worst case is a redelivery
// after the visibility timeout.
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

		if a, ok := ack.FromContext(r.Context); ok {
			a.Done()
		}
	}
	return nil
}

func (s *sqs) getMessageGroupID() *string {
	// Only return a group ID if the queue is FIFO (URL ends with .fifo)
	if strings.HasSuffix(s.QueueURL, ".fifo") {

		if s.MessageGroupId != "" {
			return &s.MessageGroupId
		}

		id := uuid.New().String()
		return &id
	}
	return nil
}
