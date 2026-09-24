package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recorded struct {
	auth string
	path string
	body map[string]any
}

func newAPIServer(t *testing.T, status int, response string, rec *recorded) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.auth, rec.path = r.Header.Get("Authorization"), r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&rec.body)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)
	return server
}

func newTestHTTPSender(t *testing.T, url string) *HTTPSender {
	t.Helper()
	sender, err := NewHTTPSender(HTTPConfig{URL: url + "/", Token: "token"})
	require.NoError(t, err)
	return sender
}

func TestHTTPSender(t *testing.T) {
	t.Run("given a sent answer then return the event", func(t *testing.T) {
		rec := &recorded{}
		server := newAPIServer(t, http.StatusOK, `{"id":"pedido-1","subject":"Oi","status":"sent","occurredAt":"2026-09-23T12:00:00Z"}`, rec)
		email := validEmail()
		email.ID = "pedido-1"

		event, err := newTestHTTPSender(t, server.URL).Deliver(context.Background(), email)

		require.NoError(t, err)
		assert.Equal(t, StatusSent, event.Status)
		assert.Equal(t, "Bearer token", rec.auth)
		assert.Equal(t, "/v1/emails", rec.path)
		assert.Equal(t, "pedido-1", rec.body["id"])
	})

	t.Run("given a failed answer then return a DeliveryError with the event", func(t *testing.T) {
		server := newAPIServer(t, http.StatusUnprocessableEntity, `{"id":"1","status":"failed","errorCode":"api_invalid_receiver","cause":"invalid to"}`, &recorded{})

		err := newTestHTTPSender(t, server.URL).Send(context.Background(), validEmail())

		var deliveryErr *DeliveryError
		require.ErrorAs(t, err, &deliveryErr)
		assert.Equal(t, http.StatusUnprocessableEntity, deliveryErr.StatusCode)
		assert.Equal(t, CodeAPIInvalidReceiver, deliveryErr.Event.ErrorCode)
		assert.Contains(t, err.Error(), "invalid to")
	})

	t.Run("given a non-event answer then return the raw body", func(t *testing.T) {
		server := newAPIServer(t, http.StatusUnauthorized, `{"error":"missing or invalid bearer token"}`, &recorded{})

		err := newTestHTTPSender(t, server.URL).Send(context.Background(), validEmail())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "401")
		assert.Contains(t, err.Error(), "invalid bearer token")
	})

	t.Run("given an invalid email then fail without calling the api", func(t *testing.T) {
		rec := &recorded{}
		server := newAPIServer(t, http.StatusOK, `{}`, rec)

		err := newTestHTTPSender(t, server.URL).Send(context.Background(), Email{})

		require.ErrorIs(t, err, ErrInvalidEmail)
		assert.Empty(t, rec.path)
	})

	t.Run("given an unreachable api then return error", func(t *testing.T) {
		sender, err := NewHTTPSender(HTTPConfig{URL: "http://127.0.0.1:1", Token: "t", Timeout: time.Second})
		require.NoError(t, err)

		err = sender.Send(context.Background(), validEmail())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "call gomailer")
		require.NoError(t, sender.HealthCheck(context.Background()))
		require.NoError(t, sender.Close())
	})

	t.Run("given missing url or token then refuse to build", func(t *testing.T) {
		_, err := NewHTTPSender(HTTPConfig{URL: "http://gomailer:8081"})

		require.Error(t, err)
	})
}

func TestParseDeliveryEvent(t *testing.T) {
	t.Run("given a callback body then decode the event", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/falhas", strings.NewReader(`{"id":"1","subject":"Oi","status":"failed","errorCode":"smtp_temporary_failure","cause":"451"}`))

		event, err := ParseDeliveryEvent(req)

		require.NoError(t, err)
		assert.Equal(t, DeliveryEvent{ID: "1", Subject: "Oi", Status: StatusFailed, ErrorCode: CodeSMTPTemporaryFailure, Cause: "451"}, event)
	})

	t.Run("given an invalid body then return error", func(t *testing.T) {
		_, err := ParseDeliveryEvent(httptest.NewRequest(http.MethodPost, "/", strings.NewReader("nope")))

		require.Error(t, err)
	})
}
