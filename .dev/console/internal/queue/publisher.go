// Package queue publishes messages to the mailer queue over AMQP.
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
)

const dialTimeout = 5 * time.Second

// Publisher opens a short-lived connection per call, which is enough for a dev tool and
// survives broker restarts without reconnect logic.
type Publisher struct {
	url   string
	queue string
}

// NewPublisher creates a Publisher for queue at the AMQP url.
func NewPublisher(url, queue string) *Publisher {
	return &Publisher{url: url, queue: queue}
}

// Publish sends email as a persistent JSON message to the queue, using its ID as message id.
func (p *Publisher) Publish(ctx context.Context, email message.Email) error {
	body, err := json.Marshal(email)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}

	return p.withChannel(func(ch *amqp.Channel) error {
		if err := p.declare(ch); err != nil {
			return err
		}
		return ch.PublishWithContext(ctx, "", p.queue, false, false, amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    email.ID,
			Timestamp:    time.Now(),
			Body:         body,
		})
	})
}

// declare mirrors the mailer declaration (durable, non-exclusive) so both sides agree.
func (p *Publisher) declare(ch *amqp.Channel) error {
	if _, err := ch.QueueDeclare(p.queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue %s: %w", p.queue, err)
	}
	return nil
}

func (p *Publisher) withChannel(fn func(ch *amqp.Channel) error) error {
	conn, err := amqp.DialConfig(p.url, amqp.Config{Dial: amqp.DefaultDial(dialTimeout)})
	if err != nil {
		return fmt.Errorf("connect to RabbitMQ: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer ch.Close()

	return fn(ch)
}
