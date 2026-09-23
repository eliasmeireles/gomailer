// Package service implements the dev console use cases on top of the queue, API and monitor
// clients.
package service

import (
	"context"

	"github.com/eliasmeireles/gomailer/dev/console/internal/api"
	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
	"github.com/eliasmeireles/gomailer/dev/console/internal/monitor"
)

// Publisher sends messages to the mailer queue.
type Publisher interface {
	Publish(ctx context.Context, email message.Email) error
}

// APIClient calls the mailer HTTP source.
type APIClient interface {
	Send(ctx context.Context, email message.Email) (api.Response, error)
}

// QueueMonitor inspects the mailer queue family (main, retry and dead-letter queues).
type QueueMonitor interface {
	Stats(ctx context.Context) (monitor.QueueStats, error)
	PeekDeadLetters(ctx context.Context) ([]monitor.DeadLetter, error)
	PurgeDeadLetters(ctx context.Context) error
}

// Inbox reads the Mailpit inbox.
type Inbox interface {
	List(ctx context.Context) ([]monitor.InboxMessage, error)
	Clear(ctx context.Context) error
}

// SMTPChaos drives the Mailpit chaos triggers.
type SMTPChaos interface {
	Chaos(ctx context.Context) (monitor.ChaosTriggers, error)
	SetChaos(ctx context.Context, triggers monitor.ChaosTriggers) (monitor.ChaosTriggers, error)
}

// MockAPI drives the mock email API.
type MockAPI interface {
	State(ctx context.Context) (monitor.MockState, error)
	Configure(ctx context.Context, behavior monitor.MockBehavior) (monitor.MockState, error)
	Reset(ctx context.Context) error
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

// Dependencies are the clients the console uses; NewID generates ids for messages without one.
type Dependencies struct {
	Publisher Publisher
	API       APIClient
	Queues    QueueMonitor
	Inbox     Inbox
	SMTPChaos SMTPChaos
	MockAPI   MockAPI
	Callbacks CallbackLog
	Health    HealthChecker
	NewID     func() string
}

// Status is the stack overview shown in the console header.
type Status struct {
	MailerReady bool
	Queue       monitor.QueueStats
	QueueError  string
}

// SendResult is what the console did with the email. Response is set for the HTTP channel.
type SendResult struct {
	Email    message.Email
	Channel  message.Channel
	Response *api.Response
}

// Console implements the console use cases.
type Console struct {
	deps Dependencies
}

// NewConsole creates the console service.
func NewConsole(deps Dependencies) *Console {
	return &Console{deps: deps}
}

// Send builds the message from form and hands it to the mailer through form.Channel
// (RabbitMQ by default).
func (c *Console) Send(ctx context.Context, form message.Form) (SendResult, error) {
	email, err := message.Build(form, c.deps.NewID)
	if err != nil {
		return SendResult{}, err
	}

	result := SendResult{Email: email, Channel: form.Channel}
	switch form.Channel {
	case message.ChannelHTTP:
		response, err := c.deps.API.Send(ctx, email)
		if err != nil {
			return SendResult{}, err
		}
		result.Response = &response
	default:
		result.Channel = message.ChannelRabbitMQ
		if err := c.deps.Publisher.Publish(ctx, email); err != nil {
			return SendResult{}, err
		}
	}
	return result, nil
}

// Status returns mailer readiness and queue stats; a queue error is reported, not returned.
func (c *Console) Status(ctx context.Context) Status {
	status := Status{MailerReady: c.deps.Health.Ready(ctx)}
	stats, err := c.deps.Queues.Stats(ctx)
	if err != nil {
		status.QueueError = err.Error()
		return status
	}
	status.Queue = stats
	return status
}

// Inbox returns the latest Mailpit messages.
func (c *Console) Inbox(ctx context.Context) ([]monitor.InboxMessage, error) {
	return c.deps.Inbox.List(ctx)
}

// ClearInbox deletes every Mailpit message.
func (c *Console) ClearInbox(ctx context.Context) error {
	return c.deps.Inbox.Clear(ctx)
}

// Callbacks returns the received delivery callbacks.
func (c *Console) Callbacks(ctx context.Context) ([]monitor.CallbackEvent, error) {
	return c.deps.Callbacks.List(ctx)
}

// ClearCallbacks removes the received delivery callbacks.
func (c *Console) ClearCallbacks(ctx context.Context) error {
	return c.deps.Callbacks.Clear(ctx)
}

// DeadLetters returns the first dead-lettered messages.
func (c *Console) DeadLetters(ctx context.Context) ([]monitor.DeadLetter, error) {
	return c.deps.Queues.PeekDeadLetters(ctx)
}

// PurgeDeadLetters empties the dead-letter queue.
func (c *Console) PurgeDeadLetters(ctx context.Context) error {
	return c.deps.Queues.PurgeDeadLetters(ctx)
}
