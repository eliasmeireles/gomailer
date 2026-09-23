package consumer

import (
	"encoding/json"

	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

// Deliverer is what the consumer needs from the service layer.
type Deliverer interface {
	Deliver(data model.SendEmailData, attempt int) mailer.Outcome
}

// MailerConsumer processes email messages received from a queue.
type MailerConsumer struct {
	service Deliverer
}

// NewMailerConsumer creates a new MailerConsumer backed by the given mailer service.
func NewMailerConsumer(service Deliverer) *MailerConsumer {
	return &MailerConsumer{service: service}
}

// Handle unmarshals a queued message and delivers it on the given attempt (1-based). A body
// that is not valid JSON can never succeed, so it goes straight to the dead-letter queue.
func (c *MailerConsumer) Handle(body []byte, attempt int) mailer.Outcome {
	var data model.SendEmailData
	if err := json.Unmarshal(body, &data); err != nil {
		cause := mailer.Errorf(model.CodeMessageInvalidJSON, "failed to unmarshal email message: %w", err)
		return mailer.Outcome{
			Event:  model.DeliveryEvent{Status: model.StatusFailed, ErrorCode: model.CodeMessageInvalidJSON, Cause: cause.Error()},
			Err:    cause,
			Action: mailer.ActionDeadLetter,
		}
	}

	return c.service.Deliver(data, attempt)
}
