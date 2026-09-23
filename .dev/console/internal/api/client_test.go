package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
)

func TestClientSend(t *testing.T) {
	t.Run("given a delivery event response then decode it with the status", func(t *testing.T) {
		var auth, path string
		var received message.Email
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth, path = r.Header.Get("Authorization"), r.URL.Path
			_ = json.NewDecoder(r.Body).Decode(&received)
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"id":"1","subject":"Oi","status":"failed","errorCode":"api_invalid_receiver","cause":"invalid to"}`))
		}))
		defer server.Close()

		response, err := NewClient(server.URL, "dev-token").Send(context.Background(), message.Email{ID: "1", Subject: "Oi"})

		require.NoError(t, err)
		assert.Equal(t, "Bearer dev-token", auth)
		assert.Equal(t, "/v1/emails", path)
		assert.Equal(t, "1", received.ID)
		assert.Equal(t, http.StatusUnprocessableEntity, response.StatusCode)
		assert.Equal(t, "api_invalid_receiver", response.Event.ErrorCode)
		assert.Empty(t, response.Raw)
	})

	t.Run("given a non-event response then keep the raw body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"missing or invalid bearer token"}`))
		}))
		defer server.Close()

		response, err := NewClient(server.URL, "wrong").Send(context.Background(), message.Email{})

		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, response.StatusCode)
		assert.Contains(t, response.Raw, "invalid bearer token")
	})

	t.Run("given an unreachable api then return error", func(t *testing.T) {
		_, err := NewClient("http://127.0.0.1:1", "t").Send(context.Background(), message.Email{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "call mailer HTTP API")
	})
}
