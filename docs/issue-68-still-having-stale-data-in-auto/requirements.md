# Requirements: Fix Stale VIX Data in Automation (DXLink Candle Client)

**Issue:** [#68 - Still having stale data in Auto](https://github.com/schardosin/juicytrade/issues/68)  
**Status:** Approved  
**Date:** 2025-06-05

## Problem Statement

The Auto (automation) view frequently encounters stale data when evaluating the VIX indicator, which blocks trade entry during the 5-minute entry window. The root cause is that the DXLink Candle Client (used by `GetHistoricalBars()` to fetch VIX data) was not updated with the same HTTP/1.1 ALPN fix and timeout configuration that was applied to the DXLink streaming connection in commit `2ffba97`.

### Root Cause

In `trade-backend-go/internal/providers/tastytrade/tastytrade.go` at line ~4635, the DXLink Candle Client uses `websocket.DefaultDialer.DialContext()` which:
1. Has no `TLSClientConfig` forcing HTTP/1.1 ALPN — causing DXLink to negotiate HTTP/2 and silently drop the connection
2. Has no `HandshakeTimeout` — connection attempts hang for the full 60s context timeout
3. Has no retry logic — a single failure immediately marks VIX as stale
4. Has no per-indicator timeout budget — one slow indicator can consume the entire evaluation budget

### Error Signature
```
failed to get VIX historical data: failed to get DXLink candles: failed to connect to DXLink WebSocket: read tcp 10.42.2.45:43088->52.5.212.192:443: i/o timeout
```

## Functional Requirements

### FR-1: Apply HTTP/1.1 ALPN Fix to DXLink Candle Client

Replace `websocket.DefaultDialer` in the DXLink Candle Client's `Connect()` method with a properly configured dialer that forces HTTP/1.1 ALPN negotiation, matching the fix already applied to the streaming connection.

**Implementation details:**
- Configure `TLSClientConfig` with `NextProtos: []string{"http/1.1"}` on the WebSocket dialer
- Set `HandshakeTimeout` to 15 seconds (prevents hanging for the full context timeout)
- This ensures the candle client WebSocket connection doesn't fall victim to HTTP/2 negotiation

### FR-2: Add Retry Logic to VIX Indicator Evaluation

Add retry logic specifically for the VIX indicator evaluation (or more generally, for any indicator that fetches data via `GetHistoricalBars`).

**Implementation details:**
- Retry up to 2 times (3 total attempts) on transient failures (connection timeout, i/o timeout)
- Use a short delay between retries (2-3 seconds)
- Log each retry attempt for observability
- If all retries fail, then mark as stale (existing behavior)

### FR-3: Add Per-Indicator Timeout Budget

Each indicator evaluation should have its own timeout rather than sharing a single 60s context.

**Implementation details:**
- VIX indicator: 20-second timeout per attempt (allows time for connection + data fetch)
- Other indicators should have reasonable individual timeouts
- Total evaluation should still be bounded by the existing 60s overall timeout

### FR-4: Leverage Cached VIX Value on Transient Failures

When the VIX indicator fails but a cached value exists from a previous successful evaluation, use the cached value instead of immediately marking as stale — provided the cached value is not excessively old.

**Implementation details:**
- If a cached VIX value exists and is less than 5 minutes old, use it on failure (with a warning log)
- If no cached value exists or it's older than 5 minutes, mark as stale (existing behavior)
- This prevents a single transient DXLink failure from blocking an entire entry window

## Acceptance Criteria

1. **AC-1:** The DXLink Candle Client WebSocket connection uses a custom dialer with `TLSClientConfig.NextProtos = ["http/1.1"]` and `HandshakeTimeout = 15s`
2. **AC-2:** VIX indicator evaluation retries up to 2 times on transient connection failures before marking as stale
3. **AC-3:** Each indicator evaluation has its own timeout context (20s for VIX) rather than relying solely on the parent 60s context
4. **AC-4:** When VIX evaluation fails but a cached value less than 5 minutes old exists, the cached value is used and a warning is logged
5. **AC-5:** All retry attempts are logged with attempt number and error details for observability
6. **AC-6:** Existing unit tests pass without modification
7. **AC-7:** The fix does not change behavior when DXLink connections are healthy (no performance regression)

## Scope Boundaries

### In Scope
- DXLink Candle Client connection configuration (`tastytrade.go`)
- VIX indicator evaluation retry logic (`indicator_service.go`)
- Per-indicator timeout budgets
- Cache fallback behavior on transient failures

### Out of Scope
- Changes to the DXLink streaming connection (already fixed in `2ffba97`)
- Changes to the automation engine's tick interval or entry time window
- Changes to other providers (alpaca, tradier, schwab)
- Frontend changes
- Changes to the indicator configuration schema

## Technical Context

### Key Files
- `trade-backend-go/internal/providers/tastytrade/tastytrade.go` — DXLink Candle Client `Connect()` method (~line 4635)
- `trade-backend-go/internal/automation/indicator_service.go` — `evaluateVIXIndicator()` and indicator evaluation logic
- `trade-backend-go/internal/automation/engine.go` — Automation tick loop and stale data handling

### Related Fix
- Commit `2ffba97` — "force HTTP/1.1 ALPN for DXLink WebSocket connection" (applied to streaming only)

## Non-Functional Requirements

- **Observability:** All retry attempts and cache fallback usage must be logged with structured logging (slog)
- **Performance:** No additional latency when DXLink connections are healthy. Retries only trigger on failure.
- **Reliability:** The fix should significantly reduce the frequency of stale data blocking trade entry during the 5-minute window
