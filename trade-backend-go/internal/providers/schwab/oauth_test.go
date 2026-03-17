package schwab

import (
	"context"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// CreateState tests
// =============================================================================

func TestOAuthStore_CreateState_GeneratesUniqueTokens(t *testing.T) {
	store := NewSchwabOAuthStore()

	token1, err := store.CreateState("key", "secret", "https://cb", "https://api")
	if err != nil {
		t.Fatalf("CreateState returned error: %v", err)
	}

	token2, err := store.CreateState("key", "secret", "https://cb", "https://api")
	if err != nil {
		t.Fatalf("CreateState returned error: %v", err)
	}

	if token1 == token2 {
		t.Errorf("expected unique tokens, both are %q", token1)
	}
}

func TestOAuthStore_CreateState_StoresCorrectData(t *testing.T) {
	store := NewSchwabOAuthStore()

	token, err := store.CreateState("my-key", "my-secret", "https://cb.example.com", "https://api.schwab.com")
	if err != nil {
		t.Fatalf("CreateState returned error: %v", err)
	}

	state := store.GetState(token)
	if state == nil {
		t.Fatal("expected state to be found")
	}

	if state.AppKey != "my-key" {
		t.Errorf("expected AppKey 'my-key', got %q", state.AppKey)
	}
	if state.AppSecret != "my-secret" {
		t.Errorf("expected AppSecret 'my-secret', got %q", state.AppSecret)
	}
	if state.CallbackURL != "https://cb.example.com" {
		t.Errorf("expected CallbackURL 'https://cb.example.com', got %q", state.CallbackURL)
	}
	if state.BaseURL != "https://api.schwab.com" {
		t.Errorf("expected BaseURL 'https://api.schwab.com', got %q", state.BaseURL)
	}
	if state.Status != "pending" {
		t.Errorf("expected Status 'pending', got %q", state.Status)
	}
	if state.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

// =============================================================================
// GetState tests
// =============================================================================

func TestOAuthStore_GetState_NotFound(t *testing.T) {
	store := NewSchwabOAuthStore()

	state := store.GetState("nonexistent-token-abc123")
	if state != nil {
		t.Errorf("expected nil for unknown token, got %+v", state)
	}
}

func TestOAuthStore_GetState_Expired(t *testing.T) {
	store := NewSchwabOAuthStore()

	token, err := store.CreateState("key", "secret", "https://cb", "https://api")
	if err != nil {
		t.Fatalf("CreateState returned error: %v", err)
	}

	// Manually set CreatedAt to 11 minutes ago to simulate expiry
	state := store.GetState(token)
	if state == nil {
		t.Fatal("expected state to exist before expiry manipulation")
	}
	state.mu.Lock()
	state.CreatedAt = time.Now().Add(-11 * time.Minute)
	state.mu.Unlock()

	// Now GetState should return nil (expired)
	expired := store.GetState(token)
	if expired != nil {
		t.Error("expected nil for expired state")
	}
}

// =============================================================================
// UpdateState tests
// =============================================================================

func TestOAuthStore_UpdateState_TransitionsStatus(t *testing.T) {
	store := NewSchwabOAuthStore()

	token, _ := store.CreateState("key", "secret", "https://cb", "https://api")

	ok := store.UpdateState(token, func(s *OAuthFlowState) {
		s.Status = "exchanging"
	})
	if !ok {
		t.Fatal("expected UpdateState to return true")
	}

	state := store.GetState(token)
	if state.Status != "exchanging" {
		t.Errorf("expected Status 'exchanging', got %q", state.Status)
	}
}

func TestOAuthStore_UpdateState_NotFound(t *testing.T) {
	store := NewSchwabOAuthStore()

	ok := store.UpdateState("nonexistent", func(s *OAuthFlowState) {
		s.Status = "should-not-happen"
	})
	if ok {
		t.Error("expected UpdateState to return false for unknown token")
	}
}

// =============================================================================
// DeleteState tests
// =============================================================================

func TestOAuthStore_DeleteState(t *testing.T) {
	store := NewSchwabOAuthStore()

	token, _ := store.CreateState("key", "secret", "https://cb", "https://api")

	// Verify it exists
	if store.GetState(token) == nil {
		t.Fatal("expected state to exist before delete")
	}

	store.DeleteState(token)

	// Verify it's gone
	if store.GetState(token) != nil {
		t.Error("expected state to be nil after delete")
	}
}

// =============================================================================
// Cleanup tests
// =============================================================================

func TestOAuthStore_StartCleanup_RemovesExpired(t *testing.T) {
	store := NewSchwabOAuthStore()

	// Create a state and backdate it beyond TTL
	token, _ := store.CreateState("key", "secret", "https://cb", "https://api")
	state := store.GetState(token)
	state.mu.Lock()
	state.CreatedAt = time.Now().Add(-15 * time.Minute)
	state.mu.Unlock()

	// Create a fresh state that should survive cleanup
	freshToken, _ := store.CreateState("key2", "secret2", "https://cb2", "https://api2")

	// Run cleanup directly (no need to wait for the goroutine ticker)
	store.removeExpired()

	// Expired state should be gone (check raw map since GetState also checks expiry)
	if _, loaded := store.states.Load(token); loaded {
		t.Error("expected expired state to be removed by cleanup")
	}

	// Fresh state should still be there
	if store.GetState(freshToken) == nil {
		t.Error("expected fresh state to survive cleanup")
	}
}

func TestOAuthStore_StartCleanup_ContextCancel(t *testing.T) {
	store := NewSchwabOAuthStore()

	ctx, cancel := context.WithCancel(context.Background())
	store.StartCleanup(ctx)

	// Cancel immediately — the goroutine should exit without error or panic
	cancel()

	// Give the goroutine a moment to exit
	time.Sleep(50 * time.Millisecond)
}

// =============================================================================
// Token generation tests
// =============================================================================

func TestOAuthStore_StateToken_Length(t *testing.T) {
	// 32 bytes → base64url (no padding) = ceil(32*4/3) = 43 characters
	token, err := generateStateToken()
	if err != nil {
		t.Fatalf("generateStateToken returned error: %v", err)
	}

	if len(token) != 43 {
		t.Errorf("expected token length 43, got %d (token=%q)", len(token), token)
	}
}

// =============================================================================
// Concurrency tests
// =============================================================================

func TestOAuthStore_ConcurrentAccess(t *testing.T) {
	store := NewSchwabOAuthStore()

	const goroutines = 50
	const opsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Collect tokens from create goroutines for use by read/update goroutines
	tokens := make(chan string, goroutines*opsPerGoroutine)

	// Half the goroutines create states
	for i := 0; i < goroutines/2; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				token, err := store.CreateState("key", "secret", "https://cb", "https://api")
				if err != nil {
					t.Errorf("CreateState error: %v", err)
					return
				}
				tokens <- token
			}
		}()
	}

	// Other half read, update, and delete states
	for i := 0; i < goroutines/2; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				select {
				case token := <-tokens:
					// Read
					store.GetState(token)

					// Update
					store.UpdateState(token, func(s *OAuthFlowState) {
						s.Status = "exchanging"
					})

					// Read again
					store.GetState(token)

					// Delete half the time
					if j%2 == 0 {
						store.DeleteState(token)
					}
				default:
					// No tokens available yet, create and immediately read
					tk, _ := store.CreateState("k", "s", "c", "b")
					store.GetState(tk)
				}
			}
		}()
	}

	// Wait with timeout to detect deadlocks
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success — no races, no deadlocks
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for concurrent operations — possible deadlock")
	}
}
