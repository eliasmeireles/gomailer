package mailer

import (
	"slices"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/eliasmeireles/gomailer/internal/core/decoder"
	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

// attachmentContent is an attachment with its raw (decoded) bytes, ready for any transport.
type attachmentContent struct {
	Name    string
	Type    string
	Content []byte
}

func validateEmail(data model.SendEmailData) error {
	if strings.TrimSpace(data.From) == "" {
		return mailer.Errorf(model.CodeMessageMissingSender, "sender email cannot be empty")
	}
	if len(data.Receiver) == 0 {
		return mailer.Errorf(model.CodeMessageMissingReceiver, "recipient email cannot be empty")
	}
	if strings.TrimSpace(data.Subject) == "" {
		return mailer.Errorf(model.CodeMessageMissingSubject, "subject cannot be empty")
	}
	if strings.TrimSpace(data.Body) == "" {
		return mailer.Errorf(model.CodeMessageMissingBody, "body cannot be empty")
	}
	return nil
}

// allRecipients returns every envelope recipient: receiver, cc and bcc.
func allRecipients(data model.SendEmailData) []string {
	return slices.Concat(data.Receiver, data.Cc, data.Bcc)
}

// decodeAttachment decodes the attachment data with its decoder, defaulting to base64.
func decodeAttachment(attachment model.Attachment) ([]byte, error) {
	decType := decoder.TypeBase64
	if attachment.Decoder != nil {
		decType = *attachment.Decoder
	}

	dec, err := decoder.GetDecoder(decType)
	if err != nil {
		return nil, err
	}
	return dec.Decode(attachment.Data)
}

// decodeAttachments returns the decodable attachments of data; invalid ones are skipped and logged.
func decodeAttachments(data model.SendEmailData) []attachmentContent {
	if data.Attachments == nil {
		return nil
	}

	var contents []attachmentContent
	for _, attachment := range *data.Attachments {
		content, err := decodeAttachment(attachment)
		if err != nil {
			log.Warnf("Skipping attachment %q: %v", attachment.Name, err)
			continue
		}
		contents = append(contents, attachmentContent{Name: attachment.Name, Type: attachment.Type, Content: content})
	}
	return contents
}
