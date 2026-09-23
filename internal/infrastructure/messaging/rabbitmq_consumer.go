package messaging

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"

	"github.com/eliasmeireles/gomailer/internal/core/mailer"
)

const (
	initialBackoff = 1 * time.Second
	maxBackoff     = 30 * time.Second
)

// Consumer is a RabbitMQ consumer with retry-on-connect, auto-reconnect on drop, delayed
// retries and a dead-letter queue (see topology).
type Consumer struct {
	config   RabbitMQConfig
	topology topology

	mu      sync.Mutex
	conn    *amqp.Connection
	channel *amqp.Channel
	ready   atomic.Bool
}

// NewConsumer creates a Consumer whose retry queues follow policy.
func NewConsumer(config RabbitMQConfig, policy mailer.RetryPolicy) *Consumer {
	return &Consumer{config: config, topology: newTopology(config.Queue, policy.Delays())}
}

// Ready reports whether the consumer has an open connection and channel.
// Safe to call from any goroutine; suitable for readiness probes.
func (c *Consumer) Ready() bool {
	return c.ready.Load()
}

// Connect retries indefinitely with exponential backoff until success or ctx is done.
// Backoff doubles each attempt up to maxBackoff. Returns ctx.Err() on cancellation.
func (c *Consumer) Connect(ctx context.Context) error {
	backoff := initialBackoff
	for {
		if err := c.dialAndOpen(); err != nil {
			log.Warnf("RabbitMQ connect failed: %v; retrying in %s", err, backoff)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}
		c.ready.Store(true)
		return nil
	}
}

func (c *Consumer) dialAndOpen() error {
	conn, err := amqp.Dial(c.config.URL)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	ch, err := c.openChannel(conn)
	if err != nil {
		_ = conn.Close()
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.channel = ch
	c.mu.Unlock()

	log.Infof("Connected to RabbitMQ, queue: %s, retry queues: %v, dead-letter: %s",
		c.topology.queue, c.topology.retryQueues(), c.topology.deadLetter)
	return nil
}

// openChannel opens a channel in confirm mode (retries and dead letters are republished with
// broker confirmation) and declares the topology.
func (c *Consumer) openChannel(conn *amqp.Connection) (*amqp.Channel, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open channel: %w", err)
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("enable publisher confirms: %w", err)
	}
	if err := c.topology.declare(ch); err != nil {
		_ = ch.Close()
		return nil, err
	}
	return ch, nil
}

// Consume runs the handler for each delivery, blocking until ctx is cancelled.
// On disconnect, marks the consumer as not-ready, reconnects with backoff, and resumes.
func (c *Consumer) Consume(ctx context.Context, handler Handler) error {
	for {
		err := c.consumeLoop(ctx, handler)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c.ready.Store(false)
		log.Warnf("RabbitMQ consumer dropped: %v; reconnecting", err)
		if reconnectErr := c.Connect(ctx); reconnectErr != nil {
			return reconnectErr
		}
	}
}

func (c *Consumer) consumeLoop(ctx context.Context, handler Handler) error {
	c.mu.Lock()
	ch := c.channel
	conn := c.conn
	c.mu.Unlock()

	if ch == nil || conn == nil {
		return errors.New("not connected")
	}

	closed := conn.NotifyClose(make(chan *amqp.Error, 1))

	msgs, err := ch.Consume(c.config.Queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("register consumer: %w", err)
	}
	log.Infof("Waiting for messages on queue: %s", c.config.Queue)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case amqpErr := <-closed:
			if amqpErr == nil {
				return errors.New("connection closed")
			}
			return fmt.Errorf("connection closed: %w", amqpErr)
		case msg, ok := <-msgs:
			if !ok {
				return errors.New("messages channel closed")
			}
			attempt := attemptOf(msg.Headers)
			c.apply(ch, msg, attempt, handler(msg.Body, attempt))
		}
	}
}

// Close closes the RabbitMQ channel and connection.
func (c *Consumer) Close() {
	c.ready.Store(false)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.channel != nil {
		if err := c.channel.Close(); err != nil {
			log.Errorf("Failed to close RabbitMQ channel: %v", err)
		}
	}
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			log.Errorf("Failed to close RabbitMQ connection: %v", err)
		}
	}
}
