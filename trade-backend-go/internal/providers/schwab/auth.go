package schwab

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// schwabTokenResponse represents the JSON response from the Schwab OAuth token endpoint.
type schwabTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`    // seconds until expiry (~1800)
	RefreshToken string `json:"refresh_token"` // new refresh token (if rotated)
	Scope        string `json:"scope"`
	IDToken      string `json:"id_token"`
}

// ErrRefreshTokenExpired is returned when the Schwab refresh token has expired.
// The user must re-authenticate at developer.schwab.com to obtain a new one.
var ErrRefreshTokenExpired = errors.New("schwab: refresh token expired, user must re-authenticate at developer.schwab.com")

// ensureValidToken ensures the access token is valid, refreshing if necessary.
// Called before every API request. Thread-safe via tokenMu.
func (s *SchwabProvider) ensureValidToken() error {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()

	// If token exists and won't expire for at least 5 minutes, use it
	if s.accessToken != "" && time.Until(s.tokenExpiry) > 5*time.Minute {
		return nil
	}

	// Token missing, expired, or about to expire — refresh it
	return s.refreshAccessToken()
}

// refreshAccessToken exchanges the refresh token for a new access token.
// Must be called with tokenMu held.
//
// Uses direct net/http (not doAuthenticatedRequest) to avoid circular dependency:
// the rate limiter and authenticated request helper both need a valid token,
// but this function is what obtains one.
func (s *SchwabProvider) refreshAccessToken() error {
	tokenURL := fmt.Sprintf("%s/v1/oauth/token", s.baseURL)

	// Build form body
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", s.refreshToken)

	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("schwab: failed to create token request: %w", err)
	}

	// HTTP Basic auth: base64(appKey:appSecret)
	basicAuth := base64.StdEncoding.EncodeToString([]byte(s.appKey + ":" + s.appSecret))
	req.Header.Set("Authorization", "Basic "+basicAuth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("schwab: token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("schwab: failed to read token response body: %w", err)
	}

	// Handle error status codes
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrRefreshTokenExpired
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("schwab: token request returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp schwabTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("schwab: failed to parse token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return fmt.Errorf("schwab: token response missing access_token")
	}

	// Update in-memory token state
	s.accessToken = tokenResp.AccessToken
	s.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	s.logger.Info("token refreshed successfully",
		"expires_in", tokenResp.ExpiresIn,
		"token_type", tokenResp.TokenType,
	)

	// Warn about token rotation — we cannot persist the new refresh token
	// back to the credential store from within the provider.
	if tokenResp.RefreshToken != "" && tokenResp.RefreshToken != s.refreshToken {
		s.logger.Warn("schwab: refresh token was rotated by server — new token cannot be persisted automatically",
			"old_prefix", truncateToken(s.refreshToken),
			"new_prefix", truncateToken(tokenResp.RefreshToken),
		)
	}

	return nil
}

// truncateToken returns the first 8 characters of a token for safe logging.
func truncateToken(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:8] + "..."
}
