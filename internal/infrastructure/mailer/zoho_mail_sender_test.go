package mailer

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

type zohoRecordedRequest struct {
	method      string
	path        string
	query       string
	auth        string
	contentType string
	body        []byte
}

type fakeZohoMail struct {
	mu            sync.Mutex
	requests      []zohoRecordedRequest
	sendStatus    int
	sendResponse  string
	uploadStatus  int
	uploadResults map[string]string
}

func (f *fakeZohoMail) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.requests = append(f.requests, zohoRecordedRequest{
		method: r.Method, path: r.URL.Path, query: r.URL.RawQuery,
		auth: r.Header.Get("Authorization"), contentType: r.Header.Get("Content-Type"), body: body,
	})
	f.mu.Unlock()

	if r.URL.Path == "/api/accounts/123456/messages/attachments" {
		w.WriteHeader(f.uploadStatus)
		_, _ = w.Write([]byte(f.uploadResults[r.URL.Query().Get("fileName")]))
		return
	}
	w.WriteHeader(f.sendStatus)
	_, _ = w.Write([]byte(f.sendResponse))
}

func newZohoMailTestSender(t *testing.T, fake *fakeZohoMail) (*zohoMailSender, *tokenServer) {
	t.Helper()
	ts := newTokenServer(t, http.StatusOK, `{"access_token":"access-1","expires_in":3600}`)
	api := httptest.NewServer(http.HandlerFunc(fake.handler))
	t.Cleanup(api.Close)
	return NewZohoMailSender(newTestZohoConfig(ts.server.URL, api.URL), api.Client()).(*zohoMailSender), ts
}

