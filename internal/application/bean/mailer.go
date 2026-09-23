package bean

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/eliasmeireles/gomailer/internal/application/config"
	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/infrastructure/callback"
	mailerInfra "github.com/eliasmeireles/gomailer/internal/infrastructure/mailer"
)

var (
	// MailerService delivers email requests and reports outcomes to the message callbacks.
	MailerService mailer.Service

	// RetryPolicy bounds the attempts of queued requests; queue sources declare their retry
	// topology from it.
	RetryPolicy mailer.RetryPolicy
)

// InitMailer wires the sender selected by MAILER_TRANSPORT / MAILER_API_CLIENT and the callback notifier.
func InitMailer() {
	settings, err := config.NewMailerSettings()
	if err != nil {
		log.Fatalf("Invalid mailer settings: %v", err)
	}

	timeout, err := config.HTTPClientTimeout()
	if err != nil {
		log.Fatalf("Invalid HTTP client settings: %v", err)
	}
	httpClient := &http.Client{Timeout: timeout}

	sender, err := newSender(settings, httpClient)
	if err != nil {
		log.Fatalf("Failed to initialize mailer sender: %v", err)
	}

	retry, err := config.NewRetryConfig()
	if err != nil {
		log.Fatalf("Invalid retry settings: %v", err)
	}
	RetryPolicy = mailer.RetryPolicy{MaxAttempts: retry.MaxAttempts, BaseDelay: retry.BaseDelay, MaxDelay: retry.MaxDelay}

	MailerService = mailer.NewService(sender, callback.NewHTTPNotifier(httpClient), RetryPolicy)
	log.Infof("Mailer initialized, transport: %s, api client: %q", settings.Transport, settings.APIClient)
}

func newSender(settings config.MailerSettings, httpClient *http.Client) (mailer.Sender, error) {
	switch settings.Transport {
	case config.TransportSMTP:
		cfg, err := config.NewSMTPConfig()
		if err != nil {
			return nil, err
		}
		return mailerInfra.NewSMTPSender(cfg), nil
	case config.TransportAPI:
		return mailerInfra.NewAPISender(settings.APIClient, httpClient)
	default:
		return nil, fmt.Errorf("unsupported transport %q", settings.Transport)
	}
}
