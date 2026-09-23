package monitor

import (
	"context"
	"net/http"
)

// ChaosTrigger makes Mailpit answer an SMTP step with ErrorCode, Probability percent of the time.
type ChaosTrigger struct {
	ErrorCode   int `json:"ErrorCode"`
	Probability int `json:"Probability"`
}

// ChaosTriggers are the SMTP steps Mailpit can fail.
type ChaosTriggers struct {
	Sender         ChaosTrigger `json:"Sender"`
	Recipient      ChaosTrigger `json:"Recipient"`
	Authentication ChaosTrigger `json:"Authentication"`
}

// Chaos returns the current Mailpit chaos triggers.
func (m *Mailpit) Chaos(ctx context.Context) (ChaosTriggers, error) {
	var triggers ChaosTriggers
	err := getJSON(ctx, m.client, m.baseURL+"/api/v1/chaos", &triggers)
	return triggers, err
}

// SetChaos replaces the Mailpit chaos triggers (requires MP_ENABLE_CHAOS).
func (m *Mailpit) SetChaos(ctx context.Context, triggers ChaosTriggers) (ChaosTriggers, error) {
	var updated ChaosTriggers
	err := call(ctx, m.client, request{Method: http.MethodPut, URL: m.baseURL + "/api/v1/chaos", Body: triggers, Out: &updated})
	return updated, err
}
