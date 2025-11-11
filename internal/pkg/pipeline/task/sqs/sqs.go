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

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

const (
	defaultConcurrency      = 10
	defaultMaxMessages      = 10
	defaultWaitTimeSeconds  = 10
	receiptsQueueMultiplier = 1000
	defaultRegion           = "us-west-2"
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

// getSQSClient creates an SQS client with the region extracted from the queue URL
func (s *sqs) getSQSClient() (*qs.Client, error) {
	region := s.extractRegionFromQueueURL()

	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}

	return qs.NewFromConfig(awsConfig), nil
}

func (s *sqs) Run(input <-chan *record.Record, output chan<- *record.Record) error {

	client, err := s.getSQSClient()
	if err != nil {
		return err
	}
	s.client = client

	if input != nil {
		return s.sendMessages(input)
	}

	// If input is nil, act as a source: start getMessages and receipt workers
	// let's create channel to which getMessages function will communicate messages receipts
	receipts := make(chan *string, s.Concurrency*receiptsQueueMultiplier)

	// now let's start a go routine that will bring messages from the queue
	go s.getMessages(ctx, output, receipts)

	// we set a pool of workers that will delete messages from the queue
	var wg sync.WaitGroup
	wg.Add(s.Concurrency)
	for i := 0; i < s.Concurrency; i++ {
		go s.processReceipts(receipts, &wg)
	}
	wg.Wait()

	return nil

}

func (s *sqs) getMessages(ctx context.Context, output chan<- *record.Record, receipts chan *string) error {

	defer close(receipts)

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
				// create new record and send it downstream

				if output != nil {
					s.SendData(ctx, []byte(*m.Body), output)
				}

				// send receipt to receipts channel for deletion
				receipts <- m.ReceiptHandle
			}
		}
	}

}

func (s *sqs) processReceipts(receipts <-chan *string, wg *sync.WaitGroup) error {

	defer wg.Done()

	for receipt := range receipts {
		if _, err := s.client.DeleteMessage(ctx, &qs.DeleteMessageInput{
			QueueUrl:      &s.QueueURL,
			ReceiptHandle: receipt,
		}); err != nil {
			return err
		}
	}

	return nil

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
