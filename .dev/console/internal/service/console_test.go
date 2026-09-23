package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/dev/console/internal/api"
	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
	"github.com/eliasmeireles/gomailer/dev/console/internal/monitor"
)

type fakePublisher struct {
	published []message.Email
	err       error
}

func (f *fakePublisher) Publish(_ context.Context, email message.Email) error {
	f.published = append(f.published, email)
	return f.err
}

type fakeAPI struct {
	sent     []message.Email
	response api.Response
	err      error
}

func (f *fakeAPI) Send(_ context.Context, email message.Email) (api.Response, error) {
	f.sent = append(f.sent, email)
	return f.response, f.err
}

type fakeQueues struct {
	stats    monitor.QueueStats
	statsErr error
	letters  []monitor.DeadLetter
	peekErr  error
	purged   bool
	purgeErr error
}

func (f *fakeQueues) Stats(context.Context) (monitor.QueueStats, error) { return f.stats, f.statsErr }
func (f *fakeQueues) PeekDeadLetters(context.Context) ([]monitor.DeadLetter, error) {
	return f.letters, f.peekErr
}
func (f *fakeQueues) PurgeDeadLetters(context.Context) error { f.purged = true; return f.purgeErr }

type fakeInbox struct {
	messages []monitor.InboxMessage
	cleared  bool
}

func (f *fakeInbox) List(context.Context) ([]monitor.InboxMessage, error) { return f.messages, nil }
func (f *fakeInbox) Clear(context.Context) error                          { f.cleared = true; return nil }

type fakeSMTPChaos struct {
	triggers monitor.ChaosTriggers
	err      error
	setErr   error
}

func (f *fakeSMTPChaos) Chaos(context.Context) (monitor.ChaosTriggers, error) {
	return f.triggers, f.err
}
func (f *fakeSMTPChaos) SetChaos(_ context.Context, triggers monitor.ChaosTriggers) (monitor.ChaosTriggers, error) {
	f.triggers = triggers
	return triggers, f.setErr
}

type fakeMockAPI struct {
	state    monitor.MockState
	err      error
	resetErr error
	reset    bool
}

func (f *fakeMockAPI) State(context.Context) (monitor.MockState, error) { return f.state, f.err }
func (f *fakeMockAPI) Configure(_ context.Context, behavior monitor.MockBehavior) (monitor.MockState, error) {
	f.state.MockBehavior = behavior
	return f.state, f.err
}
func (f *fakeMockAPI) Reset(context.Context) error { f.reset = true; return f.resetErr }

type fakeCallbacks struct {
	events  []monitor.CallbackEvent
	cleared bool
}

func (f *fakeCallbacks) List(context.Context) ([]monitor.CallbackEvent, error) { return f.events, nil }
func (f *fakeCallbacks) Clear(context.Context) error                           { f.cleared = true; return nil }

type fakeHealth struct{ ready bool }

func (f fakeHealth) Ready(context.Context) bool { return f.ready }

type fakes struct {
	publisher *fakePublisher
	kafka     *fakePublisher
	kafkaQs   *fakeQueues
	api       *fakeAPI
	queues    *fakeQueues
	inbox     *fakeInbox
	chaos     *fakeSMTPChaos
	mock      *fakeMockAPI
	callbacks *fakeCallbacks
}

func newFakes() *fakes {
	return &fakes{&fakePublisher{}, &fakePublisher{}, &fakeQueues{}, &fakeAPI{}, &fakeQueues{}, &fakeInbox{}, &fakeSMTPChaos{}, &fakeMockAPI{}, &fakeCallbacks{}}
}

func (f *fakes) console(ready bool) *Console {
	return NewConsole(Dependencies{
		Publisher: f.publisher, KafkaPublisher: f.kafka, KafkaQueues: f.kafkaQs, API: f.api, Queues: f.queues, Inbox: f.inbox, SMTPChaos: f.chaos,
		MockAPI: f.mock, Callbacks: f.callbacks, Health: fakeHealth{ready: ready}, NewID: func() string { return "id-1" },
	})
}

func validForm() message.Form {
	return message.Form{From: "no-reply@exemplo.com.br", To: "maria@exemplo.com.br", Subject: "Oi", HTML: "<p>Oi</p>", Format: message.FormatArray}
}

