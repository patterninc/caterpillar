package kafka

import (
	"context"
	"fmt"
	"sync"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/google/uuid"

	"github.com/patterninc/caterpillar/internal/pkg/duration"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

const (
	defaultTimeout          = duration.Duration(15 * time.Second)
	defaultRetryLimit       = 5
	defaultFlushInterval    = duration.Duration(2 * time.Second)
	defaultCommitIntervalMs = 5000
	defaultBatchSize        = 100
	defaultAutoOffsetReset  = "latest"

	// standaloneGroupPrefix is the group.id used for direct-assign reads (no group_id set); broker needs PREFIXED ACL on this prefix.
	standaloneGroupPrefix = "caterpillar-standalone-"
)

// schemaRegistryConfig holds Schema Registry connection details; required when format is "avro".
type schemaRegistryConfig struct {
	URL      string `yaml:"schema_registry_url,omitempty" json:"schema_registry_url,omitempty"`           // Schema Registry URL; required when format is "avro"
	Username string `yaml:"schema_registry_username,omitempty" json:"schema_registry_username,omitempty"` // Schema Registry basic auth username
	Password string `yaml:"schema_registry_password,omitempty" json:"schema_registry_password,omitempty"` // Schema Registry basic auth password
}

type kafka struct {
	task.ServerBase    `yaml:",inline" json:",inline"`
	BootstrapServer    string               `yaml:"bootstrap_server" json:"bootstrap_server"`                                                                  // "host:port"
	Topic              string               `yaml:"topic" json:"topic"`                                                                                        // topic to read from or write to
	ServerAuthType     string               `yaml:"server_auth_type,omitempty" json:"server_auth_type,omitempty"`                                              // "none", "tls"
	Cert               string               `yaml:"cert,omitempty" json:"cert,omitempty"`                                                                      // used for Server TLS authentication
	CertPath           string               `yaml:"cert_path,omitempty" json:"cert_path,omitempty"`                                                            // used for Server TLS authentication
	UserAuthType       string               `yaml:"user_auth_type" json:"user_auth_type"`                                                                      // "none", "sasl", "scram"
	Username           string               `yaml:"username,omitempty" json:"username,omitempty"`                                                              // used for user SASL/Scram authentication
	Password           string               `yaml:"password,omitempty" json:"password,omitempty"`                                                              // used for user SASL/Scram authentication
	Timeout            duration.Duration    `yaml:"timeout,omitempty" json:"timeout,omitempty"`                                                                // connection, read, write, commit timeout
	BatchFlushInterval duration.Duration    `yaml:"batch_flush_interval,omitempty" json:"batch_flush_interval,omitempty"`                                      // interval to flush incomplete batches
	GroupID            string               `yaml:"group_id,omitempty" json:"group_id,omitempty"`                                                              // the consumer group id (optional)
	AutoOffsetReset    string               `yaml:"auto_offset_reset,omitempty" json:"auto_offset_reset,omitempty" validate:"omitempty,oneof=earliest latest"` // group-mode reset policy when stored offset is out of range; "earliest" (default) or "latest"
	BatchSize          int                  `yaml:"batch_size,omitempty" json:"batch_size,omitempty"`                                                          // max messages per producer batch (maps to batch.num.messages); defaults to 100
	MaxRecords         int                  `yaml:"max_records,omitempty" json:"max_records,omitempty"`                                                        // stop reading after this many records (0 = unlimited)
	RetryLimit         *int                 `yaml:"retry_limit,omitempty" json:"retry_limit,omitempty"`                                                        // number of retries for read errors
	Idempotent         bool                 `yaml:"idempotent,omitempty" json:"idempotent,omitempty"`                                                          // enable idempotent producer
	Format             string               `yaml:"format,omitempty" json:"format,omitempty"`                                                                  // message format: "json" (default) or "avro"
	SchemaRegistry     schemaRegistryConfig `yaml:",inline" json:",inline"`                                                                                    // Schema Registry connection — required when format is "avro"
}

func New() (task.Task, error) {
	return &kafka{}, nil
}

func (k *kafka) Init() error {
	if k.BootstrapServer == "" {
		return fmt.Errorf("bootstrap_server is required")
	}
	if k.Topic == "" {
		return fmt.Errorf("topic is required")
	}
	if k.Timeout <= 0 {
		k.Timeout = defaultTimeout
	}
	if k.ServerAuthType == "" {
		k.ServerAuthType = "none"
	}
	if k.UserAuthType == "" {
		k.UserAuthType = "none"
	}
	if k.BatchFlushInterval <= 0 {
		k.BatchFlushInterval = defaultFlushInterval
	}
	if k.BatchSize <= 0 {
		k.BatchSize = defaultBatchSize
	}
	if k.RetryLimit == nil || *k.RetryLimit < 0 {
		k.RetryLimit = new(int)
		*k.RetryLimit = defaultRetryLimit
	}
	if k.AutoOffsetReset == "" {
		k.AutoOffsetReset = defaultAutoOffsetReset
	}

	cfg, err := k.buildBaseConfig()
	if err != nil {
		return fmt.Errorf("failed to build kafka config: %w", err)
	}
	a, err := ckafka.NewAdminClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create kafka admin client: %w", err)
	}
	defer a.Close()
	// Use defaultTimeout for init probe — SCRAM+TLS handshake needs multiple round trips, short user timeouts would fail.
	initTimeoutMs := int(time.Duration(defaultTimeout).Milliseconds())
	if _, err = a.GetMetadata(nil, false, initTimeoutMs); err != nil {
		return fmt.Errorf("failed to connect to kafka broker: %w", err)
	}

	return nil
}

