package web

import "github.com/eliasmeireles/gomailer/dev/console/internal/monitor"

// mockPreset is a canned Resend error the mock API can answer with.
type mockPreset struct {
	Key         string
	Label       string
	ExpectedErr string
	Behavior    monitor.MockBehavior
}

// mockPresets cover a temporary (retried) and a permanent failure of each kind.
var mockPresets = []mockPreset{
	{Key: "rate_limit", Label: "429 rate limit (temporário)", ExpectedErr: "api_rate_limited",
		Behavior: monitor.MockBehavior{Status: 429, Name: "rate_limit_exceeded", Message: "Too many requests. Please limit the number of requests per second."}},
	{Key: "unavailable", Label: "503 indisponível (temporário)", ExpectedErr: "api_provider_unavailable",
		Behavior: monitor.MockBehavior{Status: 503, Name: "service_unavailable", Message: "API is temporarily unavailable"}},
	{Key: "invalid_to", Label: "422 destinatário inválido (definitivo)", ExpectedErr: "api_invalid_receiver",
		Behavior: monitor.MockBehavior{Status: 422, Name: "validation_error", Message: "Invalid `to` field. The email address needs to follow the `email@example.com` format."}},
	{Key: "domain", Label: "403 domínio não verificado (definitivo)", ExpectedErr: "api_sender_not_allowed",
		Behavior: monitor.MockBehavior{Status: 403, Name: "validation_error", Message: "The exemplo.com.br domain is not verified."}},
	{Key: "unauthorized", Label: "401 chave inválida (definitivo)", ExpectedErr: "api_authorization_denied",
		Behavior: monitor.MockBehavior{Status: 401, Name: "validation_error", Message: "API key is invalid"}},
}

// smtpOption is an SMTP reply code the Mailpit chaos can inject for a step.
type smtpOption struct {
	Code  int
	Label string
}

var (
	envelopeOptions = []smtpOption{{451, "451 temporário (retry)"}, {550, "550 definitivo"}}
	authOptions     = []smtpOption{{454, "454 temporário (retry)"}, {535, "535 definitivo"}}
)

func findPreset(key string) (mockPreset, bool) {
	for _, preset := range mockPresets {
		if preset.Key == key {
			return preset, true
		}
	}
	return mockPreset{}, false
}
