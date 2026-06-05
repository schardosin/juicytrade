package indicators

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"trade-backend-go/internal/automation/types"
)

// --- AC-2: Retry Logic ---

func TestQA_RetryExhaustsOnThirdAttempt(t *testing.T) {
	// Fail attempts 1 and 2 (transient), succeed on attempt 3.
	callCount := 0
	mock := &mockProvider{
		getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
			callCount++
			if callCount <= 2 {
				return nil, errors.New("read tcp 10.42.2.45:43088->52.5.212.192:443: i/o timeout")
			}
			return []map[string]interface{}{{"close": 17.8}}, nil
		},
	}
	s := newTestService(mock)

	value, err := s.getVIXValueWithRetry(context.Background(), "VIX")
	if err != nil {
		t.Fatalf("expected success on 3rd attempt, got error: %v", err)
	}
	if value != 17.8 {
		t.Errorf("expected value 17.8, got %f", value)
	}
	if mock.getCalls() != 3 {
		t.Errorf("expected exactly 3 calls to GetHistoricalBars, got %d", mock.getCalls())
	}
}

func TestQA_RetryTransientThenNonTransient(t *testing.T) {
	// Attempt 1: transient error (i/o timeout)
	// Attempt 2: non-transient error (no VIX historical data available)
	// Should stop after 2 calls and return non-transient error.
	callCount := 0
	mock := &mockProvider{
		getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
			callCount++
			if callCount == 1 {
				return nil, errors.New("read tcp 10.42.2.45:43088->52.5.212.192:443: i/o timeout")
			}
			return nil, errors.New("no VIX historical data available")
		},
	}
	s := newTestService(mock)

	_, err := s.getVIXValueWithRetry(context.Background(), "VIX")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if mock.getCalls() != 2 {
		t.Errorf("expected 2 calls (transient then non-transient stops), got %d", mock.getCalls())
	}
	// Should return the non-transient error directly (not wrapped with "failed after N attempts")
	expected := "no VIX historical data available"
	if !containsSubstring(err.Error(), expected) {
		t.Errorf("expected error containing %q, got %q", expected, err.Error())
	}
}

func TestQA_RetryContextCancelledDuringDelay(t *testing.T) {
	// First attempt fails with transient error.
	// We cancel the context during the 2s retry delay.
	// Verify the function returns immediately with context cancelled error.
	attemptCount := 0
	mock := &mockProvider{
		getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
			attemptCount++
			return nil, errors.New("read tcp 10.42.2.45:43088->52.5.212.192:443: i/o timeout")
		},
	}
	s := newTestService(mock)

	ctx, cancel := context.WithCancel(context.Background())

	// Run getVIXValueWithRetry in a goroutine
	type result struct {
		value float64
		err   error
	}
	ch := make(chan result, 1)
	go func() {
		v, e := s.getVIXValueWithRetry(ctx, "VIX")
		ch <- result{v, e}
	}()

	// Wait for the first attempt to fail, then cancel during the delay.
	// The first attempt returns nearly instantly (mock is synchronous).
	// The 2s retry delay starts immediately after. Cancel after 100ms into the delay.
	time.Sleep(200 * time.Millisecond)
	cancel()

	// Should return quickly after cancellation
	select {
	case r := <-ch:
		if r.err == nil {
			t.Fatal("expected error after context cancellation, got nil")
		}
		if !containsSubstring(r.err.Error(), "context cancelled during VIX retry") {
			t.Errorf("expected context cancellation error, got: %v", r.err)
		}
		// Should have only made 1 attempt (cancelled during delay before attempt 2)
		if attemptCount != 1 {
			t.Errorf("expected 1 attempt (cancelled during delay), got %d", attemptCount)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for getVIXValueWithRetry to return after context cancel")
	}
}

// --- AC-3: Per-Indicator Timeout ---

