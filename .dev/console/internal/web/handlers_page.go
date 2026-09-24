package web

import (
	"net/http"
	"strings"
)

type pageData struct {
	Settings
	DefaultFrom string
	SampleHTML  string
}

func (s *Server) index(w http.ResponseWriter, _ *http.Request) {
	s.render(w, "index.html", pageData{
		Settings:    s.settings,
		DefaultFrom: defaultFrom(s.settings.MailerEnv),
		SampleHTML:  s.sampleHTML,
	}, http.StatusOK)
}

// defaultFrom picks a sender the selected provider accepts without a verified domain.
func defaultFrom(mailerEnv string) string {
	if strings.Contains(mailerEnv, "resend") {
		return "onboarding@resend.dev"
	}
	return "no-reply@example.com"
}
