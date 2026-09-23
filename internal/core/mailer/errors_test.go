package mailer

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/internal/core/model"
)

func TestDeliveryError(t *testing.T) {
	t.Run("given a classified error wrapped twice then recover its code and cause", func(t *testing.T) {
		cause := errors.New("535 authentication failed")
		err := fmt.Errorf("failed to send email: %w", Errorf(model.CodeSMTPAuthorizationDenied, "SMTP authentication failed: %w", cause))

		assert.Equal(t, model.CodeSMTPAuthorizationDenied, CodeOf(err))
		require.ErrorIs(t, err, cause)
		assert.EqualError(t, err, "failed to send email: SMTP authentication failed: 535 authentication failed")
	})

	t.Run("given an unclassified error then return unknown", func(t *testing.T) {
		assert.Equal(t, model.CodeUnknown, CodeOf(errors.New("boom")))
	})

	t.Run("given nil then return unknown", func(t *testing.T) {
		assert.Equal(t, model.CodeUnknown, CodeOf(nil))
	})

	t.Run("must keep the message of the wrapped error", func(t *testing.T) {
		assert.EqualError(t, NewError(model.CodeAPIRateLimited, errors.New("too many")), "too many")
	})
}
