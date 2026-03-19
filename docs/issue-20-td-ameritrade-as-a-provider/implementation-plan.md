# Schwab Provider — Step-by-Step Implementation Plan

> Derived from the [architecture document](./architecture.md) Section 14, cross-referenced
> with the existing `base.Provider` interface (809 lines, 30+ methods), TastyTrade
> constructor pattern, `provider_types.go` registration, and `manager.go` factory switch.
>
> Each step produces 1–3 source files and is independently committable.
> Steps within a phase are sequential; Phases 2–4 can proceed in parallel after Phase 1.

---

## Phase 1: Foundation (8 steps)

### Step 1.1 — Provider struct & constructor (`schwab.go`)

**Files:** `trade-backend-go/internal/providers/schwab/schwab.go`

Create the `schwab` package and define:

- `SchwabProvider` struct embedding `*base.BaseProviderImpl` with fields:
  - `appKey`, `appSecret` (OAuth client credentials)
  - `refreshToken` (long-lived refresh token from Schwab developer portal)
  - `accessToken`, `tokenExpires *time.Time` (short-lived access token state)
  - `accountHash` (encrypted account ID returned by Schwab API)
  - `accountType` (`"live"` or `"paper"`)
  - `baseURL` (default `https://api.schwabapi.com`)
  - `tokenURL` (default `https://api.schwabapi.com/v1/oauth/token`)
  - `httpClient *utils.HTTPClient`
  - `tokenMu sync.Mutex` (protects token refresh)
  - `logger *slog.Logger`
  - Streaming fields (placeholders): `streamConn *websocket.Conn`, `streamStopChan`, `streamDoneChan`, `streamSocketURL`, `streamCustomerID`, `streamCorrelID`
  - Account stream fields: `acctStreamMu sync.RWMutex`, `acctStreamActive bool`, `orderEventCallback func(*models.OrderEvent)`
- `NewSchwabProvider(appKey, appSecret, refreshToken, accountHash, baseURL, accountType string) *SchwabProvider` constructor (follow TastyTrade pattern: set defaults, return pointer)
- Stub implementations for **all 30+ `base.Provider` interface methods** that return `fmt.Errorf("schwab: not yet implemented")` — this ensures the package compiles and satisfies the interface from step 1.

**Tests:** None yet (stubs only).

**Verify:** `go build ./internal/providers/schwab/...` compiles. Interface satisfaction check: `var _ base.Provider = (*SchwabProvider)(nil)`.

---

### Step 1.2 — OAuth token management (`auth.go`)

**Files:** `trade-backend-go/internal/providers/schwab/auth.go`

Implement:

- `refreshAccessToken(ctx context.Context) error` — POST to `tokenURL` with `grant_type=refresh_token`, Basic auth header (`base64(appKey:appSecret)`), parse JSON response for `access_token`, `expires_in`, `refresh_token`. Store on struct. Wrap errors with `"schwab: "` prefix.
- `ensureValidToken(ctx context.Context) error` — check `accessToken != ""` and `tokenExpires` is >5 minutes away; if not, call `refreshAccessToken()`. Protected by `tokenMu`.
- `ErrRefreshTokenExpired` sentinel error.

**Tests:** `trade-backend-go/internal/providers/schwab/auth_test.go`
- Table-driven tests with `httptest.Server` mocking the token endpoint.
- Cases: successful refresh, expired refresh token (401), malformed JSON, missing access_token, token near-expiry triggers refresh.

**Verify:** `go test ./internal/providers/schwab/... -run TestAuth -v`

---

### Step 1.3 — HTTP helpers & error parsing (`helpers.go`)

**Files:** `trade-backend-go/internal/providers/schwab/helpers.go`

Implement:

- `doAuthenticatedRequest(ctx context.Context, method, url string, body interface{}) ([]byte, int, error)` — calls `ensureValidToken()`, sets `Authorization: Bearer <token>`, `Accept: application/json`, Content-Type for POST/PUT. Returns raw response body + status code. On 401, attempts one token refresh and retries.
- `buildMarketDataURL(path string) string` — constructs `baseURL + "/marketdata/v1" + path`
- `buildTraderURL(path string) string` — constructs `baseURL + "/trader/v1" + path`
- `parseErrorResponse(body []byte, statusCode int) error` — handles three Schwab error formats: OAuth (`{"error":...}`), API (`{"errors":[...]}`), plain text. Returns descriptive `error` with `"schwab: "` prefix.

