# QA Test Plan: Schwab Provider (Issue #20)

**Issue:** [#20 - TD Ameritrade as a Provider](https://github.com/schardosin/juicytrade/issues/20)
**Requirements:** [requirements.md](./requirements.md)
**Architecture:** [architecture.md](./architecture.md)
**Date:** 2026-03-17
**Status:** Draft

---

## Summary

This test plan covers the Schwab (formerly TD Ameritrade) provider implementation across 11 source files and 10 test files (3,843 source lines, 5,790 test lines, 182 test cases). It verifies conformance against the requirements document's 27 acceptance criteria, 6 non-functional requirements, and the architecture document's structural and behavioral specifications.

**Test scope:** Unit tests (mocked HTTP/WebSocket), structural verification, code quality checks, regression testing, and additional edge-case tests to fill coverage gaps.

**Test location:** All Go tests run from `trade-backend-go/` via:
```bash
go test ./internal/providers/schwab/... -v -count=1
```

---

## Table of Contents

1. [Provider Registration & Configuration (AC-1)](#1-provider-registration--configuration-ac-1)
2. [OAuth Token Management (AC-3, AC-4, AC-23)](#2-oauth-token-management-ac-3-ac-4-ac-23)
3. [HTTP Helpers & Error Handling (NFR-3)](#3-http-helpers--error-handling-nfr-3)
4. [Rate Limiting (NFR-2)](#4-rate-limiting-nfr-2)
5. [Symbol Conversion (AC-24)](#5-symbol-conversion-ac-24)
6. [Market Data — Stock Quotes (AC-5, AC-6)](#6-market-data--stock-quotes-ac-5-ac-6)
7. [Market Data — Options Chain & Greeks (AC-7, AC-8, AC-9, AC-10)](#7-market-data--options-chain--greeks-ac-7-ac-8-ac-9-ac-10)
8. [Market Data — Historical & Calendar (AC-11)](#8-market-data--historical--calendar-ac-11)
9. [Market Data — Symbol Lookup (AC-19)](#9-market-data--symbol-lookup-ac-19)
10. [Account & Positions (AC-12, AC-13, AC-14)](#10-account--positions-ac-12-ac-13-ac-14)
11. [Orders (AC-15, AC-16, AC-17, AC-18)](#11-orders-ac-15-ac-16-ac-17-ac-18)
12. [Streaming — Connect/Disconnect (AC-20)](#12-streaming--connectdisconnect-ac-20)
13. [Streaming — Subscribe & Data Processing (AC-21, AC-22)](#13-streaming--subscribe--data-processing-ac-21-ac-22)
14. [Streaming — Reconnection (NFR-4)](#14-streaming--reconnection-nfr-4)
15. [Account Streaming (FR-10)](#15-account-streaming-fr-10)
16. [Code Quality & Architecture Conformance](#16-code-quality--architecture-conformance)
17. [Regression Check (AC-26)](#17-regression-check-ac-26)
18. [QA Additional Edge Case Tests](#18-qa-additional-edge-case-tests)

---

## 1. Provider Registration & Configuration (AC-1)

**Requirements:** FR-1.1, FR-1.2, FR-1.3, FR-1.4, FR-1.5
**Files under test:** `provider_types.go`, `manager.go`, `schwab.go`

### What to Check

#### 1.1 Provider Type Registration (`provider_types.go`)

- [ ] **Name:** `"schwab"` key exists in `ProviderTypes` map with `Name: "Schwab"`
- [ ] **Description:** `"Charles Schwab Trader API (formerly TD Ameritrade)"`
- [ ] **Account Types:** Contains both `"live"` and `"paper"` in `AccountTypes` slice
- [ ] **REST Capabilities:** All 9 required capabilities present:
  - `stock_quotes`, `options_chain`, `trade_account`, `symbol_lookup`, `historical_data`, `market_calendar`, `greeks`, `expiration_dates`, `next_market_date`
- [ ] **Streaming Capabilities:** All 3 required capabilities present:
  - `streaming_quotes`, `streaming_greeks`, `trade_account`
- [ ] **Credential Fields (live):** 6 fields with correct types:
  | Field | Label | Type | Required | Default |
  |-------|-------|------|----------|---------|
  | `app_key` | App Key | text | Yes | — |
  | `app_secret` | App Secret | password | Yes | — |
  | `callback_url` | Callback URL | text | Yes | `https://127.0.0.1` |
  | `refresh_token` | Refresh Token | password | Yes | — |
  | `account_hash` | Account Hash | text | Yes | — |
  | `base_url` | Base URL | text | No | `https://api.schwabapi.com` |
- [ ] **Credential Fields (paper):** Same 6 fields as live (per architecture §5.1)
- [ ] **Legacy Capabilities:** `LegacyProviderCapabilities` has `"schwab"` and/or `"schwab_paper"` entries with matching boolean capability flags

#### 1.2 Factory Case (`manager.go`)

- [ ] `createProviderInstance()` has a `case "schwab":` block
- [ ] Extracts all 6 credential fields: `app_key`, `app_secret`, `callback_url`, `refresh_token`, `account_hash`, `base_url`
- [ ] Defaults `base_url` to `"https://api.schwabapi.com"` when empty
- [ ] Defaults `accountType` to `"live"` when empty
- [ ] Calls `schwab.NewSchwabProvider(...)` with correct parameter order
- [ ] Import `"trade-backend-go/internal/providers/schwab"` is present

#### 1.3 Constructor (`schwab.go`)

- [ ] `NewSchwabProvider` sets all configuration fields on the struct
- [ ] Rate limiter initialized with 120 max tokens, 2.0 tokens/second
- [ ] Logger created with `"provider"="schwab"` attribute
- [ ] `BaseProviderImpl` embedded correctly
- [ ] Compile-time interface satisfaction: `var _ base.Provider = (*SchwabProvider)(nil)`

### Tests to Run

```bash
# Existing tests for constructor and TestCredentials
go test ./internal/providers/schwab/... -run "TestNewSchwabProvider" -v
go test ./internal/providers/schwab/... -run "TestTestCredentials" -v
go test ./internal/providers/schwab/... -run "TestPing" -v
```

**Existing test coverage (schwab_test.go, 343 lines, 10 tests):**
- `TestNewSchwabProvider` — verifies all fields set correctly
- `TestNewSchwabProvider_Defaults` — empty baseURL/accountType get defaults
- `TestTestCredentials_Success` — valid creds + account hash match
- `TestTestCredentials_PaperAccount` — paper mode returns sandbox warning
- `TestTestCredentials_InvalidCredentials` — 401 returns `success=false`
- `TestTestCredentials_AccountHashMismatch` — hash not in account list
- `TestTestCredentials_EmptyResponse` — zero accounts returned
- `TestTestCredentials_APIError` — 500 on accounts endpoint
- `TestTestCredentials_MultipleAccountsMatchesCorrectOne` — hash found among 3 accounts
- `TestPing_Success`, `TestPing_SuccessWithCachedToken`, `TestPing_Failure`

**Verdict:** Registration and constructor are well covered. Manual inspection of `provider_types.go` and `manager.go` is needed for the checklist items above (no automated test verifies the registration map contents directly).

---

## 2. OAuth Token Management (AC-3, AC-4, AC-23)

**Requirements:** FR-2.1, FR-2.2, FR-2.3, FR-2.4, NFR-1
**Files under test:** `auth.go`, `schwab.go`

### What to Check

- [ ] **Token endpoint URL:** `{baseURL}/v1/oauth/token`
- [ ] **Auth method:** HTTP Basic with `base64(appKey:appSecret)`
- [ ] **Request body:** `application/x-www-form-urlencoded` with `grant_type=refresh_token&refresh_token=<token>`
- [ ] **ensureValidToken():** Uses `sync.Mutex` (not RWMutex); 5-minute expiry buffer; lazy refresh on demand
- [ ] **refreshAccessToken():** Updates `accessToken` and `tokenExpiry` in memory; called only under `tokenMu` lock
- [ ] **ErrRefreshTokenExpired:** Sentinel error returned on 401 from token endpoint
- [ ] **Token rotation logging:** Warns when server returns a new refresh token (cannot persist)
- [ ] **Token refresh does NOT use `doAuthenticatedRequest`:** Uses direct `net/http` to avoid circular dependency with rate limiter
- [ ] **Proactive refresh:** Token refreshed when remaining lifetime < 5 minutes

### Tests to Run

```bash
go test ./internal/providers/schwab/... -run "TestEnsureValidToken|TestRefreshAccessToken|TestTruncateToken" -v
```

**Existing test coverage (auth_test.go, 376 lines, 9 tests):**
- `TestEnsureValidToken_CachedToken` — valid token (10 min left), no HTTP call
- `TestEnsureValidToken_ExpiredToken` — triggers refresh
- `TestEnsureValidToken_NearExpiryToken` — 3 min left, triggers refresh (within 5-min buffer)
- `TestEnsureValidToken_NoToken` — fresh provider, triggers refresh
- `TestRefreshAccessToken_Success` — verifies POST method, path, headers, body, token update
- `TestRefreshAccessToken_ExpiredRefreshToken` — 401 → `ErrRefreshTokenExpired`
- `TestRefreshAccessToken_MalformedJSON` — 200 + bad JSON → parse error
- `TestRefreshAccessToken_MissingAccessToken` — 200 + empty `access_token` → error
- `TestRefreshAccessToken_ServerError` — 500 → error
- `TestRefreshAccessToken_TokenRotation` — new refresh token accepted, original preserved
- `TestRefreshAccessToken_ExpiresInZero` — zero expiry accepted
- `TestTruncateToken` — table-driven, 4 cases

**Gaps identified:**
- No concurrent access test for `ensureValidToken()` (multiple goroutines racing to refresh)
- No network error test (server unreachable during refresh)

---

## 3. HTTP Helpers & Error Handling (NFR-3)

**Requirements:** NFR-3, architecture §7, §12
**Files under test:** `helpers.go`

### What to Check

- [ ] **`doAuthenticatedRequest`:** Rate limiter wait → `ensureValidToken` → HTTP request → error handling
- [ ] **401 retry:** On 401, clears token, re-authenticates, retries once; returns error if retry also fails
- [ ] **429 handling:** Returns `"schwab: rate limited (HTTP 429)"` error
- [ ] **Error parsing:** Handles 4 Schwab error formats:
  1. OAuth: `{"error": "...", "error_description": "..."}`
  2. API errors array: `{"errors": [{"message": "..."}]}`
  3. Simple: `{"message": "..."}`
  4. Plain text / fallback
- [ ] **URL builders:** `buildMarketDataURL("/quotes")` → `"{baseURL}/marketdata/v1/quotes"`, `buildTraderURL("/accounts/...")` → `"{baseURL}/trader/v1/accounts/..."`
- [ ] **Request headers:** `Authorization: Bearer {token}`, `Accept: application/json`, `Content-Type: application/json` (POST/PUT only)
- [ ] **30-second HTTP timeout:** Set on request context

### Tests to Run

```bash
go test ./internal/providers/schwab/... -run "TestBuild|TestParseError|TestDoAuthenticated|TestTruncateBody" -v
```

**Existing test coverage (helpers_test.go, 418 lines, 13 tests):**
- `TestBuildMarketDataURL` — 5 paths (table-driven)
- `TestBuildTraderURL` — 3 paths (table-driven)
- `TestParseErrorResponse_OAuthError` — OAuth format
- `TestParseErrorResponse_OAuthErrorNoDescription` — OAuth without description
- `TestParseErrorResponse_APIErrors` — errors array with `message` key
- `TestParseErrorResponse_APIErrorsDetailFallback` — falls back to `detail` key
- `TestParseErrorResponse_MessageError` — simple message format
- `TestParseErrorResponse_PlainText` — non-JSON body
- `TestParseErrorResponse_EmptyBody` — empty body
- `TestParseErrorResponse_LongBody` — truncated to 200 chars
- `TestDoAuthenticatedRequest_Success` — verifies Bearer header, Accept header
- `TestDoAuthenticatedRequest_PostContentType` — POST adds Content-Type
- `TestDoAuthenticatedRequest_401Retry` — 401 → refresh → retry succeeds
- `TestDoAuthenticatedRequest_401RetryAlsoFails` — both attempts 401
- `TestDoAuthenticatedRequest_429RateLimit` — rate limit error
- `TestDoAuthenticatedRequest_500Error` — server error with parsed message
- `TestDoAuthenticatedRequest_NoToken` — auto-refresh before request
- `TestDoAuthenticatedRequest_AuthFailure` — token refresh fails
- `TestTruncateBody` — 4 cases (table-driven)

**Gaps identified:**
- No test for context cancellation during `doAuthenticatedRequest`
- No test for network-level errors (DNS failure, connection refused)
- No test for DELETE method headers
- POST body retry limitation not tested (body=nil on retry)

---

## 4. Rate Limiting (NFR-2)

**Requirements:** NFR-2, architecture §8
**Files under test:** `rate_limiter.go`

### What to Check

- [ ] **Token bucket parameters:** 120 max tokens, 2.0 tokens/second (= 120 req/min)
- [ ] **`wait()` behavior:** Blocks until token available; spin-waits with 50ms sleep
- [ ] **`allow()` behavior:** Non-blocking; returns false when exhausted
- [ ] **Refill:** Adds `elapsed * refillRate` tokens, capped at `maxTokens`
- [ ] **Thread safety:** `sync.Mutex` protects all state
- [ ] **Exclusions:** Token refresh (`refreshAccessToken`) does NOT go through rate limiter; WebSocket messages are not rate limited

### Tests to Run

```bash
go test ./internal/providers/schwab/... -run "TestRateLimiter|TestNewRateLimiter" -v
```

**Existing test coverage (rate_limiter_test.go, 208 lines, 8 tests):**
- `TestNewRateLimiter` — constructor sets fields, bucket starts full
- `TestRateLimiterWait_ImmediateReturn` — returns < 10ms with tokens available
- `TestRateLimiterAllow_Available` — returns true, consumes token
- `TestRateLimiterAllow_Exhausted` — 6th call returns false (5 max, 0 refill)
- `TestRateLimiterRefill` — tokens refill after elapsed time
- `TestRateLimiterRefill_Capped` — tokens don't exceed max
- `TestRateLimiterBurstCapacity` — 50 burst requests succeed, 51st fails
- `TestRateLimiterWait_BlocksWhenEmpty` — blocks 30-200ms when empty
- `TestRateLimiterConcurrency` — 20 goroutines, no deadlock
- `TestRateLimiterConcurrency_Allow` — 20 goroutines, exactly 100 tokens consumed

**Gaps identified:**
- No integration test verifying `doAuthenticatedRequest` calls `rateLimiter.wait()`
- No boundary test at exactly 120 tokens (matching the production configuration)

---

## 5. Symbol Conversion (AC-24)

**Requirements:** FR-11.1, architecture §9.7
**Files under test:** `symbols.go`

### What to Check

- [ ] **`convertSchwabOptionToOCC`:** Handles 3 input formats:
  1. Space-padded: `"AAPL  250117C00150000"` → `"AAPL250117C00150000"`
  2. Dot-prefix: `".AAPL250117C150"` → `"AAPL250117C00150000"`
  3. Already OCC: passthrough
- [ ] **`convertOCCToSchwab`:** Pads underlying to 6 chars with trailing spaces: `"AAPL250117C00150000"` → `"AAPL  250117C00150000"`
- [ ] **`isOptionSymbol`:** Correctly classifies equity vs option symbols
- [ ] **`classifySymbols`:** Splits mixed lists into equity and option groups
- [ ] **Round-trip identity:** OCC → Schwab → OCC and Schwab → OCC → Schwab
- [ ] **Edge cases:** Single-char underlying (`"F"`), 5-char (`"GOOGL"`), 6-char (`"SPXW  "`), high strikes (>$1000), sub-dollar strikes, empty strings

### Tests to Run

```bash
go test ./internal/providers/schwab/... -run "TestConvert|TestIsOption|TestClassify" -v
```

**Existing test coverage (symbols_test.go, 279 lines, 10 tests):**
- `TestConvertSchwabOptionToOCC` — 13 table-driven cases (space-padded, dot-prefix, passthrough, empty)
- `TestConvertOCCToSchwab` — 9 table-driven cases (1-6 char underlyings, empty, too-short)
- `TestConvertRoundTrip_OCCToSchwabToOCC` — 6 symbols
- `TestConvertRoundTrip_SchwabToOCCToSchwab` — 5 symbols
- `TestIsOptionSymbol` — 18 table-driven cases
- `TestClassifySymbols` — 7 mixed symbols
- `TestClassifySymbols_Empty` — nil and empty slice
- `TestClassifySymbols_AllEquities` — 4 equities
- `TestClassifySymbols_AllOptions` — 2 options
- `TestConvertDotPrefixToOCC_EdgeCases` — 6 cases (decimal/high/sub-dollar strikes)

**Verdict:** Symbol conversion is thoroughly tested with extensive table-driven cases and round-trip verification.

---

## 6. Market Data — Stock Quotes (AC-5, AC-6)

**Requirements:** FR-3.1, FR-3.2
**Files under test:** `market_data.go`

### What to Check

- [ ] **`GetStockQuote(ctx, symbol)`:** Calls `GET /marketdata/v1/quotes?symbols={symbol}`, transforms to `*models.StockQuote` with Symbol, Bid, Ask, Last, Timestamp
- [ ] **`GetStockQuotes(ctx, symbols)`:** Comma-separated symbols in URL, returns `map[string]*models.StockQuote`
- [ ] **Schwab response mapping:** `bidPrice` → Bid, `askPrice` → Ask, `lastPrice` → Last, `quoteTimeInLong` → Timestamp (RFC3339 from epoch millis)
- [ ] **Nested quote object:** Response may have a `"quote"` sub-object or flat structure
- [ ] **Case-insensitive lookup:** `findSymbolData` handles case mismatch between request/response
- [ ] **Empty/partial responses:** Graceful handling

### Tests to Run

```bash
go test ./internal/providers/schwab/... -run "TestGetStockQuote|TestGetStockQuotes|TestTransformSchwabQuote|TestExtractFloat64|TestExtractMap|TestFindSymbol" -v
```

**Existing test coverage (market_data_test.go):**
- `TestGetStockQuote_Success` — full AAPL quote with all fields
- `TestGetStockQuote_NotFound` — empty response → error
- `TestGetStockQuote_APIError` — 500 error
- `TestGetStockQuotes_MultipleSymbols` — 3 symbols
- `TestGetStockQuotes_PartialResponse` — 2 requested, 1 returned
- `TestGetStockQuotes_EmptyInput` — empty list, no HTTP call
- `TestTransformSchwabQuote_FullData` — pure function, all fields
- `TestTransformSchwabQuote_MissingFields` — minimal data
- `TestTransformSchwabQuote_NoQuoteSubObject` — flat response structure
- `TestTransformSchwabQuote_EmptyData` — empty map
- `TestTransformSchwabQuote_TradeTimeFallback` — timestamp fallback chain
- `TestExtractFloat64` — type coercion (float64, int, string, nil)
- `TestExtractMap` — nested map extraction
- `TestFindSymbolData_CaseInsensitive` — case mismatch handling

**Verdict:** Good coverage. No major gaps.

---

## 7. Market Data — Options Chain & Greeks (AC-7, AC-8, AC-9, AC-10)

**Requirements:** FR-4.1, FR-4.2, FR-4.3, FR-4.4
**Files under test:** `market_data.go`

### What to Check

- [ ] **`GetExpirationDates(ctx, symbol)`:** Extracts unique dates from `callExpDateMap`/`putExpDateMap` keys; returns `[]map[string]interface{}` with `date` and `dte` fields
- [ ] **`GetOptionsChainBasic`:** Builds URL with `fromDate`, `toDate`, `strikeCount`; returns `[]*models.OptionContract` without Greeks
- [ ] **`GetOptionsChainSmart`:** Includes `includeUnderlyingQuote=true`; includes Greeks (delta, gamma, theta, vega, IV) when `includeGreeks=true`
- [ ] **`GetOptionsGreeksBatch`:** Converts OCC symbols to Schwab format; fetches via `/marketdata/v1/quotes`; returns `map[string]map[string]interface{}` with delta, gamma, theta, vega, rho, implied volatility
- [ ] **Symbol conversion in chain:** Schwab option symbols converted to OCC format in all returned contracts
- [ ] **Chain date key parsing:** Schwab uses `"2025-01-17:180"` format (date:DTE) as map keys
- [ ] **`transformSchwabOptionsChain`:** Iterates both `callExpDateMap` and `putExpDateMap`

### Tests to Run

```bash
go test ./internal/providers/schwab/... -run "TestGetExpirationDates|TestGetOptionsChain|TestGetOptionsGreeks|TestTransformSchwabOptions" -v
```

**Existing test coverage (market_data_test.go):**
- `TestGetExpirationDates_Success` — 2 expiration dates with DTE
- `TestGetExpirationDates_EmptyChain` — returns 0 dates
- `TestGetOptionsChainBasic_Success` — 4 contracts, OCC format, no Greeks
- `TestGetOptionsChainSmart_WithGreeks` — Greeks populated (delta, gamma, theta, vega, IV), `includeUnderlyingQuote=true` param verified
- `TestGetOptionsGreeksBatch_Success` — OCC symbol lookup, Greeks keyed by OCC symbol
- `TestGetOptionsGreeksBatch_Empty` — empty input
- `TestTransformSchwabOptionsChain` — 4 contracts (3 calls, 1 put), specific field verification
- `TestTransformSchwabOptionsChain_EmptyMap` — 0 contracts
- `TestTransformSchwabOptionsChain_NoGreeks` — delta/gamma are nil

**Gaps identified:**
- No test for `GetOptionsChainBasic` with specific `optionType` filtering (calls only, puts only)
- No test for `GetOptionsChainBasic` with `strikeCount` parameter validation
- No test for API errors during options chain requests

---

## 8. Market Data — Historical & Calendar (AC-11)

**Requirements:** FR-5.1, FR-5.2
**Files under test:** `market_data.go`

### What to Check

- [ ] **`GetHistoricalBars`:** Calls `GET /marketdata/v1/pricehistory` with correct timeframe mapping
- [ ] **Timeframe mapping:**
  | Input | periodType | frequencyType | frequency |
  |-------|-----------|---------------|-----------|
  | `"1min"` | day | minute | 1 |
  | `"5min"` | day | minute | 5 |
  | `"15min"` | day | minute | 15 |
  | `"30min"` | day | minute | 30 |
  | `"1D"` / `"daily"` | year | daily | 1 |
  | `"1W"` / `"weekly"` | year | weekly | 1 |
- [ ] **Date range:** `startDate`/`endDate` converted to epoch milliseconds
- [ ] **Limit:** Returns last N bars when limit specified
- [ ] **OHLCV output:** Each bar has `open`, `high`, `low`, `close`, `volume`, `timestamp` (RFC3339)
- [ ] **`GetNextMarketDate`:** Returns next weekday in `YYYY-MM-DD` format (no holiday awareness — documented limitation)

### Tests to Run

```bash
go test ./internal/providers/schwab/... -run "TestGetHistoricalBars|TestGetNextMarketDate|TestMapTimeframe|TestDateToEpoch|TestIsWeekday" -v
```

**Existing test coverage (market_data_test.go):**
- `TestGetHistoricalBars_Daily` — daily bars, verifies frequencyType/frequency params
- `TestGetHistoricalBars_Minute` — 5min timeframe
- `TestGetHistoricalBars_WithDateRange` — start/end to epoch ms
- `TestGetHistoricalBars_WithLimit` — limit=2 returns last 2 of 3
- `TestGetHistoricalBars_EmptyCandles` — 0 bars
- `TestGetNextMarketDate` — returns valid weekday
- `TestMapTimeframe` — 11 table-driven entries
- `TestDateToEpochMs` — valid + invalid date
- `TestIsWeekday` — Mon-Sun coverage

**Gaps identified:**
- No negative test for `GetHistoricalBars` with API errors (500, 404)
- `GetNextMarketDate` does not account for holidays (documented; no holiday data available)

---

## 9. Market Data — Symbol Lookup (AC-19)

**Requirements:** FR-8.1
**Files under test:** `market_data.go`

### What to Check

- [ ] **`LookupSymbols(ctx, query)`:** Calls `GET /marketdata/v1/instruments?symbol={query}&projection=symbol-search`
- [ ] **Response transform:** Returns `[]*models.SymbolSearchResult` with symbol, description, exchange, type
- [ ] **Asset type mapping:** `EQUITY` → `stock`, `ETF` → `etf`, `INDEX` → `index`, `OPTION` → `option`, etc.
- [ ] **Two response formats:** Handles both array format (`instruments: [...]`) and map format (`{symbol: {...}}`)
- [ ] **Empty query:** Returns empty results, no HTTP call

### Tests to Run

```bash
go test ./internal/providers/schwab/... -run "TestLookupSymbols|TestMapSchwabAssetType" -v
```

**Existing test coverage (market_data_test.go):**
- `TestLookupSymbols_Success` — 3 results (EQUITY, ETF, OPTION)
- `TestLookupSymbols_MapFormat` — map-keyed response
- `TestLookupSymbols_NoResults` — empty instruments
- `TestLookupSymbols_EmptyQuery` — no HTTP call
- `TestMapSchwabAssetType` — 9 table-driven entries

**Verdict:** Good coverage of both response formats and edge cases.

---

## 10. Account & Positions (AC-12, AC-13, AC-14)

**Requirements:** FR-6.1, FR-6.2, FR-6.3, FR-6.4
**Files under test:** `account.go`

### What to Check

- [ ] **`GetAccount(ctx)`:** Calls `GET /trader/v1/accounts/{accountHash}?fields=positions`; returns `*models.Account` with accountID, status, currency, buyingPower, cash, portfolioValue, equity, dayTradingBuyingPower, longMarketValue, shortMarketValue, maintenanceRequirement
- [ ] **Account hash usage:** Uses `s.accountHash` (not raw account number) in URL
- [ ] **`GetPositions(ctx)`:** Extracts `securitiesAccount.positions` from same endpoint; transforms each to `*models.Position`
- [ ] **Asset class detection:** `"EQUITY"` → `"us_equity"`, `"OPTION"` → `"us_option"`
- [ ] **Option position parsing:** OCC symbol parsed for underlying, option type, strike, expiry
- [ ] **Net quantity:** `longQuantity - shortQuantity`
- [ ] **`GetPositionsEnhanced(ctx)`:** Calls `GetPositions()` then `BaseProviderImpl.ConvertPositionsToEnhanced()`

### Tests to Run

```bash
go test ./internal/providers/schwab/... -run "TestGetAccount|TestGetPositions|TestParseOCC" -v
```

**Existing test coverage (account_test.go, 373 lines, 7 tests):**
- `TestGetAccount_Success` — full account with all balance fields
- `TestGetAccount_APIError` — 500 error
- `TestGetPositions_Success` — 3 positions (2 equity + 1 option)
- `TestGetPositions_OptionPositionParsing` — OCC symbol parsing for option positions
- `TestGetPositions_NoPositions` — empty positions array
- `TestGetPositionsEnhanced_Success` — non-nil enhanced response
- `TestParseOCCSymbolIntoPosition` — 4 table-driven cases (AAPL call, SPY put, F low-strike, GOOGL call)
- `TestParseOCCSymbolIntoPosition_TooShort` — short symbol, no crash

**Gaps identified:**
- No test for short positions (`shortQuantity > 0`)
- No test for malformed JSON response from account endpoint
- `GetPositionsEnhanced` only checks non-nil, does not validate grouping
- No test for empty `currentBalances` in account response

---

## 11. Orders (AC-15, AC-16, AC-17, AC-18)

**Requirements:** FR-6.4, FR-7.1, FR-7.2, FR-7.3, FR-7.4
**Files under test:** `orders.go`

### What to Check

- [ ] **`GetOrders(ctx, status)`:** Status mapping: `"open"` → `WORKING`, `"filled"` → `FILLED`, `"canceled"` → `CANCELED`; 30-day lookback window
- [ ] **`PlaceOrder(ctx, orderData)`:** Builds `schwabOrderRequest` JSON; `POST /trader/v1/accounts/{hash}/orders`; parses order ID from response
- [ ] **`PlaceMultiLegOrder(ctx, orderData)`:** Multiple legs in `orderLegCollection`; correct instructions (`BUY_TO_OPEN`, `SELL_TO_OPEN`, etc.); option symbols converted to Schwab format
- [ ] **`CancelOrder(ctx, orderID)`:** `DELETE /trader/v1/accounts/{hash}/orders/{orderID}`; returns `(true, nil)` on success
- [ ] **`PreviewOrder(ctx, orderData)`:** Returns error `"schwab: order preview not supported by Schwab API"` (FR-7.3)
- [ ] **Order type mapping:** `market` → `MARKET`, `limit` → `LIMIT`, `stop` → `STOP`, `stop_limit` → `STOP_LIMIT`
- [ ] **Duration mapping:** `day` → `DAY`, `gtc` → `GOOD_TILL_CANCEL`, `fok` → `FILL_OR_KILL`
- [ ] **Side/instruction mapping:** `buy` (equity) → `BUY`, `buy` (option) → `BUY_TO_OPEN`, `sell` (equity) → `SELL`, `sell` (option) → `SELL_TO_CLOSE`
- [ ] **Schwab order status mapping:** `WORKING` → `open`, `FILLED` → `filled`, `CANCELED` → `canceled`, `REJECTED` → `rejected`, `EXPIRED` → `expired`, `PENDING_*`/`QUEUED`/`ACCEPTED` → `pending`

### Tests to Run

```bash
go test ./internal/providers/schwab/... -run "TestGetOrders|TestCancelOrder|TestPlaceOrder|TestPlaceMultiLeg|TestMapSchwab|TestMapSide|TestMapOrder|TestMapDuration|TestBuildSchwab|TestTransformSchwabOrder" -v
```

**Existing test coverage (orders_test.go, 871 lines, 17 tests):**
- `TestGetOrders_Success` — 2 orders (working + filled), full field verification
- `TestGetOrders_OpenFilter` — `status=WORKING` query param
- `TestGetOrders_MultiLegOrder` — 2-leg option spread
- `TestGetOrders_EmptyResponse` — 0 orders
- `TestCancelOrder_Success` — DELETE method + path
- `TestCancelOrder_NotFound` — 404 error
- `TestTransformSchwabOrder_Basic` — pure function mapping
- `TestTransformSchwabOrder_NoOrderID` — returns nil
- `TestMapSchwabOrderStatus` — 13 entries (table-driven)
- `TestMapSchwabInstruction` — 7 entries (table-driven)
- `TestMapSchwabDuration` — 4 entries (table-driven)
- `TestPlaceOrder_EquityBuy` — market buy, verifies request body
- `TestPlaceOrder_OptionSell` — option sell, Schwab symbol format
- `TestPlaceOrder_LimitOrder` — limit + GTC
- `TestPlaceMultiLegOrder_VerticalSpread` — 2-leg spread
- `TestPlaceOrder_APIError` — 400 error
- `TestPlaceOrder_MissingSymbol` — validation error
- `TestMapSideToInstruction` — 4 entries (table-driven)
- `TestMapOrderTypeToSchwab` — 7 entries (table-driven)
- `TestMapDurationToSchwab` — 4 entries (table-driven)
- `TestBuildSchwabOrderRequest` — full struct build

**Gaps identified:**
- No test for `PlaceOrder` with stop/stop_limit order types
- No test for `PlaceMultiLegOrder` with invalid/missing legs
- No test for `PreviewOrder` explicitly (though it's a simple error return)
- No test for `GetOrders` with `"filled"` or `"canceled"` filter

---

## 12. Streaming — Connect/Disconnect (AC-20)

**Requirements:** FR-9.1, FR-9.4
**Files under test:** `streaming.go`

### What to Check

- [ ] **`ConnectStreaming(ctx)`:** 5-step process:
  1. Fetch streamer info from `GET /trader/v1/userPreference`
  2. Store `streamSocketURL`, `streamCustomerID`, `streamCorrelID`
  3. Connect WebSocket (3 retries, exponential backoff: 1s, 2s, 4s)
  4. Send LOGIN request with access token
  5. Verify LOGIN response `code == 0`
- [ ] **Start `streamReadLoop`:** Goroutine launched after successful login
- [ ] **`IsConnected` state:** Set to `true` on success
- [ ] **`DisconnectStreaming(ctx)`:** Close `streamStopChan`, wait for read loop, send LOGOUT (best-effort), close WebSocket, clear state
- [ ] **Already-connected guard:** Returns immediately if already connected

### Tests to Run

```bash
go test ./internal/providers/schwab/... -run "TestConnectStreaming|TestDisconnectStreaming|TestFetchStreamerInfo|TestNextRequestID" -v
```

**Existing test coverage (streaming_test.go, 1,665 lines):**
- `TestConnectStreaming_Success` — full handshake verification
- `TestConnectStreaming_LoginFailure` — code=3 → error
- `TestConnectStreaming_AlreadyConnected` — no-op
- `TestDisconnectStreaming_Success` — IsConnected=false after disconnect
- `TestDisconnectStreaming_NotConnected` — no error
- `TestFetchStreamerInfo_Success` — parses all fields
- `TestFetchStreamerInfo_MissingURL` — error
- `TestNextRequestID` — unique, monotonic IDs

**Verdict:** Connect/disconnect lifecycle well covered.

---

## 13. Streaming — Subscribe & Data Processing (AC-21, AC-22)

**Requirements:** FR-9.2, FR-9.3
**Files under test:** `streaming.go`, `field_maps.go`

### What to Check

- [ ] **`SubscribeToSymbols`:** Classifies symbols → separate `LEVELONE_EQUITIES` and `LEVELONE_OPTIONS` subscriptions
- [ ] **Equity subscription fields:** `"0,1,2,3,4,5,8,10,11,12,17,18,33,34,35"` (15 fields)
- [ ] **Option subscription fields:** `"0,2,3,4,5,6,7,8,9,10,12,15,19,21,22,23,26,28,29,30,31,32,35,37"` (24 fields)
- [ ] **Symbol format conversion:** OCC symbols converted to Schwab format for subscription, Schwab symbols converted back to OCC in received data
- [ ] **Batching:** Groups of 50 symbols with 100ms inter-batch delay
- [ ] **Numerical field decoding:** Stream messages use numeric keys (e.g., `"1"` → `BID_PRICE`), decoded via field maps
- [ ] **MarketData construction:**
  - Equities: `DataType="quote"`, data keys: `bid`, `ask`, `last`, `volume`, `mark`, etc.
  - Options: `DataType="greeks"`, data keys: `delta`, `gamma`, `theta`, `vega`, `rho`, `underlying_price`, etc.
- [ ] **Dispatch:** Data sent to `StreamingCache` (preferred) or `StreamingQueue` (fallback)
- [ ] **`SubscribedSymbols` tracking:** All subscribed symbols tracked in the map

### Tests to Run

```bash
go test ./internal/providers/schwab/... -run "TestSubscribeToSymbols|TestUnsubscribe|TestProcessStreamData|TestDispatchMarketData|TestStreamReadLoop_DataDispatch|TestStreamReadLoop_Heartbeat" -v
```

**Existing test coverage (streaming_test.go):**
- `TestSubscribeToSymbols_NotConnected` — error when not connected
- `TestSubscribeToSymbols_EmptySymbols` — no-op
- `TestSubscribeToSymbols_ClassifiesAndSendsCorrectServices` — equity/option separation, field strings, symbol format
- `TestSubscribeToSymbols_Batching` — 120 symbols → 3 batches
- `TestUnsubscribeFromSymbols_RemovesTrackedSymbols` — UNSUBS + tracking cleanup
- `TestUnsubscribeFromSymbols_NotConnected` — no-op
- `TestProcessStreamData_EquityFields` — numerical key decoding (bid, ask, last, volume, mark, change)
- `TestProcessStreamData_OptionFieldsWithGreeks` — delta, gamma, theta, vega, rho, underlying_price; OCC symbol conversion
- `TestProcessStreamData_MultipleItems` — 2 items dispatched
- `TestProcessStreamData_UnhandledService` — silently ignored
- `TestProcessStreamData_MissingSymbol` — skipped
- `TestDispatchMarketData_ToQueue` — queue dispatch
- `TestDispatchMarketData_ToCache` — cache preferred over queue
- `TestDispatchMarketData_OptionFields` — option data keys verified
- `TestStreamReadLoop_DataDispatch` — end-to-end via WebSocket
- `TestStreamReadLoop_Heartbeat` — no crash

**Gaps identified:**
- Only ~10 of 52 equity fields and ~12 of 55 option fields verified
- No test for concurrent subscribe/unsubscribe
- No test for malformed WebSocket messages

---

## 14. Streaming — Reconnection (NFR-4)

**Requirements:** NFR-4, architecture §10.7
**Files under test:** `streaming.go`

### What to Check

- [ ] **Reconnection trigger:** Read error in `streamReadLoop` calls `handleStreamDisconnect`
- [ ] **Backoff schedule:** 5s, 10s, 20s, 40s, 60s (5 retries)
- [ ] **Re-subscription:** After reconnect, all previously subscribed symbols re-subscribed
- [ ] **Max retries:** After 5 failures, stops retrying
- [ ] **Graceful disconnect suppression:** `DisconnectStreaming()` does NOT trigger reconnection
- [ ] **`EnsureHealthyConnection`:** Detects stale connections (>120s without data); triggers reconnect
- [ ] **State management:** `IsConnected` correctly set to `false` during reconnection

### Tests to Run

```bash
go test ./internal/providers/schwab/... -run "TestReconnect|TestHandleStream|TestEnsureHealthy" -v
```

**Existing test coverage (streaming_test.go):**
- `TestReconnect_TriggeredAfterReadError` — connection count reaches 2
- `TestReconnect_ResubscribesExistingSymbols` — equity + option re-subscribed
- `TestReconnect_MaxRetryExhaustion` — exactly `reconnectMaxRetries` attempts
- `TestReconnect_NoReconnectOnGracefulDisconnect` — count stays 1
- `TestHandleStreamDisconnect_StopChanClosed` — no-op when already stopped
- `TestEnsureHealthyConnection_AlreadyHealthy` — no action
- `TestEnsureHealthyConnection_NotConnected_Connects` — auto-connects
- `TestEnsureHealthyConnection_StaleConnection` — disconnects + reconnects
- `TestEnsureHealthyConnection_TypeAssertion` — duck-type interface check

**Verdict:** Reconnection logic is well tested including edge cases.

---

## 15. Account Streaming (FR-10)

**Requirements:** FR-10.1, FR-10.2, FR-10.3
**Files under test:** `account_stream.go`

### What to Check

- [ ] **`StartAccountStream(ctx)`:** Subscribes to `ACCT_ACTIVITY` service; auto-connects streaming if not connected
- [ ] **`StopAccountStream()`:** Unsubscribes from `ACCT_ACTIVITY`; sets `acctStreamActive=false`
- [ ] **`SetOrderEventCallback(callback)`:** Stores callback under `acctStreamMu` lock; replaceable
- [ ] **`IsAccountStreamConnected()`:** Returns `acctStreamActive && IsConnected` (both required)
- [ ] **Account activity processing:** Parses `ACCT_ACTIVITY` data items into `*models.OrderEvent`; invokes callback
- [ ] **Order event fields:** `OrderID`, `Status`, `Symbol`, `Side`, `OrderType`, `Quantity`, `AveragePrice`, `Timestamp`
- [ ] **Event normalization:** Uses `models.GetGlobalNormalizer().NormalizeEvent` for state transitions
- [ ] **Shared WebSocket:** Uses same connection as market streaming (not separate)

### Tests to Run

```bash
go test ./internal/providers/schwab/... -run "TestStartAccountStream|TestStopAccountStream|TestSetOrderEventCallback|TestIsAccountStreamConnected|TestProcessAccountActivity|TestProcessStreamData_RoutesACCT|TestStreamReadLoop_AccountActivity|TestExtractStringField|TestExtractFloatField|TestMapSchwabActivitySide" -v
```

**Existing test coverage (account_stream_test.go, 806 lines, 14 tests):**
- `TestStartAccountStream_SubscribesACCT_ACTIVITY` — SUBS sent, `IsAccountStreamConnected()` true
- `TestStartAccountStream_ConnectsIfNotConnected` — auto-connects
- `TestStartAccountStream_ConnectionFailure` — unreachable WS URL
- `TestStopAccountStream_ClearsActiveState` — UNSUBS sent, active=false
- `TestStopAccountStream_NoOpWhenNotActive` — no panic
- `TestSetOrderEventCallback_StoresCallback` — stores + invocable
- `TestSetOrderEventCallback_ReplacesExisting` — replacement works
- `TestIsAccountStreamConnected_BothRequired` — 4 combinations
- `TestProcessAccountActivity_InvokesCallback` — full event parsing (orderId, status, symbol, side, qty, fill price, normalized event)
- `TestProcessAccountActivity_PendingOrder` — PENDING_ACTIVATION → pending
- `TestProcessAccountActivity_NoCallback` — no panic
- `TestProcessAccountActivity_EmptyContent` — callback not invoked
- `TestProcessAccountActivity_UnparseableItem` — callback not invoked
- `TestProcessAccountActivity_MultipleEvents` — 2 events dispatched
- `TestProcessStreamData_RoutesACCT_ACTIVITY` — routing to account handler
- `TestStreamReadLoop_AccountActivityDispatch` — end-to-end via WS
- `TestExtractStringField` — found, fallback key, not found, empty, non-string
- `TestExtractFloatField` — float64, int, int64, missing, non-numeric
- `TestMapSchwabActivitySide` — 9 table-driven entries

**Gaps identified:**
- No test for CANCELED/REJECTED/EXPIRED order events
- No test for option symbol conversion in account activity messages

---

## 16. Code Quality & Architecture Conformance

**Requirements:** NFR-3, NFR-4, NFR-5, NFR-6, architecture §1.2, §3, §4

### What to Check

#### 16.1 No Stubs Remaining

- [ ] Verify no `"not yet implemented"` or `// TODO: implement` comments in any source file
- [ ] Only `PreviewOrder` returns "not supported" (this is by design — Schwab API limitation, per FR-7.3)
- [ ] `GetNextMarketDate` has a known limitation (no holidays) — documented, not a stub

#### 16.2 Logging Conventions

- [ ] All logging uses `slog` (no `log.Printf`, no emoji-prefixed `log.*`)
- [ ] Logger created with `"provider"="schwab"` attribute
- [ ] Log levels: INFO (token refresh, connect/disconnect), WARN (429, near-expiry), ERROR (failed calls, auth failures), DEBUG (raw responses)

#### 16.3 Concurrency Safety

- [ ] `tokenMu sync.Mutex` protects `accessToken`, `tokenExpiry` (auth.go, helpers.go, streaming.go)
- [ ] `streamMu sync.RWMutex` protects `streamConn`, `IsConnected`, streaming state (streaming.go)
- [ ] `acctStreamMu sync.RWMutex` protects `acctStreamActive`, `orderEventCallback` (account_stream.go)
- [ ] `rateLimiter.mu sync.Mutex` protects token bucket state (rate_limiter.go)
- [ ] No data races: all mutable state accessed under appropriate locks

#### 16.4 Error Handling

- [ ] All errors prefixed with `"schwab: "` for identification
- [ ] Uses `fmt.Errorf("schwab: ...: %w", err)` for error wrapping
- [ ] `ErrRefreshTokenExpired` sentinel error for expired refresh tokens
- [ ] Authentication errors clearly distinguishable from other errors

#### 16.5 Interface Satisfaction

- [ ] Compile-time check: `var _ base.Provider = (*SchwabProvider)(nil)` in schwab.go
- [ ] All 28 `base.Provider` interface methods implemented
- [ ] `EnsureHealthyConnection` duck-typed interface implemented (streaming.go)

#### 16.6 File Organization

- [ ] Multi-file split matches architecture §3.2:
  | File | Purpose | Present |
  |------|---------|---------|
  | `schwab.go` | Struct, constructor, TestCredentials | ✅ |
  | `auth.go` | Token management | ✅ |
  | `market_data.go` | Quotes, chains, history | ✅ |
  | `account.go` | Account, positions | ✅ |
  | `orders.go` | Order management | ✅ |
  | `streaming.go` | WebSocket streaming | ✅ |
  | `account_stream.go` | Account event streaming | ✅ |
  | `symbols.go` | Symbol conversion | ✅ |
  | `field_maps.go` | Field key mappings | ✅ |
  | `helpers.go` | HTTP helpers | ✅ |
  | `rate_limiter.go` | Rate limiter | ✅ |

### How to Verify

```bash
# Check for stubs/TODOs
grep -rn "TODO\|FIXME\|not yet implemented\|not implemented\|stub" trade-backend-go/internal/providers/schwab/*.go

# Check for log.Printf usage (should be zero — only slog)
grep -rn "log\.Printf\|log\.Println\|log\.Fatal" trade-backend-go/internal/providers/schwab/*.go

# Check for proper error prefixing
grep -rn 'fmt\.Errorf' trade-backend-go/internal/providers/schwab/*.go | head -20

# Verify interface satisfaction
grep -rn "var _ base.Provider" trade-backend-go/internal/providers/schwab/*.go

# Check mutex usage
grep -rn "sync\.\(Mutex\|RWMutex\)" trade-backend-go/internal/providers/schwab/*.go

# Run go vet
cd trade-backend-go && go vet ./internal/providers/schwab/...
```

---

## 17. Regression Check (AC-26)

**Requirements:** AC-26

### What to Run

```bash
# Full Go backend test suite
cd trade-backend-go && go test ./... -count=1

# Go vet across entire backend
cd trade-backend-go && go vet ./...

# Schwab provider tests only (verbose)
cd trade-backend-go && go test ./internal/providers/schwab/... -v -count=1
```

### Expected Results

- [ ] All existing tests in other packages pass (`handlers`, `auth`, `indicators`, `streaming`)
- [ ] All 182 Schwab provider tests pass
- [ ] `go vet` produces no warnings
- [ ] No compilation errors introduced in any package

### Current Baseline (verified 2026-03-17)

| Package | Status |
|---------|--------|
| `internal/api/handlers` | PASS |
| `internal/auth` | PASS |
| `internal/automation/indicators` | PASS |
| `internal/providers/schwab` | PASS (182 tests, 62.9s) |
| `internal/streaming` | PASS |
| `go vet ./...` | Clean (no output) |

---

## 18. QA Additional Edge Case Tests

The following tests should be written to cover gaps not addressed by the existing developer test suite. These target specific edge cases, error boundaries, and concurrency scenarios.

### 18.1 Empty/Malformed JSON Responses

**File:** `market_data_test.go`, `account_test.go`, `orders_test.go`

| Test | Description | Priority |
|------|-------------|----------|
| `TestGetStockQuote_MalformedJSON` | API returns `200` with invalid JSON body — verify graceful error, not panic | High |
| `TestGetAccount_MalformedJSON` | Account endpoint returns corrupted JSON | High |
| `TestGetOrders_MalformedJSON` | Orders endpoint returns corrupted JSON | High |
| `TestGetOptionsChainBasic_APIError` | Options chain returns 500 — verify error propagation | Medium |
| `TestGetHistoricalBars_APIError` | Historical bars returns 404 — verify error message | Medium |
| `TestGetPositions_MalformedPosition` | Position with missing `instrument` key — no panic | Medium |
| `TestTransformSchwabQuote_NullValues` | Quote with JSON `null` for price fields — nil pointers, no crash | Medium |

### 18.2 Concurrent Token Refresh

**File:** `auth_test.go`

| Test | Description | Priority |
|------|-------------|----------|
| `TestEnsureValidToken_ConcurrentRefresh` | Launch 20 goroutines calling `ensureValidToken()` with expired token — verify only 1 HTTP refresh call made, all goroutines get the new token | High |
| `TestEnsureValidToken_ConcurrentWithAPICall` | One goroutine refreshes token while another makes an API call — verify no data race (run with `-race` flag) | High |
| `TestRefreshAccessToken_NetworkTimeout` | Token endpoint takes >30s — verify timeout error, not hang | Medium |

**Implementation guidance:**
```go
func TestEnsureValidToken_ConcurrentRefresh(t *testing.T) {
    var refreshCount atomic.Int32
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        refreshCount.Add(1)
        time.Sleep(100 * time.Millisecond) // Simulate latency
        json.NewEncoder(w).Encode(map[string]interface{}{
            "access_token": "new-token",
            "expires_in":   1800,
            "token_type":   "Bearer",
        })
    }))
    defer server.Close()

    provider := NewSchwabProvider("key", "secret", "https://127.0.0.1",
        "refresh-token", "hash", server.URL, "live")
    // Token is expired (zero value)

    var wg sync.WaitGroup
    for i := 0; i < 20; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            err := provider.ensureValidToken()
            assert.NoError(t, err)
        }()
    }
    wg.Wait()

    // Mutex should serialize — only 1 refresh (others see the new token)
    assert.Equal(t, int32(1), refreshCount.Load())
}
```

### 18.3 Rate Limiter Boundary Conditions

**File:** `rate_limiter_test.go`

| Test | Description | Priority |
|------|-------------|----------|
| `TestRateLimiter_ExactlyMaxTokens` | Consume exactly 120 tokens (matching production config) — 120th succeeds, 121st blocks | Medium |
| `TestRateLimiter_RefillTiming` | After consuming all tokens, verify first token available after `1/refillRate` seconds | Medium |
| `TestRateLimiter_ZeroMaxTokens` | Create limiter with 0 max tokens — verify `wait()` behavior (should block indefinitely or error) | Low |
| `TestRateLimiter_NegativeTokens` | Verify no negative token state possible under concurrent load | Medium |

### 18.4 Symbol Conversion Edge Cases

**File:** `symbols_test.go`

| Test | Description | Priority |
|------|-------------|----------|
| `TestConvertSchwabOptionToOCC_CorruptSpacePadded` | Wrong number of spaces (`"AAPL 250117C00150000"` — 1 space instead of 2) — verify graceful handling | Medium |
| `TestConvertSchwabOptionToOCC_UnicodeChars` | Symbol with non-ASCII characters — no panic | Low |
| `TestConvertOCCToSchwab_ExactlySixCharUnderlying` | 6-char underlying like `"GOOGL2"` — no padding needed | Medium |
| `TestIsOptionSymbol_NumericOnlySymbol` | `"12345"` or `"123456789012345678"` — verify correct classification | Low |
| `TestConvertDotPrefixToOCC_NoStrike` | `".AAPL250117C"` — missing strike price — verify graceful handling | Medium |
| `TestRoundTrip_AllEdgeCases` | Bulk round-trip test with 20+ symbols covering all underlying lengths (1-6 chars), both C/P, various strike prices | Medium |

### 18.5 Streaming Edge Cases

**File:** `streaming_test.go`

| Test | Description | Priority |
|------|-------------|----------|
| `TestStreamReadLoop_MalformedJSON` | WebSocket sends non-JSON message — verify logged error, no panic, connection continues | High |
| `TestStreamReadLoop_PartialFields` | Data message with only 2 of 15 expected fields — verify partial MarketData dispatched | Medium |
| `TestSubscribeToSymbols_Concurrent` | 5 goroutines subscribe to different symbols simultaneously — no data race, all tracked | Medium |
| `TestProcessStreamData_UnknownFieldKey` | Numerical key not in field map (e.g., `"99"`) — verify silently skipped | Medium |
| `TestConnectStreaming_WebSocketDialFailure` | Unreachable WebSocket URL — verify error after 3 retries | Medium |
| `TestDisconnectStreaming_DuringActiveSubscription` | Disconnect while data is actively streaming — clean shutdown | Medium |

### 18.6 Order Edge Cases

**File:** `orders_test.go`

| Test | Description | Priority |
|------|-------------|----------|
| `TestPlaceOrder_StopOrder` | Stop order type with stop price — verify `STOP` order type and `stopPrice` in JSON | Medium |
| `TestPlaceOrder_StopLimitOrder` | Stop-limit with both prices — verify `STOP_LIMIT` and both prices | Medium |
| `TestPlaceMultiLegOrder_MissingLegs` | `legs` key missing or empty array — verify validation error | Medium |
| `TestPlaceMultiLegOrder_InvalidLeg` | Leg with missing symbol — verify validation error | Medium |
| `TestGetOrders_FilledFilter` | `"filled"` filter sends `status=FILLED` param | Low |
| `TestGetOrders_CanceledFilter` | `"canceled"` filter sends `status=CANCELED` param | Low |
| `TestPreviewOrder_ReturnsNotSupported` | Verify exact error message for PreviewOrder | Low |

### 18.7 Account Edge Cases

**File:** `account_test.go`

| Test | Description | Priority |
|------|-------------|----------|
| `TestGetPositions_ShortPosition` | Position with `shortQuantity > longQuantity` — verify negative net quantity and `"short"` side | Medium |
| `TestGetPositions_MixedLongShort` | Position with both `longQuantity` and `shortQuantity` — verify net calculation | Medium |
| `TestGetAccount_EmptyBalances` | Account with empty `currentBalances` object — verify all balance fields default to 0 | Medium |
| `TestGetPositionsEnhanced_GroupingVerification` | Verify option positions grouped by underlying with correct hierarchy | Low |

### 18.8 Account Streaming Edge Cases

**File:** `account_stream_test.go`

| Test | Description | Priority |
|------|-------------|----------|
| `TestProcessAccountActivity_CanceledOrder` | CANCELED status event — verify `"canceled"` mapped correctly | Medium |
| `TestProcessAccountActivity_RejectedOrder` | REJECTED status event — verify `"rejected"` mapped correctly | Medium |
| `TestProcessAccountActivity_OptionSymbolConversion` | Option symbol in account activity — verify OCC conversion | Medium |
| `TestProcessAccountActivity_HighFrequencyBurst` | 100 events in rapid succession — all dispatched, no drops | Low |

### Running the Additional Tests

After implementing the edge case tests:

```bash
# Run all new edge case tests
cd trade-backend-go && go test ./internal/providers/schwab/... -run "TestGetStockQuote_MalformedJSON|TestGetAccount_MalformedJSON|TestGetOrders_MalformedJSON|TestEnsureValidToken_Concurrent|TestRateLimiter_Exactly|TestConvertSchwabOptionToOCC_Corrupt|TestStreamReadLoop_MalformedJSON|TestPlaceOrder_Stop|TestGetPositions_Short|TestProcessAccountActivity_Canceled" -v

# Run with race detector
cd trade-backend-go && go test ./internal/providers/schwab/... -race -count=1

# Run all tests to confirm no regressions
cd trade-backend-go && go test ./internal/providers/schwab/... -v -count=1
```

### Priority Summary for Additional Tests

| Priority | Count | Focus Area |
|----------|-------|------------|
| **High** | 6 | Malformed JSON responses (3), concurrent token refresh (2), malformed WebSocket (1) |
| **Medium** | 22 | Rate limiter boundaries, symbol edge cases, streaming partial data, order types, position edge cases, account stream events |
| **Low** | 7 | Unicode symbols, zero-config limiter, numeric-only symbols, order filter params, grouping verification, high-frequency bursts |

---

## Appendix: Test File Cross-Reference

| Source File | Test File | Lines | Tests | Coverage Areas |
|-------------|-----------|-------|-------|----------------|
| `schwab.go` (206 lines) | `schwab_test.go` | 343 | 10 | Constructor, TestCredentials, Ping |
| `auth.go` (126 lines) | `auth_test.go` | 376 | 9 | Token refresh, expiry buffer, sentinel errors |
| `market_data.go` (868 lines) | `market_data_test.go` | 1,451 | 26 | Quotes, chains, Greeks, history, lookup |
| `account.go` (296 lines) | `account_test.go` | 373 | 7 | Account, positions, OCC parsing |
| `orders.go` (684 lines) | `orders_test.go` | 871 | 17 | Orders, placement, cancellation, mapping |
| `streaming.go` (966 lines) | `streaming_test.go` | 1,665 | 23 | Connect, subscribe, field decode, reconnect |
| `account_stream.go` (287 lines) | `account_stream_test.go` | 806 | 14 | Account events, callback, routing |
| `symbols.go` (194 lines) | `symbols_test.go` | 279 | 10 | OCC↔Schwab conversion, classification |
| `helpers.go` (178 lines) | `helpers_test.go` | 418 | 13 | URL builders, error parsing, 401 retry |
| `rate_limiter.go` (90 lines) | `rate_limiter_test.go` | 208 | 8 | Token bucket, concurrency, refill |
| `field_maps.go` (148 lines) | *(none)* | — | — | Data-only; exercised via streaming tests |
| **Total** | | **5,790** | **137 (+45 sub-tests)** | |

## Appendix: Acceptance Criteria Traceability

| AC | Description | Test Coverage |
|----|-------------|---------------|
| AC-1 | Provider type in API response | §1 — manual + `TestNewSchwabProvider` |
| AC-3 | TestCredentials validates valid creds | §2 — `TestTestCredentials_Success` |
| AC-4 | TestCredentials rejects invalid creds | §2 — `TestTestCredentials_InvalidCredentials` |
| AC-5 | GetStockQuote returns valid quote | §6 — `TestGetStockQuote_Success` |
| AC-6 | GetStockQuotes returns multiple | §6 — `TestGetStockQuotes_MultipleSymbols` |
| AC-7 | GetExpirationDates returns dates + DTE | §7 — `TestGetExpirationDates_Success` |
| AC-8 | GetOptionsChainBasic without Greeks | §7 — `TestGetOptionsChainBasic_Success` |
| AC-9 | GetOptionsChainSmart with Greeks | §7 — `TestGetOptionsChainSmart_WithGreeks` |
| AC-10 | GetOptionsGreeksBatch returns Greeks | §7 — `TestGetOptionsGreeksBatch_Success` |
| AC-11 | GetHistoricalBars returns OHLCV | §8 — `TestGetHistoricalBars_Daily` |
| AC-12 | GetAccount returns balances | §10 — `TestGetAccount_Success` |
| AC-13 | GetPositions with asset class | §10 — `TestGetPositions_Success` |
| AC-14 | GetPositionsEnhanced hierarchical | §10 — `TestGetPositionsEnhanced_Success` |
| AC-15 | GetOrders filtered by status | §11 — `TestGetOrders_Success`, `_OpenFilter` |
| AC-16 | PlaceOrder sends correct JSON | §11 — `TestPlaceOrder_EquityBuy` |
| AC-17 | PlaceMultiLegOrder multi-leg JSON | §11 — `TestPlaceMultiLegOrder_VerticalSpread` |
| AC-18 | CancelOrder sends DELETE | §11 — `TestCancelOrder_Success` |
| AC-19 | LookupSymbols returns matches | §9 — `TestLookupSymbols_Success` |
| AC-20 | ConnectStreaming establishes WS | §12 — `TestConnectStreaming_Success` |
| AC-21 | Equity streaming produces MarketData | §13 — `TestProcessStreamData_EquityFields` |
| AC-22 | Option streaming produces Greeks | §13 — `TestProcessStreamData_OptionFieldsWithGreeks` |
| AC-23 | Auto token refresh | §2 — `TestEnsureValidToken_ExpiredToken` |
| AC-24 | Symbol conversion bidirectional | §5 — `TestConvert*`, `TestConvertRoundTrip_*` |
| AC-26 | No regressions | §17 — `go test ./...` |

---

*End of test plan.*
