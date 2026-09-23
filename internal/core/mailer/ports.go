package mailer

import "github.com/eliasmeireles/gomailer/internal/core/model"

// Sender delivers a ready-to-send email (HTML body already decoded) through a transport.
type Sender interface {
	Send(data model.SendEmailData) error
}

// DeliveryNotifier reports a delivery outcome to a callback informed in the message.
type DeliveryNotifier interface {
	Notify(target model.CallbackTarget, event model.DeliveryEvent) error
}
