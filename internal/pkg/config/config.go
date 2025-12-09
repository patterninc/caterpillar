package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/itchyny/gojq"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/jq"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

const (
	templateName     = `render`
	placeholderRegex = "([a-zA-Z0-9_]+?)"
)

var (
	// Jinja-style include directive: {% include 'path/to/file' %}
	includeRegex = regexp.MustCompile(`\{\%\s*include\s+['"]([^'"]+)['"]\s*\%\}`)
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

	// Preprocess Jinja-style includes
	configDir := filepath.Dir(configFile)
	processedContent, err := processIncludes(string(content), configDir, make(map[string]bool))
	if err != nil {
		return fmt.Errorf("failed to process includes: %w", err)
	}

	// inject secrets
	configTemplate := template.New(templateName).Funcs(template.FuncMap{
		"env":     getEnvironmentVariable, // returns environment variable
		"macro":   setMacroPlaceholder,    // set placeholder string for macro replacement
		"secret":  getSecret,              // we use this template function to inject secrets from parameter store
		"context": setContextPlaceholder,  // set placeholder string for context replacement
		"ds":      getDateString,          // returns date string (YYYY-MM-DD) from env var or current date
	})

	parsedTemplate, err := configTemplate.Parse(processedContent)
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

// processIncludes processes Jinja-style {% include 'path' %} directives
// It recursively includes files and prevents circular includes
func processIncludes(content string, baseDir string, visited map[string]bool) (string, error) {
	// Find all include directives
	matches := includeRegex.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return content, nil
	}

	result := content
	for _, match := range matches {
		if len(match) != 2 {
			continue
		}

		includePath := match[0] // Full match: {% include 'path' %}
		filePath := match[1]    // Captured group: path

		// Resolve the file path relative to baseDir
		// If the path starts with /, it's absolute, otherwise relative to baseDir
		var resolvedPath string
		if filepath.IsAbs(filePath) {
			resolvedPath = filePath
		} else {
			resolvedPath = filepath.Join(baseDir, filePath)
		}

		// Normalize the path to handle .. and . correctly
		resolvedPath = filepath.Clean(resolvedPath)

		// Check for circular includes
		absPath, err := filepath.Abs(resolvedPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute path for %s: %w", filePath, err)
		}

		if visited[absPath] {
			return "", fmt.Errorf("circular include detected: %s", absPath)
		}

		// Read the included file
		includedContent, err := os.ReadFile(resolvedPath)
		if err != nil {
			return "", fmt.Errorf("failed to read included file %s: %w", filePath, err)
		}

		// Recursively process includes in the included file
		visited[absPath] = true
		processedContent, err := processIncludes(string(includedContent), filepath.Dir(resolvedPath), visited)
		if err != nil {
			return "", err
		}
		delete(visited, absPath)

		// Replace the include directive with the file content
		result = strings.ReplaceAll(result, includePath, processedContent)
	}

	return result, nil
}

// getDateString returns a date string in YYYY-MM-DD format
// It first checks for the DS or EXECUTION_DATE environment variable
// If not set, it defaults to the current date
func getDateString() (string, error) {
	// Check for DS environment variable first (common in data pipelines)
	if ds := os.Getenv("DS"); ds != "" {
		return ds, nil
	}
	// Check for EXECUTION_DATE (alternative common name)
	if execDate := os.Getenv("EXECUTION_DATE"); execDate != "" {
		return execDate, nil
	}
	// Default to current date
	return time.Now().Format("2006-01-02"), nil
}