func (k *kafka) Run(input <-chan *record.Record, output chan<- *record.Record) error {
	if input != nil && output != nil {
		return task.ErrPresentInputOutput
	}

	if input != nil {
		return k.write(input)
	}

	return k.read(context.Background(), output)
}

// write produces records to the Kafka topic using the codec selected by the format field.
func (k *kafka) write(input <-chan *record.Record) error {
	cfg, err := k.buildProducerConfig()
	if err != nil {
		return fmt.Errorf("failed to build producer config: %w", err)
	}

	p, err := ckafka.NewProducer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create producer: %w", err)
	}
	defer p.Close()

	codec, err := k.newCodec()
	if err != nil {
		return err
	}

	// deliveryCh is drained by a goroutine; closed after Flush so wg.Wait() guarantees no race on firstDeliveryErr.
	deliveryCh := make(chan ckafka.Event, 100)
	var (
		wg               sync.WaitGroup
		firstDeliveryErr error
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for e := range deliveryCh {
			if m, ok := e.(*ckafka.Message); ok && m.TopicPartition.Error != nil && firstDeliveryErr == nil {
				firstDeliveryErr = m.TopicPartition.Error
				fmt.Printf("delivery failed for topic %s partition %d: %v\n",
					k.Topic, m.TopicPartition.Partition, m.TopicPartition.Error)
			}
		}
	}()

	var produceErr error
	for {
		r, ok := k.GetRecord(input)
		if !ok {
			break
		}

		msgBytes, err := codec.serialize(k.Topic, r.Data)
		if err != nil {
			produceErr = fmt.Errorf("failed to serialize record for topic %s: %w", k.Topic, err)
			break
		}

		if err = p.Produce(&ckafka.Message{
			TopicPartition: ckafka.TopicPartition{Topic: &k.Topic, Partition: ckafka.PartitionAny},
			Value:          msgBytes,
		}, deliveryCh); err != nil {
			produceErr = fmt.Errorf("failed to enqueue message to topic %s: %w", k.Topic, err)
			break
		}
	}

	// Always flush so enqueued messages get delivery reports and the goroutine exits cleanly.
	timeout := time.Duration(k.Timeout)
	remaining := p.Flush(int(timeout.Milliseconds()))

	// Close the producer BEFORE closing deliveryCh:
	p.Close()
	close(deliveryCh)
	wg.Wait()

	if produceErr != nil {
		return produceErr
	}
	if firstDeliveryErr != nil {
		return fmt.Errorf("delivery failed for topic %s: %w", k.Topic, firstDeliveryErr)
	}
	if remaining > 0 {
		return fmt.Errorf("%d messages failed to deliver to topic %s within %s", remaining, k.Topic, timeout)
	}
	return nil
}

