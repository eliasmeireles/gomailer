package mailer

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/smtp"
	"net/textproto"

	"github.com/eliasmeireles/gomailer/internal/application/config"
	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

type smtpSender struct {
	config config.SMTPConfig
}

// NewSMTPSender creates a Sender that delivers through an SMTP server using implicit TLS
// (e.g. smtp.zoho.com:465 or smtp.resend.com:465).
func NewSMTPSender(cfg config.SMTPConfig) mailer.Sender {
	return &smtpSender{config: cfg}
}

// Send delivers the email and returns an error if the server rejects it at any step,
// including the final acceptance of the message data.
func (s *smtpSender) Send(data model.SendEmailData) error {
	if err := validateEmail(data); err != nil {
		return err
	}

	conn, err := tls.Dial("tcp", fmt.Sprintf("%s:%d", s.config.Host, s.config.Port), &tls.Config{ServerName: s.config.Host})
	if err != nil {
		return mailer.Errorf(model.CodeSMTPConnectionFailed, "failed to establish TLS connection: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		return mailer.Errorf(model.CodeSMTPConnectionFailed, "failed to create SMTP client: %w", err)
	}
	defer client.Quit()

	if err := client.Auth(smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)); err != nil {
		return mailer.Errorf(smtpCode(model.CodeSMTPAuthorizationDenied, err), "SMTP authentication failed: %w", err)
	}

	if err := client.Mail(data.From); err != nil {
		return mailer.Errorf(smtpCode(model.CodeSMTPSenderRejected, err), "failed to set sender: %w", err)
	}

	for _, recipient := range allRecipients(data) {
		if err := client.Rcpt(recipient); err != nil {
			return mailer.Errorf(smtpCode(model.CodeSMTPReceiverRejected, err), "failed to set recipient %s: %w", recipient, err)
		}
	}

	return writeData(client, buildMessage(data))
}

func writeData(client *smtp.Client, message string) error {
	writer, err := client.Data()
	if err != nil {
		return mailer.Errorf(smtpCode(model.CodeSMTPMessageRejected, err), "failed to initialize data transfer: %w", err)
	}

	if _, err := writer.Write([]byte(message)); err != nil {
		_ = writer.Close()
		return mailer.Errorf(smtpCode(model.CodeSMTPMessageRejected, err), "failed to write message: %w", err)
	}

	if err := writer.Close(); err != nil {
		return mailer.Errorf(smtpCode(model.CodeSMTPMessageRejected, err), "SMTP server rejected the message: %w", err)
	}
	return nil
}

// smtpCode classifies an SMTP step failure: 4xx replies are temporary (smtp_temporary_failure),
// anything else keeps the code of the step that failed.
func smtpCode(stepCode model.ErrorCode, err error) model.ErrorCode {
	var reply *textproto.Error
	if errors.As(err, &reply) && reply.Code >= 400 && reply.Code < 500 {
		return model.CodeSMTPTemporaryFailure
	}
	return stepCode
}
