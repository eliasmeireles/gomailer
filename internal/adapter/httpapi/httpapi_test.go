package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/application/config"
	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

const validBody = `{"id":"pedido-1","from":"no-reply@exemplo.com.br","receiver":["maria@exemplo.com.br"],"subject":"Oi","body":"PHA+T2k8L3A+"}`

type fakeDeliverer struct {
	received model.SendEmailData
	calls    int
	outcome  mailer.Outcome
}

func (f *fakeDeliverer) DeliverOnce(data model.SendEmailData) mailer.Outcome {
	f.received = data
	f.calls++
	outcome := f.outcome
	outcome.Event.ID = data.ID
	return outcome
}

func newTestServer(deliverer *fakeDeliverer, maxBody int64) *Server {
	cfg := config.HTTPAPIConfig{Port: "0", Keys: []string{"token-a", "token-b"}, MaxBodyBytes: maxBody}
	return NewServer(cfg, deliverer, func() string { return "id-gerado" })
}

func post(server *Server, auth, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/emails", strings.NewReader(body))
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	return recorder
}

func decodeEvent(t *testing.T, recorder *httptest.ResponseRecorder) model.DeliveryEvent {
	t.Helper()
	var event model.DeliveryEvent
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &event))
	return event
}

func TestEmailsEndpoint(t *testing.T) {
	t.Run("given a valid request then deliver it and answer 200 with the sent event", func(t *testing.T) {
		deliverer := &fakeDeliverer{outcome: mailer.Outcome{Event: model.DeliveryEvent{Subject: "Oi", Status: model.StatusSent}}}

		response := post(newTestServer(deliverer, 1<<20), "Bearer token-b", validBody)

		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "application/json", response.Header().Get("Content-Type"))
		assert.Equal(t, model.DeliveryEvent{ID: "pedido-1", Subject: "Oi", Status: model.StatusSent}, decodeEvent(t, response))
		assert.Equal(t, model.Recipients{"maria@exemplo.com.br"}, deliverer.received.Receiver)
	})

	t.Run("given a request without id then generate one", func(t *testing.T) {
		deliverer := &fakeDeliverer{outcome: mailer.Outcome{}}

		response := post(newTestServer(deliverer, 1<<20), "Bearer token-a", `{"from":"a@exemplo.com.br","receiver":"b@exemplo.com.br"}`)

		assert.Equal(t, "id-gerado", deliverer.received.ID)
		assert.Equal(t, "id-gerado", decodeEvent(t, response).ID)
	})

	t.Run("given a delivery failure then answer with the mapped status and the failed event", func(t *testing.T) {
		deliverer := &fakeDeliverer{outcome: mailer.Outcome{
			Err:   errors.New("invalid to"),
			Event: model.DeliveryEvent{Status: model.StatusFailed, ErrorCode: model.CodeAPIInvalidReceiver, Cause: "invalid to"},
		}}

		response := post(newTestServer(deliverer, 1<<20), "Bearer token-a", validBody)

		assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
		event := decodeEvent(t, response)
		assert.Equal(t, model.CodeAPIInvalidReceiver, event.ErrorCode)
		assert.Equal(t, "invalid to", event.Cause)
	})

	t.Run("given no token then answer 401 without delivering", func(t *testing.T) {
		deliverer := &fakeDeliverer{}

		response := post(newTestServer(deliverer, 1<<20), "", validBody)

		assert.Equal(t, http.StatusUnauthorized, response.Code)
		assert.Equal(t, `Bearer realm="gomailer"`, response.Header().Get("WWW-Authenticate"))
		assert.Zero(t, deliverer.calls)
	})

	t.Run("given a wrong token or scheme then answer 401", func(t *testing.T) {
		server := newTestServer(&fakeDeliverer{}, 1<<20)

		assert.Equal(t, http.StatusUnauthorized, post(server, "Bearer token-c", validBody).Code)
		assert.Equal(t, http.StatusUnauthorized, post(server, "Basic token-a", validBody).Code)
		assert.Equal(t, http.StatusUnauthorized, post(server, "Bearer token-a-extra", validBody).Code)
	})

	t.Run("given malformed json then answer 400", func(t *testing.T) {
		response := post(newTestServer(&fakeDeliverer{}, 1<<20), "Bearer token-a", "{nope")

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "invalid JSON body")
	})

	t.Run("given a body over the limit then answer 413", func(t *testing.T) {
		response := post(newTestServer(&fakeDeliverer{}, 16), "Bearer token-a", validBody)

		assert.Equal(t, http.StatusRequestEntityTooLarge, response.Code)
	})

	t.Run("given another method then answer 405", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/emails", nil)
		recorder := httptest.NewRecorder()

		newTestServer(&fakeDeliverer{}, 1<<20).Handler().ServeHTTP(recorder, req)

		assert.Equal(t, http.StatusMethodNotAllowed, recorder.Code)
	})
}