func TestConsoleSend(t *testing.T) {
	t.Run("given a valid form without channel then publish to rabbitmq", func(t *testing.T) {
		f := newFakes()

		result, err := f.console(true).Send(context.Background(), validForm())

		require.NoError(t, err)
		assert.Equal(t, "id-1", result.Email.ID)
		assert.Equal(t, message.ChannelRabbitMQ, result.Channel)
		assert.Nil(t, result.Response)
		assert.Equal(t, []message.Email{result.Email}, f.publisher.published)
	})

	t.Run("given the kafka channel then produce to kafka", func(t *testing.T) {
		f := newFakes()
		form := validForm()
		form.Channel = message.ChannelKafka

		result, err := f.console(true).Send(context.Background(), form)

		require.NoError(t, err)
		assert.Equal(t, message.ChannelKafka, result.Channel)
		assert.Equal(t, []message.Email{result.Email}, f.kafka.published)
		assert.Empty(t, f.publisher.published)
	})

	t.Run("given a kafka error then return it", func(t *testing.T) {
		f := newFakes()
		f.kafka.err = errors.New("kafka down")
		form := validForm()
		form.Channel = message.ChannelKafka

		_, err := f.console(true).Send(context.Background(), form)

		require.EqualError(t, err, "kafka down")
	})

	t.Run("given the http channel then call the api and return its response", func(t *testing.T) {
		f := newFakes()
		f.api.response = api.Response{StatusCode: 200}
		form := validForm()
		form.Channel = message.ChannelHTTP

		result, err := f.console(true).Send(context.Background(), form)

		require.NoError(t, err)
		assert.Equal(t, &api.Response{StatusCode: 200}, result.Response)
		assert.Len(t, f.api.sent, 1)
		assert.Empty(t, f.publisher.published)
	})

	t.Run("given an api error then return it", func(t *testing.T) {
		f := newFakes()
		f.api.err = errors.New("mailer down")
		form := validForm()
		form.Channel = message.ChannelHTTP

		_, err := f.console(true).Send(context.Background(), form)

		require.EqualError(t, err, "mailer down")
	})

	t.Run("given an invalid form then return error without publishing", func(t *testing.T) {
		f := newFakes()
		form := validForm()
		form.From = ""

		_, err := f.console(true).Send(context.Background(), form)

		require.Error(t, err)
		assert.Empty(t, f.publisher.published)
	})

	t.Run("given a publish error then return it", func(t *testing.T) {
		f := newFakes()
		f.publisher.err = errors.New("broker down")

		_, err := f.console(true).Send(context.Background(), validForm())

		require.EqualError(t, err, "broker down")
	})
}

func TestConsoleStatus(t *testing.T) {
	t.Run("given a ready mailer and queue stats then report both", func(t *testing.T) {
		f := newFakes()
		f.queues.stats = monitor.QueueStats{Messages: 2, Consumers: 1, Retrying: 3, DeadLettered: 1}

		f.kafkaQs.stats = monitor.QueueStats{Messages: 1, Consumers: 1}

		assert.Equal(t, Status{MailerReady: true, Queue: f.queues.stats, Kafka: f.kafkaQs.stats}, f.console(true).Status(context.Background()))
	})

	t.Run("given queue errors then report each of them", func(t *testing.T) {
		f := newFakes()
		f.queues.statsErr = errors.New("management api down")
		f.kafkaQs.statsErr = errors.New("kafka down")

		assert.Equal(t, Status{QueueError: "management api down", KafkaError: "kafka down"}, f.console(false).Status(context.Background()))
	})
}

func TestConsolePanels(t *testing.T) {
	t.Run("must delegate listing and clearing of inbox, callbacks and dead letters", func(t *testing.T) {
		f := newFakes()
		f.inbox.messages = []monitor.InboxMessage{{ID: "m1"}}
		f.callbacks.events = []monitor.CallbackEvent{{Path: "/failures"}}
		f.queues.letters = []monitor.DeadLetter{{MessageID: "d1"}}
		f.kafkaQs.letters = []monitor.DeadLetter{{MessageID: "k1", Source: "kafka"}}
		console := f.console(true)

		messages, err := console.Inbox(context.Background())
		require.NoError(t, err)
		events, err := console.Callbacks(context.Background())
		require.NoError(t, err)
		letters, err := console.DeadLetters(context.Background())
		require.NoError(t, err)
		require.NoError(t, console.ClearInbox(context.Background()))
		require.NoError(t, console.ClearCallbacks(context.Background()))
		require.NoError(t, console.PurgeDeadLetters(context.Background()))

		assert.Equal(t, f.inbox.messages, messages)
		assert.Equal(t, f.callbacks.events, events)
		assert.Equal(t, append(f.queues.letters, f.kafkaQs.letters...), letters)
		assert.True(t, f.kafkaQs.purged)
		assert.True(t, f.inbox.cleared)
		assert.True(t, f.callbacks.cleared)
		assert.True(t, f.queues.purged)
	})
}

func TestConsoleDeadLettersPartialFailures(t *testing.T) {
	t.Run("given kafka unavailable then return the rabbitmq letters with the kafka error", func(t *testing.T) {
		f := newFakes()
		f.queues.letters = []monitor.DeadLetter{{MessageID: "d1"}}
		f.kafkaQs.peekErr = errors.New("kafka down")

		letters, err := f.console(true).DeadLetters(context.Background())

		require.EqualError(t, err, "kafka down")
		assert.Equal(t, f.queues.letters, letters)
	})

	t.Run("given a purge failure then still purge the other side", func(t *testing.T) {
		f := newFakes()
		f.queues.purgeErr = errors.New("rabbitmq down")

		err := f.console(true).PurgeDeadLetters(context.Background())

		require.EqualError(t, err, "rabbitmq down")
		assert.True(t, f.kafkaQs.purged)
	})
}
