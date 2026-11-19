package xpath

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

const page = `<html><body>
<div class="card"><h3>One</h3><p>first</p><a href="/1" title="t1">link</a></div>
<div class="card"><h3>Two</h3><p>second</p><p>extra</p></div>
<div class="other"><h3>Skip</h3></div>
</body></html>`

func decode(t *testing.T, r *record.Record) map[string]any {

	t.Helper()

	var m map[string]any
	require.NoError(t, json.Unmarshal(r.Data, &m))

	return m

}

func TestXPathContainerAndFields(t *testing.T) {

	spec := `
name: x
container: "//div[@class='card']"
fields:
  title: ./h3
  text: ./p
  link: ./a/@href`

	emitted, err := runTestTask(t, newTestTask(t, spec), []byte(page))
	require.NoError(t, err)
	require.Len(t, emitted, 2)

	first := decode(t, emitted[0])
	assert.Equal(t, []any{"One"}, first["title"])
	assert.Equal(t, []any{"first"}, first["text"])
	assert.Equal(t, []any{"/1"}, first["link"])

	second := decode(t, emitted[1])
	assert.Equal(t, []any{"Two"}, second["title"])
	assert.Equal(t, []any{"second", "extra"}, second["text"])
	assert.Nil(t, second["link"])

	for i, r := range emitted {
		idx, ok := r.GetContextValue(nodeIndexKey)
		require.True(t, ok)
		assert.Equal(t, []string{"1", "2"}[i], idx)
	}

}

func TestXPathNonLeafNodesAreConverted(t *testing.T) {

	emitted, err := runTestTask(t, newTestTask(t, "name: x\nfields:\n  card: \"//div[@class='card'][1]\""), []byte(page))
	require.NoError(t, err)
	require.Len(t, emitted, 1)

	card := decode(t, emitted[0])["card"].([]any)[0].(map[string]any)
	assert.Equal(t, "div", card["tag"])
	assert.Equal(t, map[string]any{"class": "card"}, card["attributes"])
	assert.Len(t, card["children"], 3)

}

func TestXPathUnionKeepsDocumentOrder(t *testing.T) {

	emitted, err := runTestTask(t, newTestTask(t, "name: x\nfields:\n  all: \"//p | //h3\""), []byte(page))
	require.NoError(t, err)
	require.Len(t, emitted, 1)

	assert.Equal(t, []any{"One", "first", "Two", "second", "extra", "Skip"}, decode(t, emitted[0])["all"])

}

func TestXPathUnionOfAttributesKeepsAttributeOrder(t *testing.T) {

	emitted, err := runTestTask(t, newTestTask(t, "name: x\nfields:\n  attrs: \"//a/@title | //a/@href\""), []byte(page))
	require.NoError(t, err)
	require.Len(t, emitted, 1)

	assert.Equal(t, []any{"/1", "t1"}, decode(t, emitted[0])["attrs"])

}

func TestXPathMissing(t *testing.T) {

	t.Run("missing container is skipped by default", func(t *testing.T) {
		emitted, err := runTestTask(t, newTestTask(t, "name: x\ncontainer: //table\nfields:\n  a: ./td"), []byte(page))
		require.NoError(t, err)
		assert.Empty(t, emitted)
	})

	t.Run("missing container is an error when not ignored", func(t *testing.T) {
		_, err := runTestTask(t, newTestTask(t, "name: x\nignore_missing: false\ncontainer: //table\nfields:\n  a: ./td"), []byte(page))
		assert.ErrorContains(t, err, "no nodes found")
	})

	t.Run("missing field is an error when not ignored", func(t *testing.T) {
		_, err := runTestTask(t, newTestTask(t, "name: x\nignore_missing: false\nfields:\n  a: //table"), []byte(page))
		assert.ErrorContains(t, err, "field 'a' not found")
	})

}

func TestXPathInitRejectsBadExpressions(t *testing.T) {

	for _, spec := range []string{"container: '//['", "fields:\n  a: '//['"} {
		t.Run(spec, func(t *testing.T) {
			tk, err := New()
			require.NoError(t, err)
			require.NoError(t, yaml.Unmarshal([]byte(spec), tk))
			assert.Error(t, tk.Init())
		})
	}

}

func TestHasUnionOperator(t *testing.T) {

	tests := []struct {
		expr     string
		expected bool
	}{
		{expr: "//a | //b", expected: true},
		{expr: "//a", expected: false},
		{expr: `//a[@x='a|b']`, expected: false},
		{expr: `//a[@x="a|b"] | //c`, expected: true},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			assert.Equal(t, tt.expected, hasUnionOperator(tt.expr))
		})
	}

}

// newTestTask builds the task from YAML exactly as a pipeline would.
func newTestTask(t *testing.T, spec string) task.Task {

	t.Helper()

	tk, err := New()
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal([]byte(spec), tk))
	require.NoError(t, tk.Init())

	return tk

}

// runTestTask feeds one record per payload through Run and collects everything emitted.
func runTestTask(t *testing.T, tk task.Task, payloads ...[]byte) ([]*record.Record, error) {

	t.Helper()

	in := make(chan *record.Record, len(payloads))
	for i, p := range payloads {
		in <- &record.Record{ID: i + 1, Data: p, Context: context.Background()}
	}
	close(in)

	out := make(chan *record.Record, 4096)
	err := tk.Run(in, out)
	close(out)

	var emitted []*record.Record
	for r := range out {
		emitted = append(emitted, r)
	}

	return emitted, err

}
