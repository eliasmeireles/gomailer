package mailer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/core/decoder"
	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

const (
	testSender    = "no-reply@example.com"
	testRecipient = "jane@example.com"
	testSubject   = "Test subject"
	testBody      = "<p>Hello</p>"
	testFileName  = "file.txt"
	testFileType  = "text/plain"
	helloBase64   = "SGVsbG8gV29ybGQ="
)

func newValidEmail() model.SendEmailData {
	return model.SendEmailData{From: testSender, Receiver: model.Recipients{testRecipient}, Subject: testSubject, Body: testBody}
}

func TestValidateEmail(t *testing.T) {
	cases := map[string]struct {
		mutate   func(*model.SendEmailData)
		expected string
		code     model.ErrorCode
	}{
		"given blank from then reject":     {func(d *model.SendEmailData) { d.From = "  " }, "sender email cannot be empty", model.CodeMessageMissingSender},
		"given blank receiver then reject": {func(d *model.SendEmailData) { d.Receiver = nil }, "recipient email cannot be empty", model.CodeMessageMissingReceiver},
		"given blank subject then reject":  {func(d *model.SendEmailData) { d.Subject = " " }, "subject cannot be empty", model.CodeMessageMissingSubject},
		"given blank body then reject":     {func(d *model.SendEmailData) { d.Body = "" }, "body cannot be empty", model.CodeMessageMissingBody},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			data := newValidEmail()
			tc.mutate(&data)

			err := validateEmail(data)

			require.EqualError(t, err, tc.expected)
			assert.Equal(t, tc.code, mailer.CodeOf(err))
		})
	}

	t.Run("given a complete email then accept", func(t *testing.T) {
		require.NoError(t, validateEmail(newValidEmail()))
	})
}

func TestAllRecipients(t *testing.T) {
	t.Run("given receiver, cc and bcc then return all of them in order", func(t *testing.T) {
		data := newValidEmail()
		data.Cc = model.Recipients{"cc1@example.com", "cc2@example.com"}
		data.Bcc = model.Recipients{"bcc@example.com"}

		assert.Equal(t,
			[]string{testRecipient, "cc1@example.com", "cc2@example.com", "bcc@example.com"},
			allRecipients(data),
		)
	})

	t.Run("given only receiver then return it", func(t *testing.T) {
		assert.Equal(t, []string{testRecipient}, allRecipients(newValidEmail()))
	})
}

func TestDecodeAttachment(t *testing.T) {
	t.Run("given no decoder then default to base64", func(t *testing.T) {
		content, err := decodeAttachment(model.Attachment{Data: helloBase64})

		require.NoError(t, err)
		assert.Equal(t, []byte("Hello World"), content)
	})

	t.Run("given the base64 decoder then decode", func(t *testing.T) {
		decType := decoder.TypeBase64

		content, err := decodeAttachment(model.Attachment{Data: helloBase64, Decoder: &decType})

		require.NoError(t, err)
		assert.Equal(t, []byte("Hello World"), content)
	})

	t.Run("given an unknown decoder then return error", func(t *testing.T) {
		decType := decoder.Type("rot13")

		_, err := decodeAttachment(model.Attachment{Data: helloBase64, Decoder: &decType})

		require.Error(t, err)
	})

	t.Run("given invalid data then return error", func(t *testing.T) {
		_, err := decodeAttachment(model.Attachment{Data: "invalid-base64!@#"})

		require.Error(t, err)
	})
}

func TestDecodeAttachments(t *testing.T) {
	t.Run("given no attachments then return nil", func(t *testing.T) {
		assert.Nil(t, decodeAttachments(newValidEmail()))
	})

	t.Run("given valid and invalid attachments then keep only valid ones", func(t *testing.T) {
		data := newValidEmail()
		data.Attachments = &[]model.Attachment{
			{Name: testFileName, Type: testFileType, Data: helloBase64},
			{Name: "broken.txt", Type: testFileType, Data: "invalid!@#"},
		}

		assert.Equal(t,
			[]attachmentContent{{Name: testFileName, Type: testFileType, Content: []byte("Hello World")}},
			decodeAttachments(data),
		)
	})
}
