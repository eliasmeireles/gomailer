// Package web serves the dev console UI: a form that publishes to the mailer queue and panels
// that follow the result (stack status, Mailpit inbox and failure callbacks).
package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
	"github.com/eliasmeireles/gomailer/dev/console/internal/monitor"
	"github.com/eliasmeireles/gomailer/dev/console/internal/service"
)

//go:embed assets
var assets embed.FS

// ConsoleService is what the handlers need from the service layer.
type ConsoleService interface {
	Send(ctx context.Context, form message.Form) (service.SendResult, error)
	Status(ctx context.Context) service.Status
	Inbox(ctx context.Context) ([]monitor.InboxMessage, error)
	ClearInbox(ctx context.Context) error
	Callbacks(ctx context.Context) ([]monitor.CallbackEvent, error)
	ClearCallbacks(ctx context.Context) error
}

// Settings are the static values shown on the page and used as form defaults.
type Settings struct {
	MailerEnv          string
	Queue              string
	MailpitPublicURL   string
	CallbackSuccessURL string
	CallbackFailureURL string
}

// Server renders the console pages and partials.
type Server struct {
	console    ConsoleService
	settings   Settings
	templates  *template.Template
	sampleHTML string
}

// NewServer parses the embedded templates and returns a Server.
func NewServer(console ConsoleService, settings Settings) (*Server, error) {
	templates, err := template.New("").Funcs(templateFuncs).ParseFS(assets, "assets/templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	sample, err := assets.ReadFile("assets/sample-email.html")
	if err != nil {
		return nil, fmt.Errorf("read sample email: %w", err)
	}
	return &Server{console: console, settings: settings, templates: templates, sampleHTML: string(sample)}, nil
}

// Routes returns the HTTP handler with every console route.
func (s *Server) Routes() http.Handler {
	static, _ := fs.Sub(assets, "assets/static")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.index)
	mux.HandleFunc("POST /send", s.send)
	mux.HandleFunc("GET /partials/status", s.status)
	mux.HandleFunc("GET /partials/inbox", s.inbox)
	mux.HandleFunc("POST /partials/inbox/clear", s.clearInbox)
	mux.HandleFunc("GET /partials/callbacks", s.callbacks)
	mux.HandleFunc("POST /partials/callbacks/clear", s.clearCallbacks)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	return mux
}

func (s *Server) render(w http.ResponseWriter, name string, data any, status int) {
	var body strings.Builder
	if err := s.templates.ExecuteTemplate(&body, name, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body.String()))
}

var templateFuncs = template.FuncMap{
	"clock": func(t time.Time) string { return t.Local().Format("15:04:05") },
	"addresses": func(list []monitor.Address) string {
		values := make([]string, 0, len(list))
		for _, address := range list {
			values = append(values, address.Address)
		}
		return strings.Join(values, ", ")
	},
}
