// Package monitor reads and drives the local stack: Mailpit inbox and chaos, received callbacks,
// RabbitMQ queues, the mock email API and mailer health.
package monitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const requestTimeout = 5 * time.Second

func newHTTPClient() *http.Client {
	return &http.Client{Timeout: requestTimeout}
}

// request describes one JSON call; Body and Out are optional, Auth sets basic auth when user is set.
type request struct {
	Method string
	URL    string
	Body   any
	Out    any
	User   string
	Pass   string
}

// call executes req, failing on transport errors and non-2xx statuses, and decodes the body
// into req.Out when set.
func call(ctx context.Context, client *http.Client, req request) error {
	var body io.Reader
	if req.Body != nil {
		raw, err := json.Marshal(req.Body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(raw)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if req.Body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	if req.User != "" {
		httpReq.SetBasicAuth(req.User, req.Pass)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("%s %s: %w", req.Method, req.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%s %s: status %d", req.Method, req.URL, resp.StatusCode)
	}
	if req.Out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(req.Out); err != nil {
		return fmt.Errorf("decode %s: %w", req.URL, err)
	}
	return nil
}

func getJSON(ctx context.Context, client *http.Client, url string, out any) error {
	return call(ctx, client, request{Method: http.MethodGet, URL: url, Out: out})
}

func send(ctx context.Context, client *http.Client, method, url string) error {
	return call(ctx, client, request{Method: method, URL: url})
}
