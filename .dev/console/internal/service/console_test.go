package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
	"github.com/eliasmeireles/gomailer/dev/console/internal/monitor"
	"github.com/eliasmeireles/gomailer/dev/console/internal/queue"
)

type fakePublisher struct {
	published []message.Email
	err       error
	stats     queue.Stats
	statsErr  error
}

func (f *fakePublisher) Publish(_ context.Context, email message.Email) error {
	f.published = append(f.published, email)
	return f.err
}

func (f *fakePublisher) Stats(context.Context) (queue.Stats, error) { return f.stats, f.statsErr }

type fakeInbox struct {
	messages []monitor.InboxMessage
	cleared  bool
}

func (f *fakeInbox) List(context.Context) ([]monitor.InboxMessage, error) { return f.messages, nil }
func (f *fakeInbox) Clear(context.Context) error                          { f.cleared = true; return nil }

type fakeCallbacks struct {
	events  []monitor.CallbackEvent
	cleared bool
}

func (f *fakeCallbacks) List(context.Context) ([]monitor.CallbackEvent, error) { return f.events, nil }
func (f *fakeCallbacks) Clear(context.Context) error                           { f.cleared = true; return nil }

type fakeHealth struct{ ready bool }

func (f fakeHealth) Ready(context.Context) bool { return f.ready }

func validForm() message.Form {
	return message.Form{From: "no-reply@exemplo.com.br", To: "maria@exemplo.com.br", Subject: "Oi", HTML: "<p>Oi</p>", Format: message.FormatArray}
}

func newTestConsole(publisher *fakePublisher, inbox *fakeInbox, callbacks *fakeCallbacks, ready bool) *Console {
	return NewConsole(publisher, inbox, callbacks, fakeHealth{ready: ready}, func() string { return "id-1" })
}

func TestConsoleSend(t *testing.T) {
	t.Run("given a valid form then publish and return the email", func(t *testing.T) {
		publisher := &fakePublisher{}

		email, err := newTestConsole(publisher, &fakeInbox{}, &fakeCallbacks{}, true).Send(context.Background(), validForm())

		require.NoError(t, err)
		assert.Equal(t, "id-1", email.ID)
		assert.Equal(t, []message.Email{email}, publisher.published)
	})

	t.Run("given an invalid form then return error without publishing", func(t *testing.T) {
		publisher := &fakePublisher{}
		form := validForm()
		form.From = ""

		_, err := newTestConsole(publisher, &fakeInbox{}, &fakeCallbacks{}, true).Send(context.Background(), form)

		require.Error(t, err)
		assert.Empty(t, publisher.published)
	})

	t.Run("given a publish error then return it", func(t *testing.T) {
		publisher := &fakePublisher{err: errors.New("broker down")}

		_, err := newTestConsole(publisher, &fakeInbox{}, &fakeCallbacks{}, true).Send(context.Background(), validForm())

		require.EqualError(t, err, "broker down")
	})
}

func TestConsoleStatus(t *testing.T) {
	t.Run("given a ready mailer and queue stats then report both", func(t *testing.T) {
		publisher := &fakePublisher{stats: queue.Stats{Messages: 2, Consumers: 1}}

		status := newTestConsole(publisher, &fakeInbox{}, &fakeCallbacks{}, true).Status(context.Background())

		assert.Equal(t, Status{MailerReady: true, Queue: queue.Stats{Messages: 2, Consumers: 1}}, status)
	})

	t.Run("given a queue error then report it", func(t *testing.T) {
		publisher := &fakePublisher{statsErr: errors.New("connect to RabbitMQ: refused")}

		status := newTestConsole(publisher, &fakeInbox{}, &fakeCallbacks{}, false).Status(context.Background())

		assert.Equal(t, Status{QueueError: "connect to RabbitMQ: refused"}, status)
	})
}

func TestConsolePanels(t *testing.T) {
	t.Run("must delegate inbox and callbacks listing and clearing", func(t *testing.T) {
		inbox := &fakeInbox{messages: []monitor.InboxMessage{{ID: "m1"}}}
		callbacks := &fakeCallbacks{events: []monitor.CallbackEvent{{Path: "/failures"}}}
		console := newTestConsole(&fakePublisher{}, inbox, callbacks, true)

		messages, err := console.Inbox(context.Background())
		require.NoError(t, err)
		events, err := console.Callbacks(context.Background())
		require.NoError(t, err)
		require.NoError(t, console.ClearInbox(context.Background()))
		require.NoError(t, console.ClearCallbacks(context.Background()))

		assert.Equal(t, inbox.messages, messages)
		assert.Equal(t, callbacks.events, events)
		assert.True(t, inbox.cleared)
		assert.True(t, callbacks.cleared)
	})
}
