# Implementation Plan: Fix Stale VIX Data (Issue #68)

**Issue:** [#68 - Still having stale data in Auto](https://github.com/schardosin/juicytrade/issues/68)  
**Date:** 2025-06-05  
**Branch:** `fleet/issue-68-still-having-stale-data-in-auto`

---

## Step 1: Fix the DXLink Candle Client ALPN Issue

**File:** `trade-backend-go/internal/providers/tastytrade/tastytrade.go` (~line 4635)

**What to do:**

Replace `websocket.DefaultDialer.DialContext(ctx, c.dxlinkURL, nil)` in the `GetCandles()` method with a custom dialer that forces HTTP/1.1 ALPN negotiation and sets a 15-second handshake timeout. This matches the fix already applied to the streaming connection at line 1794-1801 (commit `2ffba97`).

**Exact change:**

Replace:
```go
// Connect to DXLink WebSocket
conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.dxlinkURL, nil)
if err != nil {
    return nil, fmt.Errorf("failed to connect to DXLink WebSocket: %w", err)
}
```

With:
```go
// Configure WebSocket dialer with HTTP/1.1 ALPN (matches streaming fix from 2ffba97).
// CRITICAL: Without this, Go's default TLS negotiates HTTP/2 via ALPN, causing
// DXLink's WebSocket server to silently drop the connection.
dialer := &websocket.Dialer{
    HandshakeTimeout: 15 * time.Second,
    TLSClientConfig: &tls.Config{
        NextProtos: []string{"http/1.1"},
    },
}

// Connect to DXLink WebSocket
conn, _, err := dialer.DialContext(ctx, c.dxlinkURL, nil)
if err != nil {
    return nil, fmt.Errorf("failed to connect to DXLink WebSocket: %w", err)
}
```

**Import requirements:** None. The file already imports `"crypto/tls"` (line 5) and `"time"` (line 14).

**Tests to run:** `cd trade-backend-go && go build ./...` (compilation check — no unit tests exist for this code path since it requires a live DXLink connection).

---

## Step 2: Add Retry Logic, Per-Indicator Timeout, and Cache Fallback

**File:** `trade-backend-go/internal/automation/indicators/service.go`

**What to do:**

Add constants, a transient error helper, a retry wrapper for VIX evaluation, a `formatPassFail` helper, and modify the `EvaluateIndicator()` function's VIX case and error handling block.

### 2a. Add `"strings"` to the import block

The current imports (lines 3-12) do not include `"strings"`. Add it to the standard library imports:

```go
import (
    "context"
    "fmt"
    "log/slog"
    "strings"
    "sync"
    "time"

    "trade-backend-go/internal/automation/types"
    "trade-backend-go/internal/providers"
)
```

### 2b. Add constants after `quoteCacheTTL` (after line 34)

```go
// vixMaxRetries is the maximum number of retry attempts for VIX evaluation.
const vixMaxRetries = 2 // 3 total attempts (1 initial + 2 retries)

// vixRetryDelay is the delay between VIX retry attempts.
const vixRetryDelay = 2 * time.Second

// vixEvaluationTimeout is the per-attempt timeout for VIX indicator evaluation.
const vixEvaluationTimeout = 20 * time.Second

// vixCacheFallbackTTL is the maximum age of a cached VIX value that can be used
// as a fallback when evaluation fails.
const vixCacheFallbackTTL = 5 * time.Minute
```

### 2c. Add `isTransientError` helper function (after the constants)

```go
// isTransientError checks if an error is likely transient (connection/timeout related).
func isTransientError(err error) bool {
    if err == nil {
        return false
    }
    errStr := err.Error()
    return strings.Contains(errStr, "i/o timeout") ||
        strings.Contains(errStr, "connection refused") ||
        strings.Contains(errStr, "connection reset") ||
        strings.Contains(errStr, "context deadline exceeded") ||
        strings.Contains(errStr, "failed to connect") ||
        strings.Contains(errStr, "EOF")
}
```

### 2d. Add `getVIXValueWithRetry` method (after `getQuoteWithCache`)

```go
// getVIXValueWithRetry wraps GetVIXValue with retry logic for transient failures.
// Each attempt gets its own 20s timeout context. On transient failures, retries up to
// vixMaxRetries times with vixRetryDelay between attempts.
func (s *Service) getVIXValueWithRetry(ctx context.Context, symbol string) (float64, error) {
    var lastErr error

    for attempt := 0; attempt <= vixMaxRetries; attempt++ {
        if attempt > 0 {
            slog.Warn("🔄 Retrying VIX evaluation",
                "attempt", attempt+1,
                "maxAttempts", vixMaxRetries+1,
                "symbol", symbol,
                "previousError", lastErr)

            // Wait before retry, but respect context cancellation
            select {
            case <-ctx.Done():
                return 0, fmt.Errorf("context cancelled during VIX retry: %w", ctx.Err())
            case <-time.After(vixRetryDelay):
            }
        }

        // Create per-attempt timeout context
        attemptCtx, attemptCancel := context.WithTimeout(ctx, vixEvaluationTimeout)
        value, err := s.GetVIXValue(attemptCtx, symbol)
        attemptCancel()

        if err == nil {
            if attempt > 0 {
                slog.Info("✅ VIX evaluation succeeded on retry",
                    "attempt", attempt+1,
                    "symbol", symbol,
                    "value", value)
            }
            return value, nil
        }

        lastErr = err

        // Only retry on transient errors
        if !isTransientError(err) {
            return 0, err
        }

        slog.Warn("⚠️ VIX evaluation failed (transient)",
            "attempt", attempt+1,
            "maxAttempts", vixMaxRetries+1,
            "symbol", symbol,
            "error", err)
    }

    return 0, fmt.Errorf("VIX evaluation failed after %d attempts: %w", vixMaxRetries+1, lastErr)
}
```

### 2e. Add `formatPassFail` helper (after `getVIXValueWithRetry`)

```go
// formatPassFail returns a pass/fail string for display.
func (s *Service) formatPassFail(pass bool) string {
    if pass {
        return "(PASS)"
    }
    return "(FAIL)"
}
```

### 2f. Modify `EvaluateIndicator()` — VIX case (line 240)

Replace:
```go
result.Value, err = s.GetVIXValue(ctx, vixSymbol)
```

With:
```go
result.Value, err = s.getVIXValueWithRetry(ctx, vixSymbol)
```

### 2g. Modify `EvaluateIndicator()` — error handling block (lines 476-506)

Insert the VIX cache fallback check **before** the existing stale handling. Replace the error block starting at line 476:

```go
if err != nil {
    // For VIX indicators: try cache fallback before marking as stale
    if config.Type == types.IndicatorVIX && configID != "" && config.ID != "" {
        if cached := s.getCachedResult(configID, config.ID); cached != nil {
            if time.Since(cached.Timestamp) < vixCacheFallbackTTL {
                // Cache is fresh enough — use it instead of marking stale
                slog.Warn("⚠️ VIX evaluation failed, using cached value (within TTL)",
                    "indicatorID", config.ID,
                    "cachedValue", cached.Value,
                    "cachedAt", cached.Timestamp,
                    "cacheAge", time.Since(cached.Timestamp).Round(time.Second),
                    "error", err)

                result.Value = cached.Value
                result.Error = "" // Clear error — this is a valid fallback
                result.Stale = false
                result.Pass = result.Evaluate()
                result.Details = fmt.Sprintf("VIX %.2f (cached %s ago) %s",
                    cached.Value,
                    time.Since(cached.Timestamp).Round(time.Second),
                    s.formatPassFail(result.Pass))

                // Still cache this result to extend TTL for subsequent ticks
                s.setCachedResult(configID, config.ID, cached.Value)
                return result
            }
        }
    }

    // Existing stale handling for all other cases (unchanged)
    result.Error = err.Error()
    result.Pass = false
    result.Stale = true

    // Try to use last known good value for display (only if we have a configID for caching)
    if configID != "" && config.ID != "" {
        if cached := s.getCachedResult(configID, config.ID); cached != nil {
            result.Value = cached.Value
            result.LastGoodValue = &cached.Value
            result.Details = fmt.Sprintf("STALE: Last good value from %s - %s",
                cached.Timestamp.Format("15:04:05"), err.Error())
            slog.Warn("Indicator evaluation failed, using cached value",
                "type", config.Type,
                "indicatorID", config.ID,
                "cachedValue", cached.Value,
                "cachedAt", cached.Timestamp,
                "error", err)
        } else {
            result.Details = fmt.Sprintf("STALE: No cached value available - %s", err.Error())
            slog.Error("Indicator evaluation failed, no cached value available",
                "type", config.Type,
                "indicatorID", config.ID,
                "error", err)
        }
    } else {
        // Preview/test mode - no caching
        slog.Error("Indicator evaluation failed (preview mode)", "type", config.Type, "error", err)
    }
    return result
}
```

**Tests to run:** `cd trade-backend-go && go build ./...` (compilation check).

---

## Step 3: Write Unit Tests for Retry Logic and Cache Fallback

**File:** `trade-backend-go/internal/automation/indicators/service_test.go` (append to existing file)

**What to test:**

### 3a. `isTransientError` tests

| Test Case | Input Error | Expected |
|-----------|-------------|----------|
| nil error | `nil` | `false` |
| i/o timeout | `errors.New("read tcp ...: i/o timeout")` | `true` |
| connection refused | `errors.New("dial tcp ...: connection refused")` | `true` |
| connection reset | `errors.New("read tcp ...: connection reset by peer")` | `true` |
| context deadline | `errors.New("context deadline exceeded")` | `true` |
| failed to connect | `errors.New("failed to connect to DXLink WebSocket: ...")` | `true` |
| EOF | `errors.New("unexpected EOF")` | `true` |
| non-transient | `errors.New("VIX close price not available in historical data")` | `false` |
| non-transient 2 | `errors.New("no VIX historical data available")` | `false` |

### 3b. `formatPassFail` tests

| Test Case | Input | Expected |
|-----------|-------|----------|
| pass=true | `true` | `"(PASS)"` |
| pass=false | `false` | `"(FAIL)"` |

### 3c. VIX Cache Fallback Behavior (via `EvaluateIndicator`)

These tests require a mock `ProviderManager`. Create a minimal mock that implements `GetHistoricalBars` and returns either data or an error:

| Test Case | Setup | Expected |
|-----------|-------|----------|
| Cache fallback within TTL | Pre-populate cache with VIX=18.5 at `time.Now().Add(-2*time.Minute)`, then make `GetHistoricalBars` return a transient error | `result.Stale=false`, `result.Value=18.5`, `result.Pass` evaluated normally |
| Cache fallback expired TTL | Pre-populate cache with VIX=18.5 at `time.Now().Add(-6*time.Minute)`, then make `GetHistoricalBars` return a transient error | `result.Stale=true` |
| No cache fallback (non-VIX) | Same as first case but with `IndicatorGap` type | `result.Stale=true` (cache fallback is VIX-only) |
| Retry succeeds on 2nd attempt | Mock returns error on first call, success on second | `result.Stale=false`, correct value returned |

### 3d. Implementation approach for mock

Since the project uses no mocking framework, create a `mockProviderManager` struct in the test file that wraps a `*providers.ProviderManager` or satisfies the interface needed. The simplest approach: inject a function field into Service for testing, OR use the existing pattern from `qa_service_test.go`.

**Check `qa_service_test.go` for existing mock patterns** before writing new mocks.

**Tests to run:** `cd trade-backend-go && go test ./internal/automation/indicators/ -v -run "TestIsTransientError|TestFormatPassFail|TestVIXCacheFallback|TestVIXRetry"`

---

## Step 4: Run All Existing Tests, Commit, and Push

**What to do:**

1. Run full test suite to ensure no regressions:
   ```sh
   cd trade-backend-go && go test ./...
   ```

2. Verify the build compiles cleanly:
   ```sh
   cd trade-backend-go && go build ./...
   ```

3. Review changes:
   ```sh
   git diff --stat
   git diff
   ```

4. Commit with conventional commit format:
   ```sh
   git add trade-backend-go/internal/providers/tastytrade/tastytrade.go
   git add trade-backend-go/internal/automation/indicators/service.go
   git add trade-backend-go/internal/automation/indicators/service_test.go
   git commit -m "fix(tastytrade): resolve stale VIX data from DXLink candle client ALPN issue

   - Apply HTTP/1.1 ALPN fix to DXLink Candle Client (matches streaming fix 2ffba97)
   - Add retry logic (3 attempts) with per-attempt 20s timeout for VIX evaluation
   - Add cache fallback: use cached VIX value (<5 min old) on transient failures
   - Add isTransientError helper and formatPassFail helper
   - Add unit tests for retry logic and cache fallback behavior

   Fixes #68"
   ```

5. Push to branch:
   ```sh
   git push origin fleet/issue-68-still-having-stale-data-in-auto
   ```

**Acceptance criteria verification:**
- AC-1: Custom dialer with `TLSClientConfig.NextProtos = ["http/1.1"]` and `HandshakeTimeout = 15s` in `GetCandles()`
- AC-2: `getVIXValueWithRetry` retries up to 2 times on transient failures
- AC-3: Each retry attempt gets its own `context.WithTimeout(ctx, 20*time.Second)`
- AC-4: VIX cache fallback with `vixCacheFallbackTTL = 5 * time.Minute` check before marking stale
- AC-5: All retry attempts logged with `slog.Warn` including attempt number and error
- AC-6: Existing tests pass (`go test ./...`)
- AC-7: No behavior change on healthy connections (retry/cache logic only triggers on error)

---

## Summary

| Step | Files Modified | Lines Changed (est.) | Risk |
|------|---------------|---------------------|------|
| 1 | `tastytrade.go` | ~8 lines | Low — proven fix from streaming |
| 2 | `indicators/service.go` | ~85 lines added, ~5 modified | Medium — new logic in critical path |
| 3 | `indicators/service_test.go` | ~100-150 lines added | Low — test-only |
| 4 | N/A (verification) | 0 | N/A |

**Total estimated effort:** ~250 lines of code across 2 production files and 1 test file.
