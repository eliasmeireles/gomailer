// Package api calls the gomailer HTTP source (POST /v1/emails).
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
	"github.com/eliasmeireles/gomailer/dev/console/internal/monitor"
)

const requestTimeout = 60 * time.Second

// Response is the synchronous answer of the HTTP source.
type Response struct {
	StatusCode int
	Event      monitor.DeliveryEvent
	// Raw is the body when it is not a delivery event (e.g. 401 or 413 errors).
	Raw string
}

// Client posts emails to the mailer HTTP source with a Bearer token.
type Client struct {
	url    string
	token  string
	client *http.Client
}

// NewClient creates a client for baseURL (e.g. http://mailer:8081) authenticated with token.
func NewClient(baseURL, token string) *Client {
	return &Client{url: baseURL + "/v1/emails", token: token, client: &http.Client{Timeout: requestTimeout}}
}

// Send delivers email synchronously and returns the HTTP status with the decoded event.
func (c *Client) Send(ctx context.Context, email message.Email) (Response, error) {
	body, err := json.Marshal(email)
	if err != nil {
		return Response{}, fmt.Errorf("encode message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("call mailer HTTP API: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	response := Response{StatusCode: resp.StatusCode}
	if err := json.Unmarshal(raw, &response.Event); err != nil || response.Event.Status == "" {
		response.Raw = string(raw)
	}
	return response, nil
}
