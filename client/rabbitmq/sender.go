// Package rabbitmq sends emails to gomailer through its RabbitMQ queue.
package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/eliasmeireles/gomailer/client"
)

const (
	defaultQueue   = "mailer-service"
	defaultTimeout = 10 * time.Second
)

// Config configures the RabbitMQ sender.
type Config struct {
	// URL is the AMQP URL including credentials and vhost (e.g. amqp://user:pass@host:5672/notification).
	URL string
	// Queue is the gomailer queue (default mailer-service).
	Queue string
	// Timeout bounds connecting and waiting for the broker confirmation (default 10s).
	Timeout time.Duration
}

// Sender publishes emails as persistent messages and waits for the broker confirmation. The
// connection is opened lazily and reopened on the next Send after a drop. Safe for concurrent use.
type Sender struct {
	cfg Config

	mu      sync.Mutex
	conn    *amqp.Connection
	channel *amqp.Channel
}

// New creates a RabbitMQ sender; the connection is opened on the first Send or HealthCheck.
func New(cfg Config) (*Sender, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, errors.New("gomailer rabbitmq sender requires URL")
	}
	if cfg.Queue == "" {
		cfg.Queue = defaultQueue
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	return &Sender{cfg: cfg}, nil
}

// Send validates the email, fills a missing ID and publishes it, returning once the broker
// confirmed the message.
func (s *Sender) Send(ctx context.Context, email client.Email) error {
	email, err := client.Prepare(email)
	if err != nil {
		return err
	}
	body, err := json.Marshal(email)
	if err != nil {
		return fmt.Errorf("encode email: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ch, err := s.ensureChannel()
	if err != nil {
		return err
	}
	if err := s.publish(ctx, ch, email.ID, body); err != nil {
		s.reset()
		return err
	}
	return nil
}

// HealthCheck connects if needed and checks that the queue exists, without publishing.
func (s *Sender) HealthCheck(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch, err := s.ensureChannel()
	if err != nil {
		return err
	}
	if _, err := ch.QueueDeclarePassive(s.cfg.Queue, true, false, false, false, nil); err != nil {
		s.reset()
		return fmt.Errorf("gomailer queue %s is not available: %w", s.cfg.Queue, err)
	}
	return nil
}

// Close closes the channel and connection.
func (s *Sender) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reset()
}

func (s *Sender) publish(ctx context.Context, ch *amqp.Channel, id string, body []byte) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	confirmation, err := ch.PublishWithDeferredConfirmWithContext(ctx, "", s.cfg.Queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		MessageId:    id,
		Timestamp:    time.Now(),
		Body:         body,
	})
	if err != nil {
		return fmt.Errorf("publish email %s: %w", id, err)
	}

	acked, err := confirmation.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("confirm email %s: %w", id, err)
	}
	if !acked {
		return fmt.Errorf("email %s was rejected by the broker", id)
	}
	return nil
}

// ensureChannel returns an open channel in confirm mode, (re)connecting when needed. The queue
// is declared exactly like gomailer does (durable, no arguments).
func (s *Sender) ensureChannel() (*amqp.Channel, error) {
	if s.channel != nil && !s.channel.IsClosed() && s.conn != nil && !s.conn.IsClosed() {
		return s.channel, nil
	}
	s.reset()

	conn, err := amqp.DialConfig(s.cfg.URL, amqp.Config{Dial: amqp.DefaultDial(s.cfg.Timeout)})
	if err != nil {
		return nil, fmt.Errorf("connect to rabbitmq: %w", err)
	}
	ch, err := conn.Channel()
	if err == nil {
		err = ch.Confirm(false)
	}
	if err == nil {
		_, err = ch.QueueDeclare(s.cfg.Queue, true, false, false, false, nil)
	}
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("prepare rabbitmq channel: %w", err)
	}

	s.conn, s.channel = conn, ch
	return ch, nil
}

func (s *Sender) reset() error {
	var errs []error
	if s.channel != nil && !s.channel.IsClosed() {
		errs = append(errs, s.channel.Close())
	}
	if s.conn != nil && !s.conn.IsClosed() {
		errs = append(errs, s.conn.Close())
	}
	s.conn, s.channel = nil, nil
	return errors.Join(errs...)
}
