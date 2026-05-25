package kafka

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry"
	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry/serde"
	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry/serde/avrov2"
	"github.com/hamba/avro/v2"
)

// Supported message format values for the `format` config field.
const (
	FormatJSON = "json" // default — raw bytes pass through unchanged
	FormatAvro = "avro" // Confluent Avro with Schema Registry
)

// messageCodec serializes outgoing records and deserializes incoming messages for a Kafka topic.
type messageCodec interface {
	serialize(topic string, data []byte) ([]byte, error)
	deserialize(topic string, data []byte) ([]byte, error)
}

// codecFormat holds the constructor for a message codec.
type codecFormat struct {
	newCodec func(schemaCfg schemaRegistryConfig) (messageCodec, error)
}

// jsonCodec passes raw bytes through unchanged (default format).
type jsonCodec struct{}

// avroCodec encodes/decodes messages using Confluent Schema Registry Avro.
type avroCodec struct {
	ser      *avrov2.Serializer
	deser    *avrov2.Deserializer
	srClient schemaregistry.Client

	schemaMu sync.Mutex
	schema   avro.Schema
}

var formatHandlers = map[string]codecFormat{
	FormatJSON: {
		newCodec: func(_ schemaRegistryConfig) (messageCodec, error) {
			return jsonCodec{}, nil
		},
	},
	FormatAvro: {
		newCodec: func(cfg schemaRegistryConfig) (messageCodec, error) {
			if cfg.URL == "" {
				return nil, fmt.Errorf("schema_registry_url is required for format %q", FormatAvro)
			}
			return newAvroCodec(cfg)
		},
	},
}

func (jsonCodec) serialize(_ string, data []byte) ([]byte, error)   { return data, nil }
func (jsonCodec) deserialize(_ string, data []byte) ([]byte, error) { return data, nil }

// newCodecForFormat returns the codec for the given format string.
// An empty format defaults to FormatJSON (backward compatible).
func newCodecForFormat(format string, schemaCfg schemaRegistryConfig) (messageCodec, error) {
	if format == "" {
		format = FormatJSON
	}
	h, ok := formatHandlers[format]
	if !ok {
		return nil, fmt.Errorf("unsupported format %q — supported values: %s, %s", format, FormatJSON, FormatAvro)
	}
	return h.newCodec(schemaCfg)
}

func newAvroCodec(cfg schemaRegistryConfig) (*avroCodec, error) {
	srClient, err := newSchemaRegistryClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create schema registry client: %w", err)
	}

	serConf := avrov2.NewSerializerConfig()
	serConf.AutoRegisterSchemas = false
	serConf.UseLatestVersion = true
	ser, err := avrov2.NewSerializer(srClient, serde.ValueSerde, serConf)
	if err != nil {
		return nil, fmt.Errorf("failed to create avro serializer: %w", err)
	}

	deser, err := avrov2.NewDeserializer(srClient, serde.ValueSerde, avrov2.NewDeserializerConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create avro deserializer: %w", err)
	}

	return &avroCodec{ser: ser, deser: deser, srClient: srClient}, nil
}

// loadSchema fetches and parses the latest value schema for the topic, cached on success.
// Errors are not cached so transient Schema Registry failures can be retried.
func (a *avroCodec) loadSchema(topic string) (avro.Schema, error) {
	a.schemaMu.Lock()
	defer a.schemaMu.Unlock()
	if a.schema != nil {
		return a.schema, nil
	}
	meta, err := a.srClient.GetLatestSchemaMetadata(topic + "-value")
	if err != nil {
		return nil, fmt.Errorf("fetch schema for %s-value: %w", topic, err)
	}
	schema, err := avro.Parse(meta.Schema)
	if err != nil {
		return nil, fmt.Errorf("parse schema for %s-value: %w", topic, err)
	}
	a.schema = schema
	return a.schema, nil
}

// serialize marshals data as JSON, rewrites untagged unions to Avro JSON-tagged form,
// then encodes to Avro using the latest registered schema.
// Note: json.Unmarshal converts all numbers to float64; Avro int/long fields may reject these — convert before sending.
func (a *avroCodec) serialize(topic string, data []byte) ([]byte, error) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("record must be valid JSON for Avro serialization: %w", err)
	}

	schema, err := a.loadSchema(topic)
	if err != nil {
		return nil, err
	}
	tagged, ok := tagUnions(schema, msg).(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("internal: tagged root is not a record map")
	}
	return a.ser.Serialize(topic, &tagged)
}

