package kafka

import (
	"math"
	"testing"
	"time"

	"github.com/hamba/avro/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCodecForFormat(t *testing.T) {

	t.Run("empty defaults to json", func(t *testing.T) {
		c, err := newCodecForFormat("", schemaRegistryConfig{})
		require.NoError(t, err)
		assert.IsType(t, jsonCodec{}, c)
	})

	t.Run("json passes bytes through", func(t *testing.T) {
		c, err := newCodecForFormat(FormatJSON, schemaRegistryConfig{})
		require.NoError(t, err)

		out, err := c.serialize("topic", []byte(`{"a":1}`))
		require.NoError(t, err)
		assert.Equal(t, `{"a":1}`, string(out))

		out, err = c.deserialize("topic", []byte(`raw`))
		require.NoError(t, err)
		assert.Equal(t, `raw`, string(out))
	})

	t.Run("avro requires a registry url", func(t *testing.T) {
		_, err := newCodecForFormat(FormatAvro, schemaRegistryConfig{})
		assert.ErrorContains(t, err, "schema_registry_url is required")
	})

	t.Run("unknown format", func(t *testing.T) {
		_, err := newCodecForFormat("protobuf", schemaRegistryConfig{})
		assert.ErrorContains(t, err, `unsupported format "protobuf"`)
	})

}

const userSchema = `{
  "type": "record",
  "name": "User",
  "namespace": "com.example",
  "fields": [
    {"name": "id", "type": "long"},
    {"name": "age", "type": "int"},
    {"name": "email", "type": ["null", "string"], "default": null},
    {"name": "score", "type": ["null", "double"], "default": null},
    {"name": "created", "type": {"type": "long", "logicalType": "timestamp-millis"}},
    {"name": "tags", "type": {"type": "array", "items": ["null", "string"]}},
    {"name": "attrs", "type": {"type": "map", "values": "int"}},
    {"name": "address", "type": ["null", {"type": "record", "name": "Address", "fields": [{"name": "zip", "type": "int"}]}], "default": null}
  ]
}`

func TestTagUnions(t *testing.T) {

	schema := avro.MustParse(userSchema)

	in := map[string]any{
		"id":      float64(7),
		"age":     float64(30),
		"email":   "a@b.c",
		"score":   nil,
		"created": float64(1700000000000),
		"tags":    []any{"x", nil},
		"attrs":   map[string]any{"k": float64(2)},
		"address": map[string]any{"zip": float64(12345)},
		"extra":   "kept",
	}

	got, err := tagUnions(schema, in)
	require.NoError(t, err)

	m := got.(map[string]any)
	assert.Equal(t, int64(7), m["id"])
	assert.Equal(t, int32(30), m["age"])
	assert.Equal(t, map[string]any{"string": "a@b.c"}, m["email"])
	assert.Nil(t, m["score"])
	assert.Equal(t, time.UnixMilli(1700000000000).UTC(), m["created"])
	assert.Equal(t, []any{map[string]any{"string": "x"}, nil}, m["tags"])
	assert.Equal(t, map[string]any{"k": int32(2)}, m["attrs"])
	assert.Equal(t, map[string]any{"com.example.Address": map[string]any{"zip": int32(12345)}}, m["address"])
	assert.Equal(t, "kept", m["extra"])

	_, err = avro.Marshal(schema, m)
	assert.NoError(t, err)

}

func TestTagUnionsAlreadyTagged(t *testing.T) {

	schema := avro.MustParse(`["null", "long"]`)

	got, err := tagUnions(schema, map[string]any{"long": float64(5)})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"long": int64(5)}, got)

}

func TestTagUnionsRejectsAmbiguousUnions(t *testing.T) {

	schema := avro.MustParse(`{"type":"record","name":"R","fields":[{"name":"v","type":["null","string","long"]}]}`)

	_, err := tagUnions(schema, map[string]any{"v": "x"})
	assert.ErrorContains(t, err, `field "v": union with multiple non-null branches`)

}

