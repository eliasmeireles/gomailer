package monitor

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const deadLetterPeekLimit = 10

// QueueInfo is a RabbitMQ queue summary.
type QueueInfo struct {
	Name      string `json:"name"`
	Messages  int    `json:"messages"`
	Consumers int    `json:"consumers"`
}

// QueueStats groups the mailer queue family: main queue, retry queues and dead-letter queue.
type QueueStats struct {
	Messages     int
	Consumers    int
	Retrying     int
	DeadLettered int
}

// DeadLetter is a message in the dead-letter queue with the failure recorded by the mailer.
type DeadLetter struct {
	MessageID string
	Attempt   int
	ErrorCode string
	Cause     string
	FailedAt  time.Time
	Payload   string
}

// RabbitMQ reads the mailer queues through the RabbitMQ management API.
type RabbitMQ struct {
	baseURL string
	user    string
	pass    string
	queue   string
	client  *http.Client
}

// NewRabbitMQ creates a management API client for the queue family of queue.
func NewRabbitMQ(baseURL, user, pass, queue string) *RabbitMQ {
	return &RabbitMQ{baseURL: baseURL, user: user, pass: pass, queue: queue, client: newHTTPClient()}
}

// Stats aggregates the main, retry (<queue>.retry.*) and dead-letter (<queue>.dlq) queues.
func (r *RabbitMQ) Stats(ctx context.Context) (QueueStats, error) {
	var queues []QueueInfo
	err := call(ctx, r.client, request{
		Method: http.MethodGet, URL: r.baseURL + "/api/queues/%2F?columns=name,messages,consumers",
		Out: &queues, User: r.user, Pass: r.pass,
	})
	if err != nil {
		return QueueStats{}, err
	}

	var stats QueueStats
	for _, queue := range queues {
		switch {
		case queue.Name == r.queue:
			stats.Messages, stats.Consumers = queue.Messages, queue.Consumers
		case queue.Name == r.deadLetterQueue():
			stats.DeadLettered = queue.Messages
		case strings.HasPrefix(queue.Name, r.queue+".retry."):
			stats.Retrying += queue.Messages
		}
	}
	return stats, nil
}

// PeekDeadLetters returns up to 10 dead-lettered messages without removing them.
func (r *RabbitMQ) PeekDeadLetters(ctx context.Context) ([]DeadLetter, error) {
	var messages []struct {
		Payload    string `json:"payload"`
		Properties struct {
			MessageID string         `json:"message_id"`
			Headers   map[string]any `json:"headers"`
		} `json:"properties"`
	}
	err := call(ctx, r.client, request{
		Method: http.MethodPost, URL: r.queueURL() + "/get",
		Body: map[string]any{"count": deadLetterPeekLimit, "ackmode": "ack_requeue_true", "encoding": "auto"},
		Out:  &messages, User: r.user, Pass: r.pass,
	})
	if err != nil {
		return nil, err
	}

	letters := make([]DeadLetter, 0, len(messages))
	for _, message := range messages {
		headers := message.Properties.Headers
		failedAt, _ := time.Parse(time.RFC3339, stringHeader(headers, "x-failed-at"))
		letters = append(letters, DeadLetter{
			MessageID: message.Properties.MessageID,
			Attempt:   intHeader(headers, "x-attempt"),
			ErrorCode: stringHeader(headers, "x-error-code"),
			Cause:     stringHeader(headers, "x-error-cause"),
			FailedAt:  failedAt,
			Payload:   message.Payload,
		})
	}
	return letters, nil
}

// PurgeDeadLetters removes every message from the dead-letter queue.
func (r *RabbitMQ) PurgeDeadLetters(ctx context.Context) error {
	return call(ctx, r.client, request{Method: http.MethodDelete, URL: r.queueURL() + "/contents", User: r.user, Pass: r.pass})
}

func (r *RabbitMQ) deadLetterQueue() string {
	return r.queue + ".dlq"
}

func (r *RabbitMQ) queueURL() string {
	return r.baseURL + "/api/queues/%2F/" + url.PathEscape(r.deadLetterQueue())
}

func stringHeader(headers map[string]any, key string) string {
	value, _ := headers[key].(string)
	return value
}

func intHeader(headers map[string]any, key string) int {
	value, _ := headers[key].(float64)
	return int(value)
}
