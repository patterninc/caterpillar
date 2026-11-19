package config

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

func recordWithContext(kv map[string]string) *record.Record {

	r := &record.Record{Context: context.Background()}
	for k, v := range kv {
		r.SetContextValue(k, v)
	}

	return r

}

func TestSetMacroPlaceholder(t *testing.T) {

	for name := range macroFuncs {
		t.Run(name, func(t *testing.T) {
			got, err := setMacroPlaceholder(name)
			require.NoError(t, err)
			assert.Equal(t, "___CATERPILLAR___MACRO___"+name+"___", got)
		})
	}

	t.Run("unknown macro", func(t *testing.T) {
		_, err := setMacroPlaceholder("nope")
		assert.ErrorContains(t, err, "'nope' is not defined")
	})

}

func TestEvaluateMacro(t *testing.T) {

	placeholder := func(name string) string {
		p, err := setMacroPlaceholder(name)
		require.NoError(t, err)
		return p
	}

	t.Run("no placeholders is a no-op", func(t *testing.T) {
		assert.Equal(t, "plain text", evaluateMacro("plain text"))
	})

	t.Run("unixtime", func(t *testing.T) {
		before := time.Now().Unix()
		got, err := strconv.ParseInt(evaluateMacro(placeholder("unixtime")), 10, 64)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, got, before)
		assert.LessOrEqual(t, got, time.Now().Unix())
	})

	t.Run("microtimestamp", func(t *testing.T) {
		got, err := strconv.ParseInt(evaluateMacro(placeholder("microtimestamp")), 10, 64)
		require.NoError(t, err)
		assert.Greater(t, got, time.Now().Add(-time.Minute).UnixMicro())
	})

	t.Run("timestamp", func(t *testing.T) {
		_, err := time.Parse(time.RFC3339, evaluateMacro(placeholder("timestamp")))
		assert.NoError(t, err)
	})

	t.Run("one value per macro within a string", func(t *testing.T) {
		got := evaluateMacro(placeholder("uuid") + "|" + placeholder("uuid"))
		parts := strings.Split(got, "|")
		require.Len(t, parts, 2)
		assert.Regexp(t, regexp.MustCompile(`^[0-9a-f-]{36}$`), parts[0])
		assert.Equal(t, parts[0], parts[1])
	})

	t.Run("fresh value per evaluation", func(t *testing.T) {
		assert.NotEqual(t, evaluateMacro(placeholder("uuid")), evaluateMacro(placeholder("uuid")))
	})

	t.Run("surrounding text is kept", func(t *testing.T) {
		got := evaluateMacro("s3://bucket/" + placeholder("uuid") + ".json")
		assert.Regexp(t, regexp.MustCompile(`^s3://bucket/[0-9a-f-]{36}\.json$`), got)
	})

}

