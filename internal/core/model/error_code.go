package model

// ErrorCode is a stable, machine-readable reason for a delivery failure, reported in the
// failure callback as "errorCode". Codes are prefixed by where the failure happened.
type ErrorCode string

// Message validation, before any transport is called.
const (
	CodeMessageInvalidJSON     ErrorCode = "message_invalid_json"
	CodeMessageInvalidBody     ErrorCode = "message_invalid_body"
	CodeMessageMissingSender   ErrorCode = "message_missing_sender"
	CodeMessageMissingReceiver ErrorCode = "message_missing_receiver"
	CodeMessageMissingSubject  ErrorCode = "message_missing_subject"
	CodeMessageMissingBody     ErrorCode = "message_missing_body"
)

// SMTP transport, by the step that failed.
const (
	CodeSMTPConnectionFailed    ErrorCode = "smtp_connection_failed"
	CodeSMTPAuthorizationDenied ErrorCode = "smtp_authorization_denied"
	CodeSMTPSenderRejected      ErrorCode = "smtp_sender_rejected"
	CodeSMTPReceiverRejected    ErrorCode = "smtp_receiver_rejected"
	CodeSMTPMessageRejected     ErrorCode = "smtp_message_rejected"
	// CodeSMTPTemporaryFailure is any 4xx SMTP reply (e.g. 421, 450, 451): try again later.
	CodeSMTPTemporaryFailure ErrorCode = "smtp_temporary_failure"
)

// Provider HTTP APIs (resend, zoho, zeptomail), normalized across providers.
const (
	CodeAPIConnectionFailed    ErrorCode = "api_connection_failed"
	CodeAPIAuthorizationDenied ErrorCode = "api_authorization_denied"
	CodeAPISenderNotAllowed    ErrorCode = "api_sender_not_allowed"
	CodeAPIInvalidReceiver     ErrorCode = "api_invalid_receiver"
	CodeAPIInvalidAttachment   ErrorCode = "api_invalid_attachment"
	CodeAPIInvalidRequest      ErrorCode = "api_invalid_request"
	CodeAPIQuotaExceeded       ErrorCode = "api_quota_exceeded"
	CodeAPIRateLimited         ErrorCode = "api_rate_limited"
	CodeAPIProviderUnavailable ErrorCode = "api_provider_unavailable"
	CodeAPIUnexpectedResponse  ErrorCode = "api_unexpected_response"
)

// CodeUnknown is used when a failure carries no classification.
const CodeUnknown ErrorCode = "unknown_error"

// Retryable reports whether the failure is temporary, so the same request may succeed later.
func (c ErrorCode) Retryable() bool {
	switch c {
	case CodeAPIConnectionFailed, CodeAPIRateLimited, CodeAPIProviderUnavailable,
		CodeSMTPConnectionFailed, CodeSMTPTemporaryFailure:
		return true
	default:
		return false
	}
}
