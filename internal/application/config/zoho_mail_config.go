package config

import "strings"

const (
	envZohoMailAccountID  = "ZOHO_MAIL_ACCOUNT_ID"
	envZohoClientID       = "ZOHO_CLIENT_ID"
	envZohoClientSecret   = "ZOHO_CLIENT_SECRET"
	envZohoRefreshToken   = "ZOHO_REFRESH_TOKEN"
	envZohoMailAPIURL     = "ZOHO_MAIL_API_URL"
	envZohoAccountsURL    = "ZOHO_ACCOUNTS_URL"
	defaultZohoMailAPIURL = "https://mail.zoho.com"
	defaultZohoAccountURL = "https://accounts.zoho.com"
)

// ZohoMailConfig holds the Zoho Mail API (OAuth 2.0) configuration.
type ZohoMailConfig struct {
	AccountID    string
	ClientID     string
	ClientSecret string
	RefreshToken string
	// APIURL is the data-center specific Mail API host (e.g. https://mail.zoho.eu).
	APIURL string
	// AccountsURL is the data-center specific OAuth host (e.g. https://accounts.zoho.eu).
	AccountsURL string
}

// NewZohoMailConfig loads the Zoho Mail configuration. ZOHO_MAIL_ACCOUNT_ID, ZOHO_CLIENT_ID,
// ZOHO_CLIENT_SECRET and ZOHO_REFRESH_TOKEN are required; ZOHO_MAIL_API_URL and
// ZOHO_ACCOUNTS_URL default to the .com data center.
func NewZohoMailConfig() (ZohoMailConfig, error) {
	values := map[string]string{}
	for _, key := range []string{envZohoMailAccountID, envZohoClientID, envZohoClientSecret, envZohoRefreshToken} {
		value, err := requireEnv(key)
		if err != nil {
			return ZohoMailConfig{}, err
		}
		values[key] = value
	}

	return ZohoMailConfig{
		AccountID:    values[envZohoMailAccountID],
		ClientID:     values[envZohoClientID],
		ClientSecret: values[envZohoClientSecret],
		RefreshToken: values[envZohoRefreshToken],
		APIURL:       strings.TrimRight(envOrDefault(envZohoMailAPIURL, defaultZohoMailAPIURL), "/"),
		AccountsURL:  strings.TrimRight(envOrDefault(envZohoAccountsURL, defaultZohoAccountURL), "/"),
	}, nil
}
