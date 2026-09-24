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
	{Key: "rate_limit", Label: "429 rate limit (temporary)", ExpectedErr: "api_rate_limited",
		Behavior: monitor.MockBehavior{Status: 429, Name: "rate_limit_exceeded", Message: "Too many requests. Please limit the number of requests per second."}},
	{Key: "unavailable", Label: "503 unavailable (temporary)", ExpectedErr: "api_provider_unavailable",
		Behavior: monitor.MockBehavior{Status: 503, Name: "service_unavailable", Message: "API is temporarily unavailable"}},
	{Key: "invalid_to", Label: "422 invalid recipient (permanent)", ExpectedErr: "api_invalid_receiver",
		Behavior: monitor.MockBehavior{Status: 422, Name: "validation_error", Message: "Invalid `to` field. The email address needs to follow the `email@example.com` format."}},
	{Key: "domain", Label: "403 unverified domain (permanent)", ExpectedErr: "api_sender_not_allowed",
		Behavior: monitor.MockBehavior{Status: 403, Name: "validation_error", Message: "The example.com domain is not verified."}},
	{Key: "unauthorized", Label: "401 invalid key (permanent)", ExpectedErr: "api_authorization_denied",
		Behavior: monitor.MockBehavior{Status: 401, Name: "validation_error", Message: "API key is invalid"}},
}

// smtpOption is an SMTP reply code the Mailpit chaos can inject for a step.
type smtpOption struct {
	Code  int
	Label string
}

var (
	envelopeOptions = []smtpOption{{451, "451 temporary (retried)"}, {550, "550 permanent"}}
	authOptions     = []smtpOption{{454, "454 temporary (retried)"}, {535, "535 permanent"}}
)

func findPreset(key string) (mockPreset, bool) {
	for _, preset := range mockPresets {
		if preset.Key == key {
			return preset, true
		}
	}
	return mockPreset{}, false
}
