package mailer

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/eliasmeireles/gomailer/internal/application/config"
	"github.com/eliasmeireles/gomailer/internal/core/mailer"
)

const (
	// APIClientResend selects the Resend API (MAILER_API_CLIENT=resend).
	APIClientResend = "resend"
	// APIClientZoho selects the Zoho Mail API with OAuth (MAILER_API_CLIENT=zoho).
	APIClientZoho = "zoho"
	// APIClientZeptoMail selects the ZeptoMail API (MAILER_API_CLIENT=zeptomail).
	APIClientZeptoMail = "zeptomail"
)

// apiClientFactory builds an API-based Sender, loading its own configuration from the environment.
type apiClientFactory func(httpClient *http.Client) (mailer.Sender, error)

// apiClients is the registry of API strategies. To add a provider, implement a
// mailer.Sender and register its factory here.
var apiClients = map[string]apiClientFactory{
	APIClientResend: func(httpClient *http.Client) (mailer.Sender, error) {
		cfg, err := config.NewResendConfig()
		if err != nil {
			return nil, err
		}
		return NewResendSender(cfg, httpClient), nil
	},
	APIClientZoho: func(httpClient *http.Client) (mailer.Sender, error) {
		cfg, err := config.NewZohoMailConfig()
		if err != nil {
			return nil, err
		}
		return NewZohoMailSender(cfg, httpClient), nil
	},
	APIClientZeptoMail: func(httpClient *http.Client) (mailer.Sender, error) {
		cfg, err := config.NewZeptoMailConfig()
		if err != nil {
			return nil, err
		}
		return NewZeptoMailSender(cfg, httpClient), nil
	},
}

// NewAPISender returns the Sender registered under name (see MAILER_API_CLIENT).
func NewAPISender(name string, httpClient *http.Client) (mailer.Sender, error) {
	factory, ok := apiClients[name]
	if !ok {
		return nil, fmt.Errorf("unknown mailer API client %q, available: %s", name, strings.Join(availableAPIClients(), ", "))
	}
	return factory(httpClient)
}

func availableAPIClients() []string {
	names := make([]string, 0, len(apiClients))
	for name := range apiClients {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
