// Package kafka sends emails to gomailer through its Kafka topic.
package kafka

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"github.com/eliasmeireles/gomailer/client"
)

const (
	defaultTopic   = "mailer-service"
	defaultTimeout = 10 * time.Second
)

// SASL mechanisms.
const (
	SASLPlain       = "plain"
	SASLScramSHA256 = "scram-sha-256"
	SASLScramSHA512 = "scram-sha-512"
)

// Config configures the Kafka sender.
type Config struct {
	Brokers []string
	// Topic is the gomailer topic (default mailer-service).
	Topic string
	// Timeout bounds each produce (default 10s).
	Timeout       time.Duration
	TLS           bool
	SASLMechanism string
	SASLUser      string
	SASLPass      string
}

// Sender produces emails keyed by their ID and waits for the broker acknowledgement.
type Sender struct {
	client  *kgo.Client
	topic   string
	timeout time.Duration
}

// New creates a Kafka sender; connections are opened lazily.
func New(cfg Config) (*Sender, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("gomailer kafka sender requires brokers")
	}
	opts, err := options(cfg)
	if err != nil {
		return nil, err
	}
	kafkaClient, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}

	topic, timeout := cfg.Topic, cfg.Timeout
	if topic == "" {
		topic = defaultTopic
	}
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Sender{client: kafkaClient, topic: topic, timeout: timeout}, nil
}

// Send validates the email, fills a missing ID and produces it.
func (s *Sender) Send(ctx context.Context, email client.Email) error {
	email, err := client.Prepare(email)
	if err != nil {
		return err
	}
	body, err := json.Marshal(email)
	if err != nil {
		return fmt.Errorf("encode email: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	record := &kgo.Record{Topic: s.topic, Key: []byte(email.ID), Value: body}
	if err := s.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("produce email %s: %w", email.ID, err)
	}
	return nil
}

// HealthCheck pings the brokers.
func (s *Sender) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	if err := s.client.Ping(ctx); err != nil {
		return fmt.Errorf("kafka is not reachable: %w", err)
	}
	return nil
}

// Close flushes and closes the client.
func (s *Sender) Close() error {
	s.client.Close()
	return nil
}

func options(cfg Config) ([]kgo.Opt, error) {
	opts := []kgo.Opt{kgo.SeedBrokers(cfg.Brokers...)}
	if cfg.TLS {
		opts = append(opts, kgo.DialTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	}

	switch cfg.SASLMechanism {
	case "":
	case SASLPlain:
		opts = append(opts, kgo.SASL(plain.Auth{User: cfg.SASLUser, Pass: cfg.SASLPass}.AsMechanism()))
	case SASLScramSHA256:
		opts = append(opts, kgo.SASL(scram.Auth{User: cfg.SASLUser, Pass: cfg.SASLPass}.AsSha256Mechanism()))
	case SASLScramSHA512:
		opts = append(opts, kgo.SASL(scram.Auth{User: cfg.SASLUser, Pass: cfg.SASLPass}.AsSha512Mechanism()))
	default:
		return nil, fmt.Errorf("unsupported kafka SASL mechanism %q", cfg.SASLMechanism)
	}
	return opts, nil
}
