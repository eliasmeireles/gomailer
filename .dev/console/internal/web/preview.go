package web

import (
	"encoding/json"
	"fmt"

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

func shorten(value string) string {
	if len(value) <= previewMaxChars {
		return value
	}
	return fmt.Sprintf("%s… (%d chars)", value[:previewMaxChars], len(value))
}
