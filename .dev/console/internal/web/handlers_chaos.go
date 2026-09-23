package web

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/eliasmeireles/gomailer/dev/console/internal/monitor"
	"github.com/eliasmeireles/gomailer/dev/console/internal/service"
)

type chaosData struct {
	service.ChaosView
	Presets         []mockPreset
	EnvelopeOptions []smtpOption
	AuthOptions     []smtpOption
	Error           string
}

func (s *Server) chaos(w http.ResponseWriter, r *http.Request) {
	s.renderChaos(w, r, nil)
}

func (s *Server) setSMTPChaos(w http.ResponseWriter, r *http.Request) {
	triggers := monitor.ChaosTriggers{
		Sender:         trigger(r.FormValue("sender"), 451),
		Recipient:      trigger(r.FormValue("recipient"), 451),
		Authentication: trigger(r.FormValue("authentication"), 535),
	}
	s.renderChaos(w, r, s.console.SetSMTPChaos(r.Context(), triggers))
}

func (s *Server) configureMockAPI(w http.ResponseWriter, r *http.Request) {
	preset, ok := findPreset(r.FormValue("preset"))
	failNext, err := strconv.Atoi(r.FormValue("failNext"))
	if !ok || err != nil || failNext < 0 {
		s.renderChaos(w, r, fmt.Errorf("escolha um cenário e um número de falhas válido"))
		return
	}

	behavior := preset.Behavior
	behavior.FailNext = failNext
	s.renderChaos(w, r, s.console.ConfigureMockAPI(r.Context(), behavior))
}

func (s *Server) resetChaos(w http.ResponseWriter, r *http.Request) {
	s.renderChaos(w, r, s.console.ResetChaos(r.Context()))
}

func (s *Server) renderChaos(w http.ResponseWriter, r *http.Request, actionErr error) {
	s.render(w, "chaos.html", chaosData{
		ChaosView:       s.console.Chaos(r.Context()),
		Presets:         mockPresets,
		EnvelopeOptions: envelopeOptions,
		AuthOptions:     authOptions,
		Error:           firstError(actionErr),
	}, http.StatusOK)
}

// trigger turns a select value (SMTP code, or empty/"0" for off) into a chaos trigger that
// always fires when enabled.
func trigger(value string, defaultCode int) monitor.ChaosTrigger {
	code, err := strconv.Atoi(value)
	if err != nil || code == 0 {
		return monitor.ChaosTrigger{ErrorCode: defaultCode, Probability: 0}
	}
	return monitor.ChaosTrigger{ErrorCode: code, Probability: 100}
}
