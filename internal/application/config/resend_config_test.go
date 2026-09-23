package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResendConfig(t *testing.T) {
	t.Run("given only the api key then use the default url", func(t *testing.T) {
		t.Setenv(envResendAPIKey, "re_test")
		t.Setenv(envResendAPIURL, "")

		cfg, err := NewResendConfig()

		require.NoError(t, err)
		assert.Equal(t, ResendConfig{APIKey: "re_test", BaseURL: defaultResendAPIURL}, cfg)
	})

	t.Run("given a custom url with trailing slash then trim it", func(t *testing.T) {
		t.Setenv(envResendAPIKey, "re_test")
		t.Setenv(envResendAPIURL, "http://localhost:9000/")

		cfg, err := NewResendConfig()

		require.NoError(t, err)
		assert.Equal(t, "http://localhost:9000", cfg.BaseURL)
	})

	t.Run("given no api key then return error", func(t *testing.T) {
		t.Setenv(envResendAPIKey, "")

		_, err := NewResendConfig()

		require.Error(t, err)
		assert.Contains(t, err.Error(), envResendAPIKey)
	})
}