func TestEvaluateContext(t *testing.T) {

	placeholder := func(key string) string {
		p, err := setContextPlaceholder(key)
		require.NoError(t, err)
		return p
	}

	tests := []struct {
		name     string
		input    string
		ctx      map[string]string
		expected string
		wantErr  string
	}{
		{
			name:     "No placeholders",
			input:    "static",
			expected: "static",
		},
		{
			name:     "Single key",
			input:    "id=" + placeholder("id"),
			ctx:      map[string]string{"id": "42"},
			expected: "id=42",
		},
		{
			name:     "JSON string value is unquoted",
			input:    "/users/" + placeholder("user"),
			ctx:      map[string]string{"user": `"alice"`},
			expected: "/users/alice",
		},
		{
			name:     "Repeated key",
			input:    placeholder("a") + "-" + placeholder("a"),
			ctx:      map[string]string{"a": "x"},
			expected: "x-x",
		},
		{
			name:     "Multiple keys",
			input:    placeholder("a") + "/" + placeholder("b_2"),
			ctx:      map[string]string{"a": "x", "b_2": "y"},
			expected: "x/y",
		},
		{
			name:    "Missing keys are all reported",
			input:   placeholder("a") + placeholder("b"),
			ctx:     map[string]string{},
			wantErr: "context keys were not set: a, b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluateContext(tt.input, recordWithContext(tt.ctx))
			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}

}

func TestStringGet(t *testing.T) {

	ctxPlaceholder, _ := setContextPlaceholder("name")

	t.Run("nil record skips context evaluation", func(t *testing.T) {
		got, err := String("keep " + ctxPlaceholder).Get(nil)
		require.NoError(t, err)
		assert.Equal(t, "keep "+ctxPlaceholder, got)
	})

	t.Run("record context is applied", func(t *testing.T) {
		got, err := String("hi " + ctxPlaceholder).Get(recordWithContext(map[string]string{"name": `"bob"`}))
		require.NoError(t, err)
		assert.Equal(t, "hi bob", got)
	})

	t.Run("GetJQ compiles the evaluated string", func(t *testing.T) {
		q, err := String(".a + " + ctxPlaceholder).GetJQ(recordWithContext(map[string]string{"name": "1"}))
		require.NoError(t, err)
		got, err := q.Execute([]byte(`{"a":2}`))
		require.NoError(t, err)
		assert.Equal(t, float64(3), got)
	})

	t.Run("GetJQ reports parse errors", func(t *testing.T) {
		_, err := String("[.a").GetJQ(nil)
		assert.Error(t, err)
	})

	t.Run("GetJQ reports missing context", func(t *testing.T) {
		_, err := String(ctxPlaceholder).GetJQ(recordWithContext(nil))
		assert.ErrorContains(t, err, "context keys were not set")
	})

}

func TestGetEnvironmentVariable(t *testing.T) {

	t.Setenv("CATERPILLAR_TEST_ENV", "value")

	got, err := getEnvironmentVariable("CATERPILLAR_TEST_ENV")
	require.NoError(t, err)
	assert.Equal(t, "value", got)

	got, err = getEnvironmentVariable("CATERPILLAR_TEST_ENV_UNSET")
	require.NoError(t, err)
	assert.Equal(t, "", got)

}

type loadTarget struct {
	Name    string `yaml:"name"`
	Path    String `yaml:"path"`
	Stamp   String `yaml:"stamp"`
	Content string `yaml:"content"`
}

func writeConfig(t *testing.T, body string) string {

	t.Helper()

	path := filepath.Join(t.TempDir(), "pipeline.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	return path

}

func TestLoad(t *testing.T) {

	t.Setenv("CATERPILLAR_TEST_NAME", "from-env")

	path := writeConfig(t, `name: {{ env "CATERPILLAR_TEST_NAME" }}
path: /out/{{ context "file" }}.json
stamp: {{ macro "unixtime" }}
content: |
  {{ indent 2 "line1\nline2" }}
`)

	var got loadTarget
	require.NoError(t, Load(path, &got))

	assert.Equal(t, "from-env", got.Name)
	assert.Equal(t, String("/out/___CATERPILLAR___CONTEXT___file___.json"), got.Path)
	assert.Equal(t, String("___CATERPILLAR___MACRO___unixtime___"), got.Stamp)
	assert.Equal(t, "line1\nline2\n", got.Content)

	resolved, err := got.Path.Get(recordWithContext(map[string]string{"file": `"report"`}))
	require.NoError(t, err)
	assert.Equal(t, "/out/report.json", resolved)

}

func TestLoadIndentNormalisesCRLF(t *testing.T) {

	path := writeConfig(t, "content: |\n  {{ indent 2 \"a\\r\\nb\" }}\n")

	var got loadTarget
	require.NoError(t, Load(path, &got))
	assert.Equal(t, "a\nb\n", got.Content)

}

func TestLoadErrors(t *testing.T) {

	tests := []struct {
		name string
		body string
	}{
		{name: "Template parse error", body: "name: {{ env "},
		{name: "Unknown macro", body: `name: {{ macro "nope" }}`},
		{name: "Invalid YAML", body: "name: [unterminated"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got loadTarget
			assert.Error(t, Load(writeConfig(t, tt.body), &got))
		})
	}

	t.Run("Missing file", func(t *testing.T) {
		var got loadTarget
		assert.Error(t, Load(filepath.Join(t.TempDir(), "missing.yaml"), &got))
	})

}
