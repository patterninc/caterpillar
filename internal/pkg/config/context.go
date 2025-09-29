package config

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

const (
	errMissingContextKeys    = "context keys were not set: "
	errMalformedContext      = "malformed context template: "
	contextPlaceholderString = "___CATERPILLAR___CONTEXT___%s___"
)

var (
	contextTemplateRegex = regexp.MustCompile(fmt.Sprintf(contextPlaceholderString, placeholderRegex))
)

// return placeholder string for context key
func setContextPlaceholder(key string) (string, error) {
	return fmt.Sprintf(contextPlaceholderString, key), nil
}

func evaluateContext(data string, record *record.Record) (string, error) {

	// Find all context template patterns
	matches := contextTemplateRegex.FindAllStringSubmatch(data, -1)

	// If we don't have any matches, return quickly as is
	if len(matches) == 0 {
		return data, nil
	}

	var missingKeys []string

	for _, match := range matches {
		// Template should return [full template, key name]
		if len(match) != 2 {
			return ``, fmt.Errorf("%s%s", errMalformedContext, match[0])
		}

		template := match[0]
		key := match[1]

		// Get the context value
		value, ok := record.GetContextValue(key)
		if !ok {
			missingKeys = append(missingKeys, key)
			continue
		}
		// Remove quotes if the value is JSON-encoded
		if strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`) {
			value = value[1 : len(value)-1]
		}

		data = strings.ReplaceAll(data, template, value)
	}

	// Return error if any context keys are missing
	if len(missingKeys) > 0 {
		return ``, fmt.Errorf("%s", errMissingContextKeys+strings.Join(missingKeys, ", "))
	}

	return data, nil

}
