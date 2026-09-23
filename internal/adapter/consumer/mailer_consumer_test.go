package consumer

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/core/model"
)

type mockService struct {
	delivered model.SendEmailData
	calls     int
	err       error
}

func (m *mockService) Deliver(data model.SendEmailData) error {
	m.delivered = data
	m.calls++
	return m.err
}

func TestMailerConsumerHandle(t *testing.T) {
	t.Run("given a valid message then deliver it with the callback", func(t *testing.T) {
		service := &mockService{}
		body := []byte(`{"from":"sender@exemplo.com.br","receiver":"maria@exemplo.com.br","subject":"Assunto","body":"PGgxPk9pPC9oMT4=","callback":{"success":{"url":"https://exemplo.com.br/sent"},"failure":{"url":"https://exemplo.com.br/cb"}}}`)

		err := NewMailerConsumer(service).Handle(body)

		require.NoError(t, err)
		assert.Equal(t, 1, service.calls)
		assert.Equal(t, model.Recipients{"maria@exemplo.com.br"}, service.delivered.Receiver)
		assert.Equal(t, "PGgxPk9pPC9oMT4=", service.delivered.Body)
		require.NotNil(t, service.delivered.Callback)
		require.NotNil(t, service.delivered.Callback.Success)
		require.NotNil(t, service.delivered.Callback.Failure)
		assert.Equal(t, "https://exemplo.com.br/sent", service.delivered.Callback.Success.URL)
		assert.Equal(t, "https://exemplo.com.br/cb", service.delivered.Callback.Failure.URL)
	})

	t.Run("given invalid json then return error without delivering", func(t *testing.T) {
		service := &mockService{}

		err := NewMailerConsumer(service).Handle([]byte("invalid json"))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal")
		assert.Zero(t, service.calls)
	})

	t.Run("given a service error then return it", func(t *testing.T) {
		service := &mockService{err: errors.New("delivery failed")}
		body, err := json.Marshal(model.SendEmailData{From: "sender@exemplo.com.br", Receiver: model.Recipients{"maria@exemplo.com.br"}})
		require.NoError(t, err)

		err = NewMailerConsumer(service).Handle(body)

		require.EqualError(t, err, "delivery failed")
	})
}
