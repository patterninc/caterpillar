package jq

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// mustQuery parses a jq expression the way a pipeline config would.
func mustQuery(t *testing.T, expr string) *Query {

	t.Helper()

	q := &Query{}
	raw, err := json.Marshal(expr)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, q))

	return q

}

func TestQueryUnmarshal(t *testing.T) {

	t.Run("yaml", func(t *testing.T) {
		var holder struct {
			Q *Query `yaml:"q"`
		}
		require.NoError(t, yaml.Unmarshal([]byte(`q: ".a + 1"`), &holder))
		got, err := holder.Q.Execute([]byte(`{"a":1}`))
		require.NoError(t, err)
		assert.Equal(t, float64(2), got)
	})

	t.Run("json", func(t *testing.T) {
		var holder struct {
			Q *Query `json:"q"`
		}
		require.NoError(t, json.Unmarshal([]byte(`{"q":".a"}`), &holder))
		got, err := holder.Q.Execute([]byte(`{"a":"x"}`))
		require.NoError(t, err)
		assert.Equal(t, "x", got)
	})

	t.Run("invalid expression", func(t *testing.T) {
		q := &Query{}
		assert.Error(t, json.Unmarshal([]byte(`"[.a"`), q))
		assert.Error(t, yaml.Unmarshal([]byte(`"[.a"`), q))
	})

	t.Run("non-string value", func(t *testing.T) {
		q := &Query{}
		assert.Error(t, json.Unmarshal([]byte(`42`), q))
	})

}

func TestQueryExecute(t *testing.T) {

	tests := []struct {
		name     string
		expr     string
		document string
		expected any
	}{
		{
			name:     "Single scalar result",
			expr:     ".a",
			document: `{"a":1}`,
			expected: float64(1),
		},
		{
			name:     "Missing key yields nil",
			expr:     ".missing",
			document: `{"a":1}`,
			expected: nil,
		},
		{
			name:     "Empty stream yields nil",
			expr:     "empty",
			document: `{}`,
			expected: nil,
		},
		{
			name:     "Multiple results are collected into a slice",
			expr:     ".[]",
			document: `[1,2,3]`,
			expected: []any{float64(1), float64(2), float64(3)},
		},
		{
			name:     "Array result stays a single value",
			expr:     ".",
			document: `[1,2]`,
			expected: []any{float64(1), float64(2)},
		},
		{
			name:     "Object construction",
			expr:     "{b: .a}",
			document: `{"a":"x"}`,
			expected: map[string]any{"b": "x"},
		},
		{
			name:     "Non-JSON document is treated as a raw string",
			expr:     ". + \"!\"",
			document: `hello`,
			expected: "hello!",
		},
		{
			name:     "Raw string can be split",
			expr:     `split(",")`,
			document: `a,b`,
			expected: []any{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mustQuery(t, tt.expr).Execute([]byte(tt.document))
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}

}

func TestQueryExecuteErrors(t *testing.T) {

	t.Run("error() surfaces as an error", func(t *testing.T) {
		_, err := mustQuery(t, `error("boom")`).Execute([]byte(`{}`))
		assert.ErrorContains(t, err, "boom")
	})

	t.Run("type error surfaces as an error", func(t *testing.T) {
		_, err := mustQuery(t, `.a + 1`).Execute([]byte(`{"a":"x"}`))
		assert.Error(t, err)
	})

	t.Run("unknown function fails to compile", func(t *testing.T) {
		_, err := mustQuery(t, `nosuchfunction`).Execute([]byte(`{}`))
		assert.Error(t, err)
	})

}

// The document is the first value `inputs` yields, ahead of any extra inputs.
func TestQueryExecuteExtraInputs(t *testing.T) {

	got, err := mustQuery(t, `[inputs]`).Execute([]byte(`1`), map[string]any{"page_id": 2})
	require.NoError(t, err)
	assert.Equal(t, []any{float64(1), map[string]any{"page_id": 2}}, got)

}

func TestQueryExecuteIsReusable(t *testing.T) {

	q := mustQuery(t, `.n * 2`)
	for i := 0; i < 3; i++ {
		got, err := q.Execute([]byte(`{"n":5}`))
		require.NoError(t, err)
		assert.Equal(t, float64(10), got)
	}

}

// A null in the middle of a result stream ends collection early, so results after
// it are lost. Unskip once Execute stops treating nil as end-of-stream.
func TestQueryExecuteNullInStream(t *testing.T) {

	t.Skip("known bug: Query.Execute breaks on the first nil result (internal/pkg/jq/jq.go:84)")

	got, err := mustQuery(t, `.[]`).Execute([]byte(`[1,null,2]`))
	require.NoError(t, err)
	assert.Equal(t, []any{float64(1), nil, float64(2)}, got)

}

func TestCustomFunctionsRegistered(t *testing.T) {

	for _, expr := range []string{
		`"a" | md5`,
		`"a" | sha256`,
		`"a" | sha512`,
		`uuid`,
		`hmac_sha256("d"; "k")`,
		`hmac_sha512("d"; "k")`,
		`hmac_md5("d"; "k")`,
		`[1] | shuffle`,
		`sleep("0s")`,
	} {
		t.Run(expr, func(t *testing.T) {
			_, err := mustQuery(t, expr).Execute([]byte(`null`))
			assert.NoError(t, err)
		})
	}

}
