package mailer

import (
	"net/mail"
	"strings"

	"github.com/eliasmeireles/gomailer/internal/core/model"
)

// senderAddress renders the sender as an RFC 5322 address: "Name <address>"
// when a display name is set (non-ASCII names are RFC 2047-encoded and special
// characters quoted), the bare address otherwise.
func senderAddress(data model.SendEmailData) string {
	name := strings.TrimSpace(data.FromName)
	if name == "" {
		return data.From
	}
	return (&mail.Address{Name: name, Address: data.From}).String()
}
