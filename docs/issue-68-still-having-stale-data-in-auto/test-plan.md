# Test Plan: Fix Stale VIX Data in Automation (Issue #68)

**Date:** 2025-06-05  
**Source Files:**
- `trade-backend-go/internal/automation/indicators/service.go` (implementation)
- `trade-backend-go/internal/automation/indicators/vix_retry_test.go` (existing tests)
- `trade-backend-go/internal/providers/tastytrade/tastytrade.go` (ALPN fix, line ~4631-4665)
- `trade-backend-go/internal/providers/testing_helpers.go` (test infrastructure)

---

## 1. Test Compilation & Execution Status

**Result: ALL PASS**

```
$ go test ./internal/automation/indicators/ -run "TestIsTransientError|TestFormatPassFail|TestGetVIXValueWithRetry|TestVIXCacheFallback" -v -count=1
PASS
ok  trade-backend-go/internal/automation/indicators  18.017s
```

- `go build ./...` compiles without errors.
- All 4 existing test functions pass (10 subtests for `TestIsTransientError`, 2 for `TestFormatPassFail`, 4 for `TestGetVIXValueWithRetry`, 3 for `TestVIXCacheFallback`).
- Test runtime is ~18s due to the 2s `vixRetryDelay` between retry attempts (real `time.After` in production code).

---

## 2. Coverage by Acceptance Criterion

### AC-1: DXLink Candle Client ALPN Fix

**Requirement:** The DXLink Candle Client WebSocket connection uses a custom dialer with `TLSClientConfig.NextProtos = ["http/1.1"]` and `HandshakeTimeout = 15s`.

**Existing test coverage:** NONE  
The ALPN fix is in `tastytrade.go:4637-4642` and is purely configuration at the transport layer. No unit tests exercise the actual TLS handshake (would require a live DXLink endpoint or a local TLS WebSocket server).

**Additional QA tests needed:**

| # | Test | Type | Priority |
|---|------|------|----------|
| 1.1 | `qa_TestDXLinkDialerConfig` — Verify `GetCandles()` uses a dialer with `NextProtos: ["http/1.1"]` and `HandshakeTimeout: 15s`. Approach: refactor dialer construction into a `newDXLinkDialer()` function and unit-test its output fields. | Unit | Medium |
| 1.2 | `qa_TestDXLinkALPN_Integration` — Stand up a local TLS WebSocket server that rejects HTTP/2 ALPN. Confirm `GetCandles()` connects successfully (HTTP/1.1 forced). Confirm that using `websocket.DefaultDialer` would fail. | Integration | Low |
| 1.3 | Code inspection assertion: ensure the dialer is NOT `websocket.DefaultDialer`. A static analysis test or a grep-based CI check is sufficient. | Static | Low |

---

### AC-2: Retry Logic (up to 2 retries on transient failures)

**Requirement:** VIX indicator evaluation retries up to 2 times on transient connection failures before marking as stale.

**Existing test coverage:** GOOD

| Test | Covers |
|------|--------|
| `TestGetVIXValueWithRetry/succeeds_on_first_attempt` | Happy path, no retry triggered |
| `TestGetVIXValueWithRetry/fails_then_succeeds_on_retry` | 1st attempt fails (i/o timeout), 2nd succeeds |
| `TestGetVIXValueWithRetry/non-transient_error_does_not_retry` | Non-transient error exits immediately, 1 call only |
| `TestGetVIXValueWithRetry/all_attempts_fail_with_transient_error` | All 3 attempts fail, error wraps correctly |
| `TestIsTransientError/*` | 10 cases covering classification of transient vs non-transient errors |

**Additional QA tests needed:**

| # | Test | Type | Priority |
|---|------|------|----------|
| 2.1 | `qa_TestRetryExhaustsOnSecondAttempt` — Fail attempts 1 and 2, succeed on attempt 3 (the maximum). Confirm exactly 3 calls and correct value returned. | Unit | High |
| 2.2 | `qa_TestRetryWithMixedErrors` — Attempt 1 returns `i/o timeout`, attempt 2 returns `connection reset`, attempt 3 succeeds. Verify retry continues across different transient error types. | Unit | Medium |
| 2.3 | `qa_TestRetryTransientThenNonTransient` — Attempt 1 returns transient error, attempt 2 returns non-transient error (e.g., "no VIX historical data available"). Verify it stops after attempt 2 without further retries. | Unit | High |
| 2.4 | `qa_TestRetryContextCancelledDuringDelay` — Cancel the parent context during the 2s retry delay. Verify the function returns immediately with `context cancelled during VIX retry` error and does not make another attempt. | Unit | High |
| 2.5 | `qa_TestRetryContextCancelledDuringAttempt` — Cancel the parent context while `GetVIXValue` is in-flight (simulate slow provider). Verify the function returns the context error. | Unit | Medium |
| 2.6 | `qa_TestRetryDelayTiming` — Measure that ~2s elapses between retry attempts (not more, not significantly less). Confirm the delay constant is respected. | Unit | Low |

