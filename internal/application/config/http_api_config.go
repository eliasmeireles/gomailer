package config

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	envHTTPAPIPort         = "HTTP_API_PORT"
	envHTTPAPIKeys         = "HTTP_API_KEYS"
	envHTTPAPIMaxBodyBytes = "HTTP_API_MAX_BODY_BYTES"

	defaultHTTPAPIPort         = "8081"
	defaultHTTPAPIMaxBodyBytes = 25 << 20
)

// HTTPAPIConfig configures the HTTP source.
type HTTPAPIConfig struct {
	Port string
	// Keys are the accepted Bearer tokens.
	Keys         []string
	MaxBodyBytes int64
}

// NewHTTPAPIConfig loads the HTTP source configuration. HTTP_API_KEYS (comma-separated Bearer
// tokens) is required, so the endpoint can never run unauthenticated; HTTP_API_PORT defaults to
// 8081 and HTTP_API_MAX_BODY_BYTES to 25 MiB.
func NewHTTPAPIConfig() (HTTPAPIConfig, error) {
	keysValue, err := requireEnv(envHTTPAPIKeys)
	if err != nil {
		return HTTPAPIConfig{}, fmt.Errorf("the http source requires API keys: %w", err)
	}

	var keys []string
	for _, key := range strings.Split(keysValue, ",") {
		if trimmed := strings.TrimSpace(key); trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	if len(keys) == 0 {
		return HTTPAPIConfig{}, fmt.Errorf("%s must contain at least one key", envHTTPAPIKeys)
	}

	maxBody, err := positiveInt(envHTTPAPIMaxBodyBytes, defaultHTTPAPIMaxBodyBytes)
	if err != nil {
		return HTTPAPIConfig{}, err
	}

	return HTTPAPIConfig{
		Port:         envOrDefault(envHTTPAPIPort, defaultHTTPAPIPort),
		Keys:         keys,
		MaxBodyBytes: maxBody,
	}, nil
}

func positiveInt(key string, defaultValue int64) (int64, error) {
	value := envOrDefault(key, "")
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer, got %q", key, value)
	}
	return parsed, nil
}
