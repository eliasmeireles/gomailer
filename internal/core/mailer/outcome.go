package mailer

import (
	"time"

	"github.com/eliasmeireles/gomailer/internal/core/model"
)

// Action tells a queue source what to do with a message after processing it.
type Action int

const (
	// ActionDone acknowledges the message: sent, or failure accepted by the failure callback.
	ActionDone Action = iota
	// ActionRetry schedules another attempt after Outcome.RetryIn.
	ActionRetry
	// ActionDeadLetter moves the message to the dead-letter queue: final failure nobody handled.
	ActionDeadLetter
)

func (a Action) String() string {
	switch a {
	case ActionRetry:
		return "retry"
	case ActionDeadLetter:
		return "dead-letter"
	default:
		return "done"
	}
}

// Outcome is the result of processing one email request, used by every source (queues apply
// the Action, the HTTP API builds its response from the Event).
type Outcome struct {
	// Event describes the result: status sent, or failed with errorCode and cause.
	Event model.DeliveryEvent
	// Err is the delivery error; nil when the email was sent.
	Err error
	// Action is what a queue source must do with the message.
	Action Action
	// RetryIn is the wait before the next attempt when Action is ActionRetry.
	RetryIn time.Duration
}

// Sent reports whether the email was delivered.
func (o Outcome) Sent() bool {
	return o.Err == nil
}
