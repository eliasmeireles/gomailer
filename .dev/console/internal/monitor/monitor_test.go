package monitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedRequest struct {
	method string
	uri    string
}

func newServer(t *testing.T, status int, body string, recorded *recordedRequest) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorded.method, recorded.uri = r.Method, r.URL.RequestURI()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestMailpit(t *testing.T) {
	t.Run("given messages then list them", func(t *testing.T) {
		recorded := &recordedRequest{}
		server := newServer(t, http.StatusOK, `{"messages":[{"ID":"m1","Created":"2026-09-23T12:00:00Z","Subject":"Hi",
			"From":{"Address":"no-reply@example.com"},"To":[{"Address":"jane@example.com"}],"Bcc":[{"Address":"anna@example.com"}],"Attachments":1}]}`, recorded)

		messages, err := NewMailpit(server.URL).List(context.Background())

		require.NoError(t, err)
		assert.Equal(t, "/api/v1/messages?limit=20", recorded.uri)
		assert.Equal(t, []InboxMessage{{
			ID:          "m1",
			Created:     time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC),
			Subject:     "Hi",
			From:        Address{Address: "no-reply@example.com"},
			To:          []Address{{Address: "jane@example.com"}},
			Bcc:         []Address{{Address: "anna@example.com"}},
			Attachments: 1,
		}}, messages)
	})

	t.Run("given clear then delete all messages", func(t *testing.T) {
		recorded := &recordedRequest{}
		server := newServer(t, http.StatusOK, "", recorded)

		require.NoError(t, NewMailpit(server.URL).Clear(context.Background()))
		assert.Equal(t, http.MethodDelete, recorded.method)
		assert.Equal(t, "/api/v1/messages", recorded.uri)
	})

	t.Run("given an error status then return error", func(t *testing.T) {
		server := newServer(t, http.StatusInternalServerError, "", &recordedRequest{})

		_, err := NewMailpit(server.URL).List(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "status 500")
	})

	t.Run("given an invalid body then return a decode error", func(t *testing.T) {
		server := newServer(t, http.StatusOK, "not json", &recordedRequest{})

		_, err := NewMailpit(server.URL).List(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "decode")
	})
}

func TestCallbacks(t *testing.T) {
	t.Run("given events then list them", func(t *testing.T) {
		recorded := &recordedRequest{}
		server := newServer(t, http.StatusOK, `[{"receivedAt":"2026-09-23T12:00:01Z","path":"/failures","authorization":"Bearer t","status":204,
			"body":{"id":"e1","subject":"Hi","status":"failed","errorCode":"smtp_connection_failed","cause":"falhou","occurredAt":"2026-09-23T12:00:00Z"}}]`, recorded)

		events, err := NewCallbacks(server.URL).List(context.Background())

		require.NoError(t, err)
		assert.Equal(t, "/events", recorded.uri)
		assert.Equal(t, []CallbackEvent{{
			ReceivedAt:    time.Date(2026, 9, 23, 12, 0, 1, 0, time.UTC),
			Path:          "/failures",
			Authorization: "Bearer t",
			Status:        204,
			Body:          DeliveryEvent{ID: "e1", Subject: "Hi", Status: "failed", ErrorCode: "smtp_connection_failed", Cause: "falhou", OccurredAt: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)},
		}}, events)
	})

	t.Run("given clear then delete events", func(t *testing.T) {
		recorded := &recordedRequest{}
		server := newServer(t, http.StatusNoContent, "", recorded)

		require.NoError(t, NewCallbacks(server.URL).Clear(context.Background()))
		assert.Equal(t, http.MethodDelete, recorded.method)
		assert.Equal(t, "/events", recorded.uri)
	})
}

func TestHealth(t *testing.T) {
	t.Run("given a 200 readiness then report ready", func(t *testing.T) {
		server := newServer(t, http.StatusOK, "ready", &recordedRequest{})

		assert.True(t, NewHealth(server.URL).Ready(context.Background()))
	})

	t.Run("given a 503 readiness then report not ready", func(t *testing.T) {
		server := newServer(t, http.StatusServiceUnavailable, "", &recordedRequest{})

		assert.False(t, NewHealth(server.URL).Ready(context.Background()))
	})

	t.Run("given an invalid url then report not ready", func(t *testing.T) {
		assert.False(t, NewHealth("://bad").Ready(context.Background()))
	})
}
