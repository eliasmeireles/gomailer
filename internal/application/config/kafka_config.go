package config

import (
	"fmt"
	"strings"
)

const (
	envKafkaBrokers       = "KAFKA_BROKERS"
	envKafkaTopic         = "KAFKA_TOPIC"
	envKafkaGroupID       = "KAFKA_GROUP_ID"
	envKafkaCreateTopics  = "KAFKA_CREATE_TOPICS"
	envKafkaPartitions    = "KAFKA_TOPIC_PARTITIONS"
	envKafkaReplication   = "KAFKA_TOPIC_REPLICATION"
	envKafkaTLS           = "KAFKA_TLS"
	envKafkaSASLMechanism = "KAFKA_SASL_MECHANISM"
	envKafkaSASLUser      = "KAFKA_SASL_USER"
	envKafkaSASLPass      = "KAFKA_SASL_PASS"

	defaultKafkaTopic   = "mailer-service"
	defaultKafkaGroupID = "gomailer"
)

// SASL mechanisms supported by the Kafka source.
const (
	SASLPlain       = "plain"
	SASLScramSHA256 = "scram-sha-256"
	SASLScramSHA512 = "scram-sha-512"
)

// KafkaConfig configures the Kafka source.
type KafkaConfig struct {
	Brokers []string
	Topic   string
	GroupID string

	// CreateTopics creates the main, retry and dead-letter topics when missing (handy locally;
	// in production topics are usually managed by the platform).
	CreateTopics bool
	Partitions   int32
	Replication  int16

	TLS           bool
	SASLMechanism string
	SASLUser      string
	SASLPass      string
}

// NewKafkaConfig loads the Kafka source configuration. KAFKA_BROKERS (comma-separated) is
// required; KAFKA_TOPIC defaults to mailer-service and KAFKA_GROUP_ID to gomailer.
// KAFKA_SASL_MECHANISM (plain, scram-sha-256, scram-sha-512) requires KAFKA_SASL_USER and
// KAFKA_SASL_PASS.
func NewKafkaConfig() (KafkaConfig, error) {
	brokersValue, err := requireEnv(envKafkaBrokers)
	if err != nil {
		return KafkaConfig{}, fmt.Errorf("the kafka source requires brokers: %w", err)
	}

	partitions, err := positiveInt(envKafkaPartitions, 3)
	if err != nil {
		return KafkaConfig{}, err
	}
	replication, err := positiveInt(envKafkaReplication, 1)
	if err != nil {
		return KafkaConfig{}, err
	}

	cfg := KafkaConfig{
		Brokers:       splitList(brokersValue),
		Topic:         envOrDefault(envKafkaTopic, defaultKafkaTopic),
		GroupID:       envOrDefault(envKafkaGroupID, defaultKafkaGroupID),
		CreateTopics:  isTrue(envOrDefault(envKafkaCreateTopics, "false")),
		Partitions:    int32(partitions),
		Replication:   int16(replication),
		TLS:           isTrue(envOrDefault(envKafkaTLS, "false")),
		SASLMechanism: strings.ToLower(envOrDefault(envKafkaSASLMechanism, "")),
		SASLUser:      envOrDefault(envKafkaSASLUser, ""),
		SASLPass:      envOrDefault(envKafkaSASLPass, ""),
	}
	return cfg, validateSASL(cfg)
}

func validateSASL(cfg KafkaConfig) error {
	switch cfg.SASLMechanism {
	case "":
		return nil
	case SASLPlain, SASLScramSHA256, SASLScramSHA512:
		if cfg.SASLUser == "" || cfg.SASLPass == "" {
			return fmt.Errorf("%s=%s requires %s and %s", envKafkaSASLMechanism, cfg.SASLMechanism, envKafkaSASLUser, envKafkaSASLPass)
		}
		return nil
	default:
		return fmt.Errorf("%s must be %s, %s or %s, got %q", envKafkaSASLMechanism, SASLPlain, SASLScramSHA256, SASLScramSHA512, cfg.SASLMechanism)
	}
}

func splitList(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func isTrue(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