func TestTagUnionsShapeMismatchesPassThrough(t *testing.T) {

	tests := []struct {
		name   string
		schema string
		value  any
	}{
		{name: "record given a scalar", schema: `{"type":"record","name":"R","fields":[]}`, value: "s"},
		{name: "array given a map", schema: `{"type":"array","items":"int"}`, value: map[string]any{}},
		{name: "map given an array", schema: `{"type":"map","values":"int"}`, value: []any{1.0}},
		{name: "string stays a string", schema: `"string"`, value: "s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tagUnions(avro.MustParse(tt.schema), tt.value)
			require.NoError(t, err)
			assert.Equal(t, tt.value, got)
		})
	}

}

func TestCoerceNumber(t *testing.T) {

	tests := []struct {
		name     string
		schema   string
		value    any
		expected any
	}{
		{name: "long", schema: `"long"`, value: 42.0, expected: int64(42)},
		{name: "int", schema: `"int"`, value: 42.0, expected: int32(42)},
		{name: "fractional long is left alone", schema: `"long"`, value: 1.5, expected: 1.5},
		{name: "int overflow is left alone", schema: `"int"`, value: float64(math.MaxInt32) + 1, expected: float64(math.MaxInt32) + 1},
		{name: "double is left alone", schema: `"double"`, value: 1.0, expected: 1.0},
		{name: "non-float is left alone", schema: `"long"`, value: "7", expected: "7"},
		{name: "timestamp-micros", schema: `{"type":"long","logicalType":"timestamp-micros"}`, value: 1e6, expected: time.UnixMicro(1e6).UTC()},
		{name: "local-timestamp-millis", schema: `{"type":"long","logicalType":"local-timestamp-millis"}`, value: 1000.0, expected: time.UnixMilli(1000).UTC()},
		{name: "time-micros", schema: `{"type":"long","logicalType":"time-micros"}`, value: 5.0, expected: 5 * time.Microsecond},
		{name: "date", schema: `{"type":"int","logicalType":"date"}`, value: 1.0, expected: time.Unix(86400, 0).UTC()},
		{name: "time-millis", schema: `{"type":"int","logicalType":"time-millis"}`, value: 5.0, expected: 5 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := avro.MustParse(tt.schema).(*avro.PrimitiveSchema)
			assert.Equal(t, tt.expected, coerceNumber(s, tt.value))
		})
	}

}

func TestFloatToInt(t *testing.T) {

	tests := []struct {
		name   string
		f      float64
		ok64   bool
		ok32   bool
		want64 int64
	}{
		{name: "zero", f: 0, ok64: true, ok32: true, want64: 0},
		{name: "negative", f: -12, ok64: true, ok32: true, want64: -12},
		{name: "fraction", f: 0.5},
		{name: "NaN", f: math.NaN()},
		{name: "Inf", f: math.Inf(1)},
		{name: "2^53 is safe", f: 1 << 53, ok64: true, want64: 1 << 53},
		{name: "beyond 2^53", f: 1 << 54},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := floatToInt64(tt.f)
			assert.Equal(t, tt.ok64, ok)
			if ok {
				assert.Equal(t, tt.want64, got)
			}
			_, ok = floatToInt32(tt.f)
			assert.Equal(t, tt.ok32, ok)
		})
	}

}

func TestPickNonNullBranch(t *testing.T) {

	_, err := pickNonNullBranch(avro.MustParse(`["null"]`).(*avro.UnionSchema))
	assert.ErrorContains(t, err, "no non-null branch")

	branch, err := pickNonNullBranch(avro.MustParse(`["string", "null"]`).(*avro.UnionSchema))
	require.NoError(t, err)
	assert.Equal(t, avro.String, branch.Type())

}

func TestBranchTagName(t *testing.T) {

	tests := []struct {
		schema   string
		expected string
	}{
		{schema: `"string"`, expected: "string"},
		{schema: `{"type":"long","logicalType":"timestamp-millis"}`, expected: "long.timestamp-millis"},
		{schema: `{"type":"record","name":"R","namespace":"ns","fields":[]}`, expected: "ns.R"},
		{schema: `{"type":"enum","name":"E","symbols":["A"]}`, expected: "E"},
		{schema: `{"type":"array","items":"int"}`, expected: "array"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, branchTagName(avro.MustParse(tt.schema)))
		})
	}

}
