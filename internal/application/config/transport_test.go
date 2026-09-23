package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMailerSettings(t *testing.T) {
	t.Run("given no transport then default to smtp", func(t *testing.T) {
		t.Setenv(envMailerTransport, "")
		t.Setenv(envMailerAPIClient, "resend")

		settings, err := NewMailerSettings()

		require.NoError(t, err)
		assert.Equal(t, MailerSettings{Transport: TransportSMTP}, settings)
	})

	t.Run("given api transport with client then return lower-cased client", func(t *testing.T) {
		t.Setenv(envMailerTransport, "API")
		t.Setenv(envMailerAPIClient, "Resend")

		settings, err := NewMailerSettings()

		require.NoError(t, err)
		assert.Equal(t, MailerSettings{Transport: TransportAPI, APIClient: "resend"}, settings)
	})

	t.Run("given api transport without client then return error", func(t *testing.T) {
		t.Setenv(envMailerTransport, "api")
		t.Setenv(envMailerAPIClient, "")

		_, err := NewMailerSettings()

		require.Error(t, err)
		assert.Contains(t, err.Error(), envMailerAPIClient)
	})

	t.Run("given an unknown transport then return error", func(t *testing.T) {
		t.Setenv(envMailerTransport, "pigeon")

		_, err := NewMailerSettings()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "pigeon")
	})
}
