package callback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/eliasmeireles/gomailer/internal/core/model"
)

const maxDrainBytes = 4096

// HTTPNotifier reports delivery outcomes by POSTing a JSON DeliveryEvent to the callback URL.
type HTTPNotifier struct {
	httpClient *http.Client
}

// NewHTTPNotifier creates an HTTPNotifier using httpClient (its Timeout bounds each call).
func NewHTTPNotifier(httpClient *http.Client) *HTTPNotifier {
	return &HTTPNotifier{httpClient: httpClient}
}

// Notify POSTs event as JSON to target.URL with target.Headers.
// Any non-2xx response is returned as an error.
func (n *HTTPNotifier) Notify(target model.CallbackTarget, event model.DeliveryEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to encode delivery event: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, target.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create callback request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range target.Headers {
		req.Header.Set(key, value)
	}

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call callback: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("callback returned status %d", resp.StatusCode)
	}
	return nil
}
