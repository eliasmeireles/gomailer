package messaging

import (
	"context"

	log "github.com/sirupsen/logrus"
)

// Source adapts a Consumer to the runner Source contract.
type Source struct {
	consumer *Consumer
	handler  func(body []byte) error
}

// NewSource creates the RabbitMQ source that feeds every delivery to handler.
func NewSource(consumer *Consumer, handler func(body []byte) error) *Source {
	return &Source{consumer: consumer, handler: handler}
}

// Name identifies the source in logs.
func (s *Source) Name() string { return "rabbitmq" }

// Ready reports whether the consumer has an open connection.
func (s *Source) Ready() bool { return s.consumer.Ready() }

// Run connects (retrying with backoff) and consumes until ctx is cancelled.
func (s *Source) Run(ctx context.Context) error {
	if err := s.consumer.Connect(ctx); err != nil {
		return err
	}
	defer s.consumer.Close()

	log.Info("RabbitMQ source started, waiting for messages...")
	return s.consumer.Consume(ctx, s.handler)
}
