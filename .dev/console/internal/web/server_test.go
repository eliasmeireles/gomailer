package web

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/dev/console/internal/api"
	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
	"github.com/eliasmeireles/gomailer/dev/console/internal/monitor"
	"github.com/eliasmeireles/gomailer/dev/console/internal/service"
)

type fakeConsole struct {
	sentForm      message.Form
	sendErr       error
	status        service.Status
	messages      []monitor.InboxMessage
	events        []monitor.CallbackEvent
	listErr       error
	inboxCleared  bool
	eventsCleared bool
	response      *api.Response
	letters       []monitor.DeadLetter
	purged        bool
	chaos         service.ChaosView
	chaosReset    bool
}

func (f *fakeConsole) Send(_ context.Context, form message.Form) (service.SendResult, error) {
	f.sentForm = form
	if f.sendErr != nil {
		return service.SendResult{}, f.sendErr
	}
	email, err := message.Build(form, func() string { return "generated-id" })
	result := service.SendResult{Email: email, Channel: message.ChannelRabbitMQ}
	if form.Channel == message.ChannelHTTP {
		result.Channel = message.ChannelHTTP
		result.Response = f.response
	}
	return result, err
}

func (f *fakeConsole) Status(context.Context) service.Status { return f.status }

func (f *fakeConsole) Inbox(context.Context) ([]monitor.InboxMessage, error) {
	return f.messages, f.listErr
}

func (f *fakeConsole) ClearInbox(context.Context) error { f.inboxCleared = true; return nil }

func (f *fakeConsole) Callbacks(context.Context) ([]monitor.CallbackEvent, error) {
	return f.events, f.listErr
}

func (f *fakeConsole) ClearCallbacks(context.Context) error { f.eventsCleared = true; return nil }

func (f *fakeConsole) DeadLetters(context.Context) ([]monitor.DeadLetter, error) {
	return f.letters, f.listErr
}

func (f *fakeConsole) PurgeDeadLetters(context.Context) error { f.purged = true; return nil }

func (f *fakeConsole) Chaos(context.Context) service.ChaosView { return f.chaos }

func (f *fakeConsole) SetSMTPChaos(_ context.Context, triggers monitor.ChaosTriggers) error {
	f.chaos.SMTP = triggers
	return nil
}

func (f *fakeConsole) ConfigureMockAPI(_ context.Context, behavior monitor.MockBehavior) error {
	f.chaos.Mock.MockBehavior = behavior
	return nil
}

func (f *fakeConsole) ResetChaos(context.Context) error { f.chaosReset = true; return nil }

func newTestServer(t *testing.T, console *fakeConsole, mailerEnv string) http.Handler {
	t.Helper()
	server, err := NewServer(console, Settings{
		MailerEnv:          mailerEnv,
		Queue:              "mailer-service",
		MailpitPublicURL:   "http://localhost:8025",
		CallbackSuccessURL: "http://callback:9099/success",
		CallbackFailureURL: "http://callback:9099/failures",
	})
	require.NoError(t, err)
	return server.Routes()
}

