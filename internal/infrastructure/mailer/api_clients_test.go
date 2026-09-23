package mailer

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAPISender(t *testing.T) {
	t.Run("given resend with api key then return the resend sender", func(t *testing.T) {
		t.Setenv("RESEND_API_KEY", "re_test")

		sender, err := NewAPISender(APIClientResend, http.DefaultClient)

		require.NoError(t, err)
		assert.IsType(t, &resendSender{}, sender)
	})

	t.Run("given resend without api key then return the config error", func(t *testing.T) {
		t.Setenv("RESEND_API_KEY", "")

		_, err := NewAPISender(APIClientResend, http.DefaultClient)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "RESEND_API_KEY")
	})

	t.Run("given zoho with full config then return the zoho mail sender", func(t *testing.T) {
		t.Setenv("ZOHO_MAIL_ACCOUNT_ID", "123456")
		t.Setenv("ZOHO_CLIENT_ID", "client-id")
		t.Setenv("ZOHO_CLIENT_SECRET", "client-secret")
		t.Setenv("ZOHO_REFRESH_TOKEN", "refresh-token")

		sender, err := NewAPISender(APIClientZoho, http.DefaultClient)

		require.NoError(t, err)
		assert.IsType(t, &zohoMailSender{}, sender)
	})

	t.Run("given zoho without config then return the config error", func(t *testing.T) {
		t.Setenv("ZOHO_MAIL_ACCOUNT_ID", "")

		_, err := NewAPISender(APIClientZoho, http.DefaultClient)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "ZOHO_MAIL_ACCOUNT_ID")
	})

	t.Run("given zeptomail with api key then return the zeptomail sender", func(t *testing.T) {
		t.Setenv("ZEPTOMAIL_API_KEY", "wSsVR6-token")

		sender, err := NewAPISender(APIClientZeptoMail, http.DefaultClient)

		require.NoError(t, err)
		assert.IsType(t, &zeptoMailSender{}, sender)
	})

	t.Run("given zeptomail without api key then return the config error", func(t *testing.T) {
		t.Setenv("ZEPTOMAIL_API_KEY", "")

		_, err := NewAPISender(APIClientZeptoMail, http.DefaultClient)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "ZEPTOMAIL_API_KEY")
	})

	t.Run("given an unknown client then list the available ones", func(t *testing.T) {
		_, err := NewAPISender("pigeon", http.DefaultClient)

		require.EqualError(t, err, `unknown mailer API client "pigeon", available: resend, zeptomail, zoho`)
	})
}
