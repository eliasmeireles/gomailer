package web

import (
	"net/http"

	"github.com/eliasmeireles/gomailer/dev/console/internal/monitor"
	"github.com/eliasmeireles/gomailer/dev/console/internal/service"
)

type statusData struct {
	service.Status
	MailerEnv string
}

type inboxData struct {
	Messages         []monitor.InboxMessage
	MailpitPublicURL string
	Error            string
}

type callbacksData struct {
	Events []monitor.CallbackEvent
	Error  string
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	s.render(w, "status.html", statusData{Status: s.console.Status(r.Context()), MailerEnv: s.settings.MailerEnv}, http.StatusOK)
}

func (s *Server) inbox(w http.ResponseWriter, r *http.Request) {
	s.renderInbox(w, r, nil)
}

func (s *Server) clearInbox(w http.ResponseWriter, r *http.Request) {
	s.renderInbox(w, r, s.console.ClearInbox(r.Context()))
}

func (s *Server) renderInbox(w http.ResponseWriter, r *http.Request, actionErr error) {
	messages, err := s.console.Inbox(r.Context())
	data := inboxData{Messages: messages, MailpitPublicURL: s.settings.MailpitPublicURL, Error: firstError(actionErr, err)}
	s.render(w, "inbox.html", data, http.StatusOK)
}

func (s *Server) callbacks(w http.ResponseWriter, r *http.Request) {
	s.renderCallbacks(w, r, nil)
}

func (s *Server) clearCallbacks(w http.ResponseWriter, r *http.Request) {
	s.renderCallbacks(w, r, s.console.ClearCallbacks(r.Context()))
}

func (s *Server) renderCallbacks(w http.ResponseWriter, r *http.Request, actionErr error) {
	events, err := s.console.Callbacks(r.Context())
	s.render(w, "callbacks.html", callbacksData{Events: events, Error: firstError(actionErr, err)}, http.StatusOK)
}

func firstError(errs ...error) string {
	for _, err := range errs {
		if err != nil {
			return err.Error()
		}
	}
	return ""
}
