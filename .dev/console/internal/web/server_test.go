package web

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/dev/console/internal/api"
	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
	"github.com/eliasmeireles/gomailer/dev/console/internal/monitor"
	"github.com/eliasmeireles/gomailer/dev/console/internal/queue"
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
}

func (f *fakeConsole) Send(_ context.Context, form message.Form) (service.SendResult, error) {
	f.sentForm = form
	if f.sendErr != nil {
		return service.SendResult{}, f.sendErr
	}
	email, err := message.Build(form, func() string { return "id-gerado" })
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
		assert.Contains(t, response.Body.String(), `value="no-reply@exemplo.com.br"`)
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
		assert.Contains(t, response.Body.String(), "refreshPanels")
	})
}

func TestSend(t *testing.T) {
	t.Run("given a valid form then publish and render the payload preview", func(t *testing.T) {
		console := &fakeConsole{}
		req := multipartRequest(t, map[string]string{
			"from": "no-reply@exemplo.com.br", "to": "maria@exemplo.com.br", "cc": "joao@exemplo.com.br",
			"subject": "Oi", "html": "<p>Oi</p>", "format": "string",
			"successCallbackEnabled": "on", "successCallbackUrl": "http://callback:9099/success",
			"failureCallbackEnabled": "on", "failureCallbackUrl": "http://callback:9099/failures", "callbackAuthorization": "Bearer t",
		}, "nota.txt", "conteudo")

		response := serve(newTestServer(t, console, "smtp"), req)

		assert.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "Publicado na fila (rabbitmq)")
		assert.Contains(t, response.Body.String(), "id-gerado")
		assert.Equal(t, message.FormatString, console.sentForm.Format)
		assert.Equal(t, "joao@exemplo.com.br", console.sentForm.Cc)
		assert.True(t, console.sentForm.SuccessCallbackEnabled)
		assert.True(t, console.sentForm.FailureCallbackEnabled)
		assert.Equal(t, "http://callback:9099/success", console.sentForm.SuccessCallbackURL)
		assert.False(t, console.sentForm.InvalidBody)
		require.Len(t, console.sentForm.Files, 1)
		assert.Equal(t, "nota.txt", console.sentForm.Files[0].Name)
		assert.Equal(t, []byte("conteudo"), console.sentForm.Files[0].Content)
	})

	t.Run("given the http channel then render the api response", func(t *testing.T) {
		console := &fakeConsole{response: &api.Response{StatusCode: 422, Event: monitor.DeliveryEvent{
			ID: "id-gerado", Status: "failed", ErrorCode: "api_invalid_receiver", Cause: "invalid to",
		}}}
		req := multipartRequest(t, map[string]string{
			"channel": "http", "from": "no-reply@exemplo.com.br", "to": "maria@exemplo.com.br", "subject": "Oi", "html": "<p>Oi</p>",
		}, "", "")

		response := serve(newTestServer(t, console, "smtp"), req)

		assert.Equal(t, message.ChannelHTTP, console.sentForm.Channel)
		assert.Contains(t, response.Body.String(), "HTTP 422 · api_invalid_receiver")
		assert.Contains(t, response.Body.String(), "Resposta da API")
	})

	t.Run("given the http channel and a sent answer then render success", func(t *testing.T) {
		console := &fakeConsole{response: &api.Response{StatusCode: 200, Event: monitor.DeliveryEvent{ID: "id-gerado", Status: "sent"}}}
		req := multipartRequest(t, map[string]string{
			"channel": "http", "from": "no-reply@exemplo.com.br", "to": "maria@exemplo.com.br", "subject": "Oi", "html": "<p>Oi</p>",
		}, "", "")

		body := serve(newTestServer(t, console, "smtp"), req).Body.String()

		assert.Contains(t, body, "HTTP 200 · enviado")
	})

	t.Run("given a service error then render it with 422", func(t *testing.T) {
		console := &fakeConsole{sendErr: errors.New("informe o assunto")}
		req := multipartRequest(t, map[string]string{"from": "no-reply@exemplo.com.br"}, "", "")

		response := serve(newTestServer(t, console, "smtp"), req)

		assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
		assert.Contains(t, response.Body.String(), "informe o assunto")
	})

	t.Run("given a non-multipart body then return 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/send", strings.NewReader("x"))

		response := serve(newTestServer(t, &fakeConsole{}, "smtp"), req)

		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "formulário inválido")
	})
}

