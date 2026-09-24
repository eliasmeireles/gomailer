package client

import "context"

// Sender hands emails to gomailer. RabbitMQ and Kafka senders return once the broker
// acknowledged the message (delivery happens asynchronously); the HTTP sender returns once
// gomailer delivered it.
type Sender interface {
	Send(ctx context.Context, email Email) error
	// HealthCheck verifies the sender can reach gomailer or its broker without sending email.
	HealthCheck(ctx context.Context) error
	Close() error
}
