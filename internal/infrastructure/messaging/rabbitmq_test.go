package messaging

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/core/mailer"
)

func TestNewRabbitMQConfig(t *testing.T) {
	t.Run("given all env vars then build full URL", func(t *testing.T) {
		t.Setenv(envRabbitMQUser, "user")
		t.Setenv(envRabbitMQPass, "pass")
		t.Setenv(envRabbitMQHost, "rabbit.example.com")
		t.Setenv(envRabbitMQPort, "5673")
		t.Setenv(envRabbitMQVHost, "/notification")
		t.Setenv(envRabbitMQQueue, "mailer-service")

		cfg := NewRabbitMQConfig()

		assert.Equal(t, "amqp://user:pass@rabbit.example.com:5673/notification", cfg.URL)
		assert.Equal(t, "mailer-service", cfg.Queue)
	})

	t.Run("given vhost without leading slash then prepend slash", func(t *testing.T) {
		t.Setenv(envRabbitMQUser, "user")
		t.Setenv(envRabbitMQPass, "pass")
		t.Setenv(envRabbitMQHost, "rabbit")
		t.Setenv(envRabbitMQPort, "5672")
		t.Setenv(envRabbitMQVHost, "notification")
		t.Setenv(envRabbitMQQueue, "q")

		cfg := NewRabbitMQConfig()

		assert.True(t, strings.HasSuffix(cfg.URL, "/notification"))
	})

	t.Run("given optional vars empty then apply defaults", func(t *testing.T) {
		t.Setenv(envRabbitMQUser, "user")
		t.Setenv(envRabbitMQPass, "pass")
		t.Setenv(envRabbitMQHost, "rabbit")
		t.Setenv(envRabbitMQPort, "")
		t.Setenv(envRabbitMQVHost, "")
		t.Setenv(envRabbitMQQueue, "")

		cfg := NewRabbitMQConfig()

		assert.Equal(t, "amqp://user:pass@rabbit:5672/", cfg.URL)
		assert.Equal(t, defaultRabbitMQQueue, cfg.Queue)
	})
}

func TestConsumerReadyInitiallyFalse(t *testing.T) {
	t.Run("must report not ready before Connect", func(t *testing.T) {
		c := NewConsumer(RabbitMQConfig{URL: "amqp://invalid", Queue: "q"}, mailer.DefaultRetryPolicy)
		assert.False(t, c.Ready())
	})
}

func TestConsumerConnectRespectsCancelledContext(t *testing.T) {
	t.Run("when context is cancelled then Connect returns ctx error", func(t *testing.T) {
		c := NewConsumer(RabbitMQConfig{URL: "amqp://127.0.0.1:1/", Queue: "q"}, mailer.DefaultRetryPolicy)

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()

		err := c.Connect(ctx)

		require.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.False(t, c.Ready())
	})
}
