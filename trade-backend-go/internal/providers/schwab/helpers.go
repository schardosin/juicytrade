package schwab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// buildMarketDataURL constructs a full URL for the Schwab Market Data API.
func (s *SchwabProvider) buildMarketDataURL(path string) string {
	return s.baseURL + "/marketdata/v1" + path
}

// buildTraderURL constructs a full URL for the Schwab Trader API.
func (s *SchwabProvider) buildTraderURL(path string) string {
	return s.baseURL + "/trader/v1" + path
}

// doAuthenticatedRequest performs an HTTP request with OAuth Bearer token,
// rate limiting, and standard error handling.
//
// All API calls (except token refresh) go through this helper.
// On 401, it force-refreshes the token and retries once.
//
// The body parameter is a []byte (not io.Reader) so that the same payload
// can be re-sent on a 401 retry without being consumed.
func (s *SchwabProvider) doAuthenticatedRequest(ctx context.Context, method, url string, body []byte) ([]byte, int, error) {
	// 1. Wait for rate limiter (nil-safe for early development steps)
	if s.rateLimiter != nil {
		s.rateLimiter.wait()
	}

	// 2. Ensure valid access token
	if err := s.ensureValidToken(); err != nil {
		return nil, 0, fmt.Errorf("schwab: authentication failed: %w", err)
	}

	// 3. Execute the request
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	respBody, statusCode, err := s.executeHTTPRequest(ctx, method, url, bodyReader)
	if err != nil {
		return nil, 0, err
	}

	// 4. On 401, force-refresh and retry once
	if statusCode == http.StatusUnauthorized {
		s.logger.Warn("received 401, attempting token refresh and retry",
			"method", method, "url", url,
		)

		// Force refresh: clear current token so ensureValidToken must refresh
		s.tokenMu.Lock()
		s.accessToken = ""
		s.tokenExpiry = time.Time{}
		s.tokenMu.Unlock()

		if err := s.ensureValidToken(); err != nil {
			return nil, http.StatusUnauthorized, fmt.Errorf("schwab: re-authentication failed after 401: %w", err)
		}

		// Retry the request with the same body ([]byte can be re-read)
		var retryReader io.Reader
		if body != nil {
			retryReader = bytes.NewReader(body)
		}
		respBody, statusCode, err = s.executeHTTPRequest(ctx, method, url, retryReader)
		if err != nil {
			return nil, 0, err
		}

		if statusCode == http.StatusUnauthorized {
			return nil, statusCode, parseErrorResponse(respBody, statusCode)
		}
	}

	// 5. Handle rate limiting
	if statusCode == http.StatusTooManyRequests {
		s.logger.Warn("rate limited by Schwab API", "status", 429, "method", method, "url", url)
		return nil, statusCode, fmt.Errorf("schwab: rate limited (HTTP 429)")
	}

	// 6. Handle other errors
	if statusCode >= 400 {
		return nil, statusCode, parseErrorResponse(respBody, statusCode)
	}

	return respBody, statusCode, nil
}

// executeHTTPRequest builds and executes a single HTTP request with current auth headers.
func (s *SchwabProvider) executeHTTPRequest(ctx context.Context, method, url string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, 0, fmt.Errorf("schwab: failed to create request: %w", err)
	}

	// Set auth and accept headers
	s.tokenMu.Lock()
	token := s.accessToken
	s.tokenMu.Unlock()

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	// Set Content-Type for methods with request bodies
	if method == http.MethodPost || method == http.MethodPut {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("schwab: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("schwab: failed to read response body: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// parseErrorResponse extracts a meaningful error message from a Schwab API error response.
//
// Schwab error responses may have different formats:
//   - {"error": "invalid_grant", "error_description": "Token expired"} (OAuth errors)
//   - {"errors": [{"id": "...", "status": 400, "title": "Bad Request", "detail": "..."}]} (API errors)
//   - {"message": "Not Found"} (simple message errors)
//   - Plain text error messages
func parseErrorResponse(body []byte, statusCode int) error {
	if len(body) == 0 {
		return fmt.Errorf("schwab: HTTP %d (empty response)", statusCode)
	}

	// Try to parse as JSON
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		// Not JSON — return truncated plain text
		return fmt.Errorf("schwab: HTTP %d: %s", statusCode, truncateBody(body, 200))
	}

	// Format 1: OAuth error — {"error": "...", "error_description": "..."}
	if errField, ok := raw["error"].(string); ok {
		desc, _ := raw["error_description"].(string)
		if desc != "" {
			return fmt.Errorf("schwab: %s: %s", errField, desc)
		}
		return fmt.Errorf("schwab: %s", errField)
	}

	// Format 2: API errors array — {"errors": [{"message": "...", "detail": "..."}]}
	if errorsField, ok := raw["errors"]; ok {
		if errArray, ok := errorsField.([]interface{}); ok && len(errArray) > 0 {
			if firstErr, ok := errArray[0].(map[string]interface{}); ok {
				// Try "message" first, then "detail", then "title"
				for _, key := range []string{"message", "detail", "title"} {
					if msg, ok := firstErr[key].(string); ok && msg != "" {
						return fmt.Errorf("schwab: %s", msg)
					}
				}
			}
		}
	}

	// Format 3: Simple message — {"message": "..."}
	if msg, ok := raw["message"].(string); ok && msg != "" {
		return fmt.Errorf("schwab: %s", msg)
	}

	// Fallback: return truncated body
	return fmt.Errorf("schwab: HTTP %d: %s", statusCode, truncateBody(body, 200))
}

// truncateBody returns the first n bytes of a body as a string for error messages.
func truncateBody(body []byte, maxLen int) string {
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "..."
}
