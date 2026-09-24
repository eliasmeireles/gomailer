package messaging

import (
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
)

func TestTopology(t *testing.T) {
	top := newTopology("mailer-service", []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute})

	t.Run("must name the dead-letter and retry queues after the main queue", func(t *testing.T) {
		assert.Equal(t, "mailer-service.dlq", top.deadLetter)
		assert.Equal(t, []string{"mailer-service.retry.30s", "mailer-service.retry.1m", "mailer-service.retry.2m"}, top.retryQueues())
	})

	t.Run("given a delay then pick the first retry queue that waits at least that long", func(t *testing.T) {
		assert.Equal(t, "mailer-service.retry.30s", top.retryQueue(10*time.Second))
		assert.Equal(t, "mailer-service.retry.1m", top.retryQueue(time.Minute))
		assert.Equal(t, "mailer-service.retry.2m", top.retryQueue(90*time.Second))
	})

	t.Run("given a delay longer than every queue then use the longest", func(t *testing.T) {
		assert.Equal(t, "mailer-service.retry.2m", top.retryQueue(time.Hour))
	})
}

func TestCompactDuration(t *testing.T) {
	cases := map[time.Duration]string{
		500 * time.Millisecond: "500ms",
		5 * time.Second:        "5s",
		90 * time.Second:       "90s",
		4 * time.Minute:        "4m",
		2 * time.Hour:          "2h",
	}

	for duration, expected := range cases {
		t.Run("given "+duration.String()+" then "+expected, func(t *testing.T) {
			assert.Equal(t, expected, compactDuration(duration))
		})
	}
}

func TestAttemptOf(t *testing.T) {
	cases := map[string]struct {
		headers  amqp.Table
		expected int
	}{
		"given no headers then first attempt":    {nil, 1},
		"given an int64 attempt then read it":    {amqp.Table{headerAttempt: int64(3)}, 3},
		"given an int32 attempt then read it":    {amqp.Table{headerAttempt: int32(2)}, 2},
		"given an int attempt then read it":      {amqp.Table{headerAttempt: 4}, 4},
		"given a zero attempt then first":        {amqp.Table{headerAttempt: int64(0)}, 1},
		"given a non-numeric attempt then first": {amqp.Table{headerAttempt: "3"}, 1},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, attemptOf(tc.headers))
		})
	}
}

func TestTruncate(t *testing.T) {
	t.Run("given a short value then keep it", func(t *testing.T) {
		assert.Equal(t, "abc", truncate("abc", 5))
	})

	t.Run("given a long multibyte value then cut by runes", func(t *testing.T) {
		assert.Equal(t, "日本", truncate("日本語", 2))
	})
}
