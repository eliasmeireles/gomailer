package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setSMTPEnv(t *testing.T, port string) {
	t.Setenv(envSMTPServer, "smtp.exemplo.com.br")
	t.Setenv(envSMTPServerPort, port)
	t.Setenv(envSMTPServerUser, "usuario")
	t.Setenv(envSMTPServerPass, "senha")
}

func TestNewSMTPConfig(t *testing.T) {
	t.Run("given all env vars then return the config", func(t *testing.T) {
		setSMTPEnv(t, "465")

		cfg, err := NewSMTPConfig()

		require.NoError(t, err)
		assert.Equal(t, SMTPConfig{Host: "smtp.exemplo.com.br", Port: 465, Username: "usuario", Password: "senha"}, cfg)
	})

	t.Run("given a missing env var then return error", func(t *testing.T) {
		setSMTPEnv(t, "465")
		t.Setenv(envSMTPServerPass, "")

		_, err := NewSMTPConfig()

		require.Error(t, err)
		assert.Contains(t, err.Error(), envSMTPServerPass)
	})

	t.Run("given a non-numeric port then return error", func(t *testing.T) {
		setSMTPEnv(t, "abc")

		_, err := NewSMTPConfig()

		require.Error(t, err)
		assert.Contains(t, err.Error(), envSMTPServerPort)
	})
}
