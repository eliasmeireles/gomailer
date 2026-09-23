package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientTimeout(t *testing.T) {
	t.Run("given no env then return the default", func(t *testing.T) {
		t.Setenv(envHTTPClientTimeout, "")

		timeout, err := HTTPClientTimeout()

		require.NoError(t, err)
		assert.Equal(t, defaultHTTPClientTimeout, timeout)
	})

	t.Run("given a valid duration then return it", func(t *testing.T) {
		t.Setenv(envHTTPClientTimeout, "3s")

		timeout, err := HTTPClientTimeout()

		require.NoError(t, err)
		assert.Equal(t, 3*time.Second, timeout)
	})

	t.Run("given an invalid duration then return error", func(t *testing.T) {
		t.Setenv(envHTTPClientTimeout, "abc")

		_, err := HTTPClientTimeout()

		require.Error(t, err)
	})

	t.Run("given a non-positive duration then return error", func(t *testing.T) {
		t.Setenv(envHTTPClientTimeout, "0s")

		_, err := HTTPClientTimeout()

		require.Error(t, err)
	})
}

func TestEnvHelpers(t *testing.T) {
	t.Run("given a blank required env then return error", func(t *testing.T) {
		t.Setenv("MAILER_TEST_KEY", "  ")

		_, err := requireEnv("MAILER_TEST_KEY")

		require.EqualError(t, err, "required environment variable MAILER_TEST_KEY is not set")
	})

	t.Run("given a set env then return it trimmed", func(t *testing.T) {
		t.Setenv("MAILER_TEST_KEY", " value ")

		assert.Equal(t, "value", envOrDefault("MAILER_TEST_KEY", "default"))
	})

	t.Run("given an unset env then return the default", func(t *testing.T) {
		t.Setenv("MAILER_TEST_KEY", "")

		assert.Equal(t, "default", envOrDefault("MAILER_TEST_KEY", "default"))
	})
}
