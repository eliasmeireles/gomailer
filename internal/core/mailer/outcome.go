package mailer

import "github.com/eliasmeireles/gomailer/internal/core/model"

// Outcome is the result of processing one email request, used by every source (queues decide
// whether to ack, the HTTP API builds its response).
type Outcome struct {
	// Event describes the result: status sent, or failed with errorCode and cause.
	Event model.DeliveryEvent
	// Err is the delivery error; nil when the email was sent.
	Err error
	// Handled is true when nothing is left to do: the email was sent, or its failure was
	// accepted by the failure callback.
	Handled bool
}

// Sent reports whether the email was delivered.
func (o Outcome) Sent() bool {
	return o.Err == nil
}