func TestPanels(t *testing.T) {
	t.Run("given status then render pills", func(t *testing.T) {
		console := &fakeConsole{status: service.Status{MailerReady: true, Queue: queue.Stats{Messages: 3, Consumers: 1}}}

		body := serve(newTestServer(t, console, "resend"), httptest.NewRequest(http.MethodGet, "/partials/status", nil)).Body.String()

		assert.Contains(t, body, "mailer pronto")
		assert.Contains(t, body, "<strong>3</strong>")
		assert.Contains(t, body, "<strong>resend</strong>")
	})

	t.Run("given a queue error then render the unavailable pill", func(t *testing.T) {
		console := &fakeConsole{status: service.Status{QueueError: "connect to RabbitMQ: refused"}}

		body := serve(newTestServer(t, console, "smtp"), httptest.NewRequest(http.MethodGet, "/partials/status", nil)).Body.String()

		assert.Contains(t, body, "mailer indisponível")
		assert.Contains(t, body, "fila indisponível")
	})

	t.Run("given inbox messages then render them with mailpit links", func(t *testing.T) {
		console := &fakeConsole{messages: []monitor.InboxMessage{{
			ID: "m1", Subject: "Oi", Created: time.Now(),
			From:        monitor.Address{Address: "no-reply@exemplo.com.br"},
			To:          []monitor.Address{{Address: "maria@exemplo.com.br"}, {Address: "joao@exemplo.com.br"}},
			Bcc:         []monitor.Address{{Address: "ana@exemplo.com.br"}},
			Attachments: 2,
		}}}

		body := serve(newTestServer(t, console, "smtp"), httptest.NewRequest(http.MethodGet, "/partials/inbox", nil)).Body.String()

		assert.Contains(t, body, `href="http://localhost:8025/view/m1"`)
		assert.Contains(t, body, "maria@exemplo.com.br, joao@exemplo.com.br")
		assert.Contains(t, body, "bcc ana@exemplo.com.br")
		assert.Contains(t, body, "2 anexo(s)")
	})

	t.Run("given no callbacks then render the empty state", func(t *testing.T) {
		body := serve(newTestServer(t, &fakeConsole{}, "smtp"), httptest.NewRequest(http.MethodGet, "/partials/callbacks", nil)).Body.String()

		assert.Contains(t, body, "Nenhum callback recebido")
	})

	t.Run("given a success callback then render the sent badge without cause", func(t *testing.T) {
		console := &fakeConsole{events: []monitor.CallbackEvent{{
			ReceivedAt: time.Now(), Status: 204,
			Body: monitor.DeliveryEvent{ID: "s1", Subject: "Oi", Status: "sent"},
		}}}

		body := serve(newTestServer(t, console, "smtp"), httptest.NewRequest(http.MethodGet, "/partials/callbacks", nil)).Body.String()

		assert.Contains(t, body, `class="badge sent"`)
		assert.NotContains(t, body, `class="cause"`)
	})

	t.Run("given callbacks then render id, subject and cause", func(t *testing.T) {
		console := &fakeConsole{events: []monitor.CallbackEvent{{
			ReceivedAt: time.Now(), Status: 204,
			Body: monitor.DeliveryEvent{ID: "e1", Subject: "Oi", Status: "failed", ErrorCode: "api_sender_not_allowed", Cause: "domínio não verificado"},
		}}}

		body := serve(newTestServer(t, console, "smtp"), httptest.NewRequest(http.MethodGet, "/partials/callbacks", nil)).Body.String()

		assert.Contains(t, body, "e1")
		assert.Contains(t, body, "domínio não verificado")
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
