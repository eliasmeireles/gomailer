package mailer

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/application/config"
	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

type capturedRequest struct {
	method string
	path   string
	auth   string
	body   resendEmailRequest
}

func newResendServer(t *testing.T, status int, response string, captured *capturedRequest) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method, captured.path, captured.auth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&captured.body)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestResendSenderSend(t *testing.T) {
	t.Run("given a valid email then post it to the emails endpoint", func(t *testing.T) {
		captured := &capturedRequest{}
		server := newResendServer(t, http.StatusOK, `{"id":"email-id"}`, captured)
		data := newValidEmail()
		data.Receiver = model.Recipients{testRecipient, "joao@exemplo.com.br"}
		data.Cc = model.Recipients{"cc@exemplo.com.br"}
		data.Bcc = model.Recipients{"bcc1@exemplo.com.br", "bcc2@exemplo.com.br"}
		data.Attachments = &[]model.Attachment{{Name: testFileName, Type: testFileType, Data: helloBase64}}

		err := NewResendSender(config.ResendConfig{APIKey: "re_test", BaseURL: server.URL}, server.Client()).Send(data)

		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, captured.method)
		assert.Equal(t, "/emails", captured.path)
		assert.Equal(t, "Bearer re_test", captured.auth)
		assert.Equal(t, resendEmailRequest{
			From:    testSender,
			To:      []string{testRecipient, "joao@exemplo.com.br"},
			Cc:      []string{"cc@exemplo.com.br"},
			Bcc:     []string{"bcc1@exemplo.com.br", "bcc2@exemplo.com.br"},
			Subject: testSubject,
			HTML:    testBody,
			Attachments: []resendAttachment{{
				Filename:    testFileName,
				Content:     base64.StdEncoding.EncodeToString([]byte("Hello World")),
				ContentType: testFileType,
			}},
		}, captured.body)
	})

	t.Run("given a Resend error response then return its name and message", func(t *testing.T) {
		server := newResendServer(t, http.StatusForbidden,
			`{"statusCode":403,"name":"validation_error","message":"The exemplo.com.br domain is not verified."}`, &capturedRequest{})

		err := NewResendSender(config.ResendConfig{APIKey: "re_test", BaseURL: server.URL}, server.Client()).Send(newValidEmail())

		require.EqualError(t, err, "resend API returned status 403: validation_error: The exemplo.com.br domain is not verified.")
		assert.Equal(t, model.CodeAPISenderNotAllowed, mailer.CodeOf(err))
	})

	t.Run("given a non-json error response then return the raw body", func(t *testing.T) {
		server := newResendServer(t, http.StatusBadGateway, "bad gateway", &capturedRequest{})

		err := NewResendSender(config.ResendConfig{APIKey: "re_test", BaseURL: server.URL}, server.Client()).Send(newValidEmail())

		require.EqualError(t, err, "resend API returned status 502: bad gateway")
		assert.Equal(t, model.CodeAPIProviderUnavailable, mailer.CodeOf(err))
	})

	t.Run("given an invalid email then fail without calling the api", func(t *testing.T) {
		captured := &capturedRequest{}
		server := newResendServer(t, http.StatusOK, `{}`, captured)
		data := newValidEmail()
		data.From = ""

		err := NewResendSender(config.ResendConfig{APIKey: "re_test", BaseURL: server.URL}, server.Client()).Send(data)

		require.EqualError(t, err, "sender email cannot be empty")
		assert.Empty(t, captured.method)
	})

	t.Run("given an unreachable api then return error", func(t *testing.T) {
		err := NewResendSender(config.ResendConfig{APIKey: "re_test", BaseURL: "http://127.0.0.1:1"}, http.DefaultClient).Send(newValidEmail())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to call resend API")
		assert.Equal(t, model.CodeAPIConnectionFailed, mailer.CodeOf(err))
	})
}

func TestClassifyResendError(t *testing.T) {
	cases := map[string]struct {
		status   int
		body     string
		expected model.ErrorCode
	}{
		"given an invalid api key then authorization denied":     {403, `{"name":"invalid_api_key","message":"API key is invalid"}`, model.CodeAPIAuthorizationDenied},
		"given a missing api key then authorization denied":      {401, `{"name":"missing_api_key","message":"Missing API key"}`, model.CodeAPIAuthorizationDenied},
		"given a 401 validation error then authorization denied": {401, `{"statusCode":401,"name":"validation_error","message":"API key is invalid"}`, model.CodeAPIAuthorizationDenied},
		"given an unverified domain then sender not allowed":     {403, `{"name":"validation_error","message":"The exemplo.com.br domain is not verified."}`, model.CodeAPISenderNotAllowed},
		"given an invalid to field then invalid receiver":        {422, `{"name":"validation_error","message":"Invalid ` + "`to`" + ` field."}`, model.CodeAPIInvalidReceiver},
		"given an invalid from field then sender not allowed":    {422, `{"name":"validation_error","message":"Invalid ` + "`from`" + ` field."}`, model.CodeAPISenderNotAllowed},
		"given another validation error then invalid request":    {422, `{"name":"validation_error","message":"Subject too long"}`, model.CodeAPIInvalidRequest},
		"given an invalid attachment then invalid attachment":    {422, `{"name":"invalid_attachment","message":"Attachment must have content"}`, model.CodeAPIInvalidAttachment},
		"given a daily quota then quota exceeded":                {429, `{"name":"daily_quota_exceeded","message":"Daily limit"}`, model.CodeAPIQuotaExceeded},
		"given a rate limit then rate limited":                   {429, `{"name":"rate_limit_exceeded","message":"Too many requests"}`, model.CodeAPIRateLimited},
		"given an application error then provider unavailable":   {500, `{"name":"application_error","message":"Unexpected"}`, model.CodeAPIProviderUnavailable},
		"given an unknown name then no code":                     {409, `{"name":"resource_locked","message":"Locked"}`, ""},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, code := classifyResendError(tc.status, []byte(tc.body))

			assert.Equal(t, tc.expected, code)
		})
	}

	t.Run("given a Resend error body then describe name and message", func(t *testing.T) {
		detail, _ := classifyResendError(404, []byte(`{"name":"not_found","message":"missing"}`))

		assert.Equal(t, "not_found: missing", detail)
	})

	t.Run("given a non-json body then return empty detail and code", func(t *testing.T) {
		detail, code := classifyResendError(500, []byte("oops"))

		assert.Empty(t, detail)
		assert.Empty(t, code)
	})
}
