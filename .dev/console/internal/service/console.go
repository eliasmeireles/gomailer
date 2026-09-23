// Package service implements the dev console use cases on top of the queue and monitor clients.
package service

import (
	"context"

	"github.com/eliasmeireles/gomailer/dev/console/internal/api"
	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
	"github.com/eliasmeireles/gomailer/dev/console/internal/monitor"
	"github.com/eliasmeireles/gomailer/dev/console/internal/queue"
)

// Publisher sends messages to the mailer queue and inspects it.
type Publisher interface {
	Publish(ctx context.Context, email message.Email) error
	Stats(ctx context.Context) (queue.Stats, error)
}

// APIClient calls the mailer HTTP source.
type APIClient interface {
	Send(ctx context.Context, email message.Email) (api.Response, error)
}

// SendResult is what the console did with the email. Response is set for the HTTP channel.
type SendResult struct {
	Email    message.Email
	Channel  message.Channel
	Response *api.Response
}

// Inbox reads the Mailpit inbox.
type Inbox interface {
	List(ctx context.Context) ([]monitor.InboxMessage, error)
	Clear(ctx context.Context) error
}

// CallbackLog reads the callbacks received by the local callback server.
type CallbackLog interface {
	List(ctx context.Context) ([]monitor.CallbackEvent, error)
	Clear(ctx context.Context) error
}

// HealthChecker reports mailer readiness.
type HealthChecker interface {
	Ready(ctx context.Context) bool
}

// Status is the stack overview shown in the console header.
type Status struct {
	MailerReady bool
	Queue       queue.Stats
	QueueError  string
}

// Console implements the console use cases.
type Console struct {
	publisher Publisher
	api       APIClient
	inbox     Inbox
	callbacks CallbackLog
	health    HealthChecker
	newID     func() string
}

// NewConsole wires the console dependencies; newID generates ids for messages without one.
func NewConsole(publisher Publisher, apiClient APIClient, inbox Inbox, callbacks CallbackLog, health HealthChecker, newID func() string) *Console {
	return &Console{publisher: publisher, api: apiClient, inbox: inbox, callbacks: callbacks, health: health, newID: newID}
}

// Send builds the message from form and hands it to the mailer through form.Channel
// (RabbitMQ by default).
func (c *Console) Send(ctx context.Context, form message.Form) (SendResult, error) {
	email, err := message.Build(form, c.newID)
	if err != nil {
		return SendResult{}, err
	}

	result := SendResult{Email: email, Channel: form.Channel}
	switch form.Channel {
	case message.ChannelHTTP:
		response, err := c.api.Send(ctx, email)
		if err != nil {
			return SendResult{}, err
		}
		result.Response = &response
	default:
		result.Channel = message.ChannelRabbitMQ
		if err := c.publisher.Publish(ctx, email); err != nil {
			return SendResult{}, err
		}
	}
	return result, nil
}

// Status returns mailer readiness and queue stats; a queue error is reported, not returned.
func (c *Console) Status(ctx context.Context) Status {
	status := Status{MailerReady: c.health.Ready(ctx)}
	stats, err := c.publisher.Stats(ctx)
	if err != nil {
		status.QueueError = err.Error()
		return status
	}
	status.Queue = stats
	return status
}

// Inbox returns the latest Mailpit messages.
func (c *Console) Inbox(ctx context.Context) ([]monitor.InboxMessage, error) {
	return c.inbox.List(ctx)
}

// ClearInbox deletes every Mailpit message.
func (c *Console) ClearInbox(ctx context.Context) error {
	return c.inbox.Clear(ctx)
}

// Callbacks returns the received failure callbacks.
func (c *Console) Callbacks(ctx context.Context) ([]monitor.CallbackEvent, error) {
	return c.callbacks.List(ctx)
}

// ClearCallbacks removes the received failure callbacks.
func (c *Console) ClearCallbacks(ctx context.Context) error {
	return c.callbacks.Clear(ctx)
}
