package kafka

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"

	"github.com/patterninc/caterpillar/internal/pkg/duration"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
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
	ClientRack         string               `yaml:"client_rack,omitempty" json:"client_rack,omitempty"`                                                        // rack id for enabling rack-aware features
	AutoOffsetReset    string               `yaml:"auto_offset_reset,omitempty" json:"auto_offset_reset,omitempty" validate:"omitempty,oneof=earliest latest"` // group-mode reset policy when stored offset is out of range; "earliest" (default) or "latest"
	BatchSize          int                  `yaml:"batch_size,omitempty" json:"batch_size,omitempty"`                                                          // max messages per producer batch (maps to batch.num.messages); defaults to 100
	MaxRecords         int                  `yaml:"max_records,omitempty" json:"max_records,omitempty" validate:"omitempty,gte=0"`                             // stop reading after this many records (0 = unlimited); negative values are rejected at validation
	RetryLimit         *int                 `yaml:"retry_limit,omitempty" json:"retry_limit,omitempty"`                                                        // number of retries for read errors
	Idempotent         bool                 `yaml:"idempotent,omitempty" json:"idempotent,omitempty"`                                                          // enable idempotent producer
	Format             string               `yaml:"format,omitempty" json:"format,omitempty"`                                                                  // message format: "json" (default) or "avro"
	SchemaRegistry     schemaRegistryConfig `yaml:",inline" json:",inline"`                                                                                    // Schema Registry connection — required when format is "avro"

	tracker   *ack.Tracker
	readersMu sync.Mutex
	readers   []*reader
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

// Finish, not Run, waits for deferred stores: a join that emits on input close
// cannot settle until this task's output channel is closed, which happens only
// after Run returns.
func (k *kafka) Finish() error {
	if k.tracker != nil {
		k.tracker.Wait()
	}

	k.readersMu.Lock()
	readers := slices.Clone(k.readers)
	k.readers = nil
	k.readersMu.Unlock()

	var capped []int32
	for _, r := range readers {
		for partition, offset := range r.offsets.commitPositions() {
			if err := r.storeOffset(partition, offset); err != nil {
				fmt.Printf("warning: failed to store offset for topic %s partition %d: %v\n",
					k.Topic, partition, err)
			}
		}

		capped = append(capped, r.offsets.cappedPartitions()...)

		if err := r.commit(); err != nil {
			fmt.Printf("warning: failed to commit offsets for topic %s: %v\n", k.Topic, err)
		}
		if err := r.close(); err != nil {
			fmt.Printf("warning: error closing kafka consumer: %v\n", err)
		}
	}

	if len(capped) > 0 {
		slices.Sort(capped)
		return fmt.Errorf("kafka reader paused partitions with failed records: %v", capped)
	}

	return nil
}

func (k *kafka) ensureReadTracker() {
	k.readersMu.Lock()
	defer k.readersMu.Unlock()
	if k.tracker == nil {
		// StoreOffsets is local; the watermark already serializes
		k.tracker = ack.NewTracker(1)
	}
}

func (k *kafka) registerReader(r *reader) {
	k.readersMu.Lock()
	defer k.readersMu.Unlock()
	k.readers = append(k.readers, r)
}

func (k *kafka) newCodec() (messageCodec, error) {
	return newCodecForFormat(k.Format, k.SchemaRegistry)
}
