package mailer

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/eliasmeireles/gomailer/internal/core/model"
)

func TestBuildMessage(t *testing.T) {
	t.Run("given no attachments then render a plain html message", func(t *testing.T) {
		message := buildMessage(newValidEmail())

		assert.Contains(t, message, "From: "+testSender+"\r\n")
		assert.Contains(t, message, "To: "+testRecipient+"\r\n")
		assert.Contains(t, message, "Subject: "+testSubject+"\r\n")
		assert.Contains(t, message, "MIME-Version: 1.0\r\n")
		assert.Contains(t, message, "Content-Type: text/html; charset=UTF-8\r\n\r\n"+testBody)
		assert.NotContains(t, message, "multipart/mixed")
	})

	t.Run("given attachments then render a multipart message", func(t *testing.T) {
		data := newValidEmail()
		data.Attachments = &[]model.Attachment{{Name: testFileName, Type: testFileType, Data: helloBase64}}

		message := buildMessage(data)

		assert.Contains(t, message, "Content-Type: multipart/mixed; boundary=")
		assert.Contains(t, message, base64.StdEncoding.EncodeToString([]byte(testBody)))
		assert.Contains(t, message, "Content-Type: "+testFileType)
		assert.Contains(t, message, "Content-Disposition: attachment; filename=\""+testFileName+"\"")
		assert.Contains(t, message, helloBase64)
		assert.True(t, strings.HasSuffix(message, "--\r\n"), "message must close the multipart boundary")
	})

	t.Run("given cc and bcc then add the Cc header and never a Bcc header", func(t *testing.T) {
		data := newValidEmail()
		data.Cc = model.Recipients{"cc1@example.com", "cc2@example.com"}
		data.Bcc = model.Recipients{"bcc@example.com"}

		message := buildMessage(data)

		assert.Contains(t, message, "Cc: cc1@example.com, cc2@example.com\r\n")
		assert.NotContains(t, message, "Bcc")
		assert.NotContains(t, message, "bcc@example.com")
	})

	t.Run("given no cc then omit the Cc header", func(t *testing.T) {
		assert.NotContains(t, buildMessage(newValidEmail()), "Cc:")
	})

	t.Run("given only invalid attachments then render a plain html message", func(t *testing.T) {
		data := newValidEmail()
		data.Attachments = &[]model.Attachment{{Name: testFileName, Type: testFileType, Data: "invalid!@#"}}

		assert.NotContains(t, buildMessage(data), "multipart/mixed")
	})
}

func TestWriteBase64Lines(t *testing.T) {
	t.Run("must split encoded content into 76-character lines", func(t *testing.T) {
		var builder strings.Builder

		writeBase64Lines(&builder, []byte(strings.Repeat("a", 100)))

		lines := strings.Split(strings.TrimSuffix(builder.String(), "\r\n\r\n"), "\r\n")
		assert.Len(t, lines, 2)
		assert.Len(t, lines[0], base64LineLength)
	})
}
