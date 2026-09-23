package mailer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetryPolicy(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 6, BaseDelay: 30 * time.Second, MaxDelay: 2 * time.Minute}

	t.Run("must double the delay per attempt up to the max", func(t *testing.T) {
		assert.Equal(t, 30*time.Second, policy.Delay(1))
		assert.Equal(t, time.Minute, policy.Delay(2))
		assert.Equal(t, 2*time.Minute, policy.Delay(3))
		assert.Equal(t, 2*time.Minute, policy.Delay(4))
	})

	t.Run("must list distinct delays for every retry", func(t *testing.T) {
		assert.Equal(t, []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute}, policy.Delays())
	})

	t.Run("must allow retries only before the last attempt", func(t *testing.T) {
		assert.True(t, policy.CanRetry(5))
		assert.False(t, policy.CanRetry(6))
	})

	t.Run("given a single attempt then there are no delays", func(t *testing.T) {
		assert.Empty(t, RetryPolicy{MaxAttempts: 1, BaseDelay: time.Second, MaxDelay: time.Second}.Delays())
	})

	t.Run("must use 30s, 1m, 2m and 4m by default", func(t *testing.T) {
		assert.Equal(t, []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute}, DefaultRetryPolicy.Delays())
	})
}
