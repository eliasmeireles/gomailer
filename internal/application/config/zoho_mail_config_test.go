package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setZohoMailEnv(t *testing.T) {
	t.Setenv(envZohoMailAccountID, "123456")
	t.Setenv(envZohoClientID, "client-id")
	t.Setenv(envZohoClientSecret, "client-secret")
	t.Setenv(envZohoRefreshToken, "refresh-token")
	t.Setenv(envZohoMailAPIURL, "")
	t.Setenv(envZohoAccountsURL, "")
}

func TestNewZohoMailConfig(t *testing.T) {
	t.Run("given required env vars then use the default data center", func(t *testing.T) {
		setZohoMailEnv(t)

		cfg, err := NewZohoMailConfig()

		require.NoError(t, err)
		assert.Equal(t, ZohoMailConfig{
			AccountID:    "123456",
			ClientID:     "client-id",
			ClientSecret: "client-secret",
			RefreshToken: "refresh-token",
			APIURL:       defaultZohoMailAPIURL,
			AccountsURL:  defaultZohoAccountURL,
		}, cfg)
	})

	t.Run("given custom data center urls then trim trailing slashes", func(t *testing.T) {
		setZohoMailEnv(t)
		t.Setenv(envZohoMailAPIURL, "https://mail.zoho.eu/")
		t.Setenv(envZohoAccountsURL, "https://accounts.zoho.eu/")

		cfg, err := NewZohoMailConfig()

		require.NoError(t, err)
		assert.Equal(t, "https://mail.zoho.eu", cfg.APIURL)
		assert.Equal(t, "https://accounts.zoho.eu", cfg.AccountsURL)
	})

	t.Run("given a missing refresh token then return error", func(t *testing.T) {
		setZohoMailEnv(t)
		t.Setenv(envZohoRefreshToken, "")

		_, err := NewZohoMailConfig()

		require.Error(t, err)
		assert.Contains(t, err.Error(), envZohoRefreshToken)
	})
}
