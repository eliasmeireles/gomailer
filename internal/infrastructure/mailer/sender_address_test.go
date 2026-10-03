package mailer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSenderAddress(t *testing.T) {
	t.Run("given no display name then use the bare address", func(t *testing.T) {
		assert.Equal(t, testSender, senderAddress(newValidEmail()))
	})

	t.Run("given a blank display name then use the bare address", func(t *testing.T) {
		data := newValidEmail()
		data.FromName = "   "
		assert.Equal(t, testSender, senderAddress(data))
	})

	t.Run("given a display name then render name and address", func(t *testing.T) {
		data := newValidEmail()
		data.FromName = "Promogram"
		assert.Equal(t, "\"Promogram\" <"+testSender+">", senderAddress(data))
	})

	t.Run("given a non-ascii display name then encode it", func(t *testing.T) {
		data := newValidEmail()
		data.FromName = "Promoções"
		assert.Equal(t, "=?utf-8?q?Promo=C3=A7=C3=B5es?= <"+testSender+">", senderAddress(data))
	})
}
