package messaging

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSource(t *testing.T) {
	t.Run("must expose name and readiness of the consumer", func(t *testing.T) {
		source := NewSource(NewConsumer(RabbitMQConfig{URL: "amqp://invalid", Queue: "q"}), func([]byte) error { return nil })

		assert.Equal(t, "rabbitmq", source.Name())
		assert.False(t, source.Ready())
	})

	t.Run("given a cancelled context then stop while connecting", func(t *testing.T) {
		source := NewSource(NewConsumer(RabbitMQConfig{URL: "amqp://guest:guest@127.0.0.1:1/", Queue: "q"}), func([]byte) error { return nil })
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := source.Run(ctx)

		require.ErrorIs(t, err, context.Canceled)
	})
}
