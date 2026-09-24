// Package transport builds a gomailer client.Sender from configuration, so an application can
// switch between RabbitMQ, Kafka and HTTP without code changes.
package transport

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/eliasmeireles/gomailer/client"
	"github.com/eliasmeireles/gomailer/client/kafka"
	"github.com/eliasmeireles/gomailer/client/rabbitmq"
)

// Name selects the transport.
type Name string

const (
	RabbitMQ Name = "rabbitmq"
	Kafka    Name = "kafka"
	HTTP     Name = "http"
)

// Config holds the settings of the selected transport; the others are ignored.
type Config struct {
	Transport Name
	RabbitMQ  rabbitmq.Config
	Kafka     kafka.Config
	HTTP      client.HTTPConfig
}

// New creates the sender for cfg.Transport (default rabbitmq).
func New(cfg Config) (client.Sender, error) {
	switch cfg.Transport {
	case RabbitMQ, "":
		return rabbitmq.New(cfg.RabbitMQ)
	case Kafka:
		return kafka.New(cfg.Kafka)
	case HTTP:
		return client.NewHTTPSender(cfg.HTTP)
	default:
		return nil, fmt.Errorf("unknown gomailer transport %q (rabbitmq, kafka or http)", cfg.Transport)
	}
}

// FromEnv reads the configuration from environment variables:
//
//	GOMAILER_TRANSPORT       rabbitmq (default), kafka or http
//	GOMAILER_RABBITMQ_URL    amqp://user:pass@host:5672/vhost
//	GOMAILER_RABBITMQ_QUEUE  default mailer-service
//	GOMAILER_KAFKA_BROKERS   comma-separated
//	GOMAILER_KAFKA_TOPIC     default mailer-service
//	GOMAILER_KAFKA_TLS       true/false
//	GOMAILER_KAFKA_SASL_MECHANISM, GOMAILER_KAFKA_SASL_USER, GOMAILER_KAFKA_SASL_PASS
//	GOMAILER_HTTP_URL        e.g. http://gomailer:8081
//	GOMAILER_HTTP_TOKEN      one of the gomailer HTTP_API_KEYS
//	GOMAILER_TIMEOUT         Go duration applied to every transport (optional)
func FromEnv() (Config, error) {
	timeout, err := durationEnv("GOMAILER_TIMEOUT")
	if err != nil {
		return Config{}, err
	}

	return Config{
		Transport: Name(strings.ToLower(env("GOMAILER_TRANSPORT"))),
		RabbitMQ:  rabbitmq.Config{URL: env("GOMAILER_RABBITMQ_URL"), Queue: env("GOMAILER_RABBITMQ_QUEUE"), Timeout: timeout},
		Kafka: kafka.Config{
			Brokers:       list(env("GOMAILER_KAFKA_BROKERS")),
			Topic:         env("GOMAILER_KAFKA_TOPIC"),
			Timeout:       timeout,
			TLS:           strings.EqualFold(env("GOMAILER_KAFKA_TLS"), "true"),
			SASLMechanism: strings.ToLower(env("GOMAILER_KAFKA_SASL_MECHANISM")),
			SASLUser:      env("GOMAILER_KAFKA_SASL_USER"),
			SASLPass:      env("GOMAILER_KAFKA_SASL_PASS"),
		},
		HTTP: client.HTTPConfig{URL: env("GOMAILER_HTTP_URL"), Token: env("GOMAILER_HTTP_TOKEN"), Timeout: timeout},
	}, nil
}

func env(key string) string {
	return strings.TrimSpace(os.Getenv(key))
}

func list(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func durationEnv(key string) (time.Duration, error) {
	value := env(key)
	if value == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration, got %q", key, value)
	}
	return duration, nil
}
