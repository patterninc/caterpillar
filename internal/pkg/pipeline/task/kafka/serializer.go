package kafka

import (
	"encoding/json"
	"fmt"

	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry"
	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry/serde"
	"github.com/confluentinc/confluent-kafka-go/v2/schemaregistry/serde/avrov2"
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

// newCodecForFormat returns the codec for the given format string.
// An empty format defaults to FormatJSON (backward compatible).
func newCodecForFormat(format string, schemaCfg schemaRegistryConfig) (messageCodec, error) {
	switch format {
	case "", FormatJSON:
		return jsonCodec{}, nil
	case FormatAvro:
		if schemaCfg.URL == "" {
			return nil, fmt.Errorf("schema_registry_url is required for avro format")
		}
		return newAvroCodec(schemaCfg)
	default:
		return nil, fmt.Errorf("unsupported format %q — supported values: %s, %s", format, FormatJSON, FormatAvro)
	}
}

// jsonCodec passes raw bytes through unchanged (default format).
type jsonCodec struct{}

func (jsonCodec) serialize(_ string, data []byte) ([]byte, error)   { return data, nil }
func (jsonCodec) deserialize(_ string, data []byte) ([]byte, error) { return data, nil }

// avroCodec encodes/decodes messages using Confluent Schema Registry Avro.
type avroCodec struct {
	ser   *avrov2.Serializer
	deser *avrov2.Deserializer
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

	return &avroCodec{ser: ser, deser: deser}, nil
}

// serialize marshals data as JSON then encodes it to Avro using the latest registered schema.
// Note: json.Unmarshal converts all numbers to float64; Avro int/long fields may reject these — convert before sending.
func (a *avroCodec) serialize(topic string, data []byte) ([]byte, error) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("record must be valid JSON for Avro serialization: %w", err)
	}
	return a.ser.Serialize(topic, &msg)
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
