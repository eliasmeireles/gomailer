package messaging

import "github.com/eliasmeireles/gomailer/internal/core/mailer"

// Headers shared by the queue transports (RabbitMQ headers, Kafka record headers).
const (
	// headerAttempt is the 1-based attempt of the message; absent means attempt 1.
	headerAttempt = "x-attempt"
	// headerNotBefore is the Unix time in milliseconds before which a Kafka retry must wait.
	headerNotBefore = "x-not-before"
	headerErrorCode = "x-error-code"
	headerCause     = "x-error-cause"
	headerFailedAt  = "x-failed-at"

	maxCauseHeaderChars = 1024
)

// Handler processes a message body on the given attempt (1-based).
type Handler func(body []byte, attempt int) mailer.Outcome

func truncate(value string, maxChars int) string {
	runes := []rune(value)
	if len(runes) <= maxChars {
		return value
	}
	return string(runes[:maxChars])
}