// read polls messages from the topic, standalone mode reads from beginning on every run, group mode resumes from committed offsets.
func (k *kafka) read(ctx context.Context, output chan<- *record.Record) error {
	if k.EndAfter > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(k.EndAfter))
		defer cancel()
	}

	standalone := k.GroupID == ""

	var cfg *ckafka.ConfigMap
	var err error
	if standalone {
		cfg, err = k.buildStandaloneConsumerConfig()
	} else {
		cfg, err = k.buildConsumerConfig()
	}
	if err != nil {
		return fmt.Errorf("failed to build consumer config: %w", err)
	}

	c, err := ckafka.NewConsumer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			fmt.Printf("warning: error closing kafka consumer: %v\n", err)
		}
	}()

	if standalone {
		fmt.Printf("no group_id set — standalone read from beginning of topic %s\n", k.Topic)
		if err := k.assignAllPartitions(c); err != nil {
			return fmt.Errorf("failed to assign partitions: %w", err)
		}
	} else {
		if err := c.SubscribeTopics([]string{k.Topic}, nil); err != nil {
			return fmt.Errorf("failed to subscribe to topic %s: %w", k.Topic, err)
		}
	}

	codec, err := k.newCodec()
	if err != nil {
		return err
	}

	timeout := time.Duration(k.Timeout)
	retriesNumber := 0
	recordsRead := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("kafka end_after duration reached for topic %s, stopping reader\n", k.Topic)
			return nil
		default:
		}

		msg, err := c.ReadMessage(timeout)
		if err != nil {
			if kafkaErr, ok := err.(ckafka.Error); ok && kafkaErr.Code() == ckafka.ErrTimedOut {
				retriesNumber++
				fmt.Printf("kafka read timeout for attempt #%d on topic %s\n", retriesNumber, k.Topic)
			} else if !k.shouldRetry(err) {
				return err
			} else {
				retriesNumber++
				fmt.Printf("kafka error reading message attempt #%d: %v\n", retriesNumber, err)
			}

			if retriesNumber > *k.RetryLimit {
				fmt.Printf("kafka error while reading message, reached retry limit (%d), stopping reader\n", *k.RetryLimit)
				return nil
			}
			continue
		}
		retriesNumber = 0

		data, err := codec.deserialize(k.Topic, msg.Value)
		if err != nil {
			return fmt.Errorf("failed to deserialize message from topic %s: %w", k.Topic, err)
		}

		k.SendData(ctx, data, output)

		// Only store offsets for group consumers — standalone reads never commit.
		if !standalone {
			if _, err := c.StoreMessage(msg); err != nil {
				fmt.Printf("warning: failed to store offset for topic %s partition %d: %v\n",
					k.Topic, msg.TopicPartition.Partition, err)
			}
		}

		recordsRead++
		if k.MaxRecords > 0 && recordsRead >= k.MaxRecords {
			fmt.Printf("kafka max_records (%d) reached for topic %s, stopping reader\n", k.MaxRecords, k.Topic)
			return nil
		}
	}
}

func (k *kafka) newCodec() (messageCodec, error) {
	return newCodecForFormat(k.Format, k.SchemaRegistry)
}

// assignAllPartitions assigns all topic partitions at OffsetBeginning, bypassing the consumer group protocol.
func (k *kafka) assignAllPartitions(c *ckafka.Consumer) error {
	initTimeoutMs := int(time.Duration(defaultTimeout).Milliseconds())
	meta, err := c.GetMetadata(&k.Topic, false, initTimeoutMs)
	if err != nil {
		return fmt.Errorf("failed to get metadata for topic %s: %w", k.Topic, err)
	}

	topicMeta, ok := meta.Topics[k.Topic]
	if !ok || len(topicMeta.Partitions) == 0 {
		return fmt.Errorf("topic %s not found or has no partitions", k.Topic)
	}

	partitions := make([]ckafka.TopicPartition, len(topicMeta.Partitions))
	for i, p := range topicMeta.Partitions {
		partitions[i] = ckafka.TopicPartition{
			Topic:     &k.Topic,
			Partition: p.ID,
			Offset:    ckafka.OffsetBeginning,
		}
	}
	return c.Assign(partitions)
}

