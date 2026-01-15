package kafka

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"os"
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
	defaultTimeout    = duration.Duration(15 * time.Second)
	defaultBatchSize  = 100
	defaultRetryLimit = 5
)

type kafka struct {
	task.Base       `yaml:",inline" json:",inline"`
	BootstrapServer string            `yaml:"bootstrap_server" json:"bootstrap_server"`                     // "host:port"
	Topic           string            `yaml:"topic" json:"topic"`                                           // topic to read from or write to
	ServerAuthType  string            `yaml:"server_auth_type,omitempty" json:"server_auth_type,omitempty"` // "none", "tls"
	Cert            string            `yaml:"cert,omitempty" json:"cert,omitempty"`                         // used for Server TLS authentication
	CertPath        string            `yaml:"cert_path,omitempty" json:"cert_path,omitempty"`               // used for Server TLS authentication
	UserAuthType    string            `yaml:"user_auth_type" json:"user_auth_type"`                         // "none", "sasl", "scram", "mtls"
	UserCert        string            `yaml:"user_cert,omitempty" json:"user_cert,omitempty"`               // used for user mTLS authentication
	UserCertPath    string            `yaml:"user_cert_path,omitempty" json:"user_cert_path,omitempty"`     // used for user mTLS authentication
	Username        string            `yaml:"username,omitempty" json:"username,omitempty"`                 // used for user SASL/Scram authentication
	Password        string            `yaml:"password,omitempty" json:"password,omitempty"`                 // used for user SASL/Scram authentication
	Timeout         duration.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`                   // connection, read, write, commit timeout
	GroupID         string            `yaml:"group_id,omitempty" json:"group_id,omitempty"`                 // the consumer group id (optional)
	BatchSize       int               `yaml:"batch_size,omitempty" json:"batch_size,omitempty"`             // number of messages to read/write in a batch
	ctx             context.Context   // parent context
	timeout         time.Duration     // timeout duration calculated from Timeout
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
	k.timeout = time.Duration(k.Timeout)

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
			var we kg.WriteErrors
			if errors.As(err, &we) {
				errString := fmt.Sprintf("failed to write message to kafka with error count = %d.\nThe errors are:\n", we.Count())
				for i, individualErr := range we {
					errString += fmt.Sprintf("%d   : %v\n", i, individualErr)
				}
				return fmt.Errorf("%s", errString)
			}
			return fmt.Errorf("failed to write message to kafka: %w", err)
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

	deadlineExceededRetries := defaultRetryLimit
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
				if errors.Is(err, io.EOF) {
					// this is not reliable for kafka end of topic detection
					fmt.Printf("kafka reached end of topic: %v\n", k.Topic)
					return nil
				}
				if errors.Is(err, context.Canceled) {
					fmt.Printf("kafka reader context canceled: %v\n", err)
					return nil
				}
				if errors.Is(err, context.DeadlineExceeded) {
					fmt.Printf("kafka deadline exceeded while reading message: %v\n", err)
					deadlineExceededRetries--
					if deadlineExceededRetries <= 0 {
						fmt.Printf("kafka exceeded maximum deadline exceeded retries (%d), stopping reader\n", defaultRetryLimit)
						return nil
					}
					continue
				}
				fmt.Printf("kafka error reading message: %v\n", err)
				continue
			}
			deadlineExceededRetries = defaultRetryLimit
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
	if k.GroupID != "" {
		return kg.NewReader(kg.ReaderConfig{
			Brokers:       []string{k.BootstrapServer},
			Topic:         k.Topic,
			Dialer:        dialer,
			GroupID:       k.GroupID,
			QueueCapacity: k.BatchSize,
		})
	}
	fmt.Printf("No group_id specified, will consume as standalone reader.\n")
	return kg.NewReader(kg.ReaderConfig{
		Brokers:       []string{k.BootstrapServer},
		Topic:         k.Topic,
		Dialer:        dialer,
		QueueCapacity: k.BatchSize,
	})
}

// getWriter creates a kafka writer for the specified dialer
func (k *kafka) getWriter(dialer *kg.Dialer) *kg.Writer {
	return kg.NewWriter(kg.WriterConfig{
		Brokers:      []string{k.BootstrapServer},
		Topic:        k.Topic,
		Balancer:     &kg.LeastBytes{}, //TODO: look into other balancers
		Dialer:       dialer,
		BatchSize:    k.BatchSize, // number of messages to batch before sending
		BatchTimeout: k.timeout,   // wait up to timeout before sending incomplete batch
	})
}

// createDialer creates a kafka dialer with optional SASL mechanism and TLS configuration
func (k *kafka) createDialer(mechanism sasl.Mechanism) (*kg.Dialer, error) {
	dialer := &kg.Dialer{
		Timeout:   k.timeout,
		DualStack: true, // use both IPv4 and IPv6 incase either is available
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
