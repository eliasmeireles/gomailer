package queue

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
)

func unreachableURL(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())
	return "amqp://guest:guest@" + addr + "/"
}

func TestPublisher(t *testing.T) {
	t.Run("given an unreachable broker then publish returns a connection error", func(t *testing.T) {
		err := NewPublisher(unreachableURL(t), "mailer-service").Publish(context.Background(), message.Email{ID: "1"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "connect to RabbitMQ")
	})
}
