package web

import (
	"encoding/json"
	"fmt"

	"github.com/eliasmeireles/gomailer/dev/console/internal/api"
	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
)

const previewMaxChars = 80

// previewJSON renders the published message with long base64 values shortened for display.
func previewJSON(email message.Email) string {
	preview := email
	preview.Body = shorten(email.Body)
	preview.Attachments = make([]message.Attachment, len(email.Attachments))
	for i, attachment := range email.Attachments {
		attachment.Data = shorten(attachment.Data)
		preview.Attachments[i] = attachment
	}
	if len(preview.Attachments) == 0 {
		preview.Attachments = nil
	}

	raw, _ := json.MarshalIndent(preview, "", "  ")
	return string(raw)
}

// responseJSON renders the HTTP source answer (event, or raw body for non-event errors).
func responseJSON(response *api.Response) string {
	if response == nil {
		return ""
	}
	if response.Raw != "" {
		return response.Raw
	}
	raw, _ := json.MarshalIndent(response.Event, "", "  ")
	return string(raw)
}

func shorten(value string) string {
	if len(value) <= previewMaxChars {
		return value
	}
	return fmt.Sprintf("%s… (%d chars)", value[:previewMaxChars], len(value))
}
