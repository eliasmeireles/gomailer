package bean

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/application/config"
)

func TestNewSource(t *testing.T) {
	t.Run("given the http source with keys then build it", func(t *testing.T) {
		t.Setenv("HTTP_API_KEYS", "token")

		source, err := newSource(config.SourceHTTP)

		require.NoError(t, err)
		assert.Equal(t, "http", source.Name())
	})

	t.Run("given the http source without keys then return error", func(t *testing.T) {
		t.Setenv("HTTP_API_KEYS", "")

		_, err := newSource(config.SourceHTTP)

		require.Error(t, err)
	})

	t.Run("given the rabbitmq source with credentials then build it", func(t *testing.T) {
		t.Setenv("RABBITMQ_USER", "guest")
		t.Setenv("RABBITMQ_PASS", "guest")
		t.Setenv("RABBITMQ_HOST", "localhost")

		source, err := newSource(config.SourceRabbitMQ)

		require.NoError(t, err)
		assert.Equal(t, "rabbitmq", source.Name())
	})

	t.Run("given an unsupported source then return error", func(t *testing.T) {
		_, err := newSource("pigeon")

		require.EqualError(t, err, `unsupported source "pigeon"`)
	})
}