---

### AC-3: Per-Indicator Timeout (20s for VIX)

**Requirement:** Each indicator evaluation has its own timeout context (20s for VIX) rather than relying solely on the parent 60s context.

**Existing test coverage:** PARTIAL  
The per-attempt timeout is created at `service.go:706` (`context.WithTimeout(ctx, vixEvaluationTimeout)`). The existing tests use `context.Background()` which has no deadline, so the 20s timeout IS active in tests but never exercised to its limit (mock returns instantly or with the 2s retry delay).

**Additional QA tests needed:**

| # | Test | Type | Priority |
|---|------|------|----------|
| 3.1 | `qa_TestPerAttemptTimeoutEnforced` — Mock provider sleeps for 25s. Verify attempt fails with `context deadline exceeded` after ~20s (not 60s). Confirm retry is attempted. Use a tight parent context (e.g., 45s) to prove per-attempt timeout fires first. | Unit | High |
| 3.2 | `qa_TestParentContextOverridesPerAttempt` — Set a parent context with 5s timeout. Verify the attempt fails at 5s (parent overrides the 20s per-attempt timeout since `context.WithTimeout` takes the shorter deadline). | Unit | High |
| 3.3 | `qa_TestAllRetriesWithinParentBudget` — Set parent context to 60s. Mock provider sleeps 18s per attempt. Verify: attempt 1 succeeds at 18s + 2s delay + attempt 2 at 18s = ~38s total. Confirm all complete within 60s. (Timing-based; use tolerance.) | Unit | Medium |
| 3.4 | `qa_TestPerAttemptTimeoutDoesNotLeakContext` — After `attemptCancel()` is called, verify the attempt context is properly cancelled (check `attemptCtx.Err()` in the mock). Ensures no goroutine leaks. | Unit | Medium |

---

### AC-4: Cache Fallback (cached VIX value < 5 min used on failure)

**Requirement:** When VIX evaluation fails but a cached value less than 5 minutes old exists, the cached value is used and a warning is logged.

**Existing test coverage:** GOOD

| Test | Covers |
|------|--------|
| `TestVIXCacheFallback/cache_within_TTL_used_as_fallback` | 2-min-old cache used, `Stale=false`, `Pass` evaluates correctly |
| `TestVIXCacheFallback/cache_older_than_TTL_falls_through_to_stale` | 10-min-old cache rejected, `Stale=true` |
| `TestVIXCacheFallback/no_cache_exists_falls_through_to_stale` | No cache, `Stale=true`, error message set |

**Additional QA tests needed:**

