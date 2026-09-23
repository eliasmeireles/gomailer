package monitor

import (
	"context"
	"net/http"
)

// Health checks the mailer readiness endpoint.
type Health struct {
	url    string
	client *http.Client
}

// NewHealth creates a checker for the mailer /readyz url.
func NewHealth(url string) *Health {
	return &Health{url: url, client: newHTTPClient()}
}

// Ready reports whether the mailer answers 2xx on its readiness endpoint.
func (h *Health) Ready(ctx context.Context) bool {
	return send(ctx, h.client, http.MethodGet, h.url) == nil
}
