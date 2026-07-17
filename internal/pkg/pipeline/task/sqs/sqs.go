package sqs

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
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

	// inFlight bounds how many messages can be unacknowledged (received but
	// not yet deleted) at once. It's a multiple of Concurrency rather than
	// Concurrency itself so fetching can run some distance ahead of full
	// downstream completion instead of stalling every time Concurrency
	// messages are simultaneously in flight.
	inFlightMultiplier = 5
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

	client *qs.Client
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

	// Client is already initialized in RunPreHook - just use it
	if input != nil {
		return s.sendMessages(input)
	}

	// If input is nil, act as a source: read messages and, once every
	// downstream task has finished with a given message, delete its
	// receipt so it isn't redelivered.
	inFlight := make(chan struct{}, s.Concurrency*inFlightMultiplier)
	var wg sync.WaitGroup

	err := s.getMessages(ctx, output, inFlight, &wg)

	// wait for every deleteOnComplete goroutine spawned below to finish
	// (and its receipt to be deleted or left alone) before this task's Run
	// returns, so a shutdown never abandons in-flight acknowledgements.
	wg.Wait()

	return err

}

func (s *sqs) getMessages(ctx context.Context, output chan<- *record.Record, inFlight chan struct{}, wg *sync.WaitGroup) error {

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
					if _, err := s.client.DeleteMessage(ctx, &qs.DeleteMessageInput{
						QueueUrl:      &s.QueueURL,
						ReceiptHandle: m.ReceiptHandle,
					}); err != nil {
						fmt.Printf("failed to delete message %s from queue %s: %v\n", aws.ToString(m.MessageId), s.QueueURL, err)
					}
					continue
				}

				inFlight <- struct{}{}

				msgAck := ack.New()
				s.SendData(ack.WithContext(ctx, msgAck), []byte(*m.Body), output)

				wg.Add(1)
				go s.deleteOnComplete(msgAck, m.MessageId, m.ReceiptHandle, inFlight, wg)
			}
		}
	}

}

// deleteOnComplete waits until every downstream task has finished
// processing the record derived from this message, deletes its receipt so
// it isn't redelivered, and frees its inFlight slot. It runs detached from
// getMessages, since getMessages must return (closing the task's output
// channel) before downstream tasks can drain and signal completion; wg lets
// Run wait for it to finish before returning.
func (s *sqs) deleteOnComplete(msgAck *ack.Ack, messageId, receiptHandle *string, inFlight chan struct{}, wg *sync.WaitGroup) {

	defer wg.Done()
	defer func() { <-inFlight }()

	<-msgAck.Wait()

	// a downstream failure means this message wasn't fully processed:
	// leave its receipt alone so SQS redelivers it after the visibility
	// timeout instead of losing it.
	if msgAck.Failed() {
		return
	}

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
			return err
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