| # | Test | Type | Priority |
|---|------|------|----------|
| 4.1 | `qa_TestCacheFallbackBoundary_Exactly5Min` — Seed cache with timestamp exactly 5 minutes ago. Verify `time.Since(cached.Timestamp) < vixCacheFallbackTTL` is `false` (boundary: >= 5 min is stale). | Unit | High |
| 4.2 | `qa_TestCacheFallbackBoundary_JustUnder5Min` — Seed cache at 4m59s ago. Verify it IS used as fallback. | Unit | High |
| 4.3 | `qa_TestCacheFallbackEvaluatesThreshold` — Cache value is 35.0, threshold is `< 30.0`. Verify `Pass=false` (cache fallback still evaluates the condition, doesn't blindly pass). | Unit | High |
| 4.4 | `qa_TestCacheFallbackNonVIXIndicator` — Simulate a Gap indicator failure with a fresh cache. Verify it does NOT use the VIX-specific cache fallback path (falls through to stale). | Unit | Medium |
| 4.5 | `qa_TestCacheFallbackNoConfigID` — Evaluate VIX indicator with empty `configID` (preview mode). Verify cache fallback is skipped (no caching in preview mode). | Unit | Medium |
| 4.6 | `qa_TestCacheFallbackExtendsTTL` — After a successful cache fallback, verify `setCachedResult` is called again (line 517), effectively extending the cache TTL for subsequent ticks. Then fail again: the extended cache should still be within TTL. | Unit | Medium |
| 4.7 | `qa_TestCacheFallbackDetails` — Verify the `result.Details` string contains "cached" and the age (e.g., "2m0s") and the pass/fail marker. | Unit | Low |

---

### AC-5: Observability (all retry attempts logged)

**Requirement:** All retry attempts are logged with attempt number and error details.

**Existing test coverage:** INDIRECT  
The test output shows the log lines (visible in test run output above), confirming:
- `WARN 🔄 Retrying VIX evaluation attempt=2 maxAttempts=3 ...`
- `WARN ⚠️ VIX evaluation failed (transient) attempt=1 ...`
- `INFO ✅ VIX evaluation succeeded on retry attempt=2 ...`
- `WARN ⚠️ VIX evaluation failed, using cached value (within TTL) ...`

However, there are no assertions on log output in the tests.

**Additional QA tests needed:**

| # | Test | Type | Priority |
|---|------|------|----------|
| 5.1 | `qa_TestRetryLogsAttemptNumber` — Capture `slog` output (use `slog.New(slog.NewJSONHandler(buf))` or a custom handler). Verify: on retry, log contains `attempt`, `maxAttempts`, `symbol`, `previousError` keys. | Unit | Medium |
| 5.2 | `qa_TestRetrySuccessLogOnRetry` — After succeeding on retry, verify the success log contains `attempt` and `value`. | Unit | Low |
| 5.3 | `qa_TestCacheFallbackLogContainsKeys` — On cache fallback, verify log contains `indicatorID`, `cachedValue`, `cachedAt`, `cacheAge`, `error`. | Unit | Low |

---

### AC-6: Existing Tests Pass Without Modification

**Requirement:** All existing unit tests pass without modification.

**Existing test coverage:** CONFIRMED  
- `go test ./internal/automation/indicators/` — PASS (18.017s)
- `go build ./...` — compiles cleanly with zero errors

**Additional QA tests needed:**

| # | Test | Type | Priority |
|---|------|------|----------|
| 6.1 | Run full backend test suite: `go test ./...` — confirm no regressions anywhere in the codebase. | Integration | High |
| 6.2 | Verify no public API signatures changed (the mock in `vix_retry_test.go` implements `base.Provider` — if the interface changed, this would already fail to compile). | Build | N/A (covered by build) |

---

### AC-7: No Performance Regression on Healthy Connections

**Requirement:** The fix does not change behavior when DXLink connections are healthy (no performance regression).

**Existing test coverage:** PARTIAL  
`TestGetVIXValueWithRetry/succeeds_on_first_attempt` confirms: when the first attempt succeeds, no retry delay is introduced. Only 1 call is made.

**Additional QA tests needed:**

| # | Test | Type | Priority |
|---|------|------|----------|
| 7.1 | `qa_TestHappyPathNoExtraLatency` — Mock returns instantly. Measure elapsed time of `getVIXValueWithRetry`. Confirm it completes in <100ms (no sleep, no extra context creation overhead). | Unit | Medium |
| 7.2 | `qa_TestHappyPathSingleContextCreated` — On success, verify that only 1 attempt context was created and cancelled (the function doesn't pre-allocate retry contexts). Track via a custom context wrapper or mock. | Unit | Low |
| 7.3 | `qa_TestNoRetryOnSuccess` — Confirm `slog` does NOT emit any retry-related log messages when the first attempt succeeds. | Unit | Low |
| 7.4 | `qa_TestEvaluateIndicatorHappyPath_CachesResult` — After a successful VIX evaluation, verify the result is cached (so next tick can use fallback if needed). Confirm no side effects on the happy path. | Unit | Medium |

---

## 3. Race Condition & Concurrency Tests

The implementation uses mutexes for cache access (`cacheMu`, `quoteCacheMu`, `quoteFetchMu`). Additional concurrency tests:

| # | Test | Type | Priority |
|---|------|------|----------|
| R.1 | `qa_TestConcurrentVIXEvaluation` — Launch 10 goroutines all calling `EvaluateIndicator` for VIX simultaneously. Verify no data races (run with `-race`). | Race | High |
| R.2 | `qa_TestConcurrentCacheWriteAndFallback` — One goroutine writes to cache via successful evaluation. Another goroutine triggers cache fallback read simultaneously. Verify no race. | Race | High |
| R.3 | `qa_TestConcurrentRetryWithSharedProvider` — Multiple goroutines retry VIX simultaneously, all hitting the same mock provider. Verify `mock.calls` count is correct and no panics. | Race | Medium |
| R.4 | `qa_TestCacheFallbackDuringClearCache` — Call `ClearCache()` while another goroutine is in the cache fallback path. Verify no panic or data race. | Race | Medium |

**Note:** All race condition tests should be run with `go test -race`.

---

## 4. Boundary & Edge Case Tests

| # | Test | Type | Priority |
|---|------|------|----------|
| E.1 | `qa_TestVIXRetryWithEmptySymbol` — Call `getVIXValueWithRetry` with empty symbol. Verify it defaults to "VIX" and functions correctly. | Unit | Medium |
| E.2 | `qa_TestVIXRetryWithCustomSymbol_UVXY` — Call with symbol "UVXY". Verify it uses `getQuoteWithCache` path (not `GetHistoricalBars`). Retry logic should still apply. | Unit | High |
| E.3 | `qa_TestIsTransientError_WrappedError` — Test `isTransientError` with `fmt.Errorf("wrapper: %w", ioTimeoutErr)`. Verify the string matching still detects it through wrapping. | Unit | Medium |
| E.4 | `qa_TestVIXValueZero` — Provider returns `{"close": 0.0}`. Verify this is treated as an error ("VIX close price not available") and triggers retry. | Unit | Medium |
| E.5 | `qa_TestVIXValueNegative` — Provider returns `{"close": -1.0}`. Verify behavior (currently returned as-is — may want validation). | Unit | Low |
| E.6 | `qa_TestVIXRetryConstants` — Assert `vixMaxRetries == 2`, `vixRetryDelay == 2*time.Second`, `vixEvaluationTimeout == 20*time.Second`, `vixCacheFallbackTTL == 5*time.Minute`. Guards against accidental constant changes. | Unit | High |

---

## 5. Summary Matrix

| AC | Existing Coverage | Gaps | Priority of Gaps |
|----|-------------------|------|------------------|
| AC-1 (ALPN) | None | Dialer config test, integration test | Medium |
| AC-2 (Retry) | Good (4 cases) | Context cancellation, mixed errors, timing | High |
| AC-3 (Timeout) | Partial | Timeout enforcement, parent override, leak check | High |
| AC-4 (Cache) | Good (3 cases) | Boundary conditions, non-VIX, TTL extension | High |
| AC-5 (Logging) | Indirect (visible in output) | Structured log assertions | Medium |
| AC-6 (No breakage) | Confirmed | Full suite run (`go test ./...`) | High |
| AC-7 (No regression) | Partial (1 case) | Latency measurement, no-retry-on-success | Medium |
| Concurrency | None | Race detection tests | High |
| Edge Cases | None | Symbol handling, error wrapping, constants | Medium |

---

## 6. Recommended Test Execution Order

1. **Immediate (blocking):** Run `go test -race ./...` to confirm no race conditions in existing code.
2. **High priority:** Implement tests 2.1, 2.3, 2.4, 3.1, 3.2, 4.1, 4.2, 4.3, E.6, R.1, R.2.
3. **Medium priority:** Implement tests 2.2, 2.5, 3.3, 3.4, 4.4, 4.5, 4.6, 5.1, 7.1, 7.4, E.1, E.2, E.3, E.4, R.3, R.4.
4. **Low priority:** Implement tests 1.1-1.3, 2.6, 4.7, 5.2, 5.3, 7.2, 7.3, E.5.

---

## 7. Test Infrastructure Notes

- **Mock provider:** `vix_retry_test.go` defines a local `mockProvider` struct. The `newTestService()` helper uses `providers.NewTestProviderManager()` from `testing_helpers.go` to wire it up.
- **Real delays:** The retry tests currently sleep for the full `vixRetryDelay` (2s), making the retry suite take 18s. Consider extracting the delay as a configurable field on `Service` for test acceleration.
- **Log capture:** For AC-5 tests, use `slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))` in test setup (restore afterward) to capture and assert on log messages.
- **File naming convention:** QA tests should use `qa_` prefix per project convention: `qa_vix_retry_test.go`.