func serve(handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func multipartRequest(t *testing.T, fields map[string]string, fileName, fileContent string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		require.NoError(t, writer.WriteField(key, value))
	}
	if fileName != "" {
		part, err := writer.CreateFormFile("attachments", fileName)
		require.NoError(t, err)
		_, err = part.Write([]byte(fileContent))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/send", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestIndex(t *testing.T) {
	t.Run("given smtp env then render the form with default sender and settings", func(t *testing.T) {
		response := serve(newTestServer(t, &fakeConsole{}, "smtp"), httptest.NewRequest(http.MethodGet, "/", nil))

		assert.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `value="no-reply@example.com"`)
		assert.Contains(t, response.Body.String(), `value="http://callback:9099/failures"`)
		assert.Contains(t, response.Body.String(), `value="http://callback:9099/success"`)
		assert.Contains(t, response.Body.String(), "Mailer App")
	})

	t.Run("given a resend env then default to the resend sandbox sender", func(t *testing.T) {
		response := serve(newTestServer(t, &fakeConsole{}, "resend"), httptest.NewRequest(http.MethodGet, "/", nil))

		assert.Contains(t, response.Body.String(), `value="onboarding@resend.dev"`)
	})

	t.Run("given an unknown path then return 404", func(t *testing.T) {
		response := serve(newTestServer(t, &fakeConsole{}, "smtp"), httptest.NewRequest(http.MethodGet, "/nope", nil))

		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("must serve static assets", func(t *testing.T) {
		response := serve(newTestServer(t, &fakeConsole{}, "smtp"), httptest.NewRequest(http.MethodGet, "/static/app.js", nil))

		assert.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "refreshPolledPanels")
	})
}

func TestSend(t *testing.T) {
	t.Run("given a valid form then publish and render the payload preview", func(t *testing.T) {
		console := &fakeConsole{}
		req := multipartRequest(t, map[string]string{
			"from": "no-reply@example.com", "to": "jane@example.com", "cc": "john@example.com",
			"subject": "Hi", "html": "<p>Hi</p>", "format": "string",
			"successCallbackEnabled": "on", "successCallbackUrl": "http://callback:9099/success",
			"failureCallbackEnabled": "on", "failureCallbackUrl": "http://callback:9099/failures", "callbackAuthorization": "Bearer t",
		}, "note.txt", "content")

		response := serve(newTestServer(t, console, "smtp"), req)

		assert.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "Published (rabbitmq)")
		assert.Contains(t, response.Body.String(), "generated-id")
		assert.Equal(t, message.FormatString, console.sentForm.Format)
		assert.Equal(t, "john@example.com", console.sentForm.Cc)
		assert.True(t, console.sentForm.SuccessCallbackEnabled)
		assert.True(t, console.sentForm.FailureCallbackEnabled)
		assert.Equal(t, "http://callback:9099/success", console.sentForm.SuccessCallbackURL)
		assert.False(t, console.sentForm.InvalidBody)
		require.Len(t, console.sentForm.Files, 1)
		assert.Equal(t, "note.txt", console.sentForm.Files[0].Name)
		assert.Equal(t, []byte("content"), console.sentForm.Files[0].Content)
	})

	t.Run("given the http channel then render the api response", func(t *testing.T) {
		console := &fakeConsole{response: &api.Response{StatusCode: 422, Event: monitor.DeliveryEvent{
			ID: "generated-id", Status: "failed", ErrorCode: "api_invalid_receiver", Cause: "invalid to",
		}}}
		req := multipartRequest(t, map[string]string{
			"channel": "http", "from": "no-reply@example.com", "to": "jane@example.com", "subject": "Hi", "html": "<p>Hi</p>",
		}, "", "")

		response := serve(newTestServer(t, console, "smtp"), req)

		assert.Equal(t, message.ChannelHTTP, console.sentForm.Channel)
		assert.Contains(t, response.Body.String(), "HTTP 422 · api_invalid_receiver")
		assert.Contains(t, response.Body.String(), "API response")
	})

	t.Run("given the http channel and a sent answer then render success", func(t *testing.T) {
		console := &fakeConsole{response: &api.Response{StatusCode: 200, Event: monitor.DeliveryEvent{ID: "generated-id", Status: "sent"}}}
		req := multipartRequest(t, map[string]string{
			"channel": "http", "from": "no-reply@example.com", "to": "jane@example.com", "subject": "Hi", "html": "<p>Hi</p>",
		}, "", "")

		body := serve(newTestServer(t, console, "smtp"), req).Body.String()

		assert.Contains(t, body, "HTTP 200 · sent")
	})

	t.Run("given a service error then render it with 422", func(t *testing.T) {
		console := &fakeConsole{sendErr: errors.New("subject is required")}
		req := multipartRequest(t, map[string]string{"from": "no-reply@example.com"}, "", "")

		response := serve(newTestServer(t, console, "smtp"), req)

		assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
		assert.Contains(t, response.Body.String(), "subject is required")
	})

	t.Run("given a non-multipart body then return 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader("x"))

		response := serve(newTestServer(t, &fakeConsole{}, "smtp"), req)

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "invalid form")
	})
}