**Tests:** `trade-backend-go/internal/providers/schwab/helpers_test.go`
- Test `parseErrorResponse` with all three error formats.
- Test URL builders.
- Test `doAuthenticatedRequest` with mock server (success, 401 retry, 429, 500).

**Verify:** `go test ./internal/providers/schwab/... -run TestHelpers -v`

---

### Step 1.4 — Rate limiter (`rate_limiter.go`)

**Files:** `trade-backend-go/internal/providers/schwab/rate_limiter.go`

Implement:

- `rateLimiter` struct — token bucket: 120 requests/minute capacity, refill rate of 2 tokens/second.
- `newRateLimiter() *rateLimiter`
- `Wait(ctx context.Context) error` — blocks until a token is available or ctx is cancelled.
- `Allow() bool` — non-blocking check.
- Integrate into `doAuthenticatedRequest()` (call `rateLimiter.Wait()` before every HTTP request).

**Tests:** `trade-backend-go/internal/providers/schwab/rate_limiter_test.go`
- Test token refill timing.
- Test burst capacity.
- Test context cancellation.

**Verify:** `go test ./internal/providers/schwab/... -run TestRateLimiter -v`

---

### Step 1.5 — Symbol conversion (`symbols.go`)

**Files:** `trade-backend-go/internal/providers/schwab/symbols.go`

Implement:

- `convertOCCToSchwab(occSymbol string) string` — converts OCC format `AAPL  250620C00150000` → Schwab format `AAPL  250620C150` (strip leading zeros from strike, remove trailing zeros).
- `convertSchwabOptionToOCC(schwabSymbol string) string` — reverse conversion.
- `classifySymbols(symbols []string) (equities, options []string)` — splits a list of symbols into equities and options based on format detection.
- `isOptionSymbol(symbol string) bool` — helper for classification.

**Tests:** `trade-backend-go/internal/providers/schwab/symbols_test.go`
- Table-driven tests with edge cases: single-char underlying (F), long underlying (BRKB), strike prices <$1, strikes >$1000, weekly option symbols (SPXW), calls and puts.

**Verify:** `go test ./internal/providers/schwab/... -run TestSymbol -v`

---

### Step 1.6 — Streaming field maps (`field_maps.go`)

**Files:** `trade-backend-go/internal/providers/schwab/field_maps.go`

Define:

- `equityFieldMap map[int]string` — 52 entries (indices 0–51) mapping numerical field IDs to names (SYMBOL, BID_PRICE, ASK_PRICE, LAST_PRICE, etc.)
- `optionFieldMap map[int]string` — 55 entries (indices 0–54) including Greeks (DELTA, GAMMA, THETA, VEGA, RHO)
- `equitySubscriptionFields string` — `"0,1,2,3,4,5,8,10,11,12,17,18,33,34,35"`
- `optionSubscriptionFields string` — `"0,2,3,4,5,6,7,8,9,10,12,15,19,21,22,23,26,28,29,30,31,32,35,37"`

**Tests:** None needed (constant definitions). Verified by compilation.

**Verify:** `go build ./internal/providers/schwab/...`

---

### Step 1.7 — Provider type registration (`provider_types.go`, `manager.go`)

**Files modified:**
- `trade-backend-go/internal/providers/provider_types.go`
- `trade-backend-go/internal/providers/manager.go`

Changes to `provider_types.go`:
- Add `"schwab"` entry to `ProviderTypes` map with:
  - Name: `"Schwab"`
  - Description: `"Charles Schwab Trading API"`
  - SupportsAccountTypes: `["live", "paper"]`
  - Capabilities: rest = `["expiration_dates", "stock_quotes", "options_chain", "trade_account", "next_market_date", "symbol_lookup", "historical_data", "market_calendar", "greeks"]`, streaming = `["streaming_quotes", "trade_account", "streaming_greeks"]`
  - CredentialFields for `"live"` and `"paper"`: `app_key` (text, required), `app_secret` (password, required), `refresh_token` (password, required), `account_hash` (text, required), `base_url` (text, optional, default `https://api.schwabapi.com`)
