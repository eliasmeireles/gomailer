package mailer

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/application/config"
	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

func TestSMTPSenderSend(t *testing.T) {
	t.Run("given an invalid email then fail before connecting", func(t *testing.T) {
		data := newValidEmail()
		data.Subject = ""

		err := NewSMTPSender(config.SMTPConfig{Host: "smtp.exemplo.com.br", Port: 465}).Send(data)

		require.EqualError(t, err, "subject cannot be empty")
	})

	t.Run("given an unreachable server then return a connection error", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		port := listener.Addr().(*net.TCPAddr).Port
		require.NoError(t, listener.Close())

		err = NewSMTPSender(config.SMTPConfig{Host: "127.0.0.1", Port: port}).Send(newValidEmail())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to establish TLS connection")
		assert.Equal(t, model.CodeSMTPConnectionFailed, mailer.CodeOf(err))
	})
}

func TestNewSMTPSender(t *testing.T) {
	t.Run("must implement mailer.Sender", func(t *testing.T) {
		assert.Implements(t, (*mailer.Sender)(nil), NewSMTPSender(config.SMTPConfig{}))
	})
}
