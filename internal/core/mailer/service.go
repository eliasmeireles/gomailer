package mailer

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/eliasmeireles/gomailer/internal/core/model"
)

// Service delivers queued email messages and reports the outcome to the optional callbacks.
type Service interface {
	// Deliver decodes the base64 body and sends the email.
	//
	// On success, callback.success (if any) is notified; a failing success callback is only
	// logged, since the email was already sent and must not be requeued.
	//
	// On failure, callback.failure (if any) is notified. If it succeeds the failure is
	// considered handled and Deliver returns nil; otherwise the delivery error (joined with the
	// callback error, if any) is returned so the message is requeued.
	Deliver(data model.SendEmailData) error
}

type service struct {
	sender   Sender
	notifier DeliveryNotifier
	now      func() time.Time
}

// NewService creates a Service that sends through sender and reports outcomes through notifier.
func NewService(sender Sender, notifier DeliveryNotifier) Service {
	return &service{sender: sender, notifier: notifier, now: time.Now}
}

func (s *service) Deliver(data model.SendEmailData) error {
	log.Infof("Processing email id: %q, from: %s, to: %s, subject: %s", data.ID, data.From, data.Receiver, data.Subject)

	if err := s.send(data); err != nil {
		return s.handleFailure(data, err)
	}

	log.Infof("Email %q sent successfully from: %s, to: %s", data.ID, data.From, data.Receiver)
	s.notifySuccess(data)
	return nil
}

func (s *service) send(data model.SendEmailData) error {
	decodedBody, err := base64.StdEncoding.DecodeString(data.Body)
	if err != nil {
		return Errorf(model.CodeMessageInvalidBody, "failed to decode base64 body: %w", err)
	}

	email := data
	email.Body = string(decodedBody)
	email.Callback = nil

	if err := s.sender.Send(email); err != nil {
		return fmt.Errorf("failed to send email to %s: %w", data.Receiver, err)
	}
	return nil
}

func (s *service) notifySuccess(data model.SendEmailData) {
	target := usableTarget(data.Callback, func(c *model.Callback) *model.CallbackTarget { return c.Success })
	if target == nil {
		return
	}

	if err := s.notifier.Notify(*target, s.newEvent(data, model.StatusSent, nil)); err != nil {
		log.Warnf("Email %q was sent, but the success callback %s failed: %v", data.ID, target.URL, err)
	}
}

func (s *service) handleFailure(data model.SendEmailData, cause error) error {
	target := usableTarget(data.Callback, func(c *model.Callback) *model.CallbackTarget { return c.Failure })
	if target == nil {
		return cause
	}

	if err := s.notifier.Notify(*target, s.newEvent(data, model.StatusFailed, cause)); err != nil {
		return errors.Join(cause, fmt.Errorf("failed to notify failure callback %s: %w", target.URL, err))
	}

	log.Warnf("Email %q to %s failed (%v); failure reported to callback %s", data.ID, data.Receiver, cause, target.URL)
	return nil
}

func (s *service) newEvent(data model.SendEmailData, status model.DeliveryStatus, cause error) model.DeliveryEvent {
	event := model.DeliveryEvent{
		ID:         data.ID,
		Subject:    data.Subject,
		Status:     status,
		OccurredAt: s.now().UTC(),
	}
	if cause != nil {
		event.ErrorCode = CodeOf(cause)
		event.Cause = cause.Error()
	}
	return event
}

// usableTarget returns the selected target when the callback defines it with a non-blank URL.
func usableTarget(callback *model.Callback, pick func(*model.Callback) *model.CallbackTarget) *model.CallbackTarget {
	if callback == nil {
		return nil
	}
	target := pick(callback)
	if target == nil || strings.TrimSpace(target.URL) == "" {
		return nil
	}
	return target
}