func TestQA_PerAttemptTimeoutEnforced(t *testing.T) {
	// Verify per-attempt timeout is applied by checking the deadline on the context
	// received by the mock. Uses a 60s parent so per-attempt (20s) is the tighter bound.
	// Mock returns a transient error immediately so the test runs fast.
	var receivedDeadlines []time.Duration
	var mu sync.Mutex

	mock := &mockProvider{
		getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
			deadline, ok := ctx.Deadline()
			mu.Lock()
			if ok {
				receivedDeadlines = append(receivedDeadlines, time.Until(deadline))
			}
			mu.Unlock()
			return nil, errors.New("failed to connect to DXLink WebSocket: i/o timeout")
		},
	}
	s := newTestService(mock)

	parentCtx, parentCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer parentCancel()

	_, err := s.getVIXValueWithRetry(parentCtx, "VIX")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	mu.Lock()
	defer mu.Unlock()

	// All 3 attempts should have received a context with ~20s deadline
	if len(receivedDeadlines) != 3 {
		t.Fatalf("expected 3 attempts, got %d", len(receivedDeadlines))
	}
	for i, d := range receivedDeadlines {
		if d < 19*time.Second || d > 21*time.Second {
			t.Errorf("attempt %d: expected deadline ~20s, got %v", i+1, d)
		}
	}
}



func TestQA_ParentContextOverridesPerAttempt(t *testing.T) {
	// Parent context = 5s. Per-attempt = 20s. Parent should win (shorter deadline).
	// Mock blocks until context is done, then returns error.
	// Total elapsed should be ~5s (parent cancels everything).
	var receivedDeadlines []time.Duration
	var mu sync.Mutex

	mock := &mockProvider{
		getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
			deadline, ok := ctx.Deadline()
			mu.Lock()
			if ok {
				receivedDeadlines = append(receivedDeadlines, time.Until(deadline))
			}
			mu.Unlock()
			// Block until context is cancelled
			<-ctx.Done()
			return nil, fmt.Errorf("context deadline exceeded")
		},
	}
	s := newTestService(mock)

	parentCtx, parentCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer parentCancel()

	start := time.Now()
	_, err := s.getVIXValueWithRetry(parentCtx, "VIX")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should complete in ~5s (parent deadline), not 20s (per-attempt)
	if elapsed > 6*time.Second {
		t.Errorf("expected completion in ~5s (parent timeout), took %v", elapsed)
	}
	if elapsed < 4*time.Second {
		t.Errorf("expected at least ~4s (parent timeout is 5s), took %v", elapsed)
	}

	mu.Lock()
	defer mu.Unlock()

	// First attempt should receive a deadline of ~5s (parent), not 20s
	if len(receivedDeadlines) < 1 {
		t.Fatal("expected at least 1 attempt")
	}
	// The first attempt's deadline should be <= 5s (parent overrides per-attempt)
	if receivedDeadlines[0] > 6*time.Second {
		t.Errorf("attempt 1: expected deadline <= 5s (parent), got %v", receivedDeadlines[0])
	}
}

// --- AC-4: Cache Fallback Boundary ---

func TestQA_CacheFallbackBoundary_Exactly5Min(t *testing.T) {
	// Seed cache at exactly 5 minutes ago. The check is `time.Since(cached.Timestamp) < 5*time.Minute`.
	// At exactly 5 minutes, the condition is false → should fall through to stale.
	mock := &mockProvider{
		getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
			return nil, errors.New("read tcp: i/o timeout")
		},
	}
	s := newTestService(mock)

	configID := "auto_boundary"
	indicatorID := "ind_vix_boundary"
	// Set cache timestamp to exactly 5 minutes ago (plus a tiny margin to account for test execution)
	s.cache[cacheKey(configID, indicatorID)] = &cachedResult{
		Value:     20.0,
		Timestamp: time.Now().Add(-5*time.Minute - 1*time.Millisecond),
	}

	config := types.IndicatorConfig{
		ID:        indicatorID,
		Type:      types.IndicatorVIX,
		Symbol:    "VIX",
		Enabled:   true,
		Threshold: 30.0,
		Operator:  types.OperatorLessThan,
	}

	result := s.EvaluateIndicator(context.Background(), configID, config)

	if !result.Stale {
		t.Error("expected Stale=true (cache at exactly 5 min boundary should NOT be used as fallback)")
	}
}

func TestQA_CacheFallbackBoundary_JustUnder5Min(t *testing.T) {
	// Seed cache at 4m50s ago — just under the 5-min TTL (with margin for retry delays ~4s).
	// After retries exhaust (~4s elapsed), cache age will be ~4m54s, still < 5 min.
	mock := &mockProvider{
		getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
			return nil, errors.New("read tcp: i/o timeout")
		},
	}
	s := newTestService(mock)

	configID := "auto_boundary"
	indicatorID := "ind_vix_boundary"
	s.cache[cacheKey(configID, indicatorID)] = &cachedResult{
		Value:     20.0,
		Timestamp: time.Now().Add(-4*time.Minute - 50*time.Second),
	}

	config := types.IndicatorConfig{
		ID:        indicatorID,
		Type:      types.IndicatorVIX,
		Symbol:    "VIX",
		Enabled:   true,
		Threshold: 30.0,
		Operator:  types.OperatorLessThan,
	}

	result := s.EvaluateIndicator(context.Background(), configID, config)

	if result.Stale {
		t.Error("expected Stale=false (cache at 4m59s is within 5-min TTL)")
	}
	if result.Value != 20.0 {
		t.Errorf("expected cached value 20.0, got %f", result.Value)
	}
	if !result.Pass {
		t.Error("expected Pass=true (20.0 < 30.0)")
	}
}

