package config

import (
	"fmt"
	"strings"
	"time"
)

const (
	envMaxAttempts    = "MAILER_MAX_ATTEMPTS"
	envRetryBaseDelay = "MAILER_RETRY_BASE_DELAY"
	envRetryMaxDelay  = "MAILER_RETRY_MAX_DELAY"

	defaultMaxAttempts    = 5
	defaultRetryBaseDelay = 30 * time.Second
	defaultRetryMaxDelay  = 10 * time.Minute
)

// RetryConfig bounds the attempts of queued requests that fail temporarily.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// NewRetryConfig reads MAILER_MAX_ATTEMPTS (default 5, 1 disables retries),
// MAILER_RETRY_BASE_DELAY (default 30s, doubled per attempt) and MAILER_RETRY_MAX_DELAY
// (default 10m).
func NewRetryConfig() (RetryConfig, error) {
	attempts, err := positiveInt(envMaxAttempts, defaultMaxAttempts)
	if err != nil {
		return RetryConfig{}, err
	}
	base, err := positiveDuration(envRetryBaseDelay, defaultRetryBaseDelay)
	if err != nil {
		return RetryConfig{}, err
	}
	maxDelay, err := positiveDuration(envRetryMaxDelay, defaultRetryMaxDelay)
	if err != nil {
		return RetryConfig{}, err
	}
	if maxDelay < base {
		return RetryConfig{}, fmt.Errorf("%s (%s) must not be lower than %s (%s)", envRetryMaxDelay, maxDelay, envRetryBaseDelay, base)
	}

	return RetryConfig{MaxAttempts: int(attempts), BaseDelay: base, MaxDelay: maxDelay}, nil
}

func positiveDuration(key string, defaultValue time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(envOrDefault(key, ""))
	if value == "" {
		return defaultValue, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration (e.g. 30s), got %q", key, value)
	}
	return duration, nil
}
