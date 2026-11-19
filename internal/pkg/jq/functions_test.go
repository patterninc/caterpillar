package jq

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShuffle(t *testing.T) {

	t.Run("keeps every element", func(t *testing.T) {
		got, err := mustQuery(t, `shuffle | sort`).Execute([]byte(`[5,3,1,4,2]`))
		require.NoError(t, err)
		assert.Equal(t, []any{float64(1), float64(2), float64(3), float64(4), float64(5)}, got)
	})

	t.Run("does not mutate its input", func(t *testing.T) {
		in := []any{1, 2, 3, 4, 5, 6, 7, 8}
		out := shuffle(in, nil)
		assert.Equal(t, []any{1, 2, 3, 4, 5, 6, 7, 8}, in)
		assert.ElementsMatch(t, in, out)
	})

	t.Run("empty array", func(t *testing.T) {
		got, err := mustQuery(t, `shuffle`).Execute([]byte(`[]`))
		require.NoError(t, err)
		assert.Equal(t, []any{}, got)
	})

	t.Run("non-array is an error", func(t *testing.T) {
		_, err := mustQuery(t, `shuffle`).Execute([]byte(`{"a":1}`))
		assert.ErrorContains(t, err, "expected array")
	})

}

func TestSleep(t *testing.T) {

	t.Run("passes input through after sleeping", func(t *testing.T) {
		start := time.Now()
		got, err := mustQuery(t, `sleep("10ms")`).Execute([]byte(`{"a":1}`))
		require.NoError(t, err)
		assert.Equal(t, map[string]any{"a": float64(1)}, got)
		assert.GreaterOrEqual(t, time.Since(start), 10*time.Millisecond)
	})

	t.Run("invalid duration", func(t *testing.T) {
		_, err := mustQuery(t, `sleep("soon")`).Execute([]byte(`1`))
		assert.ErrorContains(t, err, "invalid duration format")
	})

	t.Run("non-string duration", func(t *testing.T) {
		_, err := mustQuery(t, `sleep(5)`).Execute([]byte(`1`))
		assert.ErrorContains(t, err, "must be a duration string")
	})

	t.Run("wrong arity is rejected directly", func(t *testing.T) {
		got := sleep(1, nil)
		assert.Error(t, got.(error))
	})

}

func TestTranslateIsRegistered(t *testing.T) {

	opts := translateOption()
	assert.Len(t, opts, 1)

}
