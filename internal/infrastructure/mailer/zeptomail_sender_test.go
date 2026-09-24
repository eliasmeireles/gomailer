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

const testZeptoAuthorization = "Zoho-enczapikey wSsVR6-token"

type capturedZeptoRequest struct {
	method string
	path   string
	auth   string
	body   zeptoMailRequest
}

func newZeptoMailServer(t *testing.T, status int, response string, captured *capturedZeptoRequest) *httptest.Server {
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

func newTestZeptoMailSender(server *httptest.Server) *zeptoMailSender {
	return NewZeptoMailSender(config.ZeptoMailConfig{Authorization: testZeptoAuthorization, BaseURL: server.URL}, server.Client()).(*zeptoMailSender)
}

func TestZeptoMailSenderSend(t *testing.T) {
	t.Run("given a valid email then post it to the email endpoint", func(t *testing.T) {
		captured := &capturedZeptoRequest{}
		server := newZeptoMailServer(t, http.StatusCreated, `{"data":[{"code":"EM_104","message":"OK"}],"message":"OK","request_id":"req-1"}`, captured)
		data := newValidEmail()
		data.Receiver = model.Recipients{testRecipient, "john@example.com"}
		data.Cc = model.Recipients{"cc@example.com"}
		data.Bcc = model.Recipients{"bcc@example.com"}
		data.Attachments = &[]model.Attachment{{Name: testFileName, Type: testFileType, Data: helloBase64}}

		err := newTestZeptoMailSender(server).Send(data)

		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, captured.method)
		assert.Equal(t, "/v1.1/email", captured.path)
		assert.Equal(t, testZeptoAuthorization, captured.auth)
		assert.Equal(t, zeptoMailRequest{
			From: zeptoMailAddress{Address: testSender},
			To: []zeptoMailRecipient{
				{EmailAddress: zeptoMailAddress{Address: testRecipient}},
				{EmailAddress: zeptoMailAddress{Address: "john@example.com"}},
			},
			Cc:       []zeptoMailRecipient{{EmailAddress: zeptoMailAddress{Address: "cc@example.com"}}},
			Bcc:      []zeptoMailRecipient{{EmailAddress: zeptoMailAddress{Address: "bcc@example.com"}}},
			Subject:  testSubject,
			HTMLBody: testBody,
			Attachments: []zeptoMailAttachment{{
				Name:     testFileName,
				MimeType: testFileType,
				Content:  base64.StdEncoding.EncodeToString([]byte("Hello World")),
			}},
		}, captured.body)
	})

	t.Run("given a ZeptoMail error response then return code, message and details", func(t *testing.T) {
		server := newZeptoMailServer(t, http.StatusUnauthorized,
			`{"error":{"code":"TM_4001","message":"Access Denied","details":[{"code":"SERR_157","message":"Invalid API Token found"}]}}`,
			&capturedZeptoRequest{})

		err := newTestZeptoMailSender(server).Send(newValidEmail())

		require.EqualError(t, err, "zeptomail API returned status 401: TM_4001: Access Denied (SERR_157: Invalid API Token found)")
		assert.Equal(t, model.CodeAPIAuthorizationDenied, mailer.CodeOf(err))
	})

	t.Run("given an invalid email then fail without calling the api", func(t *testing.T) {
		captured := &capturedZeptoRequest{}
		server := newZeptoMailServer(t, http.StatusCreated, `{}`, captured)
		data := newValidEmail()
		data.Body = ""

		err := newTestZeptoMailSender(server).Send(data)

		require.EqualError(t, err, "body cannot be empty")
		assert.Empty(t, captured.method)
	})
}

func TestClassifyZeptoMailError(t *testing.T) {
	cases := map[string]struct {
		body     string
		expected model.ErrorCode
	}{
		"given an invalid token then authorization denied":            {`{"error":{"code":"TM_3601","message":"Access Denied","details":[{"code":"SERR_157","message":"Sendmail token is invalid"}]}}`, model.CodeAPIAuthorizationDenied},
		"given an unverified domain then sender not allowed":          {`{"error":{"code":"TM_4001","message":"Access Denied","details":[{"code":"SM_111","message":"Sender address domain not verified"}]}}`, model.CodeAPISenderNotAllowed},
		"given an invalid to address then invalid receiver":           {`{"error":{"code":"TM_4001","message":"Access Denied","details":[{"code":"SM_113","message":"Invalid to address"}]}}`, model.CodeAPIInvalidReceiver},
		"given an invalid from address then sender not allowed":       {`{"error":{"code":"TM_4001","message":"Access Denied","details":[{"code":"SM_113","message":"Invalid from address"}]}}`, model.CodeAPISenderNotAllowed},
		"given a daily limit then quota exceeded":                     {`{"error":{"code":"TM_3601","message":"Limit","details":[{"code":"SMI_115","message":"Daily limit exhausted"}]}}`, model.CodeAPIQuotaExceeded},
		"given a missing field then invalid request":                  {`{"error":{"code":"TM_3201","message":"Mandatory Field 'subject' was set as Empty Value","details":[{"code":"GE_102","message":"Mandatory field"}]}}`, model.CodeAPIInvalidRequest},
		"given exhausted credits without details then quota exceeded": {`{"error":{"code":"TM_5001","message":"Credits exhausted"}}`, model.CodeAPIQuotaExceeded},
		"given an unknown code then no code":                          {`{"error":{"code":"TM_9999","message":"Other"}}`, ""},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, code := classifyZeptoMailError(400, []byte(tc.body))

			assert.Equal(t, tc.expected, code)
		})
	}

	t.Run("given an error without details then describe code and message", func(t *testing.T) {
		detail, _ := classifyZeptoMailError(400, []byte(`{"error":{"code":"TM_3201","message":"Mandatory field missing"}}`))

		assert.Equal(t, "TM_3201: Mandatory field missing", detail)
	})

	t.Run("given a body without error object then return empty", func(t *testing.T) {
		detail, code := classifyZeptoMailError(400, []byte(`{"message":"error"}`))

		assert.Empty(t, detail)
		assert.Empty(t, code)
	})
}
