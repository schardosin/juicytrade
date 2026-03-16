package schwab

import (
	"sync"
	"time"
)

// rateLimiter implements a token bucket rate limiter for Schwab API requests.
//
// Schwab enforces ~120 requests/minute. The token bucket starts full and
// refills at a steady rate, allowing short bursts while enforcing the
// average rate over time.
type rateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// newRateLimiter creates a new token bucket rate limiter.
//
// maxTokens is the burst capacity (e.g., 120 for Schwab's 120 req/min).
// refillRate is tokens added per second (e.g., 2.0 for 120 tokens/60 seconds).
func newRateLimiter(maxTokens float64, refillRate float64) *rateLimiter {
	return &rateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// wait blocks until a rate limit token is available, then consumes one.
// This is called before every API request in doAuthenticatedRequest.
func (rl *rateLimiter) wait() {
	for {
		rl.mu.Lock()
		rl.refill()

		if rl.tokens >= 1.0 {
			rl.tokens--
			rl.mu.Unlock()
			return
		}

		rl.mu.Unlock()
		// Sleep briefly and retry — avoids busy-waiting while keeping
		// latency low. 50ms is a reasonable compromise.
		time.Sleep(50 * time.Millisecond)
	}
}

// allow checks if a token is available without blocking.
// Returns true and consumes a token if available, false otherwise.
func (rl *rateLimiter) allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()

	if rl.tokens >= 1.0 {
		rl.tokens--
		return true
	}
	return false
}

// refill adds tokens based on elapsed time since last refill.
// Must be called with rl.mu held.
func (rl *rateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()

	if elapsed > 0 {
		rl.tokens += elapsed * rl.refillRate
		if rl.tokens > rl.maxTokens {
			rl.tokens = rl.maxTokens
		}
		rl.lastRefill = now
	}
}

// available returns the current number of available tokens (for testing/monitoring).
func (rl *rateLimiter) available() float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.refill()
	return rl.tokens
}
