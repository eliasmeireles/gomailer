package httpapi

import (
	"net/http"
	"strings"

	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

// statusFor maps a delivery outcome to the HTTP status of the response:
//
//	sent                                            200
//	message_*                                       400 (invalid request)
//	rejected by the provider (receiver, sender...)  422
//	api_rate_limited                                429
//	provider unreachable or unavailable             503
//	credentials rejected or unexpected response     502
func statusFor(outcome mailer.Outcome) int {
	if outcome.Sent() {
		return http.StatusOK
	}

	code := outcome.Event.ErrorCode
	switch {
	case strings.HasPrefix(string(code), "message_"):
		return http.StatusBadRequest
	case isRejection(code):
		return http.StatusUnprocessableEntity
	case code == model.CodeAPIRateLimited:
		return http.StatusTooManyRequests
	case code == model.CodeAPIConnectionFailed, code == model.CodeAPIProviderUnavailable, code == model.CodeSMTPConnectionFailed:
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadGateway
	}
}

func isRejection(code model.ErrorCode) bool {
	switch code {
	case model.CodeAPIInvalidReceiver, model.CodeAPISenderNotAllowed, model.CodeAPIInvalidAttachment,
		model.CodeAPIInvalidRequest, model.CodeAPIQuotaExceeded,
		model.CodeSMTPSenderRejected, model.CodeSMTPReceiverRejected, model.CodeSMTPMessageRejected:
		return true
	default:
		return false
	}
}
