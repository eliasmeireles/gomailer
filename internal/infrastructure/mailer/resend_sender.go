package mailer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/eliasmeireles/gomailer/internal/application/config"
	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

type resendSender struct {
	config     config.ResendConfig
	httpClient *http.Client
}

type resendEmailRequest struct {
	From        string             `json:"from"`
	To          []string           `json:"to"`
	Cc          []string           `json:"cc,omitempty"`
	Bcc         []string           `json:"bcc,omitempty"`
	Subject     string             `json:"subject"`
	HTML        string             `json:"html"`
	Attachments []resendAttachment `json:"attachments,omitempty"`
}

type resendAttachment struct {
	Filename    string `json:"filename"`
	Content     string `json:"content"`
	ContentType string `json:"content_type,omitempty"`
}

type resendEmailResponse struct {
	ID string `json:"id"`
}

type resendErrorResponse struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// NewResendSender creates a Sender that delivers through the Resend API (POST {BaseURL}/emails).
func NewResendSender(cfg config.ResendConfig, httpClient *http.Client) mailer.Sender {
	return &resendSender{config: cfg, httpClient: httpClient}
}

// Send posts the email to Resend and returns an error for any non-2xx response.
func (s *resendSender) Send(data model.SendEmailData) error {
	if err := validateEmail(data); err != nil {
		return err
	}

	req, err := newJSONRequest(http.MethodPost, s.config.BaseURL+"/emails", buildResendRequest(data),
		map[string]string{"Authorization": "Bearer " + s.config.APIKey})
	if err != nil {
		return err
	}

	var result resendEmailResponse
	if err := doRequest(s.httpClient, req, &result); err != nil {
		return describeAPIError("resend", err, classifyResendError)
	}

	log.Debugf("Resend accepted email to %s, id: %s", data.Receiver, result.ID)
	return nil
}

func buildResendRequest(data model.SendEmailData) resendEmailRequest {
	request := resendEmailRequest{
		From:    data.From,
		To:      data.Receiver,
		Cc:      data.Cc,
		Bcc:     data.Bcc,
		Subject: data.Subject,
		HTML:    data.Body,
	}

	for _, attachment := range decodeAttachments(data) {
		request.Attachments = append(request.Attachments, resendAttachment{
			Filename:    attachment.Name,
			Content:     base64.StdEncoding.EncodeToString(attachment.Content),
			ContentType: attachment.Type,
		})
	}
	return request
}

// classifyResendError extracts "name: message" from a Resend error body and maps the documented
// error names (https://resend.com/docs/api-reference/errors) to error codes.
func classifyResendError(status int, body []byte) (string, model.ErrorCode) {
	var apiErr resendErrorResponse
	if err := json.Unmarshal(body, &apiErr); err != nil || apiErr.Message == "" {
		return "", ""
	}
	return fmt.Sprintf("%s: %s", apiErr.Name, apiErr.Message), resendErrorCode(status, apiErr)
}

func resendErrorCode(status int, apiErr resendErrorResponse) model.ErrorCode {
	// An invalid key comes back as 401 "validation_error: API key is invalid", not invalid_api_key.
	if status == http.StatusUnauthorized {
		return model.CodeAPIAuthorizationDenied
	}

	switch apiErr.Name {
	case "missing_api_key", "invalid_api_key", "restricted_api_key", "suspended_api_key", "invalid_permission", "invalid_access":
		return model.CodeAPIAuthorizationDenied
	case "invalid_from_address":
		return model.CodeAPISenderNotAllowed
	case "invalid_attachment":
		return model.CodeAPIInvalidAttachment
	case "daily_quota_exceeded", "monthly_quota_exceeded":
		return model.CodeAPIQuotaExceeded
	case "rate_limit_exceeded":
		return model.CodeAPIRateLimited
	case "application_error", "internal_server_error", "service_unavailable":
		return model.CodeAPIProviderUnavailable
	case "validation_error":
		if status == http.StatusForbidden {
			return model.CodeAPISenderNotAllowed
		}
		return fieldCode(apiErr.Message, model.CodeAPIInvalidRequest)
	default:
		return ""
	}
}
