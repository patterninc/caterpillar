package sns

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/google/uuid"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

type MessageAttribute struct {
	Name  string `yaml:"name" json:"name"`
	Type  string `yaml:"type" json:"type"`
	Value string `yaml:"value" json:"value"`
}

const defaultRegion = "us-west-2"

type snsTask struct {
	task.Base  `yaml:",inline" json:",inline"`
	TopicArn   string             `yaml:"topic_arn" json:"topic_arn"`
	Region     string             `yaml:"region,omitempty" json:"region,omitempty"`
	Subject    string             `yaml:"subject,omitempty" json:"subject,omitempty"`
	Attributes []MessageAttribute `yaml:"attributes,omitempty" json:"attributes,omitempty"`

	// FIFO specific fields
	MessageGroupId         string `yaml:"message_group_id,omitempty" json:"message_group_id,omitempty"`
	MessageDeduplicationId string `yaml:"message_deduplication_id,omitempty" json:"message_deduplication_id,omitempty"`

	client *sns.Client
	isFifo bool
}

func New() (task.Task, error) {
	return &snsTask{}, nil
}

func (s *snsTask) Init() error {
	if s.TopicArn == "" {
		return fmt.Errorf("topic_arn is required")
	}

	region := s.Region
	if region == "" {
		region = defaultRegion
	}

	s.isFifo = strings.HasSuffix(s.TopicArn, ".fifo")

	// Load default config
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	s.client = sns.NewFromConfig(cfg)
	return nil
}

func (s *snsTask) Run(input <-chan *record.Record, output chan<- *record.Record) error {
	if input == nil {
		return task.ErrNilInput
	}

	if output != nil {
		return task.ErrPresentInputOutput
	}

	for {
		r, ok := s.GetRecord(input)
		if !ok {
			break
		}

		publishInput := &sns.PublishInput{
			Message:  aws.String(string(r.Data)),
			TopicArn: aws.String(s.TopicArn),
		}
		if s.Subject != "" {
			publishInput.Subject = aws.String(s.Subject)
		}

		if s.isFifo {
			s.setFifoParams(publishInput)
		}

		if len(s.Attributes) > 0 {
			publishInput.MessageAttributes = make(map[string]types.MessageAttributeValue)
			for _, attr := range s.Attributes {
				publishInput.MessageAttributes[attr.Name] = types.MessageAttributeValue{
					DataType:    aws.String(attr.Type),
					StringValue: aws.String(attr.Value),
				}
			}
		}

		_, err := s.client.Publish(r.Context, publishInput)
		if err != nil {
			return fmt.Errorf("failed to publish to SNS topic %s: %w", s.TopicArn, err)
		}
	}

	return nil
}

func (s *snsTask) setFifoParams(publishInput *sns.PublishInput) {
	groupID := s.MessageGroupId
	if groupID == "" {
		// Generate a random UUID if no group ID is provided for FIFO
		groupID = uuid.New().String()
	}
	publishInput.MessageGroupId = aws.String(groupID)

	if s.MessageDeduplicationId != "" {
		publishInput.MessageDeduplicationId = aws.String(s.MessageDeduplicationId)
	} else {
		publishInput.MessageDeduplicationId = aws.String(uuid.New().String())
	}
}
