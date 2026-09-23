package messaging

import (
	"fmt"
	"slices"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// topology is the set of queues used by the consumer:
//
//	<queue>              main queue (durable), consumed
//	<queue>.retry.<wait> one per retry delay; messages expire after <wait> and are dead-lettered
//	                     back to <queue> (no plugin needed)
//	<queue>.dlq          final failures nobody handled
type topology struct {
	queue       string
	deadLetter  string
	retryDelays []time.Duration
}

func newTopology(queue string, retryDelays []time.Duration) topology {
	return topology{queue: queue, deadLetter: queue + ".dlq", retryDelays: retryDelays}
}

// retryQueue returns the retry queue for delay: the first one waiting at least that long, or the
// longest one.
func (t topology) retryQueue(delay time.Duration) string {
	index := slices.IndexFunc(t.retryDelays, func(d time.Duration) bool { return d >= delay })
	if index < 0 {
		index = len(t.retryDelays) - 1
	}
	return retryQueueName(t.queue, t.retryDelays[index])
}

// retryQueues lists every retry queue name.
func (t topology) retryQueues() []string {
	names := make([]string, len(t.retryDelays))
	for i, delay := range t.retryDelays {
		names[i] = retryQueueName(t.queue, delay)
	}
	return names
}

func (t topology) declare(ch *amqp.Channel) error {
	if _, err := ch.QueueDeclare(t.queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue %s: %w", t.queue, err)
	}
	if _, err := ch.QueueDeclare(t.deadLetter, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue %s: %w", t.deadLetter, err)
	}

	for _, delay := range t.retryDelays {
		name := retryQueueName(t.queue, delay)
		args := amqp.Table{
			"x-message-ttl":             delay.Milliseconds(),
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": t.queue,
		}
		if _, err := ch.QueueDeclare(name, true, false, false, false, args); err != nil {
			return fmt.Errorf("declare retry queue %s: %w", name, err)
		}
	}
	return nil
}

// retryQueueName embeds the delay in the name (e.g. mailer-service.retry.30s), so changing the
// policy declares new queues instead of conflicting with the TTL of existing ones.
func retryQueueName(queue string, delay time.Duration) string {
	return fmt.Sprintf("%s.retry.%s", queue, compactDuration(delay))
}

func compactDuration(d time.Duration) string {
	switch {
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	case d%time.Second == 0:
		return fmt.Sprintf("%ds", d/time.Second)
	default:
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
}
