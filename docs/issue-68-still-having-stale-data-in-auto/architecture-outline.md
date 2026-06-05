# Architecture Outline: Fix Stale VIX Data in Automation

## Sections

1. **Overview & Problem Summary** — Brief recap of the root cause and the 4-part fix strategy.

2. **Component Architecture** — Mermaid diagram showing the data flow from automation engine → indicator service → provider manager → DXLink Candle Client, highlighting where each fix applies.

3. **Fix 1: DXLink Candle Client ALPN Configuration** — Exact change to `GetCandles()` in tastytrade.go to replace `websocket.DefaultDialer` with a custom dialer matching the streaming fix.

4. **Fix 2: Retry Logic in VIX Indicator Evaluation** — New helper function with retry-with-backoff for the VIX indicator path in indicator_service.go.

5. **Fix 3: Per-Indicator Timeout Budget** — How `EvaluateIndicator` creates a child context with indicator-specific timeouts, and how this interacts with the parent 60s budget.

6. **Fix 4: Cache Fallback on Transient Failures** — Enhancement to the existing `cachedResult` mechanism to allow using cached VIX values < 5 minutes old instead of marking stale.

7. **File Changes Summary** — Table of all files modified, what changes in each, and estimated line counts.

8. **Interaction with Existing Automation Tick Loop** — How the 4 fixes work together within the 30s tick cycle and 60s evaluation timeout.

9. **Testing Guidance** — How to verify the fix works without breaking existing tests.

10. **Trade-offs & Risks** — Explicit trade-off documentation.
