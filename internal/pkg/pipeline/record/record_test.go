package record

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextValues(t *testing.T) {

	t.Run("nil context is initialised on set", func(t *testing.T) {
		r := &Record{}
		r.SetContextValue("k", "v")
		require.NotNil(t, r.Context)
		got, ok := r.GetContextValue("k")
		assert.True(t, ok)
		assert.Equal(t, "v", got)
	})

	t.Run("nil context reads as missing", func(t *testing.T) {
		got, ok := (&Record{}).GetContextValue("k")
		assert.False(t, ok)
		assert.Equal(t, "", got)
	})

	t.Run("missing key", func(t *testing.T) {
		r := &Record{Context: context.Background()}
		_, ok := r.GetContextValue("k")
		assert.False(t, ok)
	})

	t.Run("later set shadows earlier", func(t *testing.T) {
		r := &Record{Context: context.Background()}
		r.SetContextValue("k", "1")
		r.SetContextValue("k", "2")
		got, _ := r.GetContextValue("k")
		assert.Equal(t, "2", got)
	})

	t.Run("parent context values survive", func(t *testing.T) {
		r := &Record{Context: context.Background()}
		r.SetContextValue("a", "1")
		r.SetContextValue("b", "2")
		a, _ := r.GetContextValue("a")
		b, _ := r.GetContextValue("b")
		assert.Equal(t, "1", a)
		assert.Equal(t, "2", b)
	})

	t.Run("keys do not collide with plain string keys", func(t *testing.T) {
		type foreign string
		r := &Record{Context: context.WithValue(context.Background(), foreign("k"), "x")}
		_, ok := r.GetContextValue("k")
		assert.False(t, ok)
	})

	t.Run("non-string value reads as not ok", func(t *testing.T) {
		r := &Record{Context: context.WithValue(context.Background(), contextKey("k"), 7)}
		got, ok := r.GetContextValue("k")
		assert.False(t, ok)
		assert.Equal(t, "", got)
	})

}

func TestMarshalJSON(t *testing.T) {

	tests := []struct {
		name     string
		record   Record
		expected string
	}{
		{
			name:     "Data is rendered as a string, not base64",
			record:   Record{ID: 3, Origin: "src", Data: []byte(`{"a":1}`)},
			expected: `{"data":"{\"a\":1}","id":3,"origin":"src"}`,
		},
		{
			name:     "Zero fields are omitted but data is always present",
			record:   Record{},
			expected: `{"data":""}`,
		},
		{
			name:     "Context is never serialised",
			record:   Record{ID: 1, Data: []byte("x"), Context: context.Background()},
			expected: `{"data":"x","id":1}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.record)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(got))
			assert.JSONEq(t, tt.expected, string(tt.record.Bytes()))
		})
	}

}

func TestMarshalJSONViaPointer(t *testing.T) {

	r := &Record{ID: 9, Data: []byte("hello")}
	got, err := json.Marshal(r)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(got, &decoded))
	assert.Equal(t, "hello", decoded["data"])
	assert.Equal(t, float64(9), decoded["id"])

}
