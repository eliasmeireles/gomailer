package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRetryEnv(t *testing.T, attempts, base, maxDelay string) {
	t.Setenv(envMaxAttempts, attempts)
	t.Setenv(envRetryBaseDelay, base)
	t.Setenv(envRetryMaxDelay, maxDelay)
}

func TestNewRetryConfig(t *testing.T) {
	t.Run("given no env then use the defaults", func(t *testing.T) {
		setRetryEnv(t, "", "", "")

		cfg, err := NewRetryConfig()

		require.NoError(t, err)
		assert.Equal(t, RetryConfig{MaxAttempts: 5, BaseDelay: 30 * time.Second, MaxDelay: 10 * time.Minute}, cfg)
	})

	t.Run("given overrides then use them", func(t *testing.T) {
		setRetryEnv(t, "3", "5s", "20s")

		cfg, err := NewRetryConfig()

		require.NoError(t, err)
		assert.Equal(t, RetryConfig{MaxAttempts: 3, BaseDelay: 5 * time.Second, MaxDelay: 20 * time.Second}, cfg)
	})

	t.Run("given an invalid attempts value then return error", func(t *testing.T) {
		setRetryEnv(t, "0", "", "")

		_, err := NewRetryConfig()

		require.Error(t, err)
		assert.Contains(t, err.Error(), envMaxAttempts)
	})

	t.Run("given an invalid delay then return error", func(t *testing.T) {
		setRetryEnv(t, "", "soon", "")

		_, err := NewRetryConfig()

		require.EqualError(t, err, `MAILER_RETRY_BASE_DELAY must be a positive duration (e.g. 30s), got "soon"`)
	})

	t.Run("given a max delay lower than the base then return error", func(t *testing.T) {
		setRetryEnv(t, "", "1m", "10s")

		_, err := NewRetryConfig()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must not be lower than")
	})
}
