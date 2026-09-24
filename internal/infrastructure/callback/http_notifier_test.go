package callback

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/core/model"
)

func newTestEvent() model.DeliveryEvent {
	return model.DeliveryEvent{
		ID:         "order-123",
		Subject:    "Subject",
		Status:     model.StatusFailed,
		ErrorCode:  model.CodeSMTPConnectionFailed,
		Cause:      "smtp down",
		OccurredAt: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC),
	}
}

func TestHTTPNotifierNotify(t *testing.T) {
	t.Run("given a 2xx callback then post the failure as json with headers", func(t *testing.T) {
		var received model.DeliveryEvent
		var method, contentType, auth string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			method, contentType, auth = r.Method, r.Header.Get("Content-Type"), r.Header.Get("Authorization")
			_ = json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		err := NewHTTPNotifier(server.Client()).Notify(
			model.CallbackTarget{URL: server.URL, Headers: map[string]string{"Authorization": "Bearer token"}},
			newTestEvent(),
		)

		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, method)
		assert.Equal(t, "application/json", contentType)
		assert.Equal(t, "Bearer token", auth)
		assert.Equal(t, newTestEvent(), received)
	})

	t.Run("must send only id, subject, status, errorCode, cause and occurredAt", func(t *testing.T) {
		var raw map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&raw)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer server.Close()

		require.NoError(t, NewHTTPNotifier(server.Client()).Notify(model.CallbackTarget{URL: server.URL}, newTestEvent()))

		assert.Equal(t, map[string]any{
			"id":         "order-123",
			"subject":    "Subject",
			"status":     "failed",
			"errorCode":  "smtp_connection_failed",
			"cause":      "smtp down",
			"occurredAt": "2026-09-23T12:00:00Z",
		}, raw)
	})

	t.Run("given a non-2xx callback then return error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
		defer server.Close()

		err := NewHTTPNotifier(server.Client()).Notify(model.CallbackTarget{URL: server.URL}, newTestEvent())

		require.EqualError(t, err, "callback returned status 502")
	})

	t.Run("given an invalid url then return error", func(t *testing.T) {
		err := NewHTTPNotifier(http.DefaultClient).Notify(model.CallbackTarget{URL: "://bad"}, newTestEvent())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create callback request")
	})

	t.Run("given an unreachable callback then return error", func(t *testing.T) {
		server := httptest.NewServer(http.NotFoundHandler())
		url := server.URL
		server.Close()

		err := NewHTTPNotifier(http.DefaultClient).Notify(model.CallbackTarget{URL: url}, newTestEvent())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to call callback")
	})
}