// buildBaseConfig builds the ConfigMap entries shared by both producers and consumers.
func (k *kafka) buildBaseConfig() (*ckafka.ConfigMap, error) {
	cfg := &ckafka.ConfigMap{
		"bootstrap.servers": k.BootstrapServer,
		"security.protocol": k.securityProtocol(),
	}

	if k.ServerAuthType == "tls" {
		switch {
		case k.Cert != "":
			_ = cfg.SetKey("ssl.ca.pem", k.Cert)
		case k.CertPath != "":
			_ = cfg.SetKey("ssl.ca.location", k.CertPath)
		default:
			return nil, fmt.Errorf("cert or cert_path is required when server_auth_type is tls")
		}
	}

	switch k.UserAuthType {
	case "scram":
		if k.Username == "" || k.Password == "" {
			return nil, fmt.Errorf("username and password are required for scram authentication")
		}
		_ = cfg.SetKey("sasl.mechanisms", "SCRAM-SHA-512")
		_ = cfg.SetKey("sasl.username", k.Username)
		_ = cfg.SetKey("sasl.password", k.Password)
	case "sasl":
		if k.Username == "" || k.Password == "" {
			return nil, fmt.Errorf("username and password are required for sasl authentication")
		}
		_ = cfg.SetKey("sasl.mechanisms", "PLAIN")
		_ = cfg.SetKey("sasl.username", k.Username)
		_ = cfg.SetKey("sasl.password", k.Password)
	case "mtls":
		return nil, fmt.Errorf("mtls user authentication is not implemented")
	case "none":
	default:
		return nil, fmt.Errorf("unknown user_auth_type: %s", k.UserAuthType)
	}

	return cfg, nil
}

func (k *kafka) buildProducerConfig() (*ckafka.ConfigMap, error) {
	cfg, err := k.buildBaseConfig()
	if err != nil {
		return nil, err
	}

	_ = cfg.SetKey("linger.ms", int(time.Duration(k.BatchFlushInterval).Milliseconds()))
	_ = cfg.SetKey("batch.num.messages", k.BatchSize)
	_ = cfg.SetKey("message.timeout.ms", int(time.Duration(k.Timeout).Milliseconds()))
	_ = cfg.SetKey("acks", "all")

	if k.Idempotent {
		// idempotent producer requires acks=all and max.in.flight ≤ 5
		_ = cfg.SetKey("enable.idempotence", true)
		_ = cfg.SetKey("max.in.flight.requests.per.connection", 5)
	}

	return cfg, nil
}

// buildConsumerConfig builds config for group consumer mode; auto-commits every 5s, offsets stored only after downstream delivery.
func (k *kafka) buildConsumerConfig() (*ckafka.ConfigMap, error) {
	cfg, err := k.buildBaseConfig()
	if err != nil {
		return nil, err
	}

	_ = cfg.SetKey("auto.offset.reset", k.AutoOffsetReset)
	_ = cfg.SetKey("session.timeout.ms", 30000)
	_ = cfg.SetKey("heartbeat.interval.ms", 3000)
	_ = cfg.SetKey("enable.auto.offset.store", false)
	_ = cfg.SetKey("enable.auto.commit", true)
	_ = cfg.SetKey("auto.commit.interval.ms", defaultCommitIntervalMs)
	_ = cfg.SetKey("isolation.level", "read_committed")
	_ = cfg.SetKey("group.id", k.GroupID)

	return cfg, nil
}

// buildStandaloneConsumerConfig builds config for standalone read mode; never commits offsets, always reads from OffsetBeginning.
func (k *kafka) buildStandaloneConsumerConfig() (*ckafka.ConfigMap, error) {
	cfg, err := k.buildBaseConfig()
	if err != nil {
		return nil, err
	}

	_ = cfg.SetKey("group.id", standaloneGroupPrefix+k.Topic+"-"+uuid.New().String())
	_ = cfg.SetKey("enable.auto.commit", false)
	_ = cfg.SetKey("auto.offset.reset", "earliest")
	_ = cfg.SetKey("isolation.level", "read_committed")

	return cfg, nil
}

// securityProtocol returns the Confluent security.protocol value based on TLS and auth settings.
func (k *kafka) securityProtocol() string {
	hasTLS := k.ServerAuthType == "tls"
	hasSASL := k.UserAuthType == "sasl" || k.UserAuthType == "scram"
	switch {
	case hasTLS && hasSASL:
		return "SASL_SSL"
	case hasTLS:
		return "SSL"
	case hasSASL:
		return "SASL_PLAINTEXT"
	default:
		return "PLAINTEXT"
	}
}

func (k *kafka) shouldRetry(err error) bool {
	if kafkaErr, ok := err.(ckafka.Error); ok {
		switch kafkaErr.Code() {
		case ckafka.ErrUnknownTopicOrPart, ckafka.ErrTopicException,
			ckafka.ErrGroupAuthorizationFailed, ckafka.ErrTopicAuthorizationFailed:
			return false
		}
	}
	return true
}
