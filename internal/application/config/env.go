package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	envHTTPClientTimeout     = "HTTP_CLIENT_TIMEOUT"
	defaultHTTPClientTimeout = 15 * time.Second
)

// HTTPClientTimeout returns the timeout applied to outbound HTTP calls (Resend API and
// failure callbacks), read from HTTP_CLIENT_TIMEOUT as a Go duration (e.g. "10s").
// Defaults to 15s when unset.
func HTTPClientTimeout() (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(envHTTPClientTimeout))
	if value == "" {
		return defaultHTTPClientTimeout, nil
	}

	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration (e.g. 15s), got %q", envHTTPClientTimeout, value)
	}
	return timeout, nil
}

func requireEnv(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return value, nil
}

func envOrDefault(key, defaultValue string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	return value
}