func TestQA_CacheFallbackEvaluatesThreshold(t *testing.T) {
	// Cache value is 35.0, threshold is < 30.0.
	// Even though cache is used (within TTL), the condition should FAIL.
	mock := &mockProvider{
		getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
			return nil, errors.New("read tcp: i/o timeout")
		},
	}
	s := newTestService(mock)

	configID := "auto_threshold"
	indicatorID := "ind_vix_thresh"
	s.cache[cacheKey(configID, indicatorID)] = &cachedResult{
		Value:     35.0,
		Timestamp: time.Now().Add(-2 * time.Minute),
	}

	config := types.IndicatorConfig{
		ID:        indicatorID,
		Type:      types.IndicatorVIX,
		Symbol:    "VIX",
		Enabled:   true,
		Threshold: 30.0,
		Operator:  types.OperatorLessThan, // VIX < 30 to pass
	}

	result := s.EvaluateIndicator(context.Background(), configID, config)

	if result.Stale {
		t.Error("expected Stale=false (cache is within TTL)")
	}
	if result.Value != 35.0 {
		t.Errorf("expected cached value 35.0, got %f", result.Value)
	}
	if result.Pass {
		t.Error("expected Pass=false (35.0 is NOT < 30.0; cache fallback must evaluate threshold)")
	}
}

// --- Edge Cases ---

func TestQA_VIXRetryConstants(t *testing.T) {
	// Guard against accidental changes to critical constants.
	if vixMaxRetries != 2 {
		t.Errorf("vixMaxRetries = %d, want 2", vixMaxRetries)
	}
	if vixRetryDelay != 2*time.Second {
		t.Errorf("vixRetryDelay = %v, want 2s", vixRetryDelay)
	}
	if vixEvaluationTimeout != 20*time.Second {
		t.Errorf("vixEvaluationTimeout = %v, want 20s", vixEvaluationTimeout)
	}
	if vixCacheFallbackTTL != 5*time.Minute {
		t.Errorf("vixCacheFallbackTTL = %v, want 5m", vixCacheFallbackTTL)
	}
}

// --- Race Condition ---

func TestQA_ConcurrentVIXEvaluation(t *testing.T) {
	// Launch 10 goroutines all calling EvaluateIndicator for VIX simultaneously.
	// Mock sometimes fails transiently (odd calls) and sometimes succeeds (even calls).
	// Must pass with -race flag.
	var callMu sync.Mutex
	globalCallCount := 0

	mock := &mockProvider{
		getHistoricalBarsFn: func(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
			callMu.Lock()
			globalCallCount++
			n := globalCallCount
			callMu.Unlock()

			// Alternate: odd calls fail transiently, even calls succeed
			if n%2 == 1 {
				return nil, errors.New("read tcp: i/o timeout")
			}
			return []map[string]interface{}{{"close": 19.0}}, nil
		},
	}
	s := newTestService(mock)

	// Pre-seed a cache value so cache fallback can be exercised
	configID := "auto_concurrent"
	s.cache[cacheKey(configID, "ind_conc")] = &cachedResult{
		Value:     18.0,
		Timestamp: time.Now().Add(-1 * time.Minute),
	}

	config := types.IndicatorConfig{
		ID:        "ind_conc",
		Type:      types.IndicatorVIX,
		Symbol:    "VIX",
		Enabled:   true,
		Threshold: 30.0,
		Operator:  types.OperatorLessThan,
	}

	var wg sync.WaitGroup
	const goroutines = 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := s.EvaluateIndicator(context.Background(), configID, config)
			// Basic sanity: result should not be nil and should have a type set
			if result == nil {
				t.Error("result is nil")
				return
			}
			if result.Type != types.IndicatorVIX {
				t.Errorf("expected type VIX, got %s", result.Type)
			}
		}()
	}

	wg.Wait()
	// If we reach here without panic or race detector failure, the test passes.
}
