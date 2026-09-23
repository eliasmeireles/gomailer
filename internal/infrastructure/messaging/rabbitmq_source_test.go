package messaging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/core/mailer"
)

func TestSource(t *testing.T) {
	t.Run("must expose name and readiness of the consumer", func(t *testing.T) {
		source := NewSource(NewConsumer(RabbitMQConfig{URL: "amqp://invalid", Queue: "q"}, mailer.DefaultRetryPolicy), func([]byte, int) mailer.Outcome { return mailer.Outcome{} })

		assert.Equal(t, "rabbitmq", source.Name())
		assert.False(t, source.Ready())
	})

	t.Run("given a cancelled context then stop while connecting", func(t *testing.T) {
		source := NewSource(NewConsumer(RabbitMQConfig{URL: "amqp://guest:guest@127.0.0.1:1/", Queue: "q"}, mailer.DefaultRetryPolicy), func([]byte, int) mailer.Outcome { return mailer.Outcome{} })
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := source.Run(ctx)

		require.ErrorIs(t, err, context.Canceled)
	})
}
