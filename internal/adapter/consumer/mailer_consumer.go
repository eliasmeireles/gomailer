package consumer

import (
	"encoding/json"
	"fmt"

	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

// MailerConsumer processes email messages received from RabbitMQ.
type MailerConsumer struct {
	service mailer.Service
}

// NewMailerConsumer creates a new MailerConsumer backed by the given mailer service.
func NewMailerConsumer(service mailer.Service) *MailerConsumer {
	return &MailerConsumer{service: service}
}

// Handle unmarshals a raw message body into SendEmailData and hands it to the mailer service.
// A returned error makes the message be requeued.
func (c *MailerConsumer) Handle(body []byte) error {
	var data model.SendEmailData
	if err := json.Unmarshal(body, &data); err != nil {
		return fmt.Errorf("failed to unmarshal email message: %w", err)
	}

	return c.service.Deliver(data)
}
