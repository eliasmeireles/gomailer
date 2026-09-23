package config

import (
	"fmt"
	"strings"
)

// Transport identifies how emails leave the service.
type Transport string

const (
	// TransportSMTP delivers emails through an SMTP server with implicit TLS.
	TransportSMTP Transport = "smtp"
	// TransportAPI delivers emails through a provider HTTP API selected by MAILER_API_CLIENT.
	TransportAPI Transport = "api"

	envMailerTransport = "MAILER_TRANSPORT"
	envMailerAPIClient = "MAILER_API_CLIENT"
)

// MailerSettings selects the delivery transport and, for TransportAPI, the API client strategy.
type MailerSettings struct {
	Transport Transport
	// APIClient is the lower-cased strategy name (e.g. "resend"); empty for TransportSMTP.
	APIClient string
}

// NewMailerSettings reads MAILER_TRANSPORT ("smtp" or "api", default "smtp") and, when the
// transport is "api", the required MAILER_API_CLIENT (e.g. "resend").
//
// Example:
//
//	MAILER_TRANSPORT=api MAILER_API_CLIENT=resend
func NewMailerSettings() (MailerSettings, error) {
	transport := Transport(strings.ToLower(envOrDefault(envMailerTransport, string(TransportSMTP))))

	switch transport {
	case TransportSMTP:
		return MailerSettings{Transport: transport}, nil
	case TransportAPI:
		client, err := requireEnv(envMailerAPIClient)
		if err != nil {
			return MailerSettings{}, fmt.Errorf("%s=%s requires an API client: %w", envMailerTransport, TransportAPI, err)
		}
		return MailerSettings{Transport: transport, APIClient: strings.ToLower(client)}, nil
	default:
		return MailerSettings{}, fmt.Errorf("%s must be %q or %q, got %q", envMailerTransport, TransportSMTP, TransportAPI, transport)
	}
}