func TestZohoMailSenderSend(t *testing.T) {
	t.Run("given an email without attachments then send it with the oauth token", func(t *testing.T) {
		fake := &fakeZohoMail{sendStatus: http.StatusOK, sendResponse: `{"status":{"code":200,"description":"success"}}`}
		sender, _ := newZohoMailTestSender(t, fake)
		data := newValidEmail()
		data.Receiver = model.Recipients{testRecipient, "john@example.com"}
		data.Cc = model.Recipients{"cc1@example.com", "cc2@example.com"}
		data.Bcc = model.Recipients{"bcc@example.com"}

		err := sender.Send(data)

		require.NoError(t, err)
		require.Len(t, fake.requests, 1)
		assert.Equal(t, http.MethodPost, fake.requests[0].method)
		assert.Equal(t, "/api/accounts/123456/messages", fake.requests[0].path)
		assert.Equal(t, "Zoho-oauthtoken access-1", fake.requests[0].auth)
		var sent zohoMailRequest
		require.NoError(t, json.Unmarshal(fake.requests[0].body, &sent))
		assert.Equal(t, zohoMailRequest{
			FromAddress: testSender,
			ToAddress:   testRecipient + ",john@example.com",
			CcAddress:   "cc1@example.com,cc2@example.com",
			BccAddress:  "bcc@example.com",
			Subject:     testSubject,
			Content:     testBody,
			MailFormat:  "html",
		}, sent)
	})

	t.Run("given attachments then upload them first and reference them in the message", func(t *testing.T) {
		fake := &fakeZohoMail{
			sendStatus:   http.StatusOK,
			sendResponse: `{"status":{"code":200,"description":"success"}}`,
			uploadStatus: http.StatusOK,
			uploadResults: map[string]string{
				testFileName: `{"status":{"code":200},"data":{"storeName":"5386","attachmentName":"file.txt","attachmentPath":"/Mail/abc-file.txt"}}`,
			},
		}
		sender, _ := newZohoMailTestSender(t, fake)
		data := newValidEmail()
		data.Attachments = &[]model.Attachment{{Name: testFileName, Type: testFileType, Data: helloBase64}}

		err := sender.Send(data)

		require.NoError(t, err)
		require.Len(t, fake.requests, 2)
		assert.Equal(t, "/api/accounts/123456/messages/attachments", fake.requests[0].path)
		assert.Equal(t, "fileName=file.txt", fake.requests[0].query)
		assert.Equal(t, "application/octet-stream", fake.requests[0].contentType)
		assert.Equal(t, []byte("Hello World"), fake.requests[0].body)
		var sent zohoMailRequest
		require.NoError(t, json.Unmarshal(fake.requests[1].body, &sent))
		assert.Equal(t, []zohoAttachmentRef{{StoreName: "5386", AttachmentPath: "/Mail/abc-file.txt", AttachmentName: "file.txt"}}, sent.Attachments)
	})

	t.Run("given a failed upload then return error without sending", func(t *testing.T) {
		fake := &fakeZohoMail{uploadStatus: http.StatusInternalServerError, uploadResults: map[string]string{}}
		sender, _ := newZohoMailTestSender(t, fake)
		data := newValidEmail()
		data.Attachments = &[]model.Attachment{{Name: testFileName, Type: testFileType, Data: helloBase64}}

		err := sender.Send(data)

		require.Error(t, err)
		assert.Contains(t, err.Error(), `failed to upload attachment "file.txt"`)
		assert.Len(t, fake.requests, 1)
	})

	t.Run("given a Zoho error response then describe it", func(t *testing.T) {
		fake := &fakeZohoMail{
			sendStatus:   http.StatusBadRequest,
			sendResponse: `{"status":{"code":400,"description":"Invalid Input"},"data":{"errorCode":"INVALID_DATA","moreInfo":"fromAddress not allowed"}}`,
		}
		sender, _ := newZohoMailTestSender(t, fake)

		err := sender.Send(newValidEmail())

		require.EqualError(t, err, "zoho mail API returned status 400: Invalid Input: INVALID_DATA (fromAddress not allowed)")
		assert.Equal(t, model.CodeAPIInvalidRequest, mailer.CodeOf(err))
	})

	t.Run("given a 401 then invalidate the token for the next attempt", func(t *testing.T) {
		fake := &fakeZohoMail{sendStatus: http.StatusUnauthorized, sendResponse: `{"status":{"code":401,"description":"Invalid OAuth token"}}`}
		sender, ts := newZohoMailTestSender(t, fake)

		err := sender.Send(newValidEmail())
		require.Error(t, err)
		require.Error(t, sender.Send(newValidEmail()))

		assert.Equal(t, model.CodeAPIAuthorizationDenied, mailer.CodeOf(err))

		assert.Equal(t, int32(2), ts.calls.Load())
	})

	t.Run("given a token refresh failure then return error without calling the api", func(t *testing.T) {
		fake := &fakeZohoMail{sendStatus: http.StatusOK}
		api := httptest.NewServer(http.HandlerFunc(fake.handler))
		t.Cleanup(api.Close)
		ts := newTokenServer(t, http.StatusOK, `{"error":"invalid_client"}`)
		sender := NewZohoMailSender(newTestZohoConfig(ts.server.URL, api.URL), api.Client())

		err := sender.Send(newValidEmail())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid_client")
		assert.Equal(t, model.CodeAPIAuthorizationDenied, mailer.CodeOf(err))
		assert.Empty(t, fake.requests)
	})

	t.Run("given an invalid email then fail before requesting a token", func(t *testing.T) {
		sender, ts := newZohoMailTestSender(t, &fakeZohoMail{})
		data := newValidEmail()
		data.Receiver = nil

		require.EqualError(t, sender.Send(data), "recipient email cannot be empty")
		assert.Equal(t, int32(0), ts.calls.Load())
	})
}

func TestClassifyZohoMailError(t *testing.T) {
	t.Run("given only a description then describe it and classify by status", func(t *testing.T) {
		detail, code := classifyZohoMailError(401, []byte(`{"status":{"code":401,"description":"Invalid OAuth token"},"data":{"moreInfo":"to address"}}`))

		assert.Equal(t, "Invalid OAuth token (to address)", detail)
		assert.Empty(t, code)
	})

	t.Run("given moreInfo about the recipient then invalid receiver", func(t *testing.T) {
		_, code := classifyZohoMailError(400, []byte(`{"status":{"code":400,"description":"Invalid Input"},"data":{"errorCode":"INVALID_DATA","moreInfo":"Invalid to address"}}`))

		assert.Equal(t, model.CodeAPIInvalidReceiver, code)
	})

	t.Run("given a non-json body then return empty", func(t *testing.T) {
		detail, code := classifyZohoMailError(500, []byte("oops"))

		assert.Empty(t, detail)
		assert.Empty(t, code)
	})
}
