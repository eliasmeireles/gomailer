package messaging

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"
)

const (
	envRabbitMQUser  = "RABBITMQ_USER"
	envRabbitMQPass  = "RABBITMQ_PASS"
	envRabbitMQHost  = "RABBITMQ_HOST"
	envRabbitMQPort  = "RABBITMQ_PORT"
	envRabbitMQVHost = "RABBITMQ_VHOST"
	envRabbitMQQueue = "RABBITMQ_QUEUE"

	defaultRabbitMQPort  = "5672"
	defaultRabbitMQVHost = "/"
	defaultRabbitMQQueue = "mailer-service"

	initialBackoff = 1 * time.Second
	maxBackoff     = 30 * time.Second
)

// RabbitMQConfig holds the RabbitMQ connection configuration.
type RabbitMQConfig struct {
	URL   string
	Queue string
}

// NewRabbitMQConfig creates a RabbitMQConfig from environment variables.
// Credentials (RABBITMQ_USER, RABBITMQ_PASS) are required.
func NewRabbitMQConfig() RabbitMQConfig {
	user := getRequiredEnv(envRabbitMQUser)
	pass := getRequiredEnv(envRabbitMQPass)
	host := getRequiredEnv(envRabbitMQHost)
	port := getEnvOrDefault(envRabbitMQPort, defaultRabbitMQPort)
	vhost := getEnvOrDefault(envRabbitMQVHost, defaultRabbitMQVHost)
	queue := getEnvOrDefault(envRabbitMQQueue, defaultRabbitMQQueue)

	if vhost != "" && vhost[0] != '/' {
		vhost = "/" + vhost
	}

	url := fmt.Sprintf("amqp://%s:%s@%s:%s%s", user, pass, host, port, vhost)

	return RabbitMQConfig{
		URL:   url,
		Queue: queue,
	}
}

func getRequiredEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Required environment variable %s is not set", key)
	}
	return value
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Consumer is a RabbitMQ consumer with retry-on-connect and auto-reconnect on drop.
type Consumer struct {
	config RabbitMQConfig

	mu      sync.Mutex
	conn    *amqp.Connection
	channel *amqp.Channel
	ready   atomic.Bool
}

// NewConsumer creates a new RabbitMQ Consumer.
func NewConsumer(config RabbitMQConfig) *Consumer {
	return &Consumer{config: config}
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
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
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

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("open channel: %w", err)
	}

	_, err = ch.QueueDeclare(c.config.Queue, true, false, false, false, nil)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("declare queue %s: %w", c.config.Queue, err)
	}

	c.mu.Lock()
	c.conn = conn
	c.channel = ch
	c.mu.Unlock()

	log.Infof("Connected to RabbitMQ, queue: %s", c.config.Queue)
	return nil
}

// Consume runs the handler for each delivery, blocking until ctx is cancelled.
// On disconnect, marks the consumer as not-ready, reconnects with backoff, and resumes.
func (c *Consumer) Consume(ctx context.Context, handler func(body []byte) error) error {
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

func (c *Consumer) consumeLoop(ctx context.Context, handler func(body []byte) error) error {
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
			if err := handler(msg.Body); err != nil {
				log.Errorf("Failed to process message: %v", err)
				if nackErr := msg.Nack(false, true); nackErr != nil {
					log.Errorf("Failed to nack message: %v", nackErr)
				}
				continue
			}
			if err := msg.Ack(false); err != nil {
				log.Errorf("Failed to ack message: %v", err)
			}
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
