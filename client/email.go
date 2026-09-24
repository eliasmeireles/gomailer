package client

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

// ErrInvalidEmail is returned (wrapped) by Validate and by every Sender for incomplete emails.
var ErrInvalidEmail = errors.New("invalid email")

// Email is an email request handled by gomailer.
type Email struct {
	// ID identifies the request in logs and callbacks; a UUID is generated when empty.
	ID      string
	From    string
	To      []string
	Cc      []string
	Bcc     []string
	Subject string
	// HTML is the plain HTML body; it is base64-encoded on the wire.
	HTML        string
	Attachments []Attachment
	// Callback is optional: endpoints notified with the delivery outcome.
	Callback *Callback
}

// Attachment is a file sent with the email; Content holds the raw bytes.
type Attachment struct {
	Name        string
	ContentType string
	Content     []byte
}

// Callback holds the optional endpoints notified after delivery succeeds or fails.
type Callback struct {
	Success *CallbackTarget `json:"success,omitempty"`
	Failure *CallbackTarget `json:"failure,omitempty"`
}

// CallbackTarget is an endpoint that receives a DeliveryEvent.
type CallbackTarget struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// Validate reports missing required fields (from, at least one recipient, subject and body).
func (e Email) Validate() error {
	var missing []string
	if strings.TrimSpace(e.From) == "" {
		missing = append(missing, "from")
	}
	if len(nonBlank(e.To)) == 0 {
		missing = append(missing, "to")
	}
	if strings.TrimSpace(e.Subject) == "" {
		missing = append(missing, "subject")
	}
	if strings.TrimSpace(e.HTML) == "" {
		missing = append(missing, "html")
	}
	if len(missing) > 0 {
		return errors.Join(ErrInvalidEmail, errors.New("missing "+strings.Join(missing, ", ")))
	}
	return nil
}

// Prepare validates the email and fills a missing ID, returning the email ready to be sent.
func Prepare(email Email) (Email, error) {
	if err := email.Validate(); err != nil {
		return Email{}, err
	}
	if strings.TrimSpace(email.ID) == "" {
		email.ID = NewID()
	}
	return email, nil
}

// wireEmail is the JSON message consumed by gomailer.
type wireEmail struct {
	ID          string           `json:"id,omitempty"`
	From        string           `json:"from"`
	Receiver    []string         `json:"receiver"`
	Cc          []string         `json:"cc,omitempty"`
	Bcc         []string         `json:"bcc,omitempty"`
	Subject     string           `json:"subject"`
	Body        string           `json:"body"`
	Attachments []wireAttachment `json:"attachments,omitempty"`
	Callback    *Callback        `json:"callback,omitempty"`
}

type wireAttachment struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Data    string `json:"data"`
	Decoder string `json:"decoder"`
}

// MarshalJSON encodes the email in the gomailer wire format (base64 body and attachments,
// recipients as arrays).
func (e Email) MarshalJSON() ([]byte, error) {
	wire := wireEmail{
		ID:       e.ID,
		From:     strings.TrimSpace(e.From),
		Receiver: nonBlank(e.To),
		Cc:       nonBlank(e.Cc),
		Bcc:      nonBlank(e.Bcc),
		Subject:  e.Subject,
		Body:     base64.StdEncoding.EncodeToString([]byte(e.HTML)),
		Callback: e.Callback,
	}
	for _, attachment := range e.Attachments {
		wire.Attachments = append(wire.Attachments, wireAttachment{
			Name:    attachment.Name,
			Type:    contentType(attachment.ContentType),
			Data:    base64.StdEncoding.EncodeToString(attachment.Content),
			Decoder: "base64",
		})
	}
	return json.Marshal(wire)
}

func nonBlank(values []string) []string {
	var result []string
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func contentType(value string) string {
	if value == "" {
		return "application/octet-stream"
	}
	return value
}
