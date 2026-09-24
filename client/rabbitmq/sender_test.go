package rabbitmq

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/client"
)

func TestSender(t *testing.T) {
	t.Run("given no url then refuse to build", func(t *testing.T) {
		_, err := New(Config{})

		require.Error(t, err)
	})

	t.Run("must default queue and timeout", func(t *testing.T) {
		sender, err := New(Config{URL: "amqp://guest:guest@localhost:5672/"})

		require.NoError(t, err)
		assert.Equal(t, "mailer-service", sender.cfg.Queue)
		assert.Equal(t, 10*time.Second, sender.cfg.Timeout)
	})

	t.Run("given an invalid email then fail without connecting", func(t *testing.T) {
		sender, err := New(Config{URL: "amqp://guest:guest@127.0.0.1:1/"})
		require.NoError(t, err)

		require.ErrorIs(t, sender.Send(context.Background(), client.Email{}), client.ErrInvalidEmail)
	})

	t.Run("given an unreachable broker then send and health check return errors", func(t *testing.T) {
		sender, err := New(Config{URL: "amqp://guest:guest@127.0.0.1:1/", Timeout: 300 * time.Millisecond})
		require.NoError(t, err)
		email := client.Email{From: "a@exemplo.com.br", To: []string{"b@exemplo.com.br"}, Subject: "s", HTML: "h"}

		require.ErrorContains(t, sender.Send(context.Background(), email), "connect to rabbitmq")
		require.ErrorContains(t, sender.HealthCheck(context.Background()), "connect to rabbitmq")
		require.NoError(t, sender.Close())
	})
}
