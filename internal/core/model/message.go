package model

import "github.com/eliasmeireles/gomailer/internal/core/decoder"

// Attachment defines model for Attachment.
type Attachment struct {
	Data    string        `json:"data" validate:"required,gt=5"`
	Name    string        `json:"name" validate:"required,gt=5"`
	Type    string        `json:"type" validate:"required,gt=5"`
	Decoder *decoder.Type `json:"decoder,omitempty"`
}

// SendEmailData defines model for SendEmailData.
type SendEmailData struct {
	// ID is an optional producer-defined identifier, echoed back in failure callbacks.
	ID string `json:"id,omitempty"`

	Attachments *[]Attachment `json:"attachments,omitempty"`

	Body string `json:"body" validate:"required,gt=10"`

	From string `json:"from" validate:"required,email"`

	// Receiver, Cc and Bcc accept a comma-separated string or an array of addresses.
	Receiver Recipients `json:"receiver" validate:"required"`

	Subject string `json:"subject" validate:"required,gt=5"`

	Cc  Recipients `json:"cc,omitempty"`
	Bcc Recipients `json:"bcc,omitempty"`

	// Callback is optional; when set, delivery failures are reported to it.
	Callback *Callback `json:"callback,omitempty"`
}
