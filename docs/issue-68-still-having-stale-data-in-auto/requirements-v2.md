# Requirements: Fix Streaming Regression from Issue #68 PR (v2)

## Issue Reference
- **GitHub Issue:** https://github.com/schardosin/juicytrade/issues/68
- **Context:** PR #69 (and commit `2ffba97`) introduced a regression that broke live streaming quotes

## Problem Statement

The fix for stale VIX data in the Auto view (PR #69) inadvertently broke the live streaming quotes infrastructure. The ALPN restriction (`NextProtos: ["http/1.1"]`) was correctly applied to the DXLink Candle Client (one-shot connections for historical data), but was also incorrectly applied to the persistent streaming connection (commit `2ffba97`). Additionally, the `dxlinkStreamingSetup` timeout was increased from 5s to 15s, which masks connection issues.

**Symptoms reported by the customer:**
1. Live streaming quotes are broken — prices show once on initial load but never update (they used to be fluid/live)
2. `/api/chart/historical/QQQ` endpoint times out at 15s
3. `Quotes batch completed complete=0` — streaming subscription yields no data
4. The VIX indicator fix itself works correctly (VIX values are being fetched)

**Root Cause:** The DXLink streaming server handles persistent WebSocket connections differently than one-shot connections. The persistent streaming connection worked fine with Go's default TLS negotiation (HTTP/2 ALPN). Only the short-lived candle client connections needed the HTTP/1.1 ALPN restriction. By forcing HTTP/1.1 on the streaming connection, the DXLink server connects but doesn't properly deliver FEED_DATA events.

## Proposed Fix

### Fix 1: Remove ALPN Restriction from Streaming Connection

**File:** `trade-backend-go/internal/providers/tastytrade/tastytrade.go`
**Location:** `ConnectStreaming` function (around line 1793)

**Current (broken) code:**
```go
dialer := &websocket.Dialer{
    HandshakeTimeout: 15 * time.Second,
    ReadBufferSize:   4096,
    WriteBufferSize:  4096,
    TLSClientConfig: &tls.Config{
        NextProtos: []string{"http/1.1"},
    },
}
```

**Fixed code (revert to working state):**
```go
dialer := &websocket.Dialer{
    HandshakeTimeout: 5 * time.Second,
    ReadBufferSize:   4096,
    WriteBufferSize:  4096,
}
```

**Rationale:** The streaming connection was working perfectly with Go's default TLS negotiation before commit `2ffba97`. The ALPN restriction is only needed for short-lived candle client connections.

### Fix 2: Revert `dxlinkStreamingSetup` Timeout to 5s

**File:** `trade-backend-go/internal/providers/tastytrade/tastytrade.go`
**Location:** `dxlinkStreamingSetup` function (around line 2226)

**Current (broken) code:**
```go
timeout := 15 * time.Second
```

**Fixed code:**
```go
timeout := 5 * time.Second
```

**Rationale:** The 15s timeout masks connection issues. The streaming setup (SETUP → AUTH → CHANNEL_REQUEST → FEED_SUBSCRIPTION) should complete in well under 5s for a healthy connection. A 15s timeout means failed connections take 15s to detect, degrading the user experience.

### Fix 3: Keep DXLink Candle Client ALPN Fix Intact

The DXLink Candle Client code (added by PR #69) with the ALPN restriction, retry logic, and VIX cache fallback should remain unchanged — it correctly fixes the VIX stale data issue.

## Acceptance Criteria

1. **AC1:** The streaming connection dialer in `ConnectStreaming` does NOT have `TLSClientConfig` with `NextProtos` set — it uses Go's default TLS negotiation
2. **AC2:** The streaming connection `HandshakeTimeout` is set to `5 * time.Second`
3. **AC3:** The `dxlinkStreamingSetup` timeout is set to `5 * time.Second`
4. **AC4:** The DXLink Candle Client (`GetCandles`) retains the ALPN fix with `NextProtos: []string{"http/1.1"}`
5. **AC5:** The VIX retry logic and cache fallback added by PR #69 remain intact
6. **AC6:** All existing tests pass (no regressions)
7. **AC7:** The `crypto/tls` import can be removed from the file IF it is no longer used elsewhere (check for other usages first)

## Scope Boundaries

### In Scope
- Reverting the streaming connection ALPN fix (from commit `2ffba97`)
- Reverting the streaming connection HandshakeTimeout to 5s
- Reverting the `dxlinkStreamingSetup` timeout to 5s

### Out of Scope
- Any changes to the DXLink Candle Client code
- Any changes to the VIX retry logic or cache fallback
- Any changes to the automation engine
- Frontend changes
- Changes to other providers

## Risk Assessment

- **Low risk:** These are targeted reverts of specific values in the streaming connection code
- **The streaming was confirmed working** with these exact values before commit `2ffba97`
- **No functional logic changes** — only reverting configuration values on the streaming dialer
- **The candle client fix remains** — VIX stale data issue stays fixed

## Testing Strategy

- Run existing Go backend tests to ensure no regressions
- Verify the `crypto/tls` import is still needed (for the candle client) or can be removed
- Customer will verify live streaming quotes resume updating in real-time after deployment
