package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorCodeRetryable(t *testing.T) {
	retryable := []ErrorCode{CodeAPIConnectionFailed, CodeAPIRateLimited, CodeAPIProviderUnavailable, CodeSMTPConnectionFailed, CodeSMTPTemporaryFailure}
	permanent := []ErrorCode{CodeMessageInvalidJSON, CodeMessageInvalidBody, CodeAPIInvalidReceiver, CodeAPISenderNotAllowed,
		CodeAPIAuthorizationDenied, CodeAPIQuotaExceeded, CodeSMTPAuthorizationDenied, CodeSMTPReceiverRejected, CodeUnknown}

	for _, code := range retryable {
		t.Run("given "+string(code)+" then retryable", func(t *testing.T) {
			assert.True(t, code.Retryable())
		})
	}
	for _, code := range permanent {
		t.Run("given "+string(code)+" then not retryable", func(t *testing.T) {
			assert.False(t, code.Retryable())
		})
	}
}
