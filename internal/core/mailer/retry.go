package mailer

import "time"

// RetryPolicy bounds how many times a queued request is attempted and how long to wait between
// attempts. Attempt n (1-based) that fails temporarily is retried after Delay(n).
type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// DefaultRetryPolicy tries 5 times, waiting 30s, 1m, 2m and 4m between attempts.
var DefaultRetryPolicy = RetryPolicy{MaxAttempts: 5, BaseDelay: 30 * time.Second, MaxDelay: 10 * time.Minute}

// Delay returns the wait after the given failed attempt: BaseDelay doubled per attempt, capped
// at MaxDelay.
func (p RetryPolicy) Delay(attempt int) time.Duration {
	delay := p.BaseDelay
	for i := 1; i < attempt && delay < p.MaxDelay; i++ {
		delay *= 2
	}
	return min(delay, p.MaxDelay)
}

// Delays lists the distinct waits used by the policy, one per retry, for transports that need
// to declare them up front (e.g. RabbitMQ retry queues).
func (p RetryPolicy) Delays() []time.Duration {
	var delays []time.Duration
	for attempt := 1; attempt < p.MaxAttempts; attempt++ {
		delay := p.Delay(attempt)
		if len(delays) == 0 || delays[len(delays)-1] != delay {
			delays = append(delays, delay)
		}
	}
	return delays
}

// CanRetry reports whether another attempt is allowed after the given one.
func (p RetryPolicy) CanRetry(attempt int) bool {
	return attempt < p.MaxAttempts
}
