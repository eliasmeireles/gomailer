package mailer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

const maxErrorBodyBytes = 4096

// httpStatusError is returned by doRequest for non-2xx responses; Body is capped at maxErrorBodyBytes.
type httpStatusError struct {
	StatusCode int
	Body       []byte
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("status %d: %s", e.StatusCode, e.Body)
}

// newJSONRequest builds a request with payload encoded as JSON and the given headers.
func newJSONRequest(method, url string, payload any, headers map[string]string) (*http.Request, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return req, nil
}

// doRequest executes req and, on a 2xx response, decodes the JSON body into out (when non-nil).
// Non-2xx responses return *httpStatusError.
func doRequest(client *http.Client, req *http.Request, out any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return &httpStatusError{StatusCode: resp.StatusCode, Body: body}
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	return nil
}

// apiErrorClassifier extracts a readable detail and an error code from a provider error response.
// Empty results fall back to the raw body and to codeForStatus.
type apiErrorClassifier func(status int, body []byte) (detail string, code model.ErrorCode)

// describeAPIError renders err as "<provider> API returned status N: <detail>" classified with
// an error code; transport errors are classified as api_connection_failed.
func describeAPIError(provider string, err error, classify apiErrorClassifier) error {
	statusErr, ok := err.(*httpStatusError)
	if !ok {
		return mailer.Errorf(model.CodeAPIConnectionFailed, "failed to call %s API: %w", provider, err)
	}

	detail, code := classify(statusErr.StatusCode, statusErr.Body)
	if detail == "" {
		detail = string(statusErr.Body)
	}
	if code == "" {
		code = codeForStatus(statusErr.StatusCode)
	}
	return mailer.Errorf(code, "%s API returned status %d: %s", provider, statusErr.StatusCode, detail)
}

// noClassification is used for providers whose error bodies carry no extra information.
func noClassification(int, []byte) (string, model.ErrorCode) { return "", "" }

// codeForStatus is the default classification of an HTTP error status.
func codeForStatus(status int) model.ErrorCode {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return model.CodeAPIAuthorizationDenied
	case status == http.StatusTooManyRequests:
		return model.CodeAPIRateLimited
	case status == http.StatusRequestEntityTooLarge:
		return model.CodeAPIInvalidAttachment
	case status >= http.StatusInternalServerError:
		return model.CodeAPIProviderUnavailable
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		return model.CodeAPIInvalidRequest
	default:
		return model.CodeAPIUnexpectedResponse
	}
}

// fieldCode classifies a provider validation message by the field it mentions: from ->
// api_sender_not_allowed, to/cc/bcc/recipient -> api_invalid_receiver, otherwise fallback.
func fieldCode(message string, fallback model.ErrorCode) model.ErrorCode {
	lower := strings.ToLower(message)
	switch {
	case mentionsAny(lower, "from"):
		return model.CodeAPISenderNotAllowed
	case mentionsAny(lower, "to", "cc", "bcc") || strings.Contains(lower, "recipient"):
		return model.CodeAPIInvalidReceiver
	default:
		return fallback
	}
}

// mentionsAny reports whether message names a field as `f`, 'f', "f" or "f address/field".
func mentionsAny(message string, fields ...string) bool {
	for _, field := range fields {
		for _, form := range []string{"`" + field + "`", "'" + field + "'", `"` + field + `"`, " " + field + " address", " " + field + " field"} {
			if strings.Contains(message, form) {
				return true
			}
		}
	}
	return false
}
