# Bug Fix: Stale Data in Auto View (Issue #66)

## Problem Statement

The Auto (automation) view frequently shows "Stale data" status with the error message:
```
Stale data: failed to get historical bars for QQQ: failed to get quote token: failed to get quote token: context deadline exceeded
```

This prevents trades from being triggered because the automation engine marks indicator data as stale when it cannot fetch historical bars within the evaluation timeout window.

## Root Cause Analysis

### Primary Cause: Missing Quote Token Caching

The `getQuoteToken()` function in the TastyTrade provider (`trade-backend-go/internal/providers/tastytrade/tastytrade.go`, ~line 4248) **always fetches a fresh quote token from the API**, even though:
- Quote tokens are valid for **24 hours**
- The provider struct stores a `quoteExpires` field (set to 24h after token creation)
- The function never checks `quoteExpires` before making the network request

This means every call to `GetHistoricalBars` makes an unnecessary HTTP round-trip to TastyTrade's API.

### Contributing Factor: Tight Evaluation Timeout

The automation engine's `handleWaitingState` function (`trade-backend-go/internal/automation/engine.go`, ~line 389) creates a context with a **30-second timeout** that is shared across ALL indicator evaluations in a single cycle. When multiple indicators require historical bars:

1. Each indicator calls `GetHistoricalBars` → `getQuoteToken` → HTTP request + DXLink WebSocket
2. The HTTP client has a 30-second timeout per request with 3 retries and exponential backoff
3. If any single call is slow, subsequent indicators fail with "context deadline exceeded"

### Why This Is New

The issue likely became apparent due to increased API latency from the broker side. Without token caching, even minor latency increases compound across multiple indicator evaluations within the shared 30-second window.

## Fix Requirements

### Fix 1: Implement Quote Token Caching (PRIMARY FIX)

**File:** `trade-backend-go/internal/providers/tastytrade/tastytrade.go`

**What to change:** Modify the `getQuoteToken()` function to check if the existing token is still valid before fetching a new one.

**Logic:**
```
func getQuoteToken(ctx) {
    // ADD: Check if existing token is still valid
    if p.quoteToken != "" && time.Now().Before(p.quoteExpires) {
        return p.quoteToken, nil
    }
    
    // EXISTING: Fetch new token from API (current logic)
    ...
}
```

**Requirements:**
- If `quoteToken` is non-empty AND current time is before `quoteExpires`, return the cached token immediately without making any network call
- Add a safety margin (e.g., 5 minutes before actual expiry) to avoid using a token that's about to expire
- If the cached token is empty or expired, proceed with the existing fetch logic (no behavior change)
- Thread safety: ensure the token check and update are protected by appropriate synchronization (mutex) if accessed from multiple goroutines
- Add a log message (slog) when reusing a cached token vs fetching a new one (debug level)

**Acceptance Criteria:**
- [ ] `getQuoteToken()` returns cached token when it's still valid (not expired)
- [ ] `getQuoteToken()` fetches a new token when cache is empty or expired
- [ ] Safety margin prevents using tokens within 5 minutes of expiry
- [ ] Thread-safe access to `quoteToken` and `quoteExpires` fields
- [ ] Debug-level logging indicates cache hit vs fresh fetch

### Fix 2: Increase Evaluation Timeout (SECONDARY FIX)

**File:** `trade-backend-go/internal/automation/engine.go`

**What to change:** Increase the context timeout in `handleWaitingState` from 30 seconds to 60 seconds.

**Rationale:** Even with token caching, the DXLink WebSocket connection and candle data retrieval still need time. With multiple indicators, 30 seconds is too tight. 60 seconds provides adequate headroom.

**Requirements:**
- Change the timeout from `30 * time.Second` to `60 * time.Second`
- Consider extracting this as a named constant (e.g., `evaluationTimeout`) for future configurability

**Acceptance Criteria:**
- [ ] Evaluation context timeout is 60 seconds
- [ ] Timeout value is defined as a named constant, not a magic number

### Fix 3: Per-Indicator Timeout (OPTIONAL IMPROVEMENT)

**File:** `trade-backend-go/internal/automation/indicators.go`

**What to change:** If feasible without major refactoring, give each indicator its own timeout context rather than sharing one across all indicators.

**Requirements:**
- Each indicator evaluation gets its own context with a per-indicator timeout (e.g., 30 seconds per indicator)
- A single slow indicator should not prevent other indicators from being evaluated
- This is an optional improvement — if it requires significant refactoring, skip it and document it as a future enhancement

**Acceptance Criteria:**
- [ ] Each indicator has its own timeout context (if implemented)
- [ ] A timeout in one indicator does not cascade to others (if implemented)
- [ ] OR: Documented as future enhancement if not implemented

## Scope Boundaries

### In Scope
- Quote token caching in the TastyTrade provider
- Evaluation timeout increase in the automation engine
- Unit tests for the quote token caching logic
- Running existing tests to ensure no regressions

### Out of Scope
- Changes to other providers (Alpaca, Tradier, Schwab) — they may have similar issues but are not reported
- Changes to the DXLink WebSocket reconnection logic
- Changes to the HTTP client retry/backoff configuration
- Frontend changes to the Auto view display
- Alerting or monitoring for stale data conditions

## Testing Requirements

- Unit test for `getQuoteToken()` verifying:
  - Returns cached token when valid
  - Fetches new token when cache is empty
  - Fetches new token when cache is expired
  - Respects safety margin near expiry
- Run all existing Go backend tests (`go test ./...`) to ensure no regressions
- Manual verification that the automation engine can complete evaluation cycles without "context deadline exceeded" errors (not automatable in CI)

## Related Files

| File | Purpose |
|------|---------|
| `trade-backend-go/internal/providers/tastytrade/tastytrade.go` | TastyTrade provider - `getQuoteToken()` function |
| `trade-backend-go/internal/automation/engine.go` | Automation engine - `handleWaitingState()` timeout |
| `trade-backend-go/internal/automation/indicators.go` | Indicator evaluation logic |
| `trade-backend-go/internal/utils/http_client.go` | HTTP client with retry/backoff |
