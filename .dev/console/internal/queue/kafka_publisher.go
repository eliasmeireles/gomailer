package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
)

// KafkaPublisher produces messages to the mailer Kafka topic, keyed by the email id.
type KafkaPublisher struct {
	client *kgo.Client
	topic  string
}

// NewKafkaPublisher creates a producer for topic on brokers. The connection is lazy, so the
// console starts even when Kafka is not running.
func NewKafkaPublisher(brokers []string, topic string) (*KafkaPublisher, error) {
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}
	return &KafkaPublisher{client: client, topic: topic}, nil
}

// Publish produces email as JSON and waits for the broker acknowledgement.
func (p *KafkaPublisher) Publish(ctx context.Context, email message.Email) error {
	body, err := json.Marshal(email)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, publishTimeout)
	defer cancel()
	record := &kgo.Record{Topic: p.topic, Key: []byte(email.ID), Value: body}
	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("produce to kafka: %w", err)
	}
	return nil
}
