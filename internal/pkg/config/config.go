package config

import (
	"os"
	"strings"
	"text/template"

	"github.com/itchyny/gojq"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/jq"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

const (
	templateName     = `render`
	placeholderRegex = "([a-zA-Z0-9_]+?)"
)

// Generic String type that can evaluate both macro and context templates
type String string

// Evaluate macros and context templates
func (s String) Get(r *record.Record) (string, error) {
	// Only evaluate macros
	resolveMacro := evaluateMacro(string(s))
	if r == nil {
		return resolveMacro, nil
	}

	return evaluateContext(resolveMacro, r)

}

func (s String) GetJQ(r *record.Record) (*jq.Query, error) {

	evaluatedString, err := s.Get(r)
	if err != nil {
		return nil, err
	}

	query, err := gojq.Parse(evaluatedString)
	if err != nil {
		return nil, err
	}

	jqQuery := jq.Query(*query)

	return &jqQuery, nil

}

func Load(configFile string, obj interface{}) error {

	// read full file content
	content, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}

	// inject secrets
	configTemplate := template.New(templateName).Funcs(template.FuncMap{
		"env":     getEnvironmentVariable, // returns environment variable
		"macro":   setMacroPlaceholder,    // set placeholder string for macro replacement
		"secret":  getSecret,              // we use this template function to inject secrets from parameter store
		"context": setContextPlaceholder,  // set placeholder string for context replacement
	})

	parsedTemplate, err := configTemplate.Parse(string(content))
	if err != nil {
		return err
	}

	var preparedContext strings.Builder
	if err := parsedTemplate.Execute(&preparedContext, nil); err != nil {
		return err
	}

	// unmarshal object
	if err := yaml.Unmarshal([]byte(preparedContext.String()), obj); err != nil {
		return err
	}

	return nil

}