func TestPanels(t *testing.T) {
	t.Run("given status then render pills", func(t *testing.T) {
		console := &fakeConsole{status: service.Status{
			MailerReady: true,
			Queue:       monitor.QueueStats{Messages: 3, Consumers: 1, Retrying: 2, DeadLettered: 5},
			KafkaError:  "kafka down",
		}}

		body := serve(newTestServer(t, console, "resend"), httptest.NewRequest(http.MethodGet, "/partials/status", nil)).Body.String()

		assert.Contains(t, body, "mailer ready")
		assert.Contains(t, body, "<strong>3</strong>")
		assert.Contains(t, body, "<strong>resend</strong>")
		assert.Contains(t, body, "retrying <strong>2</strong>")
		assert.Contains(t, body, "DLQ <strong>5</strong>")
		assert.Contains(t, body, `title="kafka down">Kafka unavailable`)
	})

	t.Run("given a queue error then render the unavailable pill", func(t *testing.T) {
		console := &fakeConsole{status: service.Status{QueueError: "connect to RabbitMQ: refused"}}

		body := serve(newTestServer(t, console, "smtp"), httptest.NewRequest(http.MethodGet, "/partials/status", nil)).Body.String()

		assert.Contains(t, body, "mailer unavailable")
		assert.Contains(t, body, "RabbitMQ unavailable")
	})

	t.Run("given inbox messages then render them with mailpit links", func(t *testing.T) {
		console := &fakeConsole{messages: []monitor.InboxMessage{{
			ID: "m1", Subject: "Hi", Created: time.Now(),
			From:        monitor.Address{Address: "no-reply@example.com"},
			To:          []monitor.Address{{Address: "jane@example.com"}, {Address: "john@example.com"}},
			Bcc:         []monitor.Address{{Address: "anna@example.com"}},
			Attachments: 2,
		}}}

		body := serve(newTestServer(t, console, "smtp"), httptest.NewRequest(http.MethodGet, "/partials/inbox", nil)).Body.String()

		assert.Contains(t, body, `href="http://localhost:8025/view/m1"`)
		assert.Contains(t, body, "jane@example.com, john@example.com")
		assert.Contains(t, body, "bcc anna@example.com")
		assert.Contains(t, body, "2 attachment(s)")
	})

	t.Run("given no callbacks then render the empty state", func(t *testing.T) {
		body := serve(newTestServer(t, &fakeConsole{}, "smtp"), httptest.NewRequest(http.MethodGet, "/partials/callbacks", nil)).Body.String()

		assert.Contains(t, body, "No callbacks received")
	})

	t.Run("given a success callback then render the sent badge without cause", func(t *testing.T) {
		console := &fakeConsole{events: []monitor.CallbackEvent{{
			ReceivedAt: time.Now(), Status: 204,
			Body: monitor.DeliveryEvent{ID: "s1", Subject: "Hi", Status: "sent"},
		}}}

		body := serve(newTestServer(t, console, "smtp"), httptest.NewRequest(http.MethodGet, "/partials/callbacks", nil)).Body.String()

		assert.Contains(t, body, `class="badge sent"`)
		assert.NotContains(t, body, `class="cause"`)
	})

	t.Run("given callbacks then render id, subject and cause", func(t *testing.T) {
		console := &fakeConsole{events: []monitor.CallbackEvent{{
			ReceivedAt: time.Now(), Status: 204,
			Body: monitor.DeliveryEvent{ID: "e1", Subject: "Hi", Status: "failed", ErrorCode: "api_sender_not_allowed", Cause: "unverified domain"},
		}}}

		body := serve(newTestServer(t, console, "smtp"), httptest.NewRequest(http.MethodGet, "/partials/callbacks", nil)).Body.String()

		assert.Contains(t, body, "e1")
		assert.Contains(t, body, "unverified domain")
		assert.Contains(t, body, "api_sender_not_allowed")
	})

	t.Run("given a listing error then render it", func(t *testing.T) {
		console := &fakeConsole{listErr: errors.New("mailpit fora do ar")}

		body := serve(newTestServer(t, console, "smtp"), httptest.NewRequest(http.MethodGet, "/partials/inbox", nil)).Body.String()

		assert.Contains(t, body, "mailpit fora do ar")
	})

	t.Run("given clear requests then clear inbox and callbacks", func(t *testing.T) {
		console := &fakeConsole{}
		handler := newTestServer(t, console, "smtp")

		serve(handler, httptest.NewRequest(http.MethodPost, "/partials/inbox/clear", nil))
		serve(handler, httptest.NewRequest(http.MethodPost, "/partials/callbacks/clear", nil))

		assert.True(t, console.inboxCleared)
		assert.True(t, console.eventsCleared)
	})
}

func TestResponseJSON(t *testing.T) {
	t.Run("given no response then return empty", func(t *testing.T) {
		assert.Empty(t, responseJSON(nil))
	})

	t.Run("given a raw body then return it", func(t *testing.T) {
		assert.Equal(t, `{"error":"x"}`, responseJSON(&api.Response{StatusCode: 401, Raw: `{"error":"x"}`}))
	})

	t.Run("given an event then render it as json", func(t *testing.T) {
		assert.Contains(t, responseJSON(&api.Response{Event: monitor.DeliveryEvent{ID: "1", Status: "sent"}}), `"status": "sent"`)
	})
}

func TestPreviewJSON(t *testing.T) {
	t.Run("must shorten long body and attachment data", func(t *testing.T) {
		long := strings.Repeat("A", 200)

		preview := previewJSON(message.Email{ID: "1", Body: long, Attachments: []message.Attachment{{Name: "a", Data: long}}})

		assert.NotContains(t, preview, long)
		assert.Contains(t, preview, "(200 chars)")
	})

	t.Run("must keep short values and omit empty attachments", func(t *testing.T) {
		preview := previewJSON(message.Email{ID: "1", Body: "curto"})

		assert.Contains(t, preview, `"body": "curto"`)
		assert.NotContains(t, preview, "attachments")
	})
}

