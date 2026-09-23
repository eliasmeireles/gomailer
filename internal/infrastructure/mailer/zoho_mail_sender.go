package mailer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/eliasmeireles/gomailer/internal/application/config"
	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

type zohoMailSender struct {
	config     config.ZohoMailConfig
	httpClient *http.Client
	tokens     *zohoTokenSource
}

type zohoMailRequest struct {
	FromAddress string              `json:"fromAddress"`
	ToAddress   string              `json:"toAddress"`
	CcAddress   string              `json:"ccAddress,omitempty"`
	BccAddress  string              `json:"bccAddress,omitempty"`
	Subject     string              `json:"subject"`
	Content     string              `json:"content"`
	MailFormat  string              `json:"mailFormat"`
	Attachments []zohoAttachmentRef `json:"attachments,omitempty"`
}

type zohoErrorResponse struct {
	Status struct {
		Description string `json:"description"`
	} `json:"status"`
	Data struct {
		ErrorCode string `json:"errorCode"`
		MoreInfo  string `json:"moreInfo"`
	} `json:"data"`
}

// NewZohoMailSender creates a Sender that delivers through the Zoho Mail API
// (POST {APIURL}/api/accounts/{AccountID}/messages), authenticating with an OAuth refresh token.
// The OAuth client needs the ZohoMail.messages.CREATE (or ZohoMail.messages.ALL) scope.
func NewZohoMailSender(cfg config.ZohoMailConfig, httpClient *http.Client) mailer.Sender {
	return &zohoMailSender{config: cfg, httpClient: httpClient, tokens: newZohoTokenSource(cfg, httpClient)}
}

// Send uploads the attachments (if any) and sends the email. A 401 from Zoho drops the cached
// access token so the next attempt refreshes it.
func (s *zohoMailSender) Send(data model.SendEmailData) error {
	if err := validateEmail(data); err != nil {
		return err
	}

	token, err := s.tokens.Token()
	if err != nil {
		return err
	}

	refs, err := s.uploadAttachments(token, decodeAttachments(data))
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/api/accounts/%s/messages", s.config.APIURL, url.PathEscape(s.config.AccountID))
	req, err := newJSONRequest(http.MethodPost, endpoint, buildZohoMailRequest(data, refs),
		map[string]string{"Authorization": zohoAuthorization(token)})
	if err != nil {
		return err
	}
	return s.call(req, nil)
}

// call executes a Zoho Mail API request, invalidating the token on 401 and describing errors.
func (s *zohoMailSender) call(req *http.Request, out any) error {
	err := doRequest(s.httpClient, req, out)
	if err == nil {
		return nil
	}

	if statusErr, ok := err.(*httpStatusError); ok && statusErr.StatusCode == http.StatusUnauthorized {
		s.tokens.Invalidate()
	}
	return describeAPIError("zoho mail", err, classifyZohoMailError)
}

func buildZohoMailRequest(data model.SendEmailData, refs []zohoAttachmentRef) zohoMailRequest {
	return zohoMailRequest{
		FromAddress: data.From,
		ToAddress:   strings.Join(data.Receiver, ","),
		CcAddress:   strings.Join(data.Cc, ","),
		BccAddress:  strings.Join(data.Bcc, ","),
		Subject:     data.Subject,
		Content:     data.Body,
		MailFormat:  "html",
		Attachments: refs,
	}
}

func zohoAuthorization(token string) string {
	return "Zoho-oauthtoken " + token
}

// classifyZohoMailError renders "description: errorCode (moreInfo)" from a Zoho Mail error body;
// the code comes from the field mentioned in moreInfo, falling back to the HTTP status.
func classifyZohoMailError(status int, body []byte) (string, model.ErrorCode) {
	var apiErr zohoErrorResponse
	if err := json.Unmarshal(body, &apiErr); err != nil || apiErr.Status.Description == "" {
		return "", ""
	}

	code := model.ErrorCode("")
	if status != http.StatusUnauthorized {
		code = fieldCode(apiErr.Data.MoreInfo, "")
	}
	return describeZohoMailError(apiErr), code
}

func describeZohoMailError(apiErr zohoErrorResponse) string {

	description := apiErr.Status.Description
	if apiErr.Data.ErrorCode != "" {
		description += ": " + apiErr.Data.ErrorCode
	}
	if apiErr.Data.MoreInfo != "" {
		description += " (" + apiErr.Data.MoreInfo + ")"
	}
	return description
}
