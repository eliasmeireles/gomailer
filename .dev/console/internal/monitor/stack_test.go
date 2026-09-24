package monitor

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capture struct {
	method string
	uri    string
	user   string
	body   map[string]any
}

func newCapturingServer(t *testing.T, status int, response string, captured *capture) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method, captured.uri = r.Method, r.URL.RequestURI()
		captured.user, _, _ = r.BasicAuth()
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &captured.body)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestRabbitMQ(t *testing.T) {
	t.Run("given the queue family then aggregate main, retry and dead-letter queues", func(t *testing.T) {
		captured := &capture{}
		server := newCapturingServer(t, http.StatusOK, `[
			{"name":"mailer-service","messages":2,"consumers":1},
			{"name":"mailer-service.retry.5s","messages":3,"consumers":0},
			{"name":"mailer-service.retry.10s","messages":1,"consumers":0},
			{"name":"mailer-service.dlq","messages":4,"consumers":0},
			{"name":"other","messages":9,"consumers":0}]`, captured)

		stats, err := NewRabbitMQ(server.URL, "guest", "guest", "mailer-service").Stats(context.Background())

		require.NoError(t, err)
		assert.Equal(t, QueueStats{Messages: 2, Consumers: 1, Retrying: 4, DeadLettered: 4}, stats)
		assert.Equal(t, "/api/queues/%2F?columns=name,messages,consumers", captured.uri)
		assert.Equal(t, "guest", captured.user)
	})

	t.Run("given dead letters then peek them with their failure headers", func(t *testing.T) {
		captured := &capture{}
		server := newCapturingServer(t, http.StatusOK, `[{"payload":"{}","properties":{"message_id":"m1",
			"headers":{"x-attempt":3,"x-error-code":"api_rate_limited","x-error-cause":"too many","x-failed-at":"2026-09-23T12:00:00Z"}}}]`, captured)

		letters, err := NewRabbitMQ(server.URL, "guest", "guest", "mailer-service").PeekDeadLetters(context.Background())

		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, captured.method)
		assert.Equal(t, "/api/queues/%2F/mailer-service.dlq/get", captured.uri)
		assert.Equal(t, "ack_requeue_true", captured.body["ackmode"])
		assert.Equal(t, []DeadLetter{{
			Source: "rabbitmq", MessageID: "m1", Attempt: 3, ErrorCode: "api_rate_limited", Cause: "too many",
			FailedAt: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC), Payload: "{}",
		}}, letters)
	})

	t.Run("given purge then delete the dead-letter queue contents", func(t *testing.T) {
		captured := &capture{}
		server := newCapturingServer(t, http.StatusNoContent, "", captured)

		require.NoError(t, NewRabbitMQ(server.URL, "guest", "guest", "mailer-service").PurgeDeadLetters(context.Background()))
		assert.Equal(t, http.MethodDelete, captured.method)
		assert.Equal(t, "/api/queues/%2F/mailer-service.dlq/contents", captured.uri)
	})

	t.Run("given an error status then return error", func(t *testing.T) {
		server := newCapturingServer(t, http.StatusUnauthorized, "", &capture{})

		_, err := NewRabbitMQ(server.URL, "x", "y", "q").Stats(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "status 401")
	})
}

func TestMailpitChaos(t *testing.T) {
	t.Run("given chaos triggers then read them", func(t *testing.T) {
		captured := &capture{}
		server := newCapturingServer(t, http.StatusOK, `{"Sender":{"ErrorCode":451,"Probability":100},"Recipient":{"ErrorCode":451,"Probability":0},"Authentication":{"ErrorCode":535,"Probability":0}}`, captured)

		triggers, err := NewMailpit(server.URL).Chaos(context.Background())

		require.NoError(t, err)
		assert.Equal(t, "/api/v1/chaos", captured.uri)
		assert.Equal(t, ChaosTrigger{ErrorCode: 451, Probability: 100}, triggers.Sender)
	})

	t.Run("given new triggers then put them", func(t *testing.T) {
		captured := &capture{}
		server := newCapturingServer(t, http.StatusOK, `{"Sender":{"ErrorCode":550,"Probability":100}}`, captured)

		updated, err := NewMailpit(server.URL).SetChaos(context.Background(), ChaosTriggers{Sender: ChaosTrigger{ErrorCode: 550, Probability: 100}})

		require.NoError(t, err)
		assert.Equal(t, http.MethodPut, captured.method)
		assert.Equal(t, map[string]any{"ErrorCode": float64(550), "Probability": float64(100)}, captured.body["Sender"])
		assert.Equal(t, 550, updated.Sender.ErrorCode)
	})
}

func TestMockAPI(t *testing.T) {
	t.Run("given a state then read it", func(t *testing.T) {
		server := newCapturingServer(t, http.StatusOK, `{"failNext":2,"status":429,"name":"rate_limit_exceeded","message":"x","accepted":1,"rejected":3,
			"emails":[{"id":"e1","subject":"Hi","to":["jane@example.com"]}]}`, &capture{})

		state, err := NewMockAPI(server.URL).State(context.Background())

		require.NoError(t, err)
		assert.Equal(t, 2, state.FailNext)
		assert.Equal(t, 3, state.Rejected)
		assert.Equal(t, []string{"jane@example.com"}, state.Emails[0].To)
	})

	t.Run("given a behavior then put it", func(t *testing.T) {
		captured := &capture{}
		server := newCapturingServer(t, http.StatusOK, `{"failNext":1,"status":503}`, captured)

		state, err := NewMockAPI(server.URL).Configure(context.Background(), MockBehavior{FailNext: 1, Status: 503, Name: "service_unavailable"})

		require.NoError(t, err)
		assert.Equal(t, http.MethodPut, captured.method)
		assert.Equal(t, "/state", captured.uri)
		assert.Equal(t, float64(1), captured.body["failNext"])
		assert.Equal(t, 503, state.Status)
	})

	t.Run("given reset then delete the state", func(t *testing.T) {
		captured := &capture{}
		server := newCapturingServer(t, http.StatusOK, `{}`, captured)

		require.NoError(t, NewMockAPI(server.URL).Reset(context.Background()))
		assert.Equal(t, http.MethodDelete, captured.method)
	})

	t.Run("given an unreachable api then return error", func(t *testing.T) {
		_, err := NewMockAPI("http://127.0.0.1:1").State(context.Background())

		require.Error(t, err)
	})
}
