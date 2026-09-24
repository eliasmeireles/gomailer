package message

import (
	"encoding/base64"
	"errors"
	"regexp"
	"strings"
)

// invalidBody is sent when Form.InvalidBody is set, to exercise the mailer failure path.
const invalidBody = "not-valid-base64!!!"

var addressSeparator = regexp.MustCompile(`[,;\n]+`)

// Build converts form into an Email. The body and attachments are base64-encoded, recipients
// are encoded as form.Format, and a blank ID is replaced by newID().
func Build(form Form, newID func() string) (Email, error) {
	if err := validate(form); err != nil {
		return Email{}, err
	}

	email := Email{
		ID:          strings.TrimSpace(form.ID),
		From:        strings.TrimSpace(form.From),
		Receiver:    encodeRecipients(SplitAddresses(form.To), form.Format),
		Cc:          encodeRecipients(SplitAddresses(form.Cc), form.Format),
		Bcc:         encodeRecipients(SplitAddresses(form.Bcc), form.Format),
		Subject:     strings.TrimSpace(form.Subject),
		Body:        encodeBody(form),
		Attachments: encodeFiles(form.Files),
		Callback:    buildCallback(form),
	}
	if email.ID == "" {
		email.ID = newID()
	}
	return email, nil
}

// SplitAddresses splits a list separated by commas, semicolons or new lines, dropping blanks.
func SplitAddresses(list string) []string {
	var addresses []string
	for _, address := range addressSeparator.Split(list, -1) {
		if trimmed := strings.TrimSpace(address); trimmed != "" {
			addresses = append(addresses, trimmed)
		}
	}
	return addresses
}

func validate(form Form) error {
	switch {
	case strings.TrimSpace(form.From) == "":
		return errors.New("sender (from) is required")
	case len(SplitAddresses(form.To)) == 0:
		return errors.New("at least one recipient (to) is required")
	case strings.TrimSpace(form.Subject) == "":
		return errors.New("subject is required")
	case !form.InvalidBody && strings.TrimSpace(form.HTML) == "":
		return errors.New("HTML body is required")
	case form.SuccessCallbackEnabled && strings.TrimSpace(form.SuccessCallbackURL) == "":
		return errors.New("success callback URL is required")
	case form.FailureCallbackEnabled && strings.TrimSpace(form.FailureCallbackURL) == "":
		return errors.New("failure callback URL is required")
	}
	return nil
}

func encodeRecipients(addresses []string, format RecipientsFormat) any {
	if len(addresses) == 0 {
		return nil
	}
	if format == FormatString {
		return strings.Join(addresses, ", ")
	}
	return addresses
}

func encodeBody(form Form) string {
	if form.InvalidBody {
		return invalidBody
	}
	return base64.StdEncoding.EncodeToString([]byte(form.HTML))
}

func encodeFiles(files []File) []Attachment {
	var attachments []Attachment
	for _, file := range files {
		attachments = append(attachments, Attachment{
			Name:    file.Name,
			Type:    contentType(file.Type),
			Data:    base64.StdEncoding.EncodeToString(file.Content),
			Decoder: "base64",
		})
	}
	return attachments
}

func contentType(value string) string {
	if value == "" {
		return "application/octet-stream"
	}
	return value
}

func buildCallback(form Form) *Callback {
	callback := &Callback{
		Success: buildTarget(form.SuccessCallbackEnabled, form.SuccessCallbackURL, form.CallbackAuthorization),
		Failure: buildTarget(form.FailureCallbackEnabled, form.FailureCallbackURL, form.CallbackAuthorization),
	}
	if callback.Success == nil && callback.Failure == nil {
		return nil
	}
	return callback
}

func buildTarget(enabled bool, url, authorization string) *CallbackTarget {
	if !enabled {
		return nil
	}

	target := &CallbackTarget{URL: strings.TrimSpace(url)}
	if auth := strings.TrimSpace(authorization); auth != "" {
		target.Headers = map[string]string{"Authorization": auth}
	}
	return target
}
