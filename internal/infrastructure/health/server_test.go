package health

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubChecker struct {
	ready bool
}

func (s *stubChecker) Ready() bool {
	return s.ready
}

func TestHealthServerHandler(t *testing.T) {
	t.Run("given any request must return 200 ok on healthz", func(t *testing.T) {
		srv := NewServer(&stubChecker{ready: false})
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		body, err := io.ReadAll(rec.Body)
		require.NoError(t, err)
		assert.Equal(t, "ok\n", string(body))
	})

	t.Run("given checker is ready then readyz returns 200", func(t *testing.T) {
		srv := NewServer(&stubChecker{ready: true})
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		body, err := io.ReadAll(rec.Body)
		require.NoError(t, err)
		assert.Equal(t, "ready\n", string(body))
	})

	t.Run("given checker is not ready then readyz returns 503", func(t *testing.T) {
		srv := NewServer(&stubChecker{ready: false})
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		require.Equal(t, http.StatusServiceUnavailable, rec.Code)
		body, err := io.ReadAll(rec.Body)
		require.NoError(t, err)
		assert.Equal(t, "not ready\n", string(body))
	})

	t.Run("when unknown path requested then return 404", func(t *testing.T) {
		srv := NewServer(&stubChecker{ready: true})
		req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
		rec := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestNewServerHonorsPortEnvVar(t *testing.T) {
	t.Run("given HEALTH_PORT set then bind to that port", func(t *testing.T) {
		t.Setenv(envHealthPort, "9999")
		srv := NewServer(&stubChecker{})
		assert.Equal(t, ":9999", srv.addr)
	})

	t.Run("given HEALTH_PORT unset then use default", func(t *testing.T) {
		t.Setenv(envHealthPort, "")
		srv := NewServer(&stubChecker{})
		assert.Equal(t, ":"+defaultHealthPort, srv.addr)
	})
}
