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
			ID:          "order-1",
			From:        "no-reply@example.com",
			To:          []string{"jane@example.com", "john@example.com"},
			Cc:          []string{"peter@example.com"},
			Bcc:         []string{"anna@example.com"},
			Subject:     "Hello",
			HTML:        "<p>Hello</p>",
			Attachments: []client.Attachment{{Name: "a.txt", ContentType: "text/plain", Content: []byte("hi")}},
			Callback: &client.Callback{
				Success: &client.CallbackTarget{URL: "https://example.com/ok"},
				Failure: &client.CallbackTarget{URL: "https://example.com/failure", Headers: map[string]string{"Authorization": "Bearer t"}},
			},
		}
		raw, err := json.Marshal(email)
		require.NoError(t, err)

		var data SendEmailData
		require.NoError(t, json.Unmarshal(raw, &data))

		body, err := base64.StdEncoding.DecodeString(data.Body)
		require.NoError(t, err)
		assert.Equal(t, "order-1", data.ID)
		assert.Equal(t, "no-reply@example.com", data.From)
		assert.Equal(t, Recipients{"jane@example.com", "john@example.com"}, data.Receiver)
		assert.Equal(t, Recipients{"peter@example.com"}, data.Cc)
		assert.Equal(t, Recipients{"anna@example.com"}, data.Bcc)
		assert.Equal(t, "Hello", data.Subject)
		assert.Equal(t, "<p>Hello</p>", string(body))
		base64Decoder := decoder.TypeBase64
		require.NotNil(t, data.Attachments)
		assert.Equal(t, []Attachment{{Name: "a.txt", Type: "text/plain", Data: "aGk=", Decoder: &base64Decoder}}, *data.Attachments)
		assert.Equal(t, "https://example.com/ok", data.Callback.Success.URL)
		assert.Equal(t, map[string]string{"Authorization": "Bearer t"}, data.Callback.Failure.Headers)
	})

	t.Run("must write events the client library reads", func(t *testing.T) {
		raw, err := json.Marshal(DeliveryEvent{ID: "1", Subject: "Hi", Status: StatusFailed, ErrorCode: CodeAPIRateLimited, Cause: "429"})
		require.NoError(t, err)

		var event client.DeliveryEvent
		require.NoError(t, json.Unmarshal(raw, &event))

		assert.Equal(t, client.StatusFailed, event.Status)
		assert.Equal(t, client.CodeAPIRateLimited, event.ErrorCode)
		assert.True(t, event.ErrorCode.Retryable())
	})
}
