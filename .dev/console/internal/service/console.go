// Package service implements the dev console use cases on top of the queue and monitor clients.
package service

import (
	"context"

	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
	"github.com/eliasmeireles/gomailer/dev/console/internal/monitor"
	"github.com/eliasmeireles/gomailer/dev/console/internal/queue"
)

// Publisher sends messages to the mailer queue and inspects it.
type Publisher interface {
	Publish(ctx context.Context, email message.Email) error
	Stats(ctx context.Context) (queue.Stats, error)
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
	inbox     Inbox
	callbacks CallbackLog
	health    HealthChecker
	newID     func() string
}

// NewConsole wires the console dependencies; newID generates ids for messages without one.
func NewConsole(publisher Publisher, inbox Inbox, callbacks CallbackLog, health HealthChecker, newID func() string) *Console {
	return &Console{publisher: publisher, inbox: inbox, callbacks: callbacks, health: health, newID: newID}
}

// Send builds the message from form and publishes it, returning what was published.
func (c *Console) Send(ctx context.Context, form message.Form) (message.Email, error) {
	email, err := message.Build(form, c.newID)
	if err != nil {
		return message.Email{}, err
	}
	if err := c.publisher.Publish(ctx, email); err != nil {
		return message.Email{}, err
	}
	return email, nil
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
