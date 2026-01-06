package jq

import (
	"encoding/json"

	"github.com/itchyny/gojq"
)

const (
	defaultResultLength = 100
)

type Query gojq.Query

func (q *Query) UnmarshalYAML(unmarshal func(any) error) error {
	return q.parse(unmarshal)
}

func (q *Query) UnmarshalJSON(data []byte) error {

	jsonUnmarshal := func(obj any) error {
		return json.Unmarshal(data, obj)
	}

	return q.parse(jsonUnmarshal)

}

func (q *Query) parse(unmarshal func(any) error) error {

	path := ``

	if err := unmarshal(&path); err != nil {
		return err
	}

	query, err := gojq.Parse(path)
	if err != nil {
		return err
	}

	*q = Query(*query)

	return nil

}

func (q *Query) Execute(document []byte, inputs ...any) (any, error) {

	var data any

	if err := json.Unmarshal(document, &data); err != nil {
		return nil, err
	}

	values := make([]any, 0, len(inputs)+1)
	values = append(values, data)
	values = append(values, inputs...)

	compilerOpts := []gojq.CompilerOption{gojq.WithInputIter(gojq.NewIter(values...))}
	// extending compliter with custom functions...
	compilerOpts = append(compilerOpts, customFunctionsOptions()...)

	code, err := gojq.Compile((*gojq.Query)(q), compilerOpts...)
	if err != nil {
		return nil, err
	}

	iter := code.Run(data)

	if iter == nil {
		return nil, nil
	}

	result := make([]any, 0, defaultResultLength)

	for {

		v, found := iter.Next()
		if !found || v == nil {
			break
		}

		if _, isError := v.(error); isError {
			return nil, nil
		}

		result = append(result, v)

	}

	switch l := len(result); l {
	case 0:
		return nil, nil
	case 1:
		return result[0], nil
	default:
		return result, nil
	}

}

// returns options for additional custom functions...
func customFunctionsOptions() []gojq.CompilerOption {
	var options []gojq.CompilerOption
	options = append(options, cryptoHashOptions()...)
	options = append(options, signOptions()...)
	options = append(options, uuidOptions()...)
	options = append(options, messageAuthOptions()...)
	options = append(options, shuffleOptions()...)
	options = append(options, sleepOptions()...)
	options = append(options, translateOption()...)
	return options
}
