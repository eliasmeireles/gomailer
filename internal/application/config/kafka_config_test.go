package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setKafkaEnv(t *testing.T, values map[string]string) {
	t.Helper()
	for _, key := range []string{envKafkaBrokers, envKafkaTopic, envKafkaGroupID, envKafkaCreateTopics, envKafkaPartitions,
		envKafkaReplication, envKafkaTLS, envKafkaSASLMechanism, envKafkaSASLUser, envKafkaSASLPass} {
		t.Setenv(key, values[key])
	}
}

func TestNewKafkaConfig(t *testing.T) {
	t.Run("given only brokers then use the defaults", func(t *testing.T) {
		setKafkaEnv(t, map[string]string{envKafkaBrokers: " kafka-1:9092 , kafka-2:9092 ,"})

		cfg, err := NewKafkaConfig()

		require.NoError(t, err)
		assert.Equal(t, KafkaConfig{
			Brokers: []string{"kafka-1:9092", "kafka-2:9092"}, Topic: "mailer-service", GroupID: "gomailer",
			Partitions: 3, Replication: 1,
		}, cfg)
	})

	t.Run("given overrides and scram auth then use them", func(t *testing.T) {
		setKafkaEnv(t, map[string]string{
			envKafkaBrokers: "kafka:9093", envKafkaTopic: "emails", envKafkaGroupID: "mailer", envKafkaCreateTopics: "true",
			envKafkaPartitions: "6", envKafkaReplication: "3", envKafkaTLS: "yes",
			envKafkaSASLMechanism: "SCRAM-SHA-512", envKafkaSASLUser: "user", envKafkaSASLPass: "pass",
		})

		cfg, err := NewKafkaConfig()

		require.NoError(t, err)
		assert.Equal(t, KafkaConfig{
			Brokers: []string{"kafka:9093"}, Topic: "emails", GroupID: "mailer", CreateTopics: true,
			Partitions: 6, Replication: 3, TLS: true, SASLMechanism: SASLScramSHA512, SASLUser: "user", SASLPass: "pass",
		}, cfg)
	})

	t.Run("given no brokers then return error", func(t *testing.T) {
		setKafkaEnv(t, map[string]string{})

		_, err := NewKafkaConfig()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires brokers")
	})

	t.Run("given sasl without credentials then return error", func(t *testing.T) {
		setKafkaEnv(t, map[string]string{envKafkaBrokers: "kafka:9092", envKafkaSASLMechanism: "plain"})

		_, err := NewKafkaConfig()

		require.EqualError(t, err, "KAFKA_SASL_MECHANISM=plain requires KAFKA_SASL_USER and KAFKA_SASL_PASS")
	})

	t.Run("given an unknown sasl mechanism then return error", func(t *testing.T) {
		setKafkaEnv(t, map[string]string{envKafkaBrokers: "kafka:9092", envKafkaSASLMechanism: "oauth"})

		_, err := NewKafkaConfig()

		require.Error(t, err)
		assert.Contains(t, err.Error(), `got "oauth"`)
	})

	t.Run("given an invalid partitions value then return error", func(t *testing.T) {
		setKafkaEnv(t, map[string]string{envKafkaBrokers: "kafka:9092", envKafkaPartitions: "zero"})

		_, err := NewKafkaConfig()

		require.Error(t, err)
	})
}
