package bean

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/application/config"
)

func TestNewSender(t *testing.T) {
	t.Run("given smtp transport with full config then return a sender", func(t *testing.T) {
		t.Setenv("SMTP_SERVER", "smtp.exemplo.com.br")
		t.Setenv("SMTP_SERVER_PORT", "465")
		t.Setenv("SMTP_SERVER_USER", "usuario")
		t.Setenv("SMTP_SERVER_PASS", "senha")

		sender, err := newSender(config.MailerSettings{Transport: config.TransportSMTP}, http.DefaultClient)

		require.NoError(t, err)
		assert.NotNil(t, sender)
	})

	t.Run("given smtp transport without config then return error", func(t *testing.T) {
		t.Setenv("SMTP_SERVER", "")

		_, err := newSender(config.MailerSettings{Transport: config.TransportSMTP}, http.DefaultClient)

		require.Error(t, err)
	})

	t.Run("given api transport with resend client then return a sender", func(t *testing.T) {
		t.Setenv("RESEND_API_KEY", "re_test")

		sender, err := newSender(config.MailerSettings{Transport: config.TransportAPI, APIClient: "resend"}, http.DefaultClient)

		require.NoError(t, err)
		assert.NotNil(t, sender)
	})

	t.Run("given an unsupported transport then return error", func(t *testing.T) {
		_, err := newSender(config.MailerSettings{Transport: "pigeon"}, http.DefaultClient)

		require.EqualError(t, err, `unsupported transport "pigeon"`)
	})
}
