package mailer

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/eliasmeireles/gomailer/internal/core/model"
)

const base64LineLength = 76

// buildMessage renders the MIME message: plain HTML without attachments, multipart/mixed otherwise.
func buildMessage(data model.SendEmailData) string {
	var message strings.Builder

	message.WriteString(fmt.Sprintf("From: %s\r\n", data.From))
	message.WriteString(fmt.Sprintf("To: %s\r\n", data.Receiver))
	if len(data.Cc) > 0 {
		message.WriteString(fmt.Sprintf("Cc: %s\r\n", data.Cc))
	}
	message.WriteString(fmt.Sprintf("Subject: %s\r\n", data.Subject))
	message.WriteString("MIME-Version: 1.0\r\n")

	attachments := decodeAttachments(data)
	if len(attachments) == 0 {
		message.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		message.WriteString(data.Body)
		return message.String()
	}

	boundary := fmt.Sprintf("boundary_%d", len(data.Body)+len(attachments))
	message.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n\r\n", boundary))

	message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	message.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	writeBase64Lines(&message, []byte(data.Body))

	for _, attachment := range attachments {
		message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		message.WriteString(fmt.Sprintf("Content-Type: %s\r\n", attachment.Type))
		message.WriteString("Content-Transfer-Encoding: base64\r\n")
		message.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", attachment.Name))
		writeBase64Lines(&message, attachment.Content)
	}

	message.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	return message.String()
}

// writeBase64Lines writes content as base64 split into RFC 2045 76-character lines.
func writeBase64Lines(message *strings.Builder, content []byte) {
	encoded := base64.StdEncoding.EncodeToString(content)
	for start := 0; start < len(encoded); start += base64LineLength {
		end := min(start+base64LineLength, len(encoded))
		message.WriteString(encoded[start:end])
		message.WriteString("\r\n")
	}
	message.WriteString("\r\n")
}
