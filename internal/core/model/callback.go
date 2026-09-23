package model

import "time"

// Callback holds the optional endpoints notified after a delivery attempt.
type Callback struct {
	// Success is notified after the email is accepted by the transport.
	Success *CallbackTarget `json:"success,omitempty"`
	// Failure is notified when the email cannot be delivered.
	Failure *CallbackTarget `json:"failure,omitempty"`
}

// CallbackTarget is an endpoint that receives a DeliveryEvent.
type CallbackTarget struct {
	URL string `json:"url"`
	// Headers are sent as-is with the notification (e.g. an Authorization token).
	Headers map[string]string `json:"headers,omitempty"`
}

// DeliveryStatus is the outcome reported in a DeliveryEvent.
type DeliveryStatus string

const (
	// StatusSent means the transport accepted the email.
	StatusSent DeliveryStatus = "sent"
	// StatusFailed means the email could not be delivered.
	StatusFailed DeliveryStatus = "failed"
)

// DeliveryEvent is the payload POSTed to a CallbackTarget. It only identifies the email (ID and
// Subject) instead of echoing its content, to keep callbacks lightweight.
type DeliveryEvent struct {
	ID         string         `json:"id,omitempty"`
	Subject    string         `json:"subject"`
	Status     DeliveryStatus `json:"status"`
	ErrorCode  ErrorCode      `json:"errorCode,omitempty"`
	Cause      string         `json:"cause,omitempty"`
	OccurredAt time.Time      `json:"occurredAt"`
}
