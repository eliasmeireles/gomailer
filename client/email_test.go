package client

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validEmail() Email {
	return Email{From: "no-reply@example.com", To: []string{"jane@example.com"}, Subject: "Hi", HTML: "<p>Hello</p>"}
}

func TestEmailValidate(t *testing.T) {
	t.Run("given a complete email then accept it", func(t *testing.T) {
		require.NoError(t, validEmail().Validate())
	})

	t.Run("given missing fields then list them in an ErrInvalidEmail", func(t *testing.T) {
		err := Email{To: []string{" "}}.Validate()

		require.ErrorIs(t, err, ErrInvalidEmail)
		assert.Contains(t, err.Error(), "missing from, to, subject, html")
	})
}

func TestPrepare(t *testing.T) {
	t.Run("given no id then generate one", func(t *testing.T) {
		email, err := Prepare(validEmail())

		require.NoError(t, err)
		assert.Regexp(t, `^[0-9a-f-]{36}$`, email.ID)
	})

	t.Run("given an id then keep it", func(t *testing.T) {
		input := validEmail()
		input.ID = "order-1"

		email, err := Prepare(input)

		require.NoError(t, err)
		assert.Equal(t, "order-1", email.ID)
	})

	t.Run("given an invalid email then return the validation error", func(t *testing.T) {
		_, err := Prepare(Email{})

		require.True(t, errors.Is(err, ErrInvalidEmail))
	})
}

func TestEmailMarshalJSON(t *testing.T) {
	t.Run("must encode the gomailer wire format", func(t *testing.T) {
		email := Email{
			ID:          "order-1",
			From:        " no-reply@example.com ",
			To:          []string{"jane@example.com", " "},
			Cc:          []string{"john@example.com"},
			Subject:     "Hi",
			HTML:        "<p>Hello</p>",
			Attachments: []Attachment{{Name: "a.txt", ContentType: "text/plain", Content: []byte("hi")}, {Name: "b.bin", Content: []byte{1}}},
			Callback:    &Callback{Failure: &CallbackTarget{URL: "https://example.com/failures"}},
		}

		raw, err := json.Marshal(email)

		require.NoError(t, err)
		assert.JSONEq(t, `{
			"id": "order-1",
			"from": "no-reply@example.com",
			"receiver": ["jane@example.com"],
			"cc": ["john@example.com"],
			"subject": "Hi",
			"body": "`+base64.StdEncoding.EncodeToString([]byte("<p>Hello</p>"))+`",
			"attachments": [
				{"name": "a.txt", "type": "text/plain", "data": "aGk=", "decoder": "base64"},
				{"name": "b.bin", "type": "application/octet-stream", "data": "AQ==", "decoder": "base64"}
			],
			"callback": {"failure": {"url": "https://example.com/failures"}}
		}`, string(raw))
	})

	t.Run("must omit empty optional fields", func(t *testing.T) {
		raw, err := json.Marshal(validEmail())

		require.NoError(t, err)
		assert.NotContains(t, string(raw), `"cc"`)
		assert.NotContains(t, string(raw), `"bcc"`)
		assert.NotContains(t, string(raw), `"attachments"`)
		assert.NotContains(t, string(raw), `"callback"`)
		assert.NotContains(t, string(raw), `"id"`)
	})
}

func TestNewID(t *testing.T) {
	t.Run("must return distinct version 4 uuids", func(t *testing.T) {
		first, second := NewID(), NewID()

		assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, first)
		assert.NotEqual(t, first, second)
	})
}

func TestErrorCodeRetryable(t *testing.T) {
	t.Run("must flag only temporary failures", func(t *testing.T) {
		assert.True(t, CodeAPIRateLimited.Retryable())
		assert.True(t, CodeSMTPTemporaryFailure.Retryable())
		assert.False(t, CodeAPIInvalidReceiver.Retryable())
		assert.False(t, CodeUnknown.Retryable())
	})
}
