package mailer

import (
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/eliasmeireles/gomailer/internal/application/config"
	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

// zohoTokenRefreshMargin renews the access token this long before Zoho expires it.
const zohoTokenRefreshMargin = time.Minute

type zohoTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	Error       string `json:"error"`
}

// zohoTokenSource exchanges the long-lived refresh token for short-lived access tokens and caches
// them until shortly before expiry. Safe for concurrent use.
type zohoTokenSource struct {
	config     config.ZohoMailConfig
	httpClient *http.Client
	now        func() time.Time

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

func newZohoTokenSource(cfg config.ZohoMailConfig, httpClient *http.Client) *zohoTokenSource {
	return &zohoTokenSource{config: cfg, httpClient: httpClient, now: time.Now}
}

// Token returns a valid access token, refreshing it when missing or about to expire.
func (s *zohoTokenSource) Token() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token != "" && s.now().Before(s.expiresAt) {
		return s.token, nil
	}

	token, err := s.refresh()
	if err != nil {
		return "", err
	}

	s.token = token.AccessToken
	s.expiresAt = s.now().Add(time.Duration(token.ExpiresIn)*time.Second - zohoTokenRefreshMargin)
	return s.token, nil
}

// Invalidate drops the cached token so the next Token call refreshes it (e.g. after a 401).
func (s *zohoTokenSource) Invalidate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.token = ""
}

func (s *zohoTokenSource) refresh() (zohoTokenResponse, error) {
	query := url.Values{
		"refresh_token": {s.config.RefreshToken},
		"client_id":     {s.config.ClientID},
		"client_secret": {s.config.ClientSecret},
		"grant_type":    {"refresh_token"},
	}

	req, err := http.NewRequest(http.MethodPost, s.config.AccountsURL+"/oauth/v2/token?"+query.Encode(), nil)
	if err != nil {
		return zohoTokenResponse{}, mailer.Errorf(model.CodeAPIConnectionFailed, "failed to create Zoho token request: %w", err)
	}

	var token zohoTokenResponse
	if err := doRequest(s.httpClient, req, &token); err != nil {
		return zohoTokenResponse{}, s.describeRefreshError(err)
	}
	if token.Error != "" || token.AccessToken == "" {
		return zohoTokenResponse{}, mailer.Errorf(model.CodeAPIAuthorizationDenied, "failed to refresh Zoho access token: %q", token.Error)
	}
	return token, nil
}

// describeRefreshError classifies a rejected refresh (any HTTP error status) as
// api_authorization_denied; transport errors stay api_connection_failed.
func (s *zohoTokenSource) describeRefreshError(err error) error {
	described := describeAPIError("zoho accounts", err, noClassification)
	if _, rejected := err.(*httpStatusError); rejected {
		return mailer.NewError(model.CodeAPIAuthorizationDenied, described)
	}
	return described
}
