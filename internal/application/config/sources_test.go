package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSources(t *testing.T) {
	t.Run("given no env then enable rabbitmq and http", func(t *testing.T) {
		t.Setenv(envMailerSources, "")
		t.Setenv(envHTTPAPIDisabled, "")

		sources, err := Sources()

		require.NoError(t, err)
		assert.Equal(t, []SourceName{SourceRabbitMQ, SourceHTTP}, sources)
	})

	t.Run("given queue sources then append http", func(t *testing.T) {
		t.Setenv(envMailerSources, "rabbitmq,kafka")
		t.Setenv(envHTTPAPIDisabled, "")

		sources, err := Sources()

		require.NoError(t, err)
		assert.Equal(t, []SourceName{SourceRabbitMQ, SourceKafka, SourceHTTP}, sources)
	})

	t.Run("given a list with spaces, case and duplicates then normalize it", func(t *testing.T) {
		t.Setenv(envMailerSources, " HTTP, rabbitmq ,http,")
		t.Setenv(envHTTPAPIDisabled, "")

		sources, err := Sources()

		require.NoError(t, err)
		assert.Equal(t, []SourceName{SourceHTTP, SourceRabbitMQ}, sources)
	})

	t.Run("given only http listed then run the http source alone", func(t *testing.T) {
		t.Setenv(envMailerSources, "http")
		t.Setenv(envHTTPAPIDisabled, "")

		sources, err := Sources()

		require.NoError(t, err)
		assert.Equal(t, []SourceName{SourceHTTP}, sources)
	})

	t.Run("given http disabled then enable only the listed sources", func(t *testing.T) {
		t.Setenv(envMailerSources, "rabbitmq,kafka")
		t.Setenv(envHTTPAPIDisabled, "TRUE")

		sources, err := Sources()

		require.NoError(t, err)
		assert.Equal(t, []SourceName{SourceRabbitMQ, SourceKafka}, sources)
	})

	t.Run("given http disabled and listed then return error", func(t *testing.T) {
		t.Setenv(envMailerSources, "rabbitmq,http")
		t.Setenv(envHTTPAPIDisabled, "true")

		_, err := Sources()

		require.EqualError(t, err, `MAILER_SOURCES lists "http" but HTTP_API_DISABLED is true`)
	})

	t.Run("given an unknown source then return error listing the available ones", func(t *testing.T) {
		t.Setenv(envMailerSources, "rabbitmq,pigeon")

		_, err := Sources()

		require.EqualError(t, err, `MAILER_SOURCES: unknown source "pigeon", available: rabbitmq, http, kafka`)
	})

	t.Run("given only separators then return error", func(t *testing.T) {
		t.Setenv(envMailerSources, " , ,")

		_, err := Sources()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one source")
	})
}
