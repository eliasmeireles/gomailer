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

const testHTML = "<h1>Hello</h1>"

var (
	testNow           = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	testSuccessTarget = &model.CallbackTarget{URL: "https://example.com/sent", Headers: map[string]string{"Authorization": "Bearer ok"}}
	testFailureTarget = &model.CallbackTarget{URL: "https://example.com/failed", Headers: map[string]string{"Authorization": "Bearer fail"}}
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
		ID:       "order-123",
		From:     "no-reply@example.com",
		Receiver: model.Recipients{"jane@example.com"},
		Subject:  "Test subject",
		Body:     base64.StdEncoding.EncodeToString([]byte(testHTML)),
		Callback: callback,
	}
}

var testPolicy = RetryPolicy{MaxAttempts: 3, BaseDelay: 10 * time.Second, MaxDelay: time.Minute}

func newTestService(sender *mockSender, notifier *mockNotifier) *service {
	return &service{sender: sender, notifier: notifier, policy: testPolicy, now: func() time.Time { return testNow }}
}

func TestServiceDeliverSuccess(t *testing.T) {
	t.Run("given a valid email then send the decoded body without callback", func(t *testing.T) {
		sender, notifier := &mockSender{}, &mockNotifier{}

		outcome := newTestService(sender, notifier).DeliverOnce(newTestEmail(&model.Callback{Success: testSuccessTarget}))

		assert.True(t, outcome.Sent())
		assert.Equal(t, ActionDone, outcome.Action)
		assert.Equal(t, testHTML, sender.sent.Body)
		assert.Nil(t, sender.sent.Callback)
	})

	t.Run("given a success then return the sent event", func(t *testing.T) {
		outcome := newTestService(&mockSender{}, &mockNotifier{}).DeliverOnce(newTestEmail(nil))

		assert.Equal(t, model.DeliveryEvent{ID: "order-123", Subject: "Test subject", Status: model.StatusSent, OccurredAt: testNow}, outcome.Event)
	})

	t.Run("given a success callback then notify it with the sent event", func(t *testing.T) {
		notifier := &mockNotifier{}

		newTestService(&mockSender{}, notifier).DeliverOnce(newTestEmail(&model.Callback{Success: testSuccessTarget, Failure: testFailureTarget}))

		assert.Equal(t, []notification{{
			target: *testSuccessTarget,
			event:  model.DeliveryEvent{ID: "order-123", Subject: "Test subject", Status: model.StatusSent, OccurredAt: testNow},
		}}, notifier.notifications)
	})

	t.Run("given only a failure callback then do not notify on success", func(t *testing.T) {
		notifier := &mockNotifier{}

		newTestService(&mockSender{}, notifier).DeliverOnce(newTestEmail(&model.Callback{Failure: testFailureTarget}))

		assert.Empty(t, notifier.notifications)
	})

	t.Run("given a failing success callback then the outcome stays sent and handled", func(t *testing.T) {
		sender, notifier := &mockSender{}, &mockNotifier{err: errors.New("callback down")}

		outcome := newTestService(sender, notifier).DeliverOnce(newTestEmail(&model.Callback{Success: testSuccessTarget}))

		assert.True(t, outcome.Sent())
		assert.Equal(t, ActionDone, outcome.Action)
		assert.Equal(t, 1, sender.calls)
	})

	t.Run("given a success callback with blank url then ignore it", func(t *testing.T) {
		notifier := &mockNotifier{}

		newTestService(&mockSender{}, notifier).DeliverOnce(newTestEmail(&model.Callback{Success: &model.CallbackTarget{URL: " "}}))

		assert.Empty(t, notifier.notifications)
	})
}