func postForm(handler http.Handler, path string, values url.Values) string {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return serve(handler, req).Body.String()
}

func TestDeadLetterPanel(t *testing.T) {
	t.Run("given dead letters then render code, attempts and cause", func(t *testing.T) {
		console := &fakeConsole{letters: []monitor.DeadLetter{{Source: "kafka", MessageID: "m1", Attempt: 3, ErrorCode: "api_rate_limited", Cause: "too many", FailedAt: time.Now()}}}

		body := serve(newTestServer(t, console, "smtp"), httptest.NewRequest(http.MethodGet, "/partials/dlq", nil)).Body.String()

		assert.Contains(t, body, "api_rate_limited")
		assert.Contains(t, body, "3 attempt(s)")
		assert.Contains(t, body, `<span class="badge source">kafka</span>`)
		assert.Contains(t, body, "too many")
	})

	t.Run("given no dead letters then render the empty state", func(t *testing.T) {
		body := serve(newTestServer(t, &fakeConsole{}, "smtp"), httptest.NewRequest(http.MethodGet, "/partials/dlq", nil)).Body.String()

		assert.Contains(t, body, "No messages in the DLQ")
	})

	t.Run("given purge then empty the queue", func(t *testing.T) {
		console := &fakeConsole{}

		serve(newTestServer(t, console, "smtp"), httptest.NewRequest(http.MethodPost, "/partials/dlq/purge", nil))

		assert.True(t, console.purged)
	})
}

func TestChaosPanel(t *testing.T) {
	t.Run("given the current chaos then render selects and mock counters", func(t *testing.T) {
		console := &fakeConsole{chaos: service.ChaosView{
			SMTP: monitor.ChaosTriggers{Recipient: monitor.ChaosTrigger{ErrorCode: 550, Probability: 100}},
			Mock: monitor.MockState{MockBehavior: monitor.MockBehavior{FailNext: 2, Status: 429}, Accepted: 1, Rejected: 4},
		}}

		body := serve(newTestServer(t, console, "smtp"), httptest.NewRequest(http.MethodGet, "/partials/chaos", nil)).Body.String()

		assert.Contains(t, body, `<option value="550" selected>550 permanent</option>`)
		assert.Contains(t, body, "failing 2× with 429")
		assert.Contains(t, body, "accepted 1 · rejected 4")
	})

	t.Run("given unavailable sides then render their errors", func(t *testing.T) {
		console := &fakeConsole{chaos: service.ChaosView{SMTPError: "mailpit down", MockError: "mock down"}}

		body := serve(newTestServer(t, console, "smtp"), httptest.NewRequest(http.MethodGet, "/partials/chaos", nil)).Body.String()

		assert.Contains(t, body, "Mailpit unavailable: mailpit down")
		assert.Contains(t, body, "Mock API unavailable: mock down")
	})

	t.Run("given smtp selections then enable the chosen triggers", func(t *testing.T) {
		console := &fakeConsole{}

		postForm(newTestServer(t, console, "smtp"), "/partials/chaos/smtp", url.Values{"sender": {"451"}, "recipient": {"0"}, "authentication": {"535"}})

		assert.Equal(t, monitor.ChaosTriggers{
			Sender:         monitor.ChaosTrigger{ErrorCode: 451, Probability: 100},
			Recipient:      monitor.ChaosTrigger{ErrorCode: 451, Probability: 0},
			Authentication: monitor.ChaosTrigger{ErrorCode: 535, Probability: 100},
		}, console.chaos.SMTP)
	})

	t.Run("given a mock preset then configure the failure", func(t *testing.T) {
		console := &fakeConsole{}

		postForm(newTestServer(t, console, "smtp"), "/partials/chaos/api", url.Values{"preset": {"unavailable"}, "failNext": {"3"}})

		assert.Equal(t, 3, console.chaos.Mock.FailNext)
		assert.Equal(t, 503, console.chaos.Mock.Status)
		assert.Equal(t, "service_unavailable", console.chaos.Mock.Name)
	})

	t.Run("given an unknown preset then render an error", func(t *testing.T) {
		body := postForm(newTestServer(t, &fakeConsole{}, "smtp"), "/partials/chaos/api", url.Values{"preset": {"nope"}, "failNext": {"1"}})

		assert.Contains(t, body, "choose a scenario")
	})

	t.Run("given reset then turn everything off", func(t *testing.T) {
		console := &fakeConsole{}

		serve(newTestServer(t, console, "smtp"), httptest.NewRequest(http.MethodPost, "/partials/chaos/reset", nil))

		assert.True(t, console.chaosReset)
	})
}
