package mailer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/eliasmeireles/gomailer/internal/application/config"
	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

type zeptoMailSender struct {
	config     config.ZeptoMailConfig
	httpClient *http.Client
}

type zeptoMailAddress struct {
	Address string `json:"address"`
}

type zeptoMailRecipient struct {
	EmailAddress zeptoMailAddress `json:"email_address"`
}

type zeptoMailAttachment struct {
	Name     string `json:"name"`
	MimeType string `json:"mime_type"`
	Content  string `json:"content"`
}

type zeptoMailRequest struct {
	From        zeptoMailAddress      `json:"from"`
	To          []zeptoMailRecipient  `json:"to"`
	Cc          []zeptoMailRecipient  `json:"cc,omitempty"`
	Bcc         []zeptoMailRecipient  `json:"bcc,omitempty"`
	Subject     string                `json:"subject"`
	HTMLBody    string                `json:"htmlbody"`
	Attachments []zeptoMailAttachment `json:"attachments,omitempty"`
}

type zeptoMailResponse struct {
	RequestID string `json:"request_id"`
}

type zeptoMailMessage struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type zeptoMailErrorResponse struct {
	Error *struct {
		zeptoMailMessage
		Details []zeptoMailMessage `json:"details"`
	} `json:"error"`
}

// NewZeptoMailSender creates a Sender that delivers through the ZeptoMail API (POST {BaseURL}/v1.1/email).
func NewZeptoMailSender(cfg config.ZeptoMailConfig, httpClient *http.Client) mailer.Sender {
	return &zeptoMailSender{config: cfg, httpClient: httpClient}
}

// Send posts the email to ZeptoMail and returns an error for any non-2xx response.
func (s *zeptoMailSender) Send(data model.SendEmailData) error {
	if err := validateEmail(data); err != nil {
		return err
	}

	req, err := newJSONRequest(http.MethodPost, s.config.BaseURL+"/v1.1/email", buildZeptoMailRequest(data),
		map[string]string{"Authorization": s.config.Authorization})
	if err != nil {
		return err
	}

	var result zeptoMailResponse
	if err := doRequest(s.httpClient, req, &result); err != nil {
		return describeAPIError("zeptomail", err, classifyZeptoMailError)
	}

	log.Debugf("ZeptoMail accepted email to %s, request id: %s", data.Receiver, result.RequestID)
	return nil
}

func buildZeptoMailRequest(data model.SendEmailData) zeptoMailRequest {
	request := zeptoMailRequest{
		From:     zeptoMailAddress{Address: data.From},
		To:       zeptoMailRecipients(data.Receiver),
		Cc:       zeptoMailRecipients(data.Cc),
		Bcc:      zeptoMailRecipients(data.Bcc),
		Subject:  data.Subject,
		HTMLBody: data.Body,
	}

	for _, attachment := range decodeAttachments(data) {
		request.Attachments = append(request.Attachments, zeptoMailAttachment{
			Name:     attachment.Name,
			MimeType: attachment.Type,
			Content:  base64.StdEncoding.EncodeToString(attachment.Content),
		})
	}
	return request
}

func zeptoMailRecipients(addresses model.Recipients) []zeptoMailRecipient {
	var recipients []zeptoMailRecipient
	for _, address := range addresses {
		recipients = append(recipients, zeptoMailRecipient{EmailAddress: zeptoMailAddress{Address: address}})
	}
	return recipients
}

// classifyZeptoMailError renders "CODE: message (DETAIL_CODE: detail, ...)" from a ZeptoMail error
// body and maps the documented codes (https://www.zoho.com/zeptomail/help/api/error-codes.html).
func classifyZeptoMailError(_ int, body []byte) (string, model.ErrorCode) {
	var apiErr zeptoMailErrorResponse
	if err := json.Unmarshal(body, &apiErr); err != nil || apiErr.Error == nil {
		return "", ""
	}

	details := make([]string, 0, len(apiErr.Error.Details))
	code := model.ErrorCode("")
	for _, detail := range apiErr.Error.Details {
		details = append(details, fmt.Sprintf("%s: %s", detail.Code, detail.Message))
		if code == "" {
			code = zeptoMailDetailCode(detail)
		}
	}
	if code == "" {
		code = zeptoMailCode(apiErr.Error.Code)
	}

	description := fmt.Sprintf("%s: %s", apiErr.Error.Code, apiErr.Error.Message)
	if len(details) > 0 {
		description += " (" + strings.Join(details, ", ") + ")"
	}
	return description, code
}

func zeptoMailDetailCode(detail zeptoMailMessage) model.ErrorCode {
	switch detail.Code {
	case "SERR_157":
		return model.CodeAPIAuthorizationDenied
	case "SM_111":
		return model.CodeAPISenderNotAllowed
	case "SM_113":
		return fieldCode(detail.Message, model.CodeAPIInvalidRequest)
	case "SM_133", "SMI_115", "LE_102":
		return model.CodeAPIQuotaExceeded
	case "GE_102", "SM_127":
		return model.CodeAPIInvalidRequest
	default:
		return ""
	}
}

func zeptoMailCode(code string) model.ErrorCode {
	switch code {
	case "TM_3601":
		return model.CodeAPIAuthorizationDenied
	case "TM_5001":
		return model.CodeAPIQuotaExceeded
	case "TM_3201", "TM_8001":
		return model.CodeAPIInvalidRequest
	default:
		return ""
	}
}
