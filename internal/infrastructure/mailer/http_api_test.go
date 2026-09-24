package mailer

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

func newStaticServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestNewJSONRequest(t *testing.T) {
	t.Run("given a payload and headers then build a json request", func(t *testing.T) {
		req, err := newJSONRequest(http.MethodPost, "https://example.com/x", map[string]string{"a": "b"}, map[string]string{"X-Key": "v"})

		require.NoError(t, err)
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		assert.JSONEq(t, `{"a":"b"}`, string(body))
		assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
		assert.Equal(t, "application/json", req.Header.Get("Accept"))
		assert.Equal(t, "v", req.Header.Get("X-Key"))
	})

	t.Run("given an unencodable payload then return error", func(t *testing.T) {
		_, err := newJSONRequest(http.MethodPost, "https://example.com/x", make(chan int), nil)

		require.Error(t, err)
	})

	t.Run("given an invalid url then return error", func(t *testing.T) {
		_, err := newJSONRequest(http.MethodPost, "://bad", nil, nil)

		require.Error(t, err)
	})
}

func TestDoRequest(t *testing.T) {
	t.Run("given a 2xx json response then decode it", func(t *testing.T) {
		server := newStaticServer(t, http.StatusCreated, `{"id":"abc"}`)
		req, err := http.NewRequest(http.MethodGet, server.URL, nil)
		require.NoError(t, err)
		var out struct{ ID string }

		require.NoError(t, doRequest(server.Client(), req, &out))
		assert.Equal(t, "abc", out.ID)
	})

	t.Run("given a 2xx response and nil out then ignore the body", func(t *testing.T) {
		server := newStaticServer(t, http.StatusOK, "not json")
		req, err := http.NewRequest(http.MethodGet, server.URL, nil)
		require.NoError(t, err)

		require.NoError(t, doRequest(server.Client(), req, nil))
	})

	t.Run("given an invalid 2xx json body then return a decode error", func(t *testing.T) {
		server := newStaticServer(t, http.StatusOK, "not json")
		req, err := http.NewRequest(http.MethodGet, server.URL, nil)
		require.NoError(t, err)
		var out struct{}

		err = doRequest(server.Client(), req, &out)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to decode response")
	})

	t.Run("given a non-2xx response then return a capped status error", func(t *testing.T) {
		server := newStaticServer(t, http.StatusBadRequest, strings.Repeat("x", maxErrorBodyBytes*2))
		req, err := http.NewRequest(http.MethodGet, server.URL, nil)
		require.NoError(t, err)

		err = doRequest(server.Client(), req, nil)

		var statusErr *httpStatusError
		require.ErrorAs(t, err, &statusErr)
		assert.Equal(t, http.StatusBadRequest, statusErr.StatusCode)
		assert.Len(t, statusErr.Body, maxErrorBodyBytes)
	})
}

func TestDescribeAPIError(t *testing.T) {
	classify := func(_ int, body []byte) (string, model.ErrorCode) {
		return strings.ToUpper(string(body)), model.CodeAPIQuotaExceeded
	}

	t.Run("given a status error then use the classified detail and code", func(t *testing.T) {
		err := describeAPIError("acme", &httpStatusError{StatusCode: 403, Body: []byte("denied")}, classify)

		require.EqualError(t, err, "acme API returned status 403: DENIED")
		assert.Equal(t, model.CodeAPIQuotaExceeded, mailer.CodeOf(err))
	})

	t.Run("given no classification then fall back to the raw body and status code", func(t *testing.T) {
		err := describeAPIError("acme", &httpStatusError{StatusCode: 503, Body: []byte("raw")}, noClassification)

		require.EqualError(t, err, "acme API returned status 503: raw")
		assert.Equal(t, model.CodeAPIProviderUnavailable, mailer.CodeOf(err))
	})

	t.Run("given a transport error then classify as connection failed", func(t *testing.T) {
		cause := errors.New("connection refused")

		err := describeAPIError("acme", cause, classify)

		require.ErrorIs(t, err, cause)
		assert.EqualError(t, err, "failed to call acme API: connection refused")
		assert.Equal(t, model.CodeAPIConnectionFailed, mailer.CodeOf(err))
	})
}

func TestCodeForStatus(t *testing.T) {
	cases := map[int]model.ErrorCode{
		401: model.CodeAPIAuthorizationDenied,
		403: model.CodeAPIAuthorizationDenied,
		400: model.CodeAPIInvalidRequest,
		422: model.CodeAPIInvalidRequest,
		413: model.CodeAPIInvalidAttachment,
		429: model.CodeAPIRateLimited,
		500: model.CodeAPIProviderUnavailable,
		503: model.CodeAPIProviderUnavailable,
		404: model.CodeAPIUnexpectedResponse,
	}

	for status, expected := range cases {
		t.Run(fmt.Sprintf("given status %d then return %s", status, expected), func(t *testing.T) {
			assert.Equal(t, expected, codeForStatus(status))
		})
	}
}

func TestFieldCode(t *testing.T) {
	cases := map[string]model.ErrorCode{
		"Invalid `to` field. The email address needs to follow the format.": model.CodeAPIInvalidReceiver,
		"Invalid to address":                  model.CodeAPIInvalidReceiver,
		"Invalid 'cc' value":                  model.CodeAPIInvalidReceiver,
		"Recipient not allowed":               model.CodeAPIInvalidReceiver,
		"Invalid `from` field.":               model.CodeAPISenderNotAllowed,
		"Invalid from address":                model.CodeAPISenderNotAllowed,
		"The subject is too long to be sent.": model.CodeAPIInvalidRequest,
	}

	for message, expected := range cases {
		t.Run("given "+message+" then return "+string(expected), func(t *testing.T) {
			assert.Equal(t, expected, fieldCode(message, model.CodeAPIInvalidRequest))
		})
	}
}

func TestHTTPStatusErrorMessage(t *testing.T) {
	t.Run("must include status and body", func(t *testing.T) {
		assert.EqualError(t, &httpStatusError{StatusCode: 418, Body: []byte("teapot")}, "status 418: teapot")
	})
}
