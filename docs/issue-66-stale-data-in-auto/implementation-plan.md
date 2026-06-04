# Implementation Plan: Issue #66 - Stale Data in Auto View

## Step 1: Cache Quote Token in TastyTrade Provider (Fix 1 - PRIMARY)

**File:** `trade-backend-go/internal/providers/tastytrade/tastytrade.go`

**Changes:**
1. Added `quoteTokenMu sync.RWMutex` field to the `TastyTradeProvider` struct
2. Added `quoteTokenSafetyMargin` constant (5 minutes)
3. Modified `getQuoteToken()` to:
   - Acquire read lock, check if `quoteToken != ""` and `time.Now().Before(quoteExpires.Add(-5*time.Minute))`
   - If valid: log debug "cache hit", return nil (token is already set)
   - If not valid: acquire write lock, double-check (prevent thundering herd), then fetch new token
4. Added debug-level slog logging for cache hit vs fresh fetch

**Test file:** `trade-backend-go/internal/providers/tastytrade/quote_token_cache_test.go`
- `TestGetQuoteToken_CacheHit` - verifies valid cached token is reused
- `TestGetQuoteToken_EmptyToken` - verifies empty token triggers fetch
- `TestGetQuoteToken_ExpiredToken` - verifies expired token triggers fetch
- `TestGetQuoteToken_SafetyMargin` - verifies token within 5-min margin triggers refresh
- `TestGetQuoteToken_SafetyMargin_JustOutside` - verifies token beyond margin is cached
- `TestGetQuoteToken_ConcurrentAccess` - verifies thread safety with double-check locking

## Step 2: Increase Evaluation Timeout (Fix 2 - SECONDARY)

**File:** `trade-backend-go/internal/automation/engine.go`

**Changes:**
1. Added named constant `evaluationTimeout = 60 * time.Second` at package level
2. Replaced `context.WithTimeout(context.Background(), 30*time.Second)` with `context.WithTimeout(context.Background(), evaluationTimeout)`

## Step 3: Per-Indicator Timeout (Fix 3 - DOCUMENTED AS FUTURE ENHANCEMENT)

**File:** `trade-backend-go/internal/automation/indicators/service.go`

**Assessment:** The indicator evaluation iterates indicators sequentially using the shared context. Adding per-indicator timeouts is a small code change but changes timeout semantics (individual vs aggregate). Documented as a comment/future enhancement.

**Changes:**
- Added comment above `EvaluateAllIndicators` documenting the future enhancement opportunity

## Test Results

All tests pass:
- ✅ 6 new quote token cache tests (PASS)
- ✅ Full `go test ./...` suite (all packages PASS, no regressions)
