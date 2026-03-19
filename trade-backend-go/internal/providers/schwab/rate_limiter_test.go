package schwab

import (
	"sync"
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rl := newRateLimiter(120, 2.0)

	if rl.maxTokens != 120 {
		t.Errorf("expected maxTokens 120, got %f", rl.maxTokens)
	}
	if rl.refillRate != 2.0 {
		t.Errorf("expected refillRate 2.0, got %f", rl.refillRate)
	}

	// Bucket should start full
	avail := rl.available()
	if avail < 119.9 || avail > 120.0 {
		t.Errorf("expected ~120 tokens initially, got %f", avail)
	}
}

func TestRateLimiterWait_ImmediateReturn(t *testing.T) {
	rl := newRateLimiter(10, 1.0)

	start := time.Now()
	rl.wait()
	elapsed := time.Since(start)

	// Should return nearly instantly when tokens are available
	if elapsed > 10*time.Millisecond {
		t.Errorf("wait() took too long when tokens available: %v", elapsed)
	}
}

func TestRateLimiterAllow_Available(t *testing.T) {
	rl := newRateLimiter(10, 1.0)

	ok := rl.allow()
	if !ok {
		t.Error("expected allow() to return true when tokens available")
	}

	// Should have consumed one token
	avail := rl.available()
	if avail < 8.9 || avail > 9.1 {
		t.Errorf("expected ~9 tokens after one allow(), got %f", avail)
	}
}

func TestRateLimiterAllow_Exhausted(t *testing.T) {
	rl := newRateLimiter(5, 0.0) // refillRate 0 — no refill

	// Consume all tokens
	for i := 0; i < 5; i++ {
		ok := rl.allow()
		if !ok {
			t.Fatalf("expected allow() to return true on call %d", i+1)
		}
	}

	// Next call should fail
	ok := rl.allow()
	if ok {
		t.Error("expected allow() to return false when bucket is empty")
	}
}

func TestRateLimiterRefill(t *testing.T) {
	rl := newRateLimiter(5, 100.0) // High refill rate: 100 tokens/sec

	// Consume all tokens
	for i := 0; i < 5; i++ {
		rl.allow()
	}

	// Verify empty
	if rl.allow() {
		t.Fatal("expected bucket to be empty")
	}

	// Wait for refill — at 100 tokens/sec, 50ms should add ~5 tokens
	time.Sleep(60 * time.Millisecond)

	avail := rl.available()
	if avail < 4.0 {
		t.Errorf("expected tokens to refill after waiting, got %f", avail)
	}
}

func TestRateLimiterRefill_Capped(t *testing.T) {
	rl := newRateLimiter(10, 100.0) // High refill rate

	// Bucket starts full, wait a bit — should not exceed maxTokens
	time.Sleep(50 * time.Millisecond)

	avail := rl.available()
	if avail > 10.0 {
		t.Errorf("expected tokens capped at maxTokens (10), got %f", avail)
	}
}

func TestRateLimiterBurstCapacity(t *testing.T) {
	rl := newRateLimiter(50, 1.0)

	// Should be able to make 50 requests immediately (burst)
	for i := 0; i < 50; i++ {
		ok := rl.allow()
		if !ok {
			t.Fatalf("expected burst of 50, but allow() returned false on call %d", i+1)
		}
	}

	// 51st should fail (refill rate is too slow for immediate follow-up)
	ok := rl.allow()
	if ok {
		t.Error("expected allow() to fail after burst capacity exhausted")
	}
}

func TestRateLimiterWait_BlocksWhenEmpty(t *testing.T) {
	rl := newRateLimiter(1, 20.0) // 1 token max, refills at 20/sec

	// Consume the single token
	rl.allow()

	// wait() should block briefly, then return after refill
	start := time.Now()
	rl.wait()
	elapsed := time.Since(start)

	// Should block for ~50ms (the sleep interval) then get a refilled token
	if elapsed < 30*time.Millisecond {
		t.Errorf("expected wait() to block briefly, but returned in %v", elapsed)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("wait() blocked too long: %v", elapsed)
	}
}

func TestRateLimiterConcurrency(t *testing.T) {
	rl := newRateLimiter(100, 50.0)

	var wg sync.WaitGroup
	const goroutines = 20
	const requestsPerGoroutine = 10

	// Launch many goroutines calling wait() concurrently
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				rl.wait()
			}
		}()
	}

	// Should complete without deadlock or panic
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent wait() calls did not complete within timeout — possible deadlock")
	}
}

func TestRateLimiterConcurrency_Allow(t *testing.T) {
	rl := newRateLimiter(100, 0.0) // No refill — exactly 100 tokens

	var wg sync.WaitGroup
	var successCount int64
	var mu sync.Mutex
	const goroutines = 20

	// Each goroutine tries to get tokens
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if rl.allow() {
					mu.Lock()
					successCount++
					mu.Unlock()
				} else {
					return
				}
			}
		}()
	}

	wg.Wait()

	// Exactly 100 tokens should have been consumed (no double-spending)
	if successCount != 100 {
		t.Errorf("expected exactly 100 successful allow() calls, got %d", successCount)
	}
}