func TestServiceDeliverFailure(t *testing.T) {
	t.Run("given a send error and no callback then dead-letter the failure", func(t *testing.T) {
		notifier := &mockNotifier{}

		outcome := newTestService(&mockSender{err: errors.New("smtp down")}, notifier).DeliverOnce(newTestEmail(nil))

		require.Error(t, outcome.Err)
		assert.False(t, outcome.Sent())
		assert.Equal(t, ActionDeadLetter, outcome.Action)
		assert.Equal(t, model.StatusFailed, outcome.Event.Status)
		assert.Contains(t, outcome.Event.Cause, "smtp down")
		assert.Empty(t, notifier.notifications)
	})

	t.Run("given a send error and only a success callback then dead-letter the failure", func(t *testing.T) {
		notifier := &mockNotifier{}

		outcome := newTestService(&mockSender{err: errors.New("smtp down")}, notifier).DeliverOnce(newTestEmail(&model.Callback{Success: testSuccessTarget}))

		assert.Equal(t, ActionDeadLetter, outcome.Action)
		assert.Empty(t, notifier.notifications)
	})

	t.Run("given a send error and a failure callback then notify it and finish", func(t *testing.T) {
		notifier := &mockNotifier{}
		sendErr := Errorf(model.CodeAPISenderNotAllowed, "domain not verified")

		outcome := newTestService(&mockSender{err: sendErr}, notifier).
			DeliverOnce(newTestEmail(&model.Callback{Success: testSuccessTarget, Failure: testFailureTarget}))

		assert.Equal(t, ActionDone, outcome.Action)
		assert.False(t, outcome.Sent())
		expected := model.DeliveryEvent{
			ID:         "order-123",
			Subject:    "Test subject",
			Status:     model.StatusFailed,
			ErrorCode:  model.CodeAPISenderNotAllowed,
			Cause:      "failed to send email to jane@example.com: domain not verified",
			OccurredAt: testNow,
		}
		assert.Equal(t, expected, outcome.Event)
		assert.Equal(t, []notification{{target: *testFailureTarget, event: expected}}, notifier.notifications)
	})

	t.Run("given an invalid base64 body and a failure callback then notify without sending", func(t *testing.T) {
		sender, notifier := &mockSender{}, &mockNotifier{}
		email := newTestEmail(&model.Callback{Failure: testFailureTarget})
		email.Body = "not-valid-base64!!!"

		outcome := newTestService(sender, notifier).DeliverOnce(email)

		assert.Equal(t, ActionDone, outcome.Action)
		assert.Zero(t, sender.calls)
		assert.Equal(t, model.CodeMessageInvalidBody, outcome.Event.ErrorCode)
		assert.Contains(t, outcome.Event.Cause, "failed to decode base64 body")
	})

	t.Run("given an unclassified send error then report unknown_error", func(t *testing.T) {
		outcome := newTestService(&mockSender{err: errors.New("boom")}, &mockNotifier{}).DeliverOnce(newTestEmail(nil))

		assert.Equal(t, model.CodeUnknown, outcome.Event.ErrorCode)
	})

	t.Run("given a success then the event has no error code", func(t *testing.T) {
		outcome := newTestService(&mockSender{}, &mockNotifier{}).DeliverOnce(newTestEmail(nil))

		assert.Empty(t, outcome.Event.ErrorCode)
	})

	t.Run("given a send error and a failing failure callback then dead-letter with both errors", func(t *testing.T) {
		notifier := &mockNotifier{err: errors.New("callback unreachable")}

		outcome := newTestService(&mockSender{err: errors.New("smtp down")}, notifier).DeliverOnce(newTestEmail(&model.Callback{Failure: testFailureTarget}))

		assert.Equal(t, ActionDeadLetter, outcome.Action)
		require.Error(t, outcome.Err)
		assert.Contains(t, outcome.Err.Error(), "smtp down")
		assert.Contains(t, outcome.Err.Error(), "callback unreachable")
	})
}

func TestServiceDeliverRetry(t *testing.T) {
	temporary := Errorf(model.CodeAPIRateLimited, "too many requests")

	t.Run("given a temporary failure with attempts left then retry without calling callbacks", func(t *testing.T) {
		notifier := &mockNotifier{}

		outcome := newTestService(&mockSender{err: temporary}, notifier).Deliver(newTestEmail(&model.Callback{Failure: testFailureTarget}), 2)

		assert.Equal(t, ActionRetry, outcome.Action)
		assert.Equal(t, 20*time.Second, outcome.RetryIn)
		assert.Equal(t, model.CodeAPIRateLimited, outcome.Event.ErrorCode)
		assert.Empty(t, notifier.notifications)
	})

	t.Run("given a temporary failure on the last attempt then notify the failure callback", func(t *testing.T) {
		notifier := &mockNotifier{}

		outcome := newTestService(&mockSender{err: temporary}, notifier).Deliver(newTestEmail(&model.Callback{Failure: testFailureTarget}), 3)

		assert.Equal(t, ActionDone, outcome.Action)
		assert.Len(t, notifier.notifications, 1)
	})

	t.Run("given a temporary failure on the last attempt without callback then dead-letter", func(t *testing.T) {
		outcome := newTestService(&mockSender{err: temporary}, &mockNotifier{}).Deliver(newTestEmail(nil), 3)

		assert.Equal(t, ActionDeadLetter, outcome.Action)
	})

	t.Run("given a permanent failure on the first attempt then do not retry", func(t *testing.T) {
		notifier := &mockNotifier{}
		permanent := Errorf(model.CodeAPIInvalidReceiver, "invalid to")

		outcome := newTestService(&mockSender{err: permanent}, notifier).Deliver(newTestEmail(&model.Callback{Failure: testFailureTarget}), 1)

		assert.Equal(t, ActionDone, outcome.Action)
		assert.Len(t, notifier.notifications, 1)
	})

	t.Run("given a queued success then finish", func(t *testing.T) {
		outcome := newTestService(&mockSender{}, &mockNotifier{}).Deliver(newTestEmail(nil), 2)

		assert.Equal(t, ActionDone, outcome.Action)
		assert.True(t, outcome.Sent())
	})

	t.Run("given a temporary failure on a synchronous delivery then never retry", func(t *testing.T) {
		outcome := newTestService(&mockSender{err: temporary}, &mockNotifier{}).DeliverOnce(newTestEmail(nil))

		assert.Equal(t, ActionDeadLetter, outcome.Action)
	})
}

func TestActionString(t *testing.T) {
	t.Run("must name every action", func(t *testing.T) {
		assert.Equal(t, "done", ActionDone.String())
		assert.Equal(t, "retry", ActionRetry.String())
		assert.Equal(t, "dead-letter", ActionDeadLetter.String())
	})
}

func TestNewService(t *testing.T) {
	t.Run("must create a service with a real clock", func(t *testing.T) {
		svc := NewService(&mockSender{}, &mockNotifier{}, DefaultRetryPolicy).(*service)

		assert.WithinDuration(t, time.Now(), svc.now(), time.Second)
	})
}
