package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSources(t *testing.T) {
	t.Run("given no env then default to rabbitmq", func(t *testing.T) {
		t.Setenv(envMailerSources, "")

		sources, err := Sources()

		require.NoError(t, err)
		assert.Equal(t, []SourceName{SourceRabbitMQ}, sources)
	})

	t.Run("given a list with spaces, case and duplicates then normalize it", func(t *testing.T) {
		t.Setenv(envMailerSources, " HTTP, rabbitmq ,http,")

		sources, err := Sources()

		require.NoError(t, err)
		assert.Equal(t, []SourceName{SourceHTTP, SourceRabbitMQ}, sources)
	})

	t.Run("given an unknown source then return error listing the available ones", func(t *testing.T) {
		t.Setenv(envMailerSources, "rabbitmq,pigeon")

		_, err := Sources()

		require.EqualError(t, err, `MAILER_SOURCES: unknown source "pigeon", available: rabbitmq, http`)
	})

	t.Run("given only separators then return error", func(t *testing.T) {
		t.Setenv(envMailerSources, " , ,")

		_, err := Sources()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one source")
	})
}
