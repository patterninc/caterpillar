package pipeline

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/aws/parameter_store"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/compress"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/converter"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/delay"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/echo"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/file"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/flatten"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/heimdall"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/http"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/http/server"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/join"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/jq"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/kafka"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/replace"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/sample"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/split"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/sqs"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task/xpath"
)

type tasks []task.Task

var (
	validate       = validator.New()
	supportedTasks = map[string]func() (task.Task, error){
		`aws_parameter_store`: parameter_store.New,
		`compress`:            compress.New,
		`converter`:           converter.New,
		`delay`:               delay.New,
		`echo`:                echo.New,
		`file`:                file.New,
		`flatten`:             flatten.New,
		`heimdall`:            heimdall.New,
		`http_server`:         server.New,
		`http`:                http.New,
		`join`:                join.New,
		`jq`:                  jq.New,
		`kafka`:               kafka.New,
		`replace`:             replace.New,
		`sample`:              sample.New,
		`split`:               split.New,
		`sqs`:                 sqs.New,
		`xpath`:               xpath.New,
	}
)

func (t *tasks) UnmarshalYAML(unmarshal func(any) error) error {

	nodes := make([]yaml.Node, 0, 10)

	if err := unmarshal(&nodes); err != nil {
		return err
	}

	result := make([]task.Task, 0, len(nodes))

	for _, n := range nodes {
		b := &task.Base{}
		if err := n.Decode(b); err != nil {
			return err
		}

		newFn, found := supportedTasks[b.Type]
		if !found {
			return fmt.Errorf("task type is not supported: %s", b.Type)
		}

		t, err := newFn()
		if err != nil {
			return err
		}

		if err := n.Decode(t); err != nil {
			return err
		}

		// Validate the task after decoding
		if err := validate.Struct(t); err != nil {
			return err
		}

		// Initialize task (e.g., create clients)
		if err := t.Init(); err != nil {
			return fmt.Errorf("failed to initialize task %s: %w", t.GetName(), err)
		}

		result = append(result, t)
	}

	*t = result

	return nil

}
