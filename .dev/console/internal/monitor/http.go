// Package monitor reads the state of the local stack: Mailpit inbox, received callbacks and
// mailer health.
package monitor

import (
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

// getJSON GETs url and decodes the 2xx JSON body into out.
func getJSON(ctx context.Context, client *http.Client, url string, out any) error {
	resp, err := do(ctx, client, http.MethodGet, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}

// send issues a body-less request (e.g. DELETE) and only checks the status.
func send(ctx context.Context, client *http.Client, method, url string) error {
	resp, err := do(ctx, client, method, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func do(ctx context.Context, client *http.Client, method, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, url, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		resp.Body.Close()
		return nil, fmt.Errorf("%s %s: status %d", method, url, resp.StatusCode)
	}
	return resp, nil
}