// tagUnions walks the schema and value together, rewriting any non-null union
// value into Avro JSON-tagged form ({"branchTypeName": value}). hamba/avro
// requires this disambiguation when the value is decoded from generic JSON,
// since map[string]any can match multiple union branches.
// It also coerces float64 (json.Unmarshal's default for all numbers) into
// int64/int32 for Avro long/int fields, which hamba refuses to accept as float64.
func tagUnions(schema avro.Schema, value any) any {
	if value == nil {
		return nil
	}
	switch s := schema.(type) {
	case *avro.UnionSchema:
		branch := pickNonNullBranch(s)
		if branch == nil {
			return value
		}
		if m, ok := value.(map[string]any); ok && len(m) == 1 {
			tag := branchTagName(branch)
			if inner, present := m[tag]; present {
				return map[string]any{tag: tagUnions(branch, inner)}
			}
		}
		tagged := tagUnions(branch, value)
		return map[string]any{branchTagName(branch): tagged}
	case *avro.RecordSchema:
		m, ok := value.(map[string]any)
		if !ok {
			return value
		}
		for _, f := range s.Fields() {
			if v, present := m[f.Name()]; present {
				m[f.Name()] = tagUnions(f.Type(), v)
			}
		}
		return m
	case *avro.ArraySchema:
		arr, ok := value.([]any)
		if !ok {
			return value
		}
		for i, v := range arr {
			arr[i] = tagUnions(s.Items(), v)
		}
		return arr
	case *avro.MapSchema:
		m, ok := value.(map[string]any)
		if !ok {
			return value
		}
		for k, v := range m {
			m[k] = tagUnions(s.Values(), v)
		}
		return m
	case *avro.RefSchema:
		return tagUnions(s.Schema(), value)
	case *avro.PrimitiveSchema:
		return coerceNumber(s, value)
	}
	return value
}

// coerceNumber converts float64 (from json.Unmarshal) into the Go value type
// that hamba/avro requires for the given Avro primitive schema.
//   - int  -> int32
//   - long -> int64
//   - long + timestamp-millis / timestamp-micros / local-timestamp-* -> time.Time
//   - int  + date -> time.Time
//   - long + time-micros -> time.Duration
//   - int  + time-millis -> time.Duration
func coerceNumber(s *avro.PrimitiveSchema, value any) any {
	f, ok := value.(float64)
	if !ok {
		return value
	}
	logical := ""
	if l := s.Logical(); l != nil {
		logical = string(l.Type())
	}
	switch s.Type() {
	case avro.Long:
		i, ok := floatToInt64(f)
		if !ok {
			return value
		}
		switch logical {
		case "timestamp-millis", "local-timestamp-millis":
			return time.UnixMilli(i).UTC()
		case "timestamp-micros", "local-timestamp-micros":
			return time.UnixMicro(i).UTC()
		case "time-micros":
			return time.Duration(i) * time.Microsecond
		}
		return i
	case avro.Int:
		i, ok := floatToInt32(f)
		if !ok {
			return value
		}
		switch logical {
		case "date":
			return time.Unix(int64(i)*86400, 0).UTC()
		case "time-millis":
			return time.Duration(i) * time.Millisecond
		}
		return i
	}
	return value
}

// floatToInt64 returns f as int64 only if f is a finite integer representable
// in int64. float64 has 53 bits of mantissa so values beyond ±2^53 cannot be
// distinguished from neighbours and are rejected.
func floatToInt64(f float64) (int64, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) {
		return 0, false
	}
	const maxSafe = 1 << 53
	if f > maxSafe || f < -maxSafe {
		return 0, false
	}
	return int64(f), true
}

// floatToInt32 returns f as int32 only if f is a finite integer in int32 range.
func floatToInt32(f float64) (int32, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) || f != math.Trunc(f) {
		return 0, false
	}
	if f > math.MaxInt32 || f < math.MinInt32 {
		return 0, false
	}
	return int32(f), true
}

// pickNonNullBranch returns the single non-null branch of a [null, T] union,
// or nil if the union shape isn't nullable-with-one-branch.
func pickNonNullBranch(u *avro.UnionSchema) avro.Schema {
	var nonNull avro.Schema
	for _, t := range u.Types() {
		if t.Type() == avro.Null {
			continue
		}
		if nonNull != nil {
			// More than one non-null branch — can't disambiguate without inspecting the value type.
			return nil
		}
		nonNull = t
	}
	return nonNull
}

// branchTagName returns the Avro JSON tag for a union branch.
// Named types use full name; logical-typed primitives use "<type>.<logicalType>"
// (matches hamba/avro v2's schemaTypeName); everything else uses the type keyword.
func branchTagName(s avro.Schema) string {
	if ref, ok := s.(*avro.RefSchema); ok {
		s = ref.Schema()
	}
	if named, ok := s.(avro.NamedSchema); ok {
		return named.FullName()
	}
	name := string(s.Type())
	if lts, ok := s.(avro.LogicalTypeSchema); ok {
		if lt := lts.Logical(); lt != nil {
			name += "." + string(lt.Type())
		}
	}
	return name
}

func (a *avroCodec) deserialize(topic string, data []byte) ([]byte, error) {
	var result map[string]interface{}
	if err := a.deser.DeserializeInto(topic, data, &result); err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

// newSchemaRegistryClient creates a schema registry client, with basic auth when credentials are set.
func newSchemaRegistryClient(cfg schemaRegistryConfig) (schemaregistry.Client, error) {
	var srCfg *schemaregistry.Config
	if cfg.Username != "" {
		srCfg = schemaregistry.NewConfigWithBasicAuthentication(cfg.URL, cfg.Username, cfg.Password)
	} else {
		srCfg = schemaregistry.NewConfig(cfg.URL)
	}
	return schemaregistry.NewClient(srCfg)
}