- Add `"schwab"` and `"schwab_paper"` entries to `LegacyProviderCapabilities`.

Changes to `manager.go`:
- Add import: `"trade-backend-go/internal/providers/schwab"`
- Add `case "schwab":` in `createProviderInstance()` switch — extract credentials and call `schwab.NewSchwabProvider(...)`.

**Tests:** None (registration only). Verified by existing `go test ./...`.

**Verify:** `go build ./...` compiles. `go test ./...` passes (no regressions).

---

### Step 1.8 — TestCredentials & HealthCheck (`schwab.go` update)

**Files:** `trade-backend-go/internal/providers/schwab/schwab.go` (update stubs)

Replace stub implementations for:

- `TestCredentials(ctx) (map[string]interface{}, error)` — call `ensureValidToken()`, then `GET /trader/v1/accounts` to verify credentials work. Return `{"success": true, "message": "..."}` with account info. If `accountType == "paper"`, append sandbox warning.
- `HealthCheck(ctx) (map[string]interface{}, error)` — return base health info + token expiry status + streaming connection status.
- `Ping(ctx) error` — call `ensureValidToken()` as a lightweight health check.

**Tests:** `trade-backend-go/internal/providers/schwab/schwab_test.go`
- Test `TestCredentials` with mock server returning account data.
- Test `TestCredentials` with paper account type includes warning.
- Test `TestCredentials` with invalid credentials returns error.

**Verify:** `go test ./internal/providers/schwab/... -run TestCredentials -v` and `go test ./...`

---

## Phase 2: Market Data (4 steps)

> **Prerequisite:** Phase 1 complete.

### Step 2.1 — Stock quotes (`market_data.go` — part 1)

**Files:** `trade-backend-go/internal/providers/schwab/market_data.go`

Implement:

- `GetStockQuote(ctx, symbol) (*models.StockQuote, error)` — `GET /marketdata/v1/quotes?symbols={symbol}&fields=quote`. Transform Schwab response to `models.StockQuote`.
- `GetStockQuotes(ctx, symbols) (map[string]*models.StockQuote, error)` — batch version, comma-joined symbols.

**Tests:** `trade-backend-go/internal/providers/schwab/market_data_test.go` (part 1)
- Mock Schwab `/quotes` response JSON.
- Table-driven: single symbol, multiple symbols, empty response, partial data.

**Verify:** `go test ./internal/providers/schwab/... -run TestStockQuote -v`

---

### Step 2.2 — Options chain & Greeks (`market_data.go` — part 2)

**Files:** `trade-backend-go/internal/providers/schwab/market_data.go` (append)

Implement:

- `GetExpirationDates(ctx, symbol) ([]map[string]interface{}, error)` — `GET /marketdata/v1/expirationchain?symbol={symbol}`. Transform to enhanced expiration structure.
- `GetOptionsChainBasic(ctx, symbol, expiry, underlyingPrice, strikeCount, optionType, underlyingSymbol) ([]*models.OptionContract, error)` — `GET /marketdata/v1/chains?symbol={symbol}&...`. Transform Schwab chain response (nested call/put maps) to flat `[]*models.OptionContract`. Convert Schwab option symbols to OCC format.
- `GetOptionsChainSmart(ctx, symbol, expiry, underlyingPrice, atmRange, includeGreeks, strikesOnly) ([]*models.OptionContract, error)` — same endpoint with different parameters for ATM filtering.
- `GetOptionsGreeksBatch(ctx, optionSymbols) (map[string]map[string]interface{}, error)` — `GET /marketdata/v1/quotes?symbols={schwabSymbols}&fields=quote`. Extract Greeks from quote response.

**Tests:** `trade-backend-go/internal/providers/schwab/market_data_test.go` (append)
- Mock Schwab chain response with calls and puts.
- Test strike filtering, ATM range logic.
- Test symbol conversion within chain response.
- Test Greeks extraction.

**Verify:** `go test ./internal/providers/schwab/... -run TestOptions -v`

---

### Step 2.3 — Historical data & market calendar (`market_data.go` — part 3)

**Files:** `trade-backend-go/internal/providers/schwab/market_data.go` (append)

Implement:

