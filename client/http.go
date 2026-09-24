package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultHTTPTimeout = 60 * time.Second

// HTTPConfig configures the HTTP sender.
type HTTPConfig struct {
	// URL is the gomailer HTTP source base URL (e.g. http://gomailer:8081).
	URL string
	// Token is one of the gomailer HTTP_API_KEYS.
	Token string
	// Timeout bounds each request (default 60s, the delivery is synchronous).
	Timeout time.Duration
}

// DeliveryError is returned by the HTTP sender when gomailer answered with a failed delivery.
type DeliveryError struct {
	StatusCode int
	Event      DeliveryEvent
}

func (e *DeliveryError) Error() string {
	return fmt.Sprintf("gomailer returned %d (%s): %s", e.StatusCode, e.Event.ErrorCode, e.Event.Cause)
}

// HTTPSender delivers emails synchronously through POST /v1/emails.
type HTTPSender struct {
	url    string
	token  string
	client *http.Client
}

// NewHTTPSender creates an HTTP sender.
func NewHTTPSender(cfg HTTPConfig) (*HTTPSender, error) {
	if strings.TrimSpace(cfg.URL) == "" || strings.TrimSpace(cfg.Token) == "" {
		return nil, fmt.Errorf("gomailer http sender requires URL and Token")
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultHTTPTimeout
	}
	return &HTTPSender{
		url:    strings.TrimRight(cfg.URL, "/") + "/v1/emails",
		token:  cfg.Token,
		client: &http.Client{Timeout: timeout},
	}, nil
}

// Send delivers the email; failed deliveries are returned as *DeliveryError.
func (s *HTTPSender) Send(ctx context.Context, email Email) error {
	_, err := s.Deliver(ctx, email)
	return err
}

// Deliver delivers the email and returns the delivery event.
func (s *HTTPSender) Deliver(ctx context.Context, email Email) (DeliveryEvent, error) {
	email, err := Prepare(email)
	if err != nil {
		return DeliveryEvent{}, err
	}
	body, err := json.Marshal(email)
	if err != nil {
		return DeliveryEvent{}, fmt.Errorf("encode email: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return DeliveryEvent{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := s.client.Do(req)
	if err != nil {
		return DeliveryEvent{}, fmt.Errorf("call gomailer: %w", err)
	}
	defer resp.Body.Close()
	return decodeResponse(resp)
}

func decodeResponse(resp *http.Response) (DeliveryEvent, error) {
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxEventBytes))

	var event DeliveryEvent
	if err := json.Unmarshal(raw, &event); err != nil || event.Status == "" {
		return DeliveryEvent{}, fmt.Errorf("gomailer returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if event.Status != StatusSent {
		return event, &DeliveryError{StatusCode: resp.StatusCode, Event: event}
	}
	return event, nil
}

// HealthCheck is a no-op for HTTP: every request is independent.
func (s *HTTPSender) HealthCheck(context.Context) error { return nil }

// Close releases idle connections.
func (s *HTTPSender) Close() error {
	s.client.CloseIdleConnections()
	return nil
}