func TestStatusFor(t *testing.T) {
	failed := func(code model.ErrorCode) mailer.Outcome {
		return mailer.Outcome{Err: errors.New("x"), Event: model.DeliveryEvent{ErrorCode: code}}
	}
	cases := map[string]struct {
		outcome  mailer.Outcome
		expected int
	}{
		"sent":                      {mailer.Outcome{}, http.StatusOK},
		"message_invalid_body":      {failed(model.CodeMessageInvalidBody), http.StatusBadRequest},
		"message_missing_receiver":  {failed(model.CodeMessageMissingReceiver), http.StatusBadRequest},
		"api_invalid_receiver":      {failed(model.CodeAPIInvalidReceiver), http.StatusUnprocessableEntity},
		"smtp_receiver_rejected":    {failed(model.CodeSMTPReceiverRejected), http.StatusUnprocessableEntity},
		"api_quota_exceeded":        {failed(model.CodeAPIQuotaExceeded), http.StatusUnprocessableEntity},
		"api_rate_limited":          {failed(model.CodeAPIRateLimited), http.StatusTooManyRequests},
		"api_connection_failed":     {failed(model.CodeAPIConnectionFailed), http.StatusServiceUnavailable},
		"smtp_connection_failed":    {failed(model.CodeSMTPConnectionFailed), http.StatusServiceUnavailable},
		"api_provider_unavailable":  {failed(model.CodeAPIProviderUnavailable), http.StatusServiceUnavailable},
		"api_authorization_denied":  {failed(model.CodeAPIAuthorizationDenied), http.StatusBadGateway},
		"smtp_authorization_denied": {failed(model.CodeSMTPAuthorizationDenied), http.StatusBadGateway},
		"unknown_error":             {failed(model.CodeUnknown), http.StatusBadGateway},
	}

	for name, tc := range cases {
		t.Run("given "+name+" then map the status", func(t *testing.T) {
			assert.Equal(t, tc.expected, statusFor(tc.outcome))
		})
	}
}

func TestServerRun(t *testing.T) {
	t.Run("given a free port then serve until the context is cancelled", func(t *testing.T) {
		server := newTestServer(&fakeDeliverer{}, 1<<20)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)

		go func() { done <- server.Run(ctx) }()
		require.Eventually(t, server.Ready, 2*time.Second, 10*time.Millisecond)
		cancel()

		require.NoError(t, <-done)
		assert.False(t, server.Ready())
		assert.Equal(t, "http", server.Name())
	})

	t.Run("given a port in use then return an error", func(t *testing.T) {
		listener, err := net.Listen("tcp", ":0")
		require.NoError(t, err)
		defer listener.Close()
		port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
		server := NewServer(config.HTTPAPIConfig{Port: port, Keys: []string{"k"}, MaxBodyBytes: 1}, &fakeDeliverer{}, model.NewID)

		err = server.Run(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "listen on :"+port)
	})
}
