package messaging

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	log "github.com/sirupsen/logrus"

	"github.com/eliasmeireles/gomailer/internal/core/mailer"
)

const (
	headerAttempt   = "x-attempt"
	headerErrorCode = "x-error-code"
	headerCause     = "x-error-cause"
	headerFailedAt  = "x-failed-at"

	maxCauseHeaderChars = 1024
	publishTimeout      = 10 * time.Second
)

// Handler processes a message body on the given attempt (1-based).
type Handler func(body []byte, attempt int) mailer.Outcome

// apply acks the message after executing outcome.Action: retries are republished to the retry
// queue with the next attempt number, final unhandled failures to the dead-letter queue. When
// the republish is not confirmed, the message is nacked and requeued so it is never lost.
func (c *Consumer) apply(ch *amqp.Channel, msg amqp.Delivery, attempt int, outcome mailer.Outcome) {
	var (
		target  string
		headers amqp.Table
	)
	switch outcome.Action {
	case mailer.ActionRetry:
		target = c.topology.retryQueue(outcome.RetryIn)
		headers = amqp.Table{headerAttempt: int64(attempt + 1)}
	case mailer.ActionDeadLetter:
		target = c.topology.deadLetter
		headers = amqp.Table{
			headerAttempt:   int64(attempt),
			headerErrorCode: string(outcome.Event.ErrorCode),
			headerCause:     truncate(outcome.Event.Cause, maxCauseHeaderChars),
			headerFailedAt:  time.Now().UTC().Format(time.RFC3339),
		}
	default:
		ack(msg)
		return
	}

	if err := publishConfirmed(ch, target, msg, headers); err != nil {
		log.Errorf("Failed to move message to %s (%v); requeueing", target, err)
		if nackErr := msg.Nack(false, true); nackErr != nil {
			log.Errorf("Failed to nack message: %v", nackErr)
		}
		return
	}

	log.Infof("Message %s moved to %s (%s)", msg.MessageId, target, outcome.Action)
	ack(msg)
}

func publishConfirmed(ch *amqp.Channel, queue string, msg amqp.Delivery, headers amqp.Table) error {
	ctx, cancel := context.WithTimeout(context.Background(), publishTimeout)
	defer cancel()

	confirmation, err := ch.PublishWithDeferredConfirmWithContext(ctx, "", queue, false, false, amqp.Publishing{
		Headers:      headers,
		ContentType:  msg.ContentType,
		DeliveryMode: amqp.Persistent,
		MessageId:    msg.MessageId,
		Timestamp:    msg.Timestamp,
		Body:         msg.Body,
	})
	if err != nil {
		return err
	}

	acked, err := confirmation.WaitContext(ctx)
	if err != nil {
		return err
	}
	if !acked {
		return fmt.Errorf("broker rejected the publish")
	}
	return nil
}

// attemptOf reads the attempt number from the headers; messages without it are on attempt 1.
func attemptOf(headers amqp.Table) int {
	switch value := headers[headerAttempt].(type) {
	case int64:
		return max(int(value), 1)
	case int32:
		return max(int(value), 1)
	case int:
		return max(value, 1)
	default:
		return 1
	}
}

func ack(msg amqp.Delivery) {
	if err := msg.Ack(false); err != nil {
		log.Errorf("Failed to ack message: %v", err)
	}
}

func truncate(value string, maxChars int) string {
	runes := []rune(value)
	if len(runes) <= maxChars {
		return value
	}
	return string(runes[:maxChars])
}
