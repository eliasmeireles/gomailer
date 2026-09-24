package model

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/client"
	"github.com/eliasmeireles/gomailer/internal/core/decoder"
)

func TestClientContract(t *testing.T) {
	t.Run("must read every field the client library writes", func(t *testing.T) {
		email := client.Email{
			ID:          "pedido-1",
			From:        "no-reply@exemplo.com.br",
			To:          []string{"maria@exemplo.com.br", "joao@exemplo.com.br"},
			Cc:          []string{"pedro@exemplo.com.br"},
			Bcc:         []string{"ana@exemplo.com.br"},
			Subject:     "Olá",
			HTML:        "<p>Olá</p>",
			Attachments: []client.Attachment{{Name: "a.txt", ContentType: "text/plain", Content: []byte("oi")}},
			Callback: &client.Callback{
				Success: &client.CallbackTarget{URL: "https://exemplo.com.br/ok"},
				Failure: &client.CallbackTarget{URL: "https://exemplo.com.br/falha", Headers: map[string]string{"Authorization": "Bearer t"}},
			},
		}
		raw, err := json.Marshal(email)
		require.NoError(t, err)

		var data SendEmailData
		require.NoError(t, json.Unmarshal(raw, &data))

		body, err := base64.StdEncoding.DecodeString(data.Body)
		require.NoError(t, err)
		assert.Equal(t, "pedido-1", data.ID)
		assert.Equal(t, "no-reply@exemplo.com.br", data.From)
		assert.Equal(t, Recipients{"maria@exemplo.com.br", "joao@exemplo.com.br"}, data.Receiver)
		assert.Equal(t, Recipients{"pedro@exemplo.com.br"}, data.Cc)
		assert.Equal(t, Recipients{"ana@exemplo.com.br"}, data.Bcc)
		assert.Equal(t, "Olá", data.Subject)
		assert.Equal(t, "<p>Olá</p>", string(body))
		base64Decoder := decoder.TypeBase64
		require.NotNil(t, data.Attachments)
		assert.Equal(t, []Attachment{{Name: "a.txt", Type: "text/plain", Data: "b2k=", Decoder: &base64Decoder}}, *data.Attachments)
		assert.Equal(t, "https://exemplo.com.br/ok", data.Callback.Success.URL)
		assert.Equal(t, map[string]string{"Authorization": "Bearer t"}, data.Callback.Failure.Headers)
	})

	t.Run("must write events the client library reads", func(t *testing.T) {
		raw, err := json.Marshal(DeliveryEvent{ID: "1", Subject: "Oi", Status: StatusFailed, ErrorCode: CodeAPIRateLimited, Cause: "429"})
		require.NoError(t, err)

		var event client.DeliveryEvent
		require.NoError(t, json.Unmarshal(raw, &event))

		assert.Equal(t, client.StatusFailed, event.Status)
		assert.Equal(t, client.CodeAPIRateLimited, event.ErrorCode)
		assert.True(t, event.ErrorCode.Retryable())
	})
}
