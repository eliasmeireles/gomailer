package mailer

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/application/config"
	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

func newTestZohoConfig(accountsURL, apiURL string) config.ZohoMailConfig {
	return config.ZohoMailConfig{
		AccountID:    "123456",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		RefreshToken: "refresh-token",
		APIURL:       apiURL,
		AccountsURL:  accountsURL,
	}
}

type tokenServer struct {
	server *httptest.Server
	calls  atomic.Int32
	query  url.Values
}

func newTokenServer(t *testing.T, status int, response string) *tokenServer {
	t.Helper()
	ts := &tokenServer{}
	ts.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ts.calls.Add(1)
		ts.query = r.URL.Query()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(ts.server.Close)
	return ts
}

func TestZohoTokenSourceToken(t *testing.T) {
	t.Run("given a valid refresh token then request and cache the access token", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusOK, `{"access_token":"access-1","expires_in":3600}`)
		source := newZohoTokenSource(newTestZohoConfig(ts.server.URL, ""), ts.server.Client())

		first, err := source.Token()
		require.NoError(t, err)
		second, err := source.Token()
		require.NoError(t, err)

		assert.Equal(t, "access-1", first)
		assert.Equal(t, "access-1", second)
		assert.Equal(t, int32(1), ts.calls.Load())
		assert.Equal(t, "refresh-token", ts.query.Get("refresh_token"))
		assert.Equal(t, "client-id", ts.query.Get("client_id"))
		assert.Equal(t, "client-secret", ts.query.Get("client_secret"))
		assert.Equal(t, "refresh_token", ts.query.Get("grant_type"))
	})

	t.Run("given an expired cached token then refresh it", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusOK, `{"access_token":"access-1","expires_in":3600}`)
		source := newZohoTokenSource(newTestZohoConfig(ts.server.URL, ""), ts.server.Client())
		now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
		source.now = func() time.Time { return now }

		_, err := source.Token()
		require.NoError(t, err)
		now = now.Add(time.Hour - zohoTokenRefreshMargin)
		_, err = source.Token()
		require.NoError(t, err)

		assert.Equal(t, int32(2), ts.calls.Load())
	})

	t.Run("given an invalidated token then refresh it", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusOK, `{"access_token":"access-1","expires_in":3600}`)
		source := newZohoTokenSource(newTestZohoConfig(ts.server.URL, ""), ts.server.Client())

		_, err := source.Token()
		require.NoError(t, err)
		source.Invalidate()
		_, err = source.Token()
		require.NoError(t, err)

		assert.Equal(t, int32(2), ts.calls.Load())
	})

	t.Run("given an oauth error in a 200 response then return error", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusOK, `{"error":"invalid_code"}`)
		source := newZohoTokenSource(newTestZohoConfig(ts.server.URL, ""), ts.server.Client())

		_, err := source.Token()

		require.EqualError(t, err, `failed to refresh Zoho access token: "invalid_code"`)
	})

	t.Run("given a non-2xx response then return error", func(t *testing.T) {
		ts := newTokenServer(t, http.StatusBadRequest, "bad request")
		source := newZohoTokenSource(newTestZohoConfig(ts.server.URL, ""), ts.server.Client())

		_, err := source.Token()

		require.EqualError(t, err, "zoho accounts API returned status 400: bad request")
		assert.Equal(t, model.CodeAPIAuthorizationDenied, mailer.CodeOf(err))
	})

	t.Run("given an invalid accounts url then return error", func(t *testing.T) {
		source := newZohoTokenSource(newTestZohoConfig("://bad", ""), http.DefaultClient)

		_, err := source.Token()

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create Zoho token request")
	})
}