- `GetHistoricalBars(ctx, symbol, timeframe, startDate, endDate, limit) ([]map[string]interface{}, error)` — `GET /marketdata/v1/pricehistory?symbol={symbol}&periodType=...&frequencyType=...`. Map JuicyTrade timeframe strings to Schwab period/frequency params. Transform candle array.
- `GetNextMarketDate(ctx) (string, error)` — `GET /marketdata/v1/markets?markets=equity`. Parse market hours response to find next open date. Fallback: simple business-day calculation skipping weekends.

**Tests:** `trade-backend-go/internal/providers/schwab/market_data_test.go` (append)
- Mock historical bars response.
- Test timeframe mapping (1min, 5min, 1hour, 1day, 1week).
- Test next market date logic (weekday, Friday→Monday, holiday handling).

**Verify:** `go test ./internal/providers/schwab/... -run "TestHistorical|TestMarketDate" -v`

---

### Step 2.4 — Symbol lookup (`market_data.go` — part 4)

**Files:** `trade-backend-go/internal/providers/schwab/market_data.go` (append)

Implement:

- `LookupSymbols(ctx, query) ([]*models.SymbolSearchResult, error)` — `GET /marketdata/v1/instruments?symbol={query}&projection=symbol-search`. Transform to `[]*models.SymbolSearchResult`.

**Tests:** `trade-backend-go/internal/providers/schwab/market_data_test.go` (append)
- Mock instruments response.
- Test partial match, exact match, no results.

**Verify:** `go test ./internal/providers/schwab/... -run TestLookup -v`

---

## Phase 3: Account & Orders (3 steps)

> **Prerequisite:** Phase 1 complete. Can proceed in parallel with Phases 2 and 4.

### Step 3.1 — Account info & positions (`account.go`)

**Files:** `trade-backend-go/internal/providers/schwab/account.go`

Implement:

- `GetAccount(ctx) (*models.Account, error)` — `GET /trader/v1/accounts/{accountHash}?fields=positions`. Transform `securitiesAccount` block to `models.Account`. Map Schwab balance fields (`liquidationValue` → `portfolio_value`, `buyingPower`, `cashBalance`, etc.).
- `GetPositions(ctx) ([]*models.Position, error)` — same endpoint, transform `positions[]` array. For each position: detect asset class (`"EQUITY"` → `"us_equity"`, `"OPTION"` → `"us_option"`), extract option-specific fields (underlying, strike, expiry, type), convert Schwab option symbols to OCC.
- `GetPositionsEnhanced(ctx) (*models.EnhancedPositionsResponse, error)` — call `GetPositions()` then use inherited `BaseProviderImpl.ConvertPositionsToEnhanced()`.

**Tests:** `trade-backend-go/internal/providers/schwab/account_test.go`
- Mock account response with balances.
- Mock positions with equity + option positions.
- Test symbol conversion in positions.
- Test enhanced positions grouping.

**Verify:** `go test ./internal/providers/schwab/... -run "TestAccount|TestPosition" -v`

---

### Step 3.2 — Order retrieval & cancellation (`orders.go` — part 1)

**Files:** `trade-backend-go/internal/providers/schwab/orders.go`

Implement:

- `GetOrders(ctx, status) ([]*models.Order, error)` — `GET /trader/v1/accounts/{accountHash}/orders?...`. Map Schwab status values to normalized statuses. Map Schwab order structure (which nests `orderLegCollection[]`) to flat `models.Order` with `Legs`.
- `CancelOrder(ctx, orderID) (bool, error)` — `DELETE /trader/v1/accounts/{accountHash}/orders/{orderID}`. Return `(true, nil)` on 200/204.

**Tests:** `trade-backend-go/internal/providers/schwab/orders_test.go`
- Mock orders response with single-leg and multi-leg orders.
- Test status mapping (FILLED, CANCELED, WORKING, etc.).
- Test cancel order success and failure.

**Verify:** `go test ./internal/providers/schwab/... -run "TestOrders|TestCancel" -v`

---

### Step 3.3 — Place orders & preview (`orders.go` — part 2)

**Files:** `trade-backend-go/internal/providers/schwab/orders.go` (append)

Implement:

