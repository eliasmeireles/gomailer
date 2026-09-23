package monitor

import (
	"context"
	"net/http"
	"time"
)

// DeliveryEvent is the payload the mailer POSTs to a success or failure callback.
type DeliveryEvent struct {
	ID         string    `json:"id"`
	Subject    string    `json:"subject"`
	Status     string    `json:"status"`
	ErrorCode  string    `json:"errorCode"`
	Cause      string    `json:"cause"`
	OccurredAt time.Time `json:"occurredAt"`
}

// CallbackEvent is a callback received by the local callback server.
type CallbackEvent struct {
	ReceivedAt    time.Time     `json:"receivedAt"`
	Path          string        `json:"path"`
	Authorization string        `json:"authorization"`
	Status        int           `json:"status"`
	Body          DeliveryEvent `json:"body"`
}

// Callbacks reads and clears the events stored by the .dev callback server.
type Callbacks struct {
	baseURL string
	client  *http.Client
}

// NewCallbacks creates a client for the callback server at baseURL (e.g. http://callback:9099).
func NewCallbacks(baseURL string) *Callbacks {
	return &Callbacks{baseURL: baseURL, client: newHTTPClient()}
}

// List returns the received callbacks, newest first.
func (c *Callbacks) List(ctx context.Context) ([]CallbackEvent, error) {
	var events []CallbackEvent
	if err := getJSON(ctx, c.client, c.baseURL+"/events", &events); err != nil {
		return nil, err
	}
	return events, nil
}

// Clear removes every stored callback.
func (c *Callbacks) Clear(ctx context.Context) error {
	return send(ctx, c.client, http.MethodDelete, c.baseURL+"/events")
}
