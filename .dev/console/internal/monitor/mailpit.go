package monitor

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

const inboxLimit = 20

// Address is a Mailpit address.
type Address struct {
	Name    string `json:"Name"`
	Address string `json:"Address"`
}

// InboxMessage is a Mailpit message summary.
type InboxMessage struct {
	ID          string    `json:"ID"`
	Created     time.Time `json:"Created"`
	Subject     string    `json:"Subject"`
	From        Address   `json:"From"`
	To          []Address `json:"To"`
	Cc          []Address `json:"Cc"`
	Bcc         []Address `json:"Bcc"`
	Attachments int       `json:"Attachments"`
	Snippet     string    `json:"Snippet"`
}

// Mailpit reads and clears the Mailpit inbox through its API.
type Mailpit struct {
	baseURL string
	client  *http.Client
}

// NewMailpit creates a Mailpit client for the API at baseURL (e.g. http://mailpit:8025).
func NewMailpit(baseURL string) *Mailpit {
	return &Mailpit{baseURL: baseURL, client: newHTTPClient()}
}

// List returns the latest messages, newest first.
func (m *Mailpit) List(ctx context.Context) ([]InboxMessage, error) {
	var result struct {
		Messages []InboxMessage `json:"messages"`
	}
	if err := getJSON(ctx, m.client, m.baseURL+"/api/v1/messages?limit="+strconv.Itoa(inboxLimit), &result); err != nil {
		return nil, err
	}
	return result.Messages, nil
}

// Clear deletes every message.
func (m *Mailpit) Clear(ctx context.Context) error {
	return send(ctx, m.client, http.MethodDelete, m.baseURL+"/api/v1/messages")
}
