package parameter_store

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/patterninc/caterpillar/internal/pkg/jq"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

var (
	ctx     = context.Background()
	awsTrue = aws.Bool(true)
)

type parameterStore struct {
	task.Base     `yaml:",inline" json:",inline"`
	SetParameters map[string]*jq.Query `yaml:"set,omitempty" json:"set,omitempty"`
	GetParameters map[string]string    `yaml:"get,omitempty" json:"get,omitempty"`
	Secure        bool                 `yaml:"secure,omitempty" json:"secure,omitempty"`
	Overwrite     *bool                `yaml:"overwrite,omitempty" json:"overwrite,omitempty"`
	client        *ssm.Client
}

func New() (task.Task, error) {
	return &parameterStore{
		Secure:    true,
		Overwrite: awsTrue,
	}, nil
}

func (p *parameterStore) GetTaskConcurrency() int {
	if p.Base.TaskConcurrency > 1 {
		fmt.Printf("WARN: task_concurrency (%d) is not supported for task '%s'. Only one ssm client instance will run.\n",
			p.Base.TaskConcurrency, p.Base.Type)
	}
	return 1
}

func (p *parameterStore) Init() error {
	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}

	p.client = ssm.NewFromConfig(awsConfig)
	return nil
}

func (p *parameterStore) Run(input <-chan *record.Record, output chan<- *record.Record) (err error) {

	for {
		r, ok := p.GetRecord(input)
		if !ok {
			break
		}

		// first let's set parameters
		for parameterName, parameterQuery := range p.SetParameters {
			parameterValue, err := parameterQuery.Execute(r.Data)
			if err != nil {
				return err
			}

			parameterValueString, isString := parameterValue.(string)
			if !isString {
				return fmt.Errorf("%s parameter value is not string", parameterName)
			}

			putParameterInput := &ssm.PutParameterInput{
				Name:      aws.String(parameterName),
				Value:     aws.String(parameterValueString),
				Overwrite: p.Overwrite,
			}

			if p.Secure {
				putParameterInput.Type = types.ParameterTypeSecureString
			}

			if _, err := p.client.PutParameter(ctx, putParameterInput); err != nil {
				return err
			}

			if output != nil {
				p.SendRecord(r, output)
			}
		}
	}

	return nil

}
