package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DeliveryStatus is the outcome reported by gomailer.
type DeliveryStatus string

const (
	// StatusSent means the transport accepted the email.
	StatusSent DeliveryStatus = "sent"
	// StatusFailed means the email could not be delivered.
	StatusFailed DeliveryStatus = "failed"
)

// DeliveryEvent is the payload gomailer POSTs to a callback and returns from its HTTP API.
type DeliveryEvent struct {
	ID         string         `json:"id,omitempty"`
	Subject    string         `json:"subject"`
	Status     DeliveryStatus `json:"status"`
	ErrorCode  ErrorCode      `json:"errorCode,omitempty"`
	Cause      string         `json:"cause,omitempty"`
	OccurredAt time.Time      `json:"occurredAt"`
}

// maxEventBytes bounds the callback body read by ParseDeliveryEvent.
const maxEventBytes = 64 << 10

// ParseDeliveryEvent decodes the DeliveryEvent posted by gomailer to a callback endpoint.
//
//	http.HandleFunc("POST /mailer/failures", func(w http.ResponseWriter, r *http.Request) {
//		event, err := client.ParseDeliveryEvent(r)
//		...
//	})
func ParseDeliveryEvent(r *http.Request) (DeliveryEvent, error) {
	var event DeliveryEvent
	if err := json.NewDecoder(io.LimitReader(r.Body, maxEventBytes)).Decode(&event); err != nil {
		return DeliveryEvent{}, fmt.Errorf("decode delivery event: %w", err)
	}
	return event, nil
}
