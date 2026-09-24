package consumer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

type mockService struct {
	delivered model.SendEmailData
	attempt   int
	calls     int
	outcome   mailer.Outcome
}

func (m *mockService) Deliver(data model.SendEmailData, attempt int) mailer.Outcome {
	m.delivered = data
	m.attempt = attempt
	m.calls++
	return m.outcome
}

func (m *mockService) DeliverOnce(data model.SendEmailData) mailer.Outcome {
	return m.Deliver(data, 1)
}

func TestMailerConsumerHandle(t *testing.T) {
	t.Run("given a valid message then deliver it on the given attempt with the callback", func(t *testing.T) {
		service := &mockService{outcome: mailer.Outcome{Action: mailer.ActionRetry}}
		body := []byte(`{"from":"sender@example.com","receiver":"jane@example.com","subject":"Subject","body":"PGgxPkhpPC9oMT4=","callback":{"success":{"url":"https://example.com/sent"},"failure":{"url":"https://example.com/cb"}}}`)

		outcome := NewMailerConsumer(service).Handle(body, 3)

		assert.Equal(t, mailer.ActionRetry, outcome.Action)
		assert.Equal(t, 3, service.attempt)
		assert.Equal(t, model.Recipients{"jane@example.com"}, service.delivered.Receiver)
		require.NotNil(t, service.delivered.Callback)
		require.NotNil(t, service.delivered.Callback.Failure)
		assert.Equal(t, "https://example.com/cb", service.delivered.Callback.Failure.URL)
	})

	t.Run("given invalid json then dead-letter it without delivering", func(t *testing.T) {
		service := &mockService{}

		outcome := NewMailerConsumer(service).Handle([]byte("invalid json"), 1)

		assert.Equal(t, mailer.ActionDeadLetter, outcome.Action)
		assert.Equal(t, model.CodeMessageInvalidJSON, outcome.Event.ErrorCode)
		assert.Contains(t, outcome.Event.Cause, "failed to unmarshal")
		require.Error(t, outcome.Err)
		assert.Zero(t, service.calls)
	})
}