- `PlaceOrder(ctx, orderData) (*models.Order, error)` — `POST /trader/v1/accounts/{accountHash}/orders`. Build Schwab order JSON from generic `orderData` map. Handle `orderType` mapping (market/limit/stop/stop_limit), `timeInForce` (DAY/GTC), `instruction` (BUY/SELL/BUY_TO_OPEN/etc.), `assetType` (EQUITY/OPTION). Extract order ID from `Location` response header.
- `PlaceMultiLegOrder(ctx, orderData) (*models.Order, error)` — same endpoint with `orderStrategyType: "SINGLE"` or multi-leg with `orderLegCollection[]`. Build complex leg array from `orderData["legs"]`.
- `PreviewOrder(ctx, orderData) (map[string]interface{}, error)` — return `fmt.Errorf("schwab: order preview not supported by Schwab API")`.

**Tests:** `trade-backend-go/internal/providers/schwab/orders_test.go` (append)
- Test single equity order JSON construction.
- Test single option order JSON construction (BUY_TO_OPEN, SELL_TO_CLOSE).
- Test multi-leg order (vertical spread) JSON construction.
- Test PreviewOrder returns expected error.
- Mock POST response with Location header parsing.

**Verify:** `go test ./internal/providers/schwab/... -run "TestPlace|TestPreview" -v`

---

## Phase 4: Streaming (3 steps)

> **Prerequisite:** Phase 1 complete. Can proceed in parallel with Phases 2 and 3.

### Step 4.1 — Connect, login, disconnect (`streaming.go` — part 1)

**Files:** `trade-backend-go/internal/providers/schwab/streaming.go`

Implement:

- `ConnectStreaming(ctx) (bool, error)` — full lifecycle:
  1. `GET /trader/v1/userPreference` → extract `streamerInfo` (WebSocket URL, customer ID, correlation ID)
  2. Dial WebSocket with `gorilla/websocket` (3 retries, exponential backoff 1s/2s/4s)
  3. Send LOGIN request JSON with access token
  4. Read LOGIN response, verify `code == 0`
  5. Launch `go streamReadLoop()`
  6. Set `IsConnected = true`
- `DisconnectStreaming(ctx) (bool, error)` — close `streamStopChan`, wait for `streamDoneChan`, send LOGOUT (best-effort), close WebSocket, set `IsConnected = false`, clear `SubscribedSymbols`.
- Schwab-specific stream message structs: `schwabStreamRequest`, `schwabStreamResponse`, `schwabStreamDataItem`, `schwabStreamNotify`.

**Tests:** `trade-backend-go/internal/providers/schwab/streaming_test.go` (part 1)
- Mock WebSocket server (use `httptest` + `gorilla/websocket.Upgrader`).
- Test successful connect + LOGIN handshake.
- Test LOGIN failure (code != 0).
- Test disconnect cleanup.

**Verify:** `go test ./internal/providers/schwab/... -run "TestConnect|TestDisconnect" -v`

---

### Step 4.2 — Subscribe, read loop, field decoding (`streaming.go` — part 2)

**Files:** `trade-backend-go/internal/providers/schwab/streaming.go` (append)

Implement:

- `SubscribeToSymbols(ctx, symbols, dataTypes) (bool, error)` — classify into equities/options, convert option symbols to Schwab format, send SUBS messages. Batch in groups of 50 with 100ms delay.
- `UnsubscribeFromSymbols(ctx, symbols, dataTypes) (bool, error)` — send UNSUBS messages, remove from `SubscribedSymbols`.
- `streamReadLoop()` — goroutine: set 120s read deadline, read messages, dispatch to `processStreamData()` / handle heartbeats / handle response confirmations.
- `processStreamData(data)` — decode numerical field keys using `equityFieldMap` / `optionFieldMap`, build and dispatch `models.MarketData` to `StreamingQueue` and `StreamingCache`.
- `dispatchMarketData(service, symbol, decoded)` — convert decoded fields to `models.MarketData`, send to queue/cache.

**Tests:** `trade-backend-go/internal/providers/schwab/streaming_test.go` (append)
- Test `processStreamData` with mock equity data (numerical keys → named fields).
- Test `processStreamData` with mock option data including Greeks.
- Test `classifySymbols` integration in `SubscribeToSymbols`.
- Test batching (>50 symbols).
- Test `dispatchMarketData` sends to queue.

