package transport

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/client"
	"github.com/eliasmeireles/gomailer/client/kafka"
	"github.com/eliasmeireles/gomailer/client/rabbitmq"
)

func TestNew(t *testing.T) {
	t.Run("given no transport then default to rabbitmq", func(t *testing.T) {
		sender, err := New(Config{RabbitMQ: rabbitmq.Config{URL: "amqp://guest:guest@localhost:5672/"}})

		require.NoError(t, err)
		assert.IsType(t, &rabbitmq.Sender{}, sender)
	})

	t.Run("given kafka and http then build their senders", func(t *testing.T) {
		kafkaSender, err := New(Config{Transport: Kafka, Kafka: kafka.Config{Brokers: []string{"b:9092"}}})
		require.NoError(t, err)
		httpSender, err := New(Config{Transport: HTTP, HTTP: client.HTTPConfig{URL: "http://gomailer:8081", Token: "t"}})
		require.NoError(t, err)

		assert.IsType(t, &kafka.Sender{}, kafkaSender)
		assert.IsType(t, &client.HTTPSender{}, httpSender)
	})

	t.Run("given an unknown transport then return error", func(t *testing.T) {
		_, err := New(Config{Transport: "pigeon"})

		require.ErrorContains(t, err, `unknown gomailer transport "pigeon"`)
	})
}

func TestFromEnv(t *testing.T) {
	t.Run("must read every setting", func(t *testing.T) {
		values := map[string]string{
			"GOMAILER_TRANSPORT": "KAFKA", "GOMAILER_RABBITMQ_URL": "amqp://u:p@r:5672/v", "GOMAILER_RABBITMQ_QUEUE": "q",
			"GOMAILER_KAFKA_BROKERS": "k1:9092, k2:9092", "GOMAILER_KAFKA_TOPIC": "t", "GOMAILER_KAFKA_TLS": "true",
			"GOMAILER_KAFKA_SASL_MECHANISM": "PLAIN", "GOMAILER_KAFKA_SASL_USER": "u", "GOMAILER_KAFKA_SASL_PASS": "p",
			"GOMAILER_HTTP_URL": "http://g:8081", "GOMAILER_HTTP_TOKEN": "tok", "GOMAILER_TIMEOUT": "5s",
		}
		for key, value := range values {
			t.Setenv(key, value)
		}

		cfg, err := FromEnv()

		require.NoError(t, err)
		assert.Equal(t, Config{
			Transport: Kafka,
			RabbitMQ:  rabbitmq.Config{URL: "amqp://u:p@r:5672/v", Queue: "q", Timeout: 5 * time.Second},
			Kafka: kafka.Config{Brokers: []string{"k1:9092", "k2:9092"}, Topic: "t", Timeout: 5 * time.Second, TLS: true,
				SASLMechanism: "plain", SASLUser: "u", SASLPass: "p"},
			HTTP: client.HTTPConfig{URL: "http://g:8081", Token: "tok", Timeout: 5 * time.Second},
		}, cfg)
	})

	t.Run("given an invalid timeout then return error", func(t *testing.T) {
		t.Setenv("GOMAILER_TIMEOUT", "soon")

		_, err := FromEnv()

		require.Error(t, err)
	})
}
