package monitor

import (
	"context"
	"net/http"
	"time"
)

// MockBehavior is how the mock email API answers the next requests.
type MockBehavior struct {
	FailNext int    `json:"failNext"`
	Status   int    `json:"status"`
	Name     string `json:"name"`
	Message  string `json:"message"`
}

// MockEmail is an email accepted by the mock API.
type MockEmail struct {
	ID          string    `json:"id"`
	ReceivedAt  time.Time `json:"receivedAt"`
	From        string    `json:"from"`
	To          []string  `json:"to"`
	Cc          []string  `json:"cc"`
	Bcc         []string  `json:"bcc"`
	Subject     string    `json:"subject"`
	Attachments int       `json:"attachments"`
}

// MockState is the mock API behavior with its counters and accepted emails.
type MockState struct {
	MockBehavior
	Accepted int         `json:"accepted"`
	Rejected int         `json:"rejected"`
	Emails   []MockEmail `json:"emails"`
}

// MockAPI drives the .dev mock email API (Resend-compatible).
type MockAPI struct {
	baseURL string
	client  *http.Client
}

// NewMockAPI creates a client for the mock API at baseURL (e.g. http://mockapi:9100).
func NewMockAPI(baseURL string) *MockAPI {
	return &MockAPI{baseURL: baseURL, client: newHTTPClient()}
}

// State returns the current behavior, counters and accepted emails.
func (m *MockAPI) State(ctx context.Context) (MockState, error) {
	var state MockState
	err := getJSON(ctx, m.client, m.baseURL+"/state", &state)
	return state, err
}

// Configure sets how the next requests are answered.
func (m *MockAPI) Configure(ctx context.Context, behavior MockBehavior) (MockState, error) {
	var state MockState
	err := call(ctx, m.client, request{Method: http.MethodPut, URL: m.baseURL + "/state", Body: behavior, Out: &state})
	return state, err
}

// Reset restores the default behavior and clears counters and emails.
func (m *MockAPI) Reset(ctx context.Context) error {
	return send(ctx, m.client, http.MethodDelete, m.baseURL+"/state")
}
