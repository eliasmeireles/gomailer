// Package message builds queue messages in the gomailer contract, as an external producer would.
package message

// Attachment is a base64-encoded file.
type Attachment struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Data    string `json:"data"`
	Decoder string `json:"decoder,omitempty"`
}

// Callback holds the endpoints notified after delivery succeeds or fails.
type Callback struct {
	Success *CallbackTarget `json:"success,omitempty"`
	Failure *CallbackTarget `json:"failure,omitempty"`
}

// CallbackTarget is an endpoint that receives the delivery event.
type CallbackTarget struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// Email is the message published to the mailer queue. Receiver, Cc and Bcc hold either a
// []string or a comma-separated string, depending on the chosen RecipientsFormat.
type Email struct {
	ID          string       `json:"id,omitempty"`
	From        string       `json:"from"`
	Receiver    any          `json:"receiver"`
	Cc          any          `json:"cc,omitempty"`
	Bcc         any          `json:"bcc,omitempty"`
	Subject     string       `json:"subject"`
	Body        string       `json:"body"`
	Attachments []Attachment `json:"attachments,omitempty"`
	Callback    *Callback    `json:"callback,omitempty"`
}
