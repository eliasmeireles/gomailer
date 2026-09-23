package mailer

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/core/model"
)

const testHTML = "<h1>Olá</h1>"

var (
	testNow           = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	testSuccessTarget = &model.CallbackTarget{URL: "https://exemplo.com.br/sent", Headers: map[string]string{"Authorization": "Bearer ok"}}
	testFailureTarget = &model.CallbackTarget{URL: "https://exemplo.com.br/failed", Headers: map[string]string{"Authorization": "Bearer fail"}}
)

type mockSender struct {
	sent  model.SendEmailData
	calls int
	err   error
}

func (m *mockSender) Send(data model.SendEmailData) error {
	m.sent = data
	m.calls++
	return m.err
}

type notification struct {
	target model.CallbackTarget
	event  model.DeliveryEvent
}

type mockNotifier struct {
	notifications []notification
	err           error
}

func (m *mockNotifier) Notify(target model.CallbackTarget, event model.DeliveryEvent) error {
	m.notifications = append(m.notifications, notification{target: target, event: event})
	return m.err
}

func newTestEmail(callback *model.Callback) model.SendEmailData {
	return model.SendEmailData{
		ID:       "pedido-123",
		From:     "no-reply@exemplo.com.br",
		Receiver: model.Recipients{"maria@exemplo.com.br"},
		Subject:  "Assunto de teste",
		Body:     base64.StdEncoding.EncodeToString([]byte(testHTML)),
		Callback: callback,
	}
}

func newTestService(sender *mockSender, notifier *mockNotifier) *service {
	return &service{sender: sender, notifier: notifier, now: func() time.Time { return testNow }}
}

func TestServiceDeliverSuccess(t *testing.T) {
	t.Run("given a valid email then send the decoded body without callback", func(t *testing.T) {
		sender, notifier := &mockSender{}, &mockNotifier{}

		err := newTestService(sender, notifier).Deliver(newTestEmail(&model.Callback{Success: testSuccessTarget}))

		require.NoError(t, err)
		assert.Equal(t, testHTML, sender.sent.Body)
		assert.Nil(t, sender.sent.Callback)
	})

	t.Run("given a success callback then notify it with the sent event", func(t *testing.T) {
		notifier := &mockNotifier{}

		err := newTestService(&mockSender{}, notifier).Deliver(newTestEmail(&model.Callback{Success: testSuccessTarget, Failure: testFailureTarget}))

		require.NoError(t, err)
		assert.Equal(t, []notification{{
			target: *testSuccessTarget,
			event:  model.DeliveryEvent{ID: "pedido-123", Subject: "Assunto de teste", Status: model.StatusSent, OccurredAt: testNow},
		}}, notifier.notifications)
	})

	t.Run("given only a failure callback then do not notify on success", func(t *testing.T) {
		notifier := &mockNotifier{}

		err := newTestService(&mockSender{}, notifier).Deliver(newTestEmail(&model.Callback{Failure: testFailureTarget}))

		require.NoError(t, err)
		assert.Empty(t, notifier.notifications)
	})

	t.Run("given a failing success callback then still return nil to avoid resending", func(t *testing.T) {
		sender, notifier := &mockSender{}, &mockNotifier{err: errors.New("callback down")}

		err := newTestService(sender, notifier).Deliver(newTestEmail(&model.Callback{Success: testSuccessTarget}))

		require.NoError(t, err)
		assert.Equal(t, 1, sender.calls)
		assert.Len(t, notifier.notifications, 1)
	})

	t.Run("given a success callback with blank url then ignore it", func(t *testing.T) {
		notifier := &mockNotifier{}

		err := newTestService(&mockSender{}, notifier).Deliver(newTestEmail(&model.Callback{Success: &model.CallbackTarget{URL: " "}}))

		require.NoError(t, err)
		assert.Empty(t, notifier.notifications)
	})
}

func TestServiceDeliverFailure(t *testing.T) {
	t.Run("given a send error and no callback then return the error", func(t *testing.T) {
		notifier := &mockNotifier{}

		err := newTestService(&mockSender{err: errors.New("smtp down")}, notifier).Deliver(newTestEmail(nil))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "smtp down")
		assert.Empty(t, notifier.notifications)
	})

	t.Run("given a send error and only a success callback then return the error", func(t *testing.T) {
		notifier := &mockNotifier{}

		err := newTestService(&mockSender{err: errors.New("smtp down")}, notifier).Deliver(newTestEmail(&model.Callback{Success: testSuccessTarget}))

		require.Error(t, err)
		assert.Empty(t, notifier.notifications)
	})

	t.Run("given a send error and a failure callback then notify it and return nil", func(t *testing.T) {
		notifier := &mockNotifier{}
		sendErr := Errorf(model.CodeAPISenderNotAllowed, "domain not verified")

		err := newTestService(&mockSender{err: sendErr}, notifier).
			Deliver(newTestEmail(&model.Callback{Success: testSuccessTarget, Failure: testFailureTarget}))

		require.NoError(t, err)
		require.Len(t, notifier.notifications, 1)
		assert.Equal(t, *testFailureTarget, notifier.notifications[0].target)
		assert.Equal(t, model.DeliveryEvent{
			ID:         "pedido-123",
			Subject:    "Assunto de teste",
			Status:     model.StatusFailed,
			ErrorCode:  model.CodeAPISenderNotAllowed,
			Cause:      "failed to send email to maria@exemplo.com.br: domain not verified",
			OccurredAt: testNow,
		}, notifier.notifications[0].event)
	})

	t.Run("given an invalid base64 body and a failure callback then notify without sending", func(t *testing.T) {
		sender, notifier := &mockSender{}, &mockNotifier{}
		email := newTestEmail(&model.Callback{Failure: testFailureTarget})
		email.Body = "not-valid-base64!!!"

		err := newTestService(sender, notifier).Deliver(email)

		require.NoError(t, err)
		assert.Zero(t, sender.calls)
		require.Len(t, notifier.notifications, 1)
		assert.Contains(t, notifier.notifications[0].event.Cause, "failed to decode base64 body")
		assert.Equal(t, model.CodeMessageInvalidBody, notifier.notifications[0].event.ErrorCode)
	})

	t.Run("given an unclassified send error then report unknown_error", func(t *testing.T) {
		notifier := &mockNotifier{}

		err := newTestService(&mockSender{err: errors.New("boom")}, notifier).Deliver(newTestEmail(&model.Callback{Failure: testFailureTarget}))

		require.NoError(t, err)
		assert.Equal(t, model.CodeUnknown, notifier.notifications[0].event.ErrorCode)
	})

	t.Run("given a success then the event has no error code", func(t *testing.T) {
		notifier := &mockNotifier{}

		require.NoError(t, newTestService(&mockSender{}, notifier).Deliver(newTestEmail(&model.Callback{Success: testSuccessTarget})))

		assert.Empty(t, notifier.notifications[0].event.ErrorCode)
	})

	t.Run("given a send error and a failing failure callback then return both errors", func(t *testing.T) {
		notifier := &mockNotifier{err: errors.New("callback unreachable")}

		err := newTestService(&mockSender{err: errors.New("smtp down")}, notifier).Deliver(newTestEmail(&model.Callback{Failure: testFailureTarget}))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "smtp down")
		assert.Contains(t, err.Error(), "callback unreachable")
	})
}

func TestNewService(t *testing.T) {
	t.Run("must create a service with a real clock", func(t *testing.T) {
		svc := NewService(&mockSender{}, &mockNotifier{}).(*service)

		assert.WithinDuration(t, time.Now(), svc.now(), time.Second)
	})
}
