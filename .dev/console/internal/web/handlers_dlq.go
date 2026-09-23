package web

import (
	"net/http"

	"github.com/eliasmeireles/gomailer/dev/console/internal/monitor"
)

type deadLettersData struct {
	Letters []monitor.DeadLetter
	Error   string
}

func (s *Server) deadLetters(w http.ResponseWriter, r *http.Request) {
	s.renderDeadLetters(w, r, nil)
}

func (s *Server) purgeDeadLetters(w http.ResponseWriter, r *http.Request) {
	s.renderDeadLetters(w, r, s.console.PurgeDeadLetters(r.Context()))
}

func (s *Server) renderDeadLetters(w http.ResponseWriter, r *http.Request, actionErr error) {
	letters, err := s.console.DeadLetters(r.Context())
	s.render(w, "dlq.html", deadLettersData{Letters: letters, Error: firstError(actionErr, err)}, http.StatusOK)
}
