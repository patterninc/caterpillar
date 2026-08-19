package parameter_store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"

	cfg "github.com/patterninc/caterpillar/internal/pkg/config"
	"github.com/patterninc/caterpillar/internal/pkg/duration"
	"github.com/patterninc/caterpillar/internal/pkg/jq"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

const defaultCacheTTL = 5 * time.Minute

type onMissingBehavior string

const (
	onMissingError onMissingBehavior = "error"
	onMissingSkip  onMissingBehavior = "skip"
)

var (
	ctx     = context.Background()
	awsTrue = aws.Bool(true)

	errBothModes     = fmt.Errorf(`set and lookup are mutually exclusive`)
	errNoMode        = fmt.Errorf(`either set or lookup must be configured`)
	errInputRequired = fmt.Errorf(`aws_parameter_store requires an input channel`)

	errParameterNotFound = fmt.Errorf(`parameter not found`)
)

type ssmAPI interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
	PutParameter(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error)
}

type cacheEntry struct {
	value     string
	fetchedAt time.Time
}

type parameterStore struct {
	task.Base     `yaml:",inline" json:",inline"`
	SetParameters map[string]*jq.Query  `yaml:"set,omitempty" json:"set,omitempty"`
	Lookup        map[string]cfg.String `yaml:"lookup,omitempty" json:"lookup,omitempty"`
	CacheTTL      duration.Duration     `yaml:"cache_ttl,omitempty" json:"cache_ttl,omitempty"`
	OnMissing     onMissingBehavior     `yaml:"on_missing,omitempty" json:"on_missing,omitempty"`
	Secure        bool                  `yaml:"secure,omitempty" json:"secure,omitempty"`
	Overwrite     *bool                 `yaml:"overwrite,omitempty" json:"overwrite,omitempty"`
	client        ssmAPI
	cache         map[string]cacheEntry
	cacheMu       sync.RWMutex
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

	if len(p.SetParameters) > 0 && len(p.Lookup) > 0 {
		return errBothModes
	}

	if len(p.SetParameters) == 0 && len(p.Lookup) == 0 {
		return errNoMode
	}

	if p.OnMissing == `` {
		p.OnMissing = onMissingError
	}

	if p.OnMissing != onMissingError && p.OnMissing != onMissingSkip {
		return fmt.Errorf("invalid on_missing value %q: must be %q or %q", p.OnMissing, onMissingError, onMissingSkip)
	}

	if p.CacheTTL == 0 && len(p.Lookup) > 0 {
		p.CacheTTL = duration.Duration(defaultCacheTTL)
	}

	if len(p.Lookup) > 0 {
		p.cache = make(map[string]cacheEntry)
	}

	awsConfig, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return err
	}

	p.client = ssm.NewFromConfig(awsConfig)
	return nil
}

func (p *parameterStore) Run(input <-chan *record.Record, output chan<- *record.Record) error {

	// GetRecord treats a nil channel as an immediately closed one, so without this
	// a source-positioned task would exit successfully having done nothing
	if input == nil {
		return errInputRequired
	}

	for {
		r, ok := p.GetRecord(input)
		if !ok {
			break
		}

		if err := p.processRecord(r); err != nil {
			// a missing parameter is the one failure the pipeline author can route,
			// so a single misconfigured tenant need not stop every other one
			if errors.Is(err, errParameterNotFound) && p.OnMissing == onMissingSkip {
				fmt.Printf("WARN: skipping record: %s\n", err)
				continue
			}
			return err
		}

		p.SendRecord(r, output)
	}

	return nil

}

func (p *parameterStore) processRecord(r *record.Record) error {

	if len(p.Lookup) > 0 {
		return p.lookupParameters(r)
	}

	return p.setParameters(r)

}

func (p *parameterStore) setParameters(r *record.Record) error {

	for parameterName, parameterQuery := range p.SetParameters {
		parameterValue, err := parameterQuery.Execute(r.Data)
		if err != nil {
			return err
		}

		parameterValueString, isString := parameterValue.(string)
		if !isString {
			return fmt.Errorf("%s parameter value is not string", parameterName)
		}

		if err := p.setParameter(parameterName, parameterValueString); err != nil {
			return err
		}
	}

	return nil

}

func (p *parameterStore) setParameter(name, value string) error {

	putParameterInput := &ssm.PutParameterInput{
		Name:      aws.String(name),
		Value:     aws.String(value),
		Overwrite: p.Overwrite,
	}

	if p.Secure {
		putParameterInput.Type = types.ParameterTypeSecureString
	}

	_, err := p.client.PutParameter(ctx, putParameterInput)

	return err

}

func (p *parameterStore) lookupParameters(r *record.Record) error {

	for contextKey, parameterPath := range p.Lookup {
		value, err := p.lookupValue(parameterPath, r)
		if err != nil {
			return err
		}

		r.SetContextValue(contextKey, value)
	}

	return nil

}

func (p *parameterStore) lookupValue(path cfg.String, r *record.Record) (string, error) {

	resolvedPath, err := path.Get(r)
	if err != nil {
		return ``, err
	}

	if cached, ok := p.getCached(resolvedPath); ok {
		return cached, nil
	}

	parameter, err := p.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(resolvedPath),
		WithDecryption: awsTrue,
	})
	if err != nil {
		var notFound *types.ParameterNotFound
		if errors.As(err, &notFound) {
			return ``, fmt.Errorf("%w: %s", errParameterNotFound, resolvedPath)
		}
		return ``, err
	}

	if parameter == nil || parameter.Parameter == nil || parameter.Parameter.Value == nil {
		return ``, fmt.Errorf("%w: %s", errParameterNotFound, resolvedPath)
	}

	value := *parameter.Parameter.Value
	p.setCached(resolvedPath, value)

	return value, nil

}

func (p *parameterStore) getCached(path string) (string, bool) {
	if time.Duration(p.CacheTTL) == 0 {
		return ``, false
	}

	p.cacheMu.RLock()
	defer p.cacheMu.RUnlock()

	entry, ok := p.cache[path]
	if !ok {
		return ``, false
	}

	if time.Since(entry.fetchedAt) > time.Duration(p.CacheTTL) {
		return ``, false
	}

	return entry.value, true
}

func (p *parameterStore) setCached(path, value string) {
	if time.Duration(p.CacheTTL) == 0 {
		return
	}

	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()

	p.cache[path] = cacheEntry{
		value:     value,
		fetchedAt: time.Now(),
	}
}
