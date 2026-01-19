package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	kg "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"

	"github.com/patterninc/caterpillar/internal/pkg/duration"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/task"
)

const (
	defaultTimeout       = duration.Duration(15 * time.Second)
	defaultBatchSize     = 100
	defaultRetryLimit    = 5
	defaultFlushInterval = duration.Duration(2 * time.Second)
)

type kafka struct {
	task.Base                   `yaml:",inline" json:",inline"`
	BootstrapServer             string            `yaml:"bootstrap_server" json:"bootstrap_server"`                             // "host:port"
	Topic                       string            `yaml:"topic" json:"topic"`                                                   // topic to read from or write to
	ServerAuthType              string            `yaml:"server_auth_type,omitempty" json:"server_auth_type,omitempty"`         // "none", "tls"
	Cert                        string            `yaml:"cert,omitempty" json:"cert,omitempty"`                                 // used for Server TLS authentication
	CertPath                    string            `yaml:"cert_path,omitempty" json:"cert_path,omitempty"`                       // used for Server TLS authentication
	UserAuthType                string            `yaml:"user_auth_type" json:"user_auth_type"`                                 // "none", "sasl", "scram", "mtls"
	UserCert                    string            `yaml:"user_cert,omitempty" json:"user_cert,omitempty"`                       // used for user mTLS authentication
	UserCertPath                string            `yaml:"user_cert_path,omitempty" json:"user_cert_path,omitempty"`             // used for user mTLS authentication
	Username                    string            `yaml:"username,omitempty" json:"username,omitempty"`                         // used for user SASL/Scram authentication
	Password                    string            `yaml:"password,omitempty" json:"password,omitempty"`                         // used for user SASL/Scram authentication
	Timeout                     duration.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`                           // connection, read, write, commit timeout
	BatchFlushInterval          duration.Duration `yaml:"batch_flush_interval,omitempty" json:"batch_flush_interval,omitempty"` // interval to flush incomplete batches
	GroupID                     string            `yaml:"group_id,omitempty" json:"group_id,omitempty"`                         // the consumer group id (optional)
	BatchSize                   int               `yaml:"batch_size,omitempty" json:"batch_size,omitempty"`                     // number of messages to read/write in a batch
	RetryLimit                  *int              `yaml:"retry_limit,omitempty" json:"retry_limit,omitempty"`                   // number of retries for read errors
	ctx                         context.Context   // parent context
	timeout                     time.Duration     // timeout duration calculated from Timeout
	batchFlushInterval          time.Duration     // batch flush interval calculated from BatchFlushInterval
	deadlineExceededReadRetries int               // number of retries left for deadline exceeded errors
	otherErrorReadRetries       int               // number of retries left for other errors
}

func New() (task.Task, error) {
	return &kafka{}, nil
}

func (k *kafka) Init() error {
	k.ctx = context.Background()
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
	if k.BatchSize <= 0 {
		k.BatchSize = defaultBatchSize
	}
	if k.BatchFlushInterval <= 0 {
		k.BatchFlushInterval = defaultFlushInterval
	}
	if k.RetryLimit == nil || *k.RetryLimit < 0 {
		k.RetryLimit = new(int)
		*k.RetryLimit = defaultRetryLimit
	}
	k.timeout = time.Duration(k.Timeout)
	k.batchFlushInterval = time.Duration(k.BatchFlushInterval)
	k.deadlineExceededReadRetries = *k.RetryLimit
	k.otherErrorReadRetries = *k.RetryLimit
	if k.batchFlushInterval >= k.timeout {
		return fmt.Errorf("batch_flush_interval (%s) must be less than timeout (%s)", k.batchFlushInterval, k.timeout)
	}

	// try connecting to kafka broker to validate config
	dialer, err := k.dial()
	if err != nil {
		return fmt.Errorf("failed to create kafka dialer: %w", err)
	}
	dialCtx, cancel := context.WithTimeout(k.ctx, k.timeout)
	defer cancel()
	conn, err := dialer.DialContext(dialCtx, "tcp", k.BootstrapServer)
	if err != nil {
		return fmt.Errorf("failed to connect to kafka broker: %w", err)
	}
	conn.Close()

	return nil
}

func (k *kafka) Run(input <-chan *record.Record, output chan<- *record.Record) error {
	if input != nil && output != nil {
		return task.ErrPresentInputOutput
	}

	// if input is not nil, this is a sink task
	if input != nil {
		return k.write(input)
	}

	// else, this is a source task
	return k.read(output)
}

// write writes records from the input channel to the Kafka topic
func (k *kafka) write(input <-chan *record.Record) error {
	dialer, err := k.dial()
	if err != nil {
		return fmt.Errorf("failed to create kafka dialer: %w", err)
	}

	writer := k.getWriter(dialer)

	defer func() {
		if err := writer.Close(); err != nil {
			fmt.Printf("warning: error closing kafka writer: %v\n", err)
		}
	}()

	for {
		r, ok := k.GetRecord(input)
		if !ok {
			break
		}

		// create a write context with timeout per message batch
		wctx, cancel := context.WithTimeout(k.ctx, k.timeout)
		err := writer.WriteMessages(wctx, kg.Message{Value: r.Data})
		cancel()
		if err != nil {
			return k.handleWriteError(err)
		}
	}
	return nil
}

// read reads messages from the Kafka topic and sends them to the output channel
func (k *kafka) read(output chan<- *record.Record) error {
	dialer, err := k.dial()
	if err != nil {
		return fmt.Errorf("failed to create kafka dialer: %w", err)
	}
	reader := k.getReader(dialer)
	defer func() {
		if err := reader.Close(); err != nil {
			fmt.Printf("warning: error closing kafka reader: %v\n", err)
		}
	}()

	for {
		select {
		case <-k.ctx.Done():
			return nil
		default:
			// read with a timeout so we can check for cancellation periodically
			fetchCtx, cancel := context.WithTimeout(k.ctx, k.timeout)
			m, err := reader.FetchMessage(fetchCtx)
			cancel()

			if err != nil {
				err, ok := k.handleReadError(err)
				if !ok {
					return err
				}
				continue
			}
			k.deadlineExceededReadRetries = *k.RetryLimit
			k.otherErrorReadRetries = *k.RetryLimit

			// process the message
			k.SendData(k.ctx, m.Value, output)

			if k.GroupID == "" {
				// if not using consumer group, no need to commit messages
				continue
			}

			// commit the message after successful processing
			cctx, cancel := context.WithTimeout(k.ctx, k.timeout)
			if err = reader.CommitMessages(cctx, m); err != nil {
				// log the commit error but continue processing
				// any new message will ensure that all previous
				// messages are eventually committed
				fmt.Printf("failed to commit message: %v\n", err)
			}
			cancel()
		}
	}
}

// dial creates a kafka dialer based on the authentication configuration
func (k *kafka) dial() (*kg.Dialer, error) {
	var mechanism sasl.Mechanism
	switch k.UserAuthType {
	case "none":
		mechanism = nil
	case "sasl", "scram":
		m, err := k.getSASLMechanism()
		if err != nil {
			return nil, err
		}
		mechanism = m
	case "mtls":
		// TODO: implement mTLS authentication
		return nil, fmt.Errorf("mtls user authentication is not implemented")
	default:
		return nil, fmt.Errorf("unknown user_auth_type: %s", k.UserAuthType)
	}

	return k.createDialer(mechanism)
}

// getReader creates a kafka reader based on whether GroupID is specified
func (k *kafka) getReader(dialer *kg.Dialer) *kg.Reader {
	readerConfig := kg.ReaderConfig{
		Brokers:       []string{k.BootstrapServer},
		Topic:         k.Topic,
		Dialer:        dialer,
		QueueCapacity: k.BatchSize,
	}
	if k.GroupID != "" {
		readerConfig.GroupID = k.GroupID
	} else {
		fmt.Printf("No group_id specified, will consume as standalone reader.\n")
	}
	return kg.NewReader(readerConfig)
}

// getWriter creates a kafka writer for the specified dialer
func (k *kafka) getWriter(dialer *kg.Dialer) *kg.Writer {
	return kg.NewWriter(kg.WriterConfig{
		Brokers:      []string{k.BootstrapServer},
		Topic:        k.Topic,
		Dialer:       dialer,
		Balancer:     &kg.LeastBytes{},     // TODO: look into other balancers
		BatchSize:    k.BatchSize,          // number of messages to batch before sending
		BatchTimeout: k.batchFlushInterval, // set flush interval for batch
	})
}

// handleWriteError processes errors returned from writer.WriteMessages
func (k *kafka) handleWriteError(err error) error {
	var we kg.WriteErrors
	if errors.As(err, &we) {
		var errStringBuilder strings.Builder
		errStringBuilder.WriteString(fmt.Sprintf("failed to write message to kafka with error count = %d.\nThe errors are:\n", we.Count()))
		for i, individualErr := range we {
			errStringBuilder.WriteString(fmt.Sprintf("%d   : %v\n", i, individualErr))
		}
		return fmt.Errorf("%s", errStringBuilder.String())
	}
	return fmt.Errorf("failed to write message to kafka: %w", err)
}

// handleReadError processes errors returned from reader.FetchMessage
func (k *kafka) handleReadError(err error) (returnErr error, shouldRetry bool) {
	if errors.Is(err, io.EOF) {
		// this is not reliable for kafka end of topic detection
		fmt.Printf("kafka reached end of topic: %v\n", k.Topic)
		return nil, false
	}
	if errors.Is(err, context.Canceled) {
		// not an error, just context cancellation
		fmt.Printf("kafka reader context canceled: %v\n", err)
		return nil, false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		// not an error, just context deadline exceeded, but need to limit retries
		fmt.Printf("kafka deadline exceeded while reading message for attempt #%d with error: %v\n", *k.RetryLimit-k.deadlineExceededReadRetries+1, err)
		k.deadlineExceededReadRetries--
		if k.deadlineExceededReadRetries <= 0 {
			fmt.Printf("kafka exceeded maximum deadline exceeded retries (%d), stopping reader\n", *k.RetryLimit)
			return nil, false
		}
		return nil, true
	}
	// other errors - log and retry up to retry limit
	fmt.Printf("kafka error while reading message for attempt #%d with error: %v\n", *k.RetryLimit-k.otherErrorReadRetries+1, err)
	k.otherErrorReadRetries--
	if k.otherErrorReadRetries <= 0 {
		return fmt.Errorf("kafka exceeded maximum other error retries (%d), last error: %w", *k.RetryLimit, err), false
	}
	return nil, true
}

// createDialer creates a kafka dialer with optional SASL mechanism and TLS configuration
func (k *kafka) createDialer(mechanism sasl.Mechanism) (*kg.Dialer, error) {
	dialer := &kg.Dialer{
		Timeout:   k.timeout,
		DualStack: true, // use both IPv4 and IPv6 in case either is available
	}
	if mechanism != nil {
		dialer.SASLMechanism = mechanism
	}
	if k.ServerAuthType == "tls" {
		tlsCfg, err := k.getTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create tls config: %w", err)
		}
		dialer.TLS = tlsCfg
	}
	return dialer, nil
}

// getSASLMechanism returns the appropriate SASL mechanism based on the UserAuthType
func (k *kafka) getSASLMechanism() (sasl.Mechanism, error) {
	if k.Username == "" || k.Password == "" {
		return nil, fmt.Errorf("username and password are required for SASL authentication")
	}
	if k.UserAuthType == "sasl" {
		return plain.Mechanism{Username: k.Username, Password: k.Password}, nil
	}
	if k.UserAuthType == "scram" {
		return scram.Mechanism(scram.SHA512, k.Username, k.Password)
	}
	return nil, fmt.Errorf("incorrect auth_type for SASL mechanism: %s", k.UserAuthType)
}

// getTLSConfig creates a TLS configuration using the provided certificate path for server authentication
func (k *kafka) getTLSConfig() (*tls.Config, error) {
	var caCert []byte
	if k.Cert == "" {
		if k.CertPath == "" {
			return nil, fmt.Errorf("cert or cert_path is required for TLS server authentication")
		}
		cert, err := os.ReadFile(k.CertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read cert file: %w", err)
		}
		caCert = cert
	} else {
		caCert = []byte(k.Cert)
	}

	caPool := x509.NewCertPool()
	ok := caPool.AppendCertsFromPEM(caCert)
	if !ok {
		return nil, fmt.Errorf("failed to append CA certificate")
	}
	return &tls.Config{
		RootCAs: caPool,
	}, nil
}
