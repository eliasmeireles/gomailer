package config

import "strings"

const (
	envZeptoMailAPIKey = "ZEPTOMAIL_API_KEY"
	envZeptoMailAPIURL = "ZEPTOMAIL_API_URL"

	defaultZeptoMailAPIURL = "https://api.zeptomail.com"
	zeptoMailKeyPrefix     = "Zoho-enczapikey "
)

// ZeptoMailConfig holds the ZeptoMail (Zoho transactional email) API configuration.
type ZeptoMailConfig struct {
	// Authorization is the full header value, always prefixed with "Zoho-enczapikey ".
	Authorization string
	// BaseURL is the region-specific API host (e.g. https://api.zeptomail.eu).
	BaseURL string
}

// NewZeptoMailConfig loads the ZeptoMail configuration. ZEPTOMAIL_API_KEY is required and may be
// given with or without the "Zoho-enczapikey " prefix; ZEPTOMAIL_API_URL defaults to
// https://api.zeptomail.com.
func NewZeptoMailConfig() (ZeptoMailConfig, error) {
	apiKey, err := requireEnv(envZeptoMailAPIKey)
	if err != nil {
		return ZeptoMailConfig{}, err
	}

	return ZeptoMailConfig{
		Authorization: zeptoMailKeyPrefix + strings.TrimSpace(strings.TrimPrefix(apiKey, zeptoMailKeyPrefix)),
		BaseURL:       strings.TrimRight(envOrDefault(envZeptoMailAPIURL, defaultZeptoMailAPIURL), "/"),
	}, nil
}