**Verify:** `go test ./internal/providers/schwab/... -run "TestSubscribe|TestStreamData|TestDispatch" -v`

---

### Step 4.3 — Reconnection logic (`streaming.go` — part 3)

**Files:** `trade-backend-go/internal/providers/schwab/streaming.go` (append)

Implement:

- `handleStreamDisconnect()` — set `IsConnected = false`, attempt reconnection.
- Reconnection loop: exponential backoff (5s, 10s, 20s, 40s, 60s), max 5 retries. On reconnect, re-subscribe all symbols in `SubscribedSymbols`.
- Optional `EnsureHealthyConnection(ctx) error` — type-asserted by streaming manager for proactive health checks.

**Tests:** `trade-backend-go/internal/providers/schwab/streaming_test.go` (append)
- Test reconnection triggers after read error.
- Test reconnection re-subscribes existing symbols.
- Test max retry exhaustion.

**Verify:** `go test ./internal/providers/schwab/... -run TestReconnect -v`

---

## Phase 5: Account Streaming & Polish (2 steps)

> **Prerequisite:** Phase 4 complete.

### Step 5.1 — Account event streaming (`account_stream.go`)

**Files:** `trade-backend-go/internal/providers/schwab/account_stream.go`

Implement:

- `StartAccountStream(ctx) error` — if not connected, call `ConnectStreaming()` first. Send SUBS for `ACCT_ACTIVITY` service (no keys/fields). Set `acctStreamActive = true`.
- `StopAccountStream()` — send UNSUBS for `ACCT_ACTIVITY`. Set `acctStreamActive = false`.
- `SetOrderEventCallback(callback func(*models.OrderEvent))` — store callback under `acctStreamMu`.
- `IsAccountStreamConnected() bool` — return `acctStreamActive && IsConnected`.
- `processAccountActivity(data schwabStreamDataItem)` — parse account activity content into `models.OrderEvent`, invoke callback.

