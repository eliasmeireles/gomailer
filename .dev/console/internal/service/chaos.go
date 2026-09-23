package service

import (
	"context"
	"errors"

	"github.com/eliasmeireles/gomailer/dev/console/internal/monitor"
)

// ChaosView is the current failure injection of the SMTP server and the mock email API; each
// side reports its own error, since only one of them is used by a given MAILER_ENV.
type ChaosView struct {
	SMTP      monitor.ChaosTriggers
	SMTPError string
	Mock      monitor.MockState
	MockError string
}

// Chaos returns the current failure injection.
func (c *Console) Chaos(ctx context.Context) ChaosView {
	var view ChaosView
	smtp, err := c.deps.SMTPChaos.Chaos(ctx)
	view.SMTP, view.SMTPError = smtp, errorText(err)
	mock, err := c.deps.MockAPI.State(ctx)
	view.Mock, view.MockError = mock, errorText(err)
	return view
}

// SetSMTPChaos replaces the Mailpit chaos triggers.
func (c *Console) SetSMTPChaos(ctx context.Context, triggers monitor.ChaosTriggers) error {
	_, err := c.deps.SMTPChaos.SetChaos(ctx, triggers)
	return err
}

// ConfigureMockAPI makes the mock API fail the next requests as described by behavior.
func (c *Console) ConfigureMockAPI(ctx context.Context, behavior monitor.MockBehavior) error {
	_, err := c.deps.MockAPI.Configure(ctx, behavior)
	return err
}

// ResetChaos turns every SMTP trigger off and resets the mock API.
func (c *Console) ResetChaos(ctx context.Context) error {
	current, err := c.deps.SMTPChaos.Chaos(ctx)
	if err == nil {
		current.Sender.Probability, current.Recipient.Probability, current.Authentication.Probability = 0, 0, 0
		_, err = c.deps.SMTPChaos.SetChaos(ctx, current)
	}
	return errors.Join(err, c.deps.MockAPI.Reset(ctx))
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
