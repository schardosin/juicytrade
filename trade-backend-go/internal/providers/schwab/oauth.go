package schwab

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// =============================================================================
// OAuth Flow State
// =============================================================================

// OAuthFlowState holds all state for one OAuth authorization flow.
// Sensitive fields are excluded from JSON serialization.
type OAuthFlowState struct {
	// Configuration (set at creation)
	AppKey      string `json:"-"`
	AppSecret   string `json:"-"`
	CallbackURL string `json:"-"`
	BaseURL     string `json:"-"`

	// Flow metadata
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"` // pending → exchanging → completed → finalized | failed

	// Tokens (set after callback)
	RefreshToken string    `json:"-"`
	AccessToken  string    `json:"-"`
	TokenExpiry  time.Time `json:"-"`

	// Accounts (set after account fetch)
	Accounts []SchwabAccountInfo `json:"accounts,omitempty"`

	// Error (set on failure)
	Error string `json:"error,omitempty"`

	// Re-auth context
	ExistingInstanceID string `json:"-"`

	// Protects state transitions
	mu sync.Mutex `json:"-"` //nolint:structcheck
}

// SchwabAccountInfo represents one Schwab brokerage account returned during OAuth.
type SchwabAccountInfo struct {
	AccountNumber string `json:"account_number"`
	HashValue     string `json:"hash_value"`
}

// =============================================================================
// OAuth State Store
// =============================================================================

const (
	oauthStateTTL      = 10 * time.Minute
	oauthCleanupPeriod = 60 * time.Second
)

// SchwabOAuthStore is a thread-safe in-memory store for OAuth flow states.
// Uses sync.Map for lock-free concurrent reads.
type SchwabOAuthStore struct {
	states sync.Map // map[string]*OAuthFlowState
}

// NewSchwabOAuthStore creates a new OAuth state store.
func NewSchwabOAuthStore() *SchwabOAuthStore {
	return &SchwabOAuthStore{}
}

// CreateState generates a cryptographically random state token and stores
// a new pending OAuth flow state. Returns the token or an error.
func (s *SchwabOAuthStore) CreateState(appKey, appSecret, callbackURL, baseURL string) (string, error) {
	token, err := generateStateToken()
	if err != nil {
		return "", fmt.Errorf("failed to generate state token: %w", err)
	}

	state := &OAuthFlowState{
		AppKey:      appKey,
		AppSecret:   appSecret,
		CallbackURL: callbackURL,
		BaseURL:     baseURL,
		CreatedAt:   time.Now(),
		Status:      "pending",
	}

	s.states.Store(token, state)
	return token, nil
}

// GetState returns the OAuth flow state for the given token, or nil if
// the token is not found or the state has expired (>10 minutes old).
func (s *SchwabOAuthStore) GetState(stateToken string) *OAuthFlowState {
	val, ok := s.states.Load(stateToken)
	if !ok {
		return nil
	}

	state := val.(*OAuthFlowState)

	// Check expiry
	if time.Since(state.CreatedAt) > oauthStateTTL {
		s.states.Delete(stateToken)
		return nil
	}

	return state
}

// UpdateState applies the given function to the state identified by stateToken.
// The state's mutex is held during the call to updateFn.
// Returns false if the state is not found or has expired.
func (s *SchwabOAuthStore) UpdateState(stateToken string, updateFn func(*OAuthFlowState)) bool {
	state := s.GetState(stateToken)
	if state == nil {
		return false
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	updateFn(state)
	return true
}

// DeleteState removes the state entry for the given token.
func (s *SchwabOAuthStore) DeleteState(stateToken string) {
	s.states.Delete(stateToken)
}

// StartCleanup launches a background goroutine that removes expired state
// entries every 60 seconds. The goroutine exits when ctx is cancelled.
func (s *SchwabOAuthStore) StartCleanup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(oauthCleanupPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.removeExpired()
			}
		}
	}()
}

// removeExpired deletes all state entries older than oauthStateTTL.
func (s *SchwabOAuthStore) removeExpired() {
	now := time.Now()
	removed := 0

	s.states.Range(func(key, value interface{}) bool {
		state := value.(*OAuthFlowState)
		if now.Sub(state.CreatedAt) > oauthStateTTL {
			s.states.Delete(key)
			removed++
		}
		return true
	})

	if removed > 0 {
		slog.Debug("cleaned up expired OAuth states", "removed", removed)
	}
}

// =============================================================================
// Token Generation
// =============================================================================

// generateStateToken generates a 32-byte cryptographically random token
// encoded as base64url (no padding). The result is 43 characters long.
func generateStateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
