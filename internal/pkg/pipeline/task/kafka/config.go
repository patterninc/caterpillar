package kafka

import (
	"fmt"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/google/uuid"
)

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

// auto.offset.store is off so the watermark, not librdkafka, chooses the commit position.
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

	if k.ClientRack != "" {
		_ = cfg.SetKey("client.rack", k.ClientRack)
	}

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

	if k.ClientRack != "" {
		_ = cfg.SetKey("client.rack", k.ClientRack)
	}

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
