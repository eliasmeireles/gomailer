package client

// ErrorCode is the machine-readable reason of a failed delivery.
type ErrorCode string

// Error codes reported by gomailer (see the gomailer README for their meaning).
const (
	CodeMessageInvalidJSON      ErrorCode = "message_invalid_json"
	CodeMessageInvalidBody      ErrorCode = "message_invalid_body"
	CodeMessageMissingSender    ErrorCode = "message_missing_sender"
	CodeMessageMissingReceiver  ErrorCode = "message_missing_receiver"
	CodeMessageMissingSubject   ErrorCode = "message_missing_subject"
	CodeMessageMissingBody      ErrorCode = "message_missing_body"
	CodeSMTPConnectionFailed    ErrorCode = "smtp_connection_failed"
	CodeSMTPAuthorizationDenied ErrorCode = "smtp_authorization_denied"
	CodeSMTPSenderRejected      ErrorCode = "smtp_sender_rejected"
	CodeSMTPReceiverRejected    ErrorCode = "smtp_receiver_rejected"
	CodeSMTPMessageRejected     ErrorCode = "smtp_message_rejected"
	CodeSMTPTemporaryFailure    ErrorCode = "smtp_temporary_failure"
	CodeAPIConnectionFailed     ErrorCode = "api_connection_failed"
	CodeAPIAuthorizationDenied  ErrorCode = "api_authorization_denied"
	CodeAPISenderNotAllowed     ErrorCode = "api_sender_not_allowed"
	CodeAPIInvalidReceiver      ErrorCode = "api_invalid_receiver"
	CodeAPIInvalidAttachment    ErrorCode = "api_invalid_attachment"
	CodeAPIInvalidRequest       ErrorCode = "api_invalid_request"
	CodeAPIQuotaExceeded        ErrorCode = "api_quota_exceeded"
	CodeAPIRateLimited          ErrorCode = "api_rate_limited"
	CodeAPIProviderUnavailable  ErrorCode = "api_provider_unavailable"
	CodeAPIUnexpectedResponse   ErrorCode = "api_unexpected_response"
	CodeUnknown                 ErrorCode = "unknown_error"
)

// Retryable reports whether the failure is temporary (gomailer retries these for queued emails).
func (c ErrorCode) Retryable() bool {
	switch c {
	case CodeAPIConnectionFailed, CodeAPIRateLimited, CodeAPIProviderUnavailable,
		CodeSMTPConnectionFailed, CodeSMTPTemporaryFailure:
		return true
	default:
		return false
	}
}
