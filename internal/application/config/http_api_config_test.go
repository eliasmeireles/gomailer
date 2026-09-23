package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPAPIConfig(t *testing.T) {
	t.Run("given keys then use defaults for port and body size", func(t *testing.T) {
		t.Setenv(envHTTPAPIKeys, " token-a , token-b ,")
		t.Setenv(envHTTPAPIPort, "")
		t.Setenv(envHTTPAPIMaxBodyBytes, "")

		cfg, err := NewHTTPAPIConfig()

		require.NoError(t, err)
		assert.Equal(t, HTTPAPIConfig{Port: "8081", Keys: []string{"token-a", "token-b"}, MaxBodyBytes: 25 << 20}, cfg)
	})

	t.Run("given overrides then use them", func(t *testing.T) {
		t.Setenv(envHTTPAPIKeys, "token")
		t.Setenv(envHTTPAPIPort, "9000")
		t.Setenv(envHTTPAPIMaxBodyBytes, "1024")

		cfg, err := NewHTTPAPIConfig()

		require.NoError(t, err)
		assert.Equal(t, "9000", cfg.Port)
		assert.Equal(t, int64(1024), cfg.MaxBodyBytes)
	})

	t.Run("given no keys then refuse to start", func(t *testing.T) {
		t.Setenv(envHTTPAPIKeys, "")

		_, err := NewHTTPAPIConfig()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires API keys")
	})

	t.Run("given only separators as keys then return error", func(t *testing.T) {
		t.Setenv(envHTTPAPIKeys, " , ")

		_, err := NewHTTPAPIConfig()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one key")
	})

	t.Run("given an invalid body size then return error", func(t *testing.T) {
		t.Setenv(envHTTPAPIKeys, "token")
		t.Setenv(envHTTPAPIMaxBodyBytes, "-1")

		_, err := NewHTTPAPIConfig()

		require.EqualError(t, err, `HTTP_API_MAX_BODY_BYTES must be a positive integer, got "-1"`)
	})
}
