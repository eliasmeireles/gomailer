package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewZeptoMailConfig(t *testing.T) {
	t.Run("given a bare key then add the auth prefix and default url", func(t *testing.T) {
		t.Setenv(envZeptoMailAPIKey, "wSsVR6-token")
		t.Setenv(envZeptoMailAPIURL, "")

		cfg, err := NewZeptoMailConfig()

		require.NoError(t, err)
		assert.Equal(t, ZeptoMailConfig{Authorization: "Zoho-enczapikey wSsVR6-token", BaseURL: defaultZeptoMailAPIURL}, cfg)
	})

	t.Run("given a prefixed key then keep a single prefix", func(t *testing.T) {
		t.Setenv(envZeptoMailAPIKey, "Zoho-enczapikey wSsVR6-token")
		t.Setenv(envZeptoMailAPIURL, "https://api.zeptomail.eu/")

		cfg, err := NewZeptoMailConfig()

		require.NoError(t, err)
		assert.Equal(t, "Zoho-enczapikey wSsVR6-token", cfg.Authorization)
		assert.Equal(t, "https://api.zeptomail.eu", cfg.BaseURL)
	})

	t.Run("given no key then return error", func(t *testing.T) {
		t.Setenv(envZeptoMailAPIKey, "")

		_, err := NewZeptoMailConfig()

		require.Error(t, err)
		assert.Contains(t, err.Error(), envZeptoMailAPIKey)
	})
}
