package config

import "strings"

const (
	envResendAPIKey = "RESEND_API_KEY"
	envResendAPIURL = "RESEND_API_URL"

	defaultResendAPIURL = "https://api.resend.com"
)

// ResendConfig holds the Resend HTTP API configuration.
type ResendConfig struct {
	APIKey  string
	BaseURL string
}

// NewResendConfig loads the Resend configuration. RESEND_API_KEY is required;
// RESEND_API_URL defaults to https://api.resend.com.
func NewResendConfig() (ResendConfig, error) {
	apiKey, err := requireEnv(envResendAPIKey)
	if err != nil {
		return ResendConfig{}, err
	}

	return ResendConfig{
		APIKey:  apiKey,
		BaseURL: strings.TrimRight(envOrDefault(envResendAPIURL, defaultResendAPIURL), "/"),
	}, nil
}