Update `streaming.go`: In `processStreamData()`, route `"ACCT_ACTIVITY"` service to `processAccountActivity()` (this is already stubbed in Step 4.2's code).

**Tests:** `trade-backend-go/internal/providers/schwab/account_stream_test.go`
- Test `StartAccountStream` sends correct subscription.
- Test `processAccountActivity` invokes callback with correct `OrderEvent`.
- Test `StopAccountStream` clears active state.
- Test `IsAccountStreamConnected` reflects both stream and activity state.

**Verify:** `go test ./internal/providers/schwab/... -run TestAccountStream -v`

---

### Step 5.2 — Full integration test & regression check

**Files:** No new files. Final verification step.

Actions:

- Run full test suite: `cd trade-backend-go && go test ./... -v`
- Confirm no regressions in existing provider tests.
- Confirm `go vet ./...` and `go build ./...` pass cleanly.
- Verify Schwab test file count and coverage:
  - `auth_test.go` — token refresh tests
  - `helpers_test.go` — HTTP helper and error parsing tests
  - `rate_limiter_test.go` — rate limiter tests
  - `symbols_test.go` — symbol conversion tests
  - `schwab_test.go` — TestCredentials, HealthCheck tests
  - `market_data_test.go` — all market data transformation tests
  - `account_test.go` — account and position tests
  - `orders_test.go` — order placement, retrieval, cancellation tests
  - `streaming_test.go` — WebSocket connect, subscribe, decode, reconnect tests
  - `account_stream_test.go` — account event streaming tests
- Optionally: Run integration tests with `SCHWAB_INTEGRATION_TEST=true` if real credentials are available.

**Verify:** `go test ./... && go vet ./... && go build ./...`

---

## Summary

| Step | Phase | Files Created/Modified | Description | ~Lines |
|------|-------|-----------------------|-------------|--------|
| 1.1 | 1 | `schwab/schwab.go` | Provider struct, constructor, interface stubs | ~200 |
| 1.2 | 1 | `schwab/auth.go`, `schwab/auth_test.go` | OAuth token refresh, ensureValidToken | ~250 |
| 1.3 | 1 | `schwab/helpers.go`, `schwab/helpers_test.go` | HTTP helpers, error parsing, URL builders | ~250 |
| 1.4 | 1 | `schwab/rate_limiter.go`, `schwab/rate_limiter_test.go` | Token bucket rate limiter | ~150 |
| 1.5 | 1 | `schwab/symbols.go`, `schwab/symbols_test.go` | OCC↔Schwab symbol conversion | ~200 |
| 1.6 | 1 | `schwab/field_maps.go` | Streaming field map constants | ~120 |
| 1.7 | 1 | `provider_types.go` *(mod)*, `manager.go` *(mod)* | Register Schwab provider type + factory case | ~50 |
| 1.8 | 1 | `schwab/schwab.go` *(mod)*, `schwab/schwab_test.go` | TestCredentials, HealthCheck, Ping | ~150 |
| 2.1 | 2 | `schwab/market_data.go`, `schwab/market_data_test.go` | Stock quotes | ~200 |
| 2.2 | 2 | `schwab/market_data.go` *(append)*, tests *(append)* | Options chain, Greeks batch | ~350 |
| 2.3 | 2 | `schwab/market_data.go` *(append)*, tests *(append)* | Historical bars, market calendar | ~200 |
| 2.4 | 2 | `schwab/market_data.go` *(append)*, tests *(append)* | Symbol lookup | ~100 |
| 3.1 | 3 | `schwab/account.go`, `schwab/account_test.go` | Account info, positions, enhanced positions | ~300 |
| 3.2 | 3 | `schwab/orders.go`, `schwab/orders_test.go` | Get orders, cancel order | ~250 |
| 3.3 | 3 | `schwab/orders.go` *(append)*, tests *(append)* | Place order, multi-leg, preview stub | ~250 |
| 4.1 | 4 | `schwab/streaming.go`, `schwab/streaming_test.go` | WebSocket connect, login, disconnect | ~300 |
| 4.2 | 4 | `schwab/streaming.go` *(append)*, tests *(append)* | Subscribe, read loop, field decoding | ~350 |
| 4.3 | 4 | `schwab/streaming.go` *(append)*, tests *(append)* | Reconnection logic | ~150 |
| 5.1 | 5 | `schwab/account_stream.go`, `schwab/account_stream_test.go` | Account event streaming (shared WebSocket) | ~200 |
| 5.2 | 5 | — | Full regression test run | — |
| | | **Total** | | **~3,820** |

### Dependency Graph

```
Step 1.1 → 1.2 → 1.3 → 1.4 → 1.5 → 1.6 → 1.7 → 1.8
                                                    ↓
                              ┌──────────────────────┼──────────────────────┐
                              ↓                      ↓                      ↓
                    2.1 → 2.2 → 2.3 → 2.4    3.1 → 3.2 → 3.3    4.1 → 4.2 → 4.3
                                                                            ↓
                                                                          5.1 → 5.2
```

### File Inventory (new files in `trade-backend-go/internal/providers/schwab/`)

| File | Purpose |
|------|---------|
| `schwab.go` | Provider struct, constructor, TestCredentials, HealthCheck, Ping |
| `auth.go` | OAuth token management (refresh, ensure valid) |
| `helpers.go` | Authenticated HTTP requests, URL builders, error parsing |
| `rate_limiter.go` | Token bucket rate limiter |
| `symbols.go` | OCC ↔ Schwab option symbol conversion |
| `field_maps.go` | Streaming field map constants (equity + option) |
| `market_data.go` | All market data methods (quotes, chains, Greeks, historical, lookup) |
| `account.go` | Account info, positions, enhanced positions |
| `orders.go` | Order retrieval, placement, cancellation, preview |
| `streaming.go` | WebSocket streaming (connect, subscribe, read loop, reconnect) |
| `account_stream.go` | Account event streaming via shared WebSocket |
| `auth_test.go` | Tests for auth.go |
| `helpers_test.go` | Tests for helpers.go |
| `rate_limiter_test.go` | Tests for rate_limiter.go |
| `symbols_test.go` | Tests for symbols.go |
| `schwab_test.go` | Tests for schwab.go (TestCredentials, etc.) |
| `market_data_test.go` | Tests for market_data.go |
| `account_test.go` | Tests for account.go |
| `orders_test.go` | Tests for orders.go |
| `streaming_test.go` | Tests for streaming.go |
| `account_stream_test.go` | Tests for account_stream.go |
