# Requirements: Schwab (formerly TD Ameritrade) as a Provider

**Issue:** [#20 - TD Ameritrade as a Provider](https://github.com/schardosin/juicytrade/issues/20)
**Date:** 2025-07-14
**Status:** Draft — Awaiting Customer Approval

---

## 1. Overview

### 1.1 Summary

Add Charles Schwab as a new broker/data provider in JuicyTrade, implementing the full `base.Provider` interface. This enables users to connect their Schwab brokerage accounts for market data retrieval, options chain analysis, real-time streaming quotes, and order management.

### 1.2 Context & Motivation

The original request references "TD Ameritrade" as a provider. However, **TD Ameritrade's public API was permanently shut down in May 2024** as part of the Charles Schwab acquisition. All TD Ameritrade developer functionality has been migrated to the **Schwab Trader API** (available at `developer.schwab.com`). There is no longer a functioning TD Ameritrade API.

Therefore, this implementation will target the **Schwab Trader API** — which is the direct successor to the TD Ameritrade API and provides equivalent (and in some cases expanded) capabilities for market data, options, streaming, and trading.

Schwab is one of the largest retail brokerages in the US. Supporting it significantly expands JuicyTrade's reach and gives users access to:
- Real-time stock and option quotes via REST and WebSocket streaming
- Full options chain data **with Greeks included in both REST and streaming responses**
- Account management, positions, and order placement (including multi-leg options orders)
- Historical price data for charting
- Symbol search

### 1.3 Important API Characteristics

| Aspect | Detail |
|--------|--------|
| **API Name** | Schwab Trader API (Individual) |
| **Developer Portal** | https://developer.schwab.com |
| **Authentication** | OAuth 2.0 Authorization Code flow (3-legged) |
| **REST Base URL (Production)** | `https://api.schwabapi.com/trader/v1` (accounts/trading) and `https://api.schwabapi.com/marketdata/v1` (market data) |
| **Streaming** | WebSocket-based streaming API (Level One equities, options, futures; OHLCV charts; account activity) |
| **Token Lifetime** | Access token ~30 minutes; Refresh token ~7 days |
| **Account Identifier** | Account hash (not raw account number) — obtained via `/accounts/accountNumbers` endpoint |
| **Greeks Support** | ✅ REST (option chain response includes delta, gamma, theta, vega, rho) and ✅ Streaming (LEVELONE_OPTIONS includes DELTA, GAMMA, THETA, VEGA, RHO fields) |
| **Paper Trading** | Schwab provides a sandbox environment for testing; however, it has known limitations (some asset classes/order types may not work) |

---

## 2. Functional Requirements

### FR-1: Provider Registration & Configuration

**FR-1.1** Register a new provider type `"schwab"` in `provider_types.go` with:
- **Name:** `"Schwab"`
- **Description:** `"Charles Schwab Trader API (formerly TD Ameritrade)"`
- **Supported account types:** `["live"]` (Note: Schwab has a sandbox environment, but it has significant limitations; see Section 6 for discussion on whether to expose it as "paper")
- **REST capabilities:** `stock_quotes`, `options_chain`, `trade_account`, `symbol_lookup`, `historical_data`, `market_calendar`, `greeks`, `expiration_dates`, `next_market_date`
- **Streaming capabilities:** `streaming_quotes`, `streaming_greeks`, `trade_account`

**FR-1.2** Define credential fields for the Schwab provider:

| Field | Label | Type | Required | Default | Notes |
|-------|-------|------|----------|---------|-------|
| `app_key` | App Key | text | Yes | — | Schwab Developer Portal App Key (client_id) |
| `app_secret` | App Secret | password | Yes | — | Schwab Developer Portal Secret (client_secret) |
| `callback_url` | Callback URL | text | Yes | `https://127.0.0.1` | OAuth redirect URI registered with Schwab |
| `refresh_token` | Refresh Token | password | Yes | — | OAuth2 refresh token obtained after initial authorization |
| `account_hash` | Account Hash | text | Yes | — | Account hash value (from `/accounts/accountNumbers`) |
| `base_url` | Base URL | text | No | `https://api.schwabapi.com` | Schwab API base URL |

**FR-1.3** Add the factory case in `manager.go` → `createProviderInstance()` to instantiate the Schwab provider from stored credentials.

**FR-1.4** Add legacy provider capabilities entries for `"schwab"` in `LegacyProviderCapabilities` for backward compatibility.

### FR-2: OAuth 2.0 Authentication & Token Management

**FR-2.1** Implement OAuth 2.0 token exchange using the Authorization Code grant type:
- Token endpoint: `https://api.schwabapi.com/v1/oauth/token`
- Use HTTP Basic authentication with `app_key:app_secret` (Base64 encoded)
- Request body: `application/x-www-form-urlencoded` with `grant_type`, `code` or `refresh_token`, and `redirect_uri`

**FR-2.2** Implement automatic access token refresh:
- Access tokens expire approximately every 30 minutes
- The provider MUST automatically refresh the access token using the stored refresh token before or upon expiration
- If the refresh token is also expired (~7 days), the provider should return a clear error indicating the user must re-authenticate

**FR-2.3** Implement `TestCredentials()`:
- Use the refresh token to obtain an access token
- Make a real API call (e.g., `GET /accounts/accountNumbers`) to validate the credentials
- Return `{"success": true/false, "message": "..."}` following the existing pattern

**FR-2.4** Store and manage tokens in memory during the provider's lifetime. The refresh token is persisted in the credential store; the short-lived access token is managed in-memory with automatic refresh logic.

### FR-3: Market Data — Stock Quotes

**FR-3.1** Implement `GetStockQuote(ctx, symbol)`:
- Endpoint: `GET /marketdata/v1/quotes?symbols={symbol}`
- Transform Schwab quote response to `*models.StockQuote` (Symbol, Ask, Bid, Last, Timestamp)

**FR-3.2** Implement `GetStockQuotes(ctx, symbols)`:
- Endpoint: `GET /marketdata/v1/quotes?symbols={comma-separated symbols}`
- Return `map[string]*models.StockQuote`
- Handle the Schwab quote response format which includes fields like `bidPrice`, `askPrice`, `lastPrice`, `mark`, `totalVolume`, `openPrice`, `highPrice`, `lowPrice`, `closePrice`, `quoteTime`, `tradeTime`, etc.

### FR-4: Market Data — Options Chain

**FR-4.1** Implement `GetExpirationDates(ctx, symbol)`:
- Endpoint: `GET /marketdata/v1/chains?symbol={symbol}`
- Extract unique expiration dates from the option chain response
- Transform to the universal enhanced structure: `[]map[string]interface{}` with date and DTE

**FR-4.2** Implement `GetOptionsChainBasic(ctx, symbol, expiry, underlyingPrice, strikeCount, optionType, underlyingSymbol)`:
- Endpoint: `GET /marketdata/v1/chains?symbol={symbol}&fromDate={expiry}&toDate={expiry}&strikeCount={strikeCount}`
- Transform to `[]*models.OptionContract` without Greeks for fast loading
- Map Schwab option symbol format to standard OCC format

**FR-4.3** Implement `GetOptionsChainSmart(ctx, symbol, expiry, underlyingPrice, atmRange, includeGreeks, strikesOnly)`:
- Same endpoint with additional parameters: `includeUnderlyingQuote=true`
- When `includeGreeks` is true, include delta, gamma, theta, vega from the Schwab response (Schwab option chain responses include Greeks by default)
- Transform to `[]*models.OptionContract` with all fields populated

**FR-4.4** Implement `GetOptionsGreeksBatch(ctx, optionSymbols)`:
- Use `GET /marketdata/v1/quotes?symbols={option-symbols}` to fetch individual option quotes with Greeks
- Return `map[string]map[string]interface{}` with delta, gamma, theta, vega, rho, implied volatility

### FR-5: Market Data — Historical Data & Calendar

**FR-5.1** Implement `GetHistoricalBars(ctx, symbol, timeframe, startDate, endDate, limit)`:
- Endpoint: `GET /marketdata/v1/pricehistory?symbol={symbol}&periodType={type}&frequencyType={freq}&frequency={n}`
- Map JuicyTrade timeframe values to Schwab's periodType/frequencyType/frequency parameters:
  - `"1min"` → frequencyType=minute, frequency=1
  - `"5min"` → frequencyType=minute, frequency=5
  - `"15min"` → frequencyType=minute, frequency=15
  - `"30min"` → frequencyType=minute, frequency=30
  - `"1D"` / `"daily"` → frequencyType=daily, frequency=1
  - `"1W"` / `"weekly"` → frequencyType=weekly, frequency=1
- Transform candle data to `[]map[string]interface{}` with open, high, low, close, volume, timestamp

**FR-5.2** Implement `GetNextMarketDate(ctx)`:
- Schwab API does not have a dedicated market calendar endpoint
- Implement using business-day calculation logic (skip weekends and known US market holidays), consistent with how other providers handle this when a direct endpoint is unavailable
- Return date in `YYYY-MM-DD` format

### FR-6: Account & Portfolio

**FR-6.1** Implement `GetAccount(ctx)`:
- Endpoint: `GET /trader/v1/accounts/{accountHash}?fields=positions`
- Transform to `*models.Account` with accountID, status, currency, buyingPower, cash, portfolioValue, equity, etc.
- Note: Schwab uses account hashes, not raw account numbers

**FR-6.2** Implement `GetPositions(ctx)`:
- Endpoint: `GET /trader/v1/accounts/{accountHash}?fields=positions`
- Extract positions from account response
- Transform each to `*models.Position` with proper asset class detection (`us_equity` vs `us_option`)
- For option positions, parse underlying symbol, option type, strike price, and expiry date

**FR-6.3** Implement `GetPositionsEnhanced(ctx)`:
- Use `GetPositions()` then call `BaseProviderImpl.ConvertPositionsToEnhanced()` for hierarchical grouping

**FR-6.4** Implement `GetOrders(ctx, status)`:
- Endpoint: `GET /trader/v1/accounts/{accountHash}/orders`
- Filter by status parameter mapping: `"all"`, `"open"` → `WORKING`, `"filled"` → `FILLED`, `"canceled"` → `CANCELED`
- Transform to `[]*models.Order` including multi-leg order support

### FR-7: Order Management

**FR-7.1** Implement `PlaceOrder(ctx, orderData)`:
- Endpoint: `POST /trader/v1/accounts/{accountHash}/orders`
- Transform JuicyTrade order format to Schwab JSON order format:
  - `session`: `"NORMAL"`, `"AM"`, `"PM"`, `"SEAMLESS"`
  - `duration`: `"DAY"`, `"GOOD_TILL_CANCEL"`, `"FILL_OR_KILL"`
  - `orderType`: `"MARKET"`, `"LIMIT"`, `"STOP"`, `"STOP_LIMIT"`
  - `orderStrategyType`: `"SINGLE"`
  - `orderLegCollection`: array of `{instruction, quantity, instrument: {symbol, assetType}}`
- Parse the order ID from the `Location` header in the 201 response

**FR-7.2** Implement `PlaceMultiLegOrder(ctx, orderData)`:
- Same endpoint as FR-7.1 but with multiple entries in `orderLegCollection`
- Support `orderStrategyType`: `"SINGLE"`, `"OCO"`, `"TRIGGER"`
- For option spreads, each leg specifies `assetType: "OPTION"` with instruction: `"BUY_TO_OPEN"`, `"SELL_TO_OPEN"`, `"BUY_TO_CLOSE"`, `"SELL_TO_CLOSE"`

**FR-7.3** Implement `PreviewOrder(ctx, orderData)`:
- Schwab API does not have a dedicated order preview endpoint
- Implement by returning a computed estimate based on the order parameters (or return a "not supported" indication)
- This is a known limitation — document it clearly

**FR-7.4** Implement `CancelOrder(ctx, orderID)`:
- Endpoint: `DELETE /trader/v1/accounts/{accountHash}/orders/{orderID}`
- Return `(true, nil)` on successful cancellation

### FR-8: Symbol Search

**FR-8.1** Implement `LookupSymbols(ctx, query)`:
- Endpoint: `GET /marketdata/v1/instruments?symbol={query}&projection=symbol-search`
- Transform to `[]*models.SymbolSearchResult` with symbol, description, exchange, type

### FR-9: Streaming — Real-Time Market Data

**FR-9.1** Implement `ConnectStreaming(ctx)`:
- Obtain streaming connection info from the Schwab user preferences/streaming endpoint
- Connect via WebSocket
- Authenticate the stream session (login message)
- Set `IsConnected = true` on success

**FR-9.2** Implement `SubscribeToSymbols(ctx, symbols, dataTypes)`:
- For equity symbols: subscribe to `LEVELONE_EQUITIES` service with fields: SYMBOL, BID_PRICE, ASK_PRICE, LAST_PRICE, BID_SIZE, ASK_SIZE, TOTAL_VOLUME, HIGH_PRICE, LOW_PRICE, CLOSE_PRICE, OPEN_PRICE, NET_CHANGE, MARK, QUOTE_TIME_MILLIS, TRADE_TIME_MILLIS
- For option symbols: subscribe to `LEVELONE_OPTIONS` service with fields: SYMBOL, BID_PRICE, ASK_PRICE, LAST_PRICE, TOTAL_VOLUME, OPEN_INTEREST, VOLATILITY, DELTA, GAMMA, THETA, VEGA, RHO, OPEN_PRICE, HIGH_PRICE, LOW_PRICE, CLOSE_PRICE, MARK, STRIKE_PRICE, CONTRACT_TYPE, EXPIRATION_DAY, EXPIRATION_MONTH, EXPIRATION_YEAR, UNDERLYING_PRICE
- Track subscribed symbols in `SubscribedSymbols` map

**FR-9.3** Implement streaming message processing:
- Read messages from the WebSocket connection in a goroutine
- Decode numerical field keys to named fields
- Transform to `*models.MarketData` with appropriate `DataType` (`"quote"`, `"trade"`, `"greeks"`)
- Push to `StreamingCache` and/or `StreamingQueue`
- For option streams, extract Greeks (delta, gamma, theta, vega, rho) into the market data payload — this enables `streaming_greeks` capability

**FR-9.4** Implement `DisconnectStreaming(ctx)` and `UnsubscribeFromSymbols(ctx, symbols, dataTypes)`:
- Send UNSUBS messages for specified services
- Clean up WebSocket connection on disconnect
- Update `IsConnected` and `SubscribedSymbols` state

### FR-10: Account Event Streaming

**FR-10.1** Implement `StartAccountStream(ctx)`:
- Subscribe to `ACCT_ACTIVITY` service on the streaming WebSocket
- This provides real-time order fill notifications, order status changes, etc.

**FR-10.2** Implement `SetOrderEventCallback(callback)` and order event processing:
- Parse account activity messages into `*models.OrderEvent`
- Invoke the registered callback for each order event

**FR-10.3** Implement `StopAccountStream()` and `IsAccountStreamConnected()`:
- Unsubscribe from `ACCT_ACTIVITY` service
- Return connection status

### FR-11: Option Symbol Format Mapping

**FR-11.1** Implement bidirectional symbol mapping between Schwab format and OCC format:
- Schwab option symbols follow a format like: `AAPL  250117C00150000` (with spaces) or `.AAPL250117C150`
- Standard OCC format: `AAPL250117C00150000`
- Implement `convertSchwabOptionToOCC()` and `convertOCCToSchwab()` helper functions
- Ensure all option symbols returned to the JuicyTrade system are in standard OCC format
- Ensure all option symbols sent to the Schwab API are in Schwab's expected format

### FR-12: Frontend Integration

**FR-12.1** The provider will automatically appear in the frontend's provider selection grid, credential form, and service routing dropdowns — no mandatory frontend code changes are required for basic functionality (this is handled by the dynamic `GET /api/providers/types` discovery).

**FR-12.2** (Optional) Add Schwab logo SVG file:
- Create `trade-app/public/logos/schwab.svg`
- Add `"schwab"` to the `svgProviders` arrays in:
  - `trade-app/src/components/settings/ProvidersTab.vue`
  - `trade-app/src/components/setup/WizardStep2Providers.vue`
  - `trade-app/src/components/setup/WizardStep5Complete.vue`

**FR-12.3** (Optional) Update the About section in `trade-app/src/components/SettingsDialog.vue` to list Schwab as a supported provider.

---

## 3. Non-Functional Requirements

### NFR-1: Token Refresh Resilience
- The provider MUST handle access token expiration transparently — API calls should never fail due to an expired access token if a valid refresh token is available
- Implement a token refresh mutex to prevent concurrent refresh attempts
- Log token refresh events at INFO level

### NFR-2: Rate Limiting
- Implement client-side rate limiting (suggested: 120 requests/minute based on community experience)
- Use exponential backoff on HTTP 429 responses

### NFR-3: Error Handling
- All API errors must be logged with structured logging (`slog`)
- HTTP error responses from Schwab must be parsed and returned as meaningful Go errors
- Authentication failures must be clearly distinguished from other errors

### NFR-4: Concurrency Safety
- Use `sync.RWMutex` for thread-safe access to streaming state, token state, and subscribed symbols
- WebSocket message reading must run in a dedicated goroutine

### NFR-5: Testing
- Unit tests for all data transformation functions (quote, option chain, position, order mapping)
- Unit tests for option symbol format conversion (OCC ↔ Schwab)
- Unit tests for token refresh logic
- Integration tests (skipped in CI, run manually with real credentials) for key endpoints

### NFR-6: Code Organization
- Provider code goes in `trade-backend-go/internal/providers/schwab/schwab.go`
- Follow existing patterns from `tastytrade.go` and `tradier.go`
- Use the shared `utils.HTTPClient` for all HTTP requests

---

## 4. Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| AC-1 | The `schwab` provider type appears in `GET /api/providers/types` response with correct capabilities and credential fields | API call returns provider definition |
| AC-2 | A user can create a Schwab provider instance via the setup wizard or settings dialog by entering App Key, App Secret, Callback URL, Refresh Token, and Account Hash | Manual UI test |
| AC-3 | `TestCredentials` successfully validates valid Schwab credentials and returns `{"success": true}` | Unit test + manual test |
| AC-4 | `TestCredentials` returns `{"success": false}` with a meaningful message for invalid credentials | Unit test |
| AC-5 | `GetStockQuote("AAPL")` returns a valid `StockQuote` with bid, ask, last, and timestamp | Unit test (mocked) + integration test |
| AC-6 | `GetStockQuotes(["AAPL", "MSFT", "SPY"])` returns quotes for all three symbols | Unit test (mocked) |
| AC-7 | `GetExpirationDates("AAPL")` returns a list of available expiration dates with DTE | Unit test (mocked) |
| AC-8 | `GetOptionsChainBasic` returns option contracts for a given symbol and expiry without Greeks | Unit test (mocked) |
| AC-9 | `GetOptionsChainSmart` with `includeGreeks=true` returns option contracts with delta, gamma, theta, vega populated | Unit test (mocked) |
| AC-10 | `GetOptionsGreeksBatch` returns Greeks for a list of option symbols | Unit test (mocked) |
| AC-11 | `GetHistoricalBars("AAPL", "1D", ...)` returns OHLCV candle data | Unit test (mocked) |
| AC-12 | `GetAccount` returns account information with balance, buying power, equity | Unit test (mocked) |
| AC-13 | `GetPositions` returns positions with correct asset class identification (equity vs option) | Unit test (mocked) |
| AC-14 | `GetPositionsEnhanced` returns hierarchical position grouping | Unit test (mocked) |
| AC-15 | `GetOrders` returns orders filtered by status | Unit test (mocked) |
| AC-16 | `PlaceOrder` sends correct JSON to Schwab API and parses order ID from response | Unit test (mocked) |
| AC-17 | `PlaceMultiLegOrder` sends correct multi-leg JSON with proper option instructions | Unit test (mocked) |
| AC-18 | `CancelOrder` sends DELETE request and returns success | Unit test (mocked) |
| AC-19 | `LookupSymbols("AAPL")` returns matching symbols with descriptions | Unit test (mocked) |
| AC-20 | `ConnectStreaming` establishes WebSocket connection and authenticates | Unit test (mocked) |
| AC-21 | `SubscribeToSymbols` for equity symbols produces `MarketData` messages with quote data | Unit test (mocked) |
| AC-22 | `SubscribeToSymbols` for option symbols produces `MarketData` messages with Greeks (delta, gamma, theta, vega) | Unit test (mocked) |
| AC-23 | Access token is automatically refreshed when expired, without user intervention | Unit test |
| AC-24 | Option symbols are correctly converted between Schwab format and OCC format in both directions | Unit test |
| AC-25 | The Schwab provider can be selected for any service type in the provider configuration UI | Manual UI test |
| AC-26 | All existing tests pass (no regressions) | `go test ./...` and `npm test` |
| AC-27 | Provider instance can be assigned to service types (e.g., `stock_quotes → schwab_live_MyAccount`) and used for real-time data | Manual integration test |

---

## 5. Scope Boundaries — What Is NOT Included

| Exclusion | Rationale |
|-----------|-----------|
| **Level Two (depth-of-book) data** | Not required by JuicyTrade's current UI; Level One quotes are sufficient |
| **Futures trading support** | JuicyTrade focuses on equities and options; futures are out of scope |
| **OAuth initial authorization flow in the UI** | The initial 3-legged OAuth authorization (browser redirect to Schwab login) is NOT handled within JuicyTrade. Users must obtain their refresh token externally (e.g., via the Schwab developer portal or a helper script) and paste it into the credential form. This is consistent with how TastyTrade handles OAuth. |
| **Automatic refresh token renewal** | Refresh tokens expire after ~7 days. When this happens, the user must manually obtain a new refresh token. Automating this would require embedding a browser-based OAuth flow in JuicyTrade, which is out of scope for this initial implementation. |
| **Schwab sandbox ("paper") account type** | The Schwab sandbox has significant limitations (some order types rejected, inconsistent behavior). It will NOT be exposed as a "paper" account type in this initial implementation. This can be added later if there is demand. |
| **Watchlists API** | Schwab removed watchlists from their API (was available in TD Ameritrade) |
| **Saved orders API** | Schwab removed saved orders from their API (was available in TD Ameritrade) |

---

## 6. Risks & Considerations

| Risk | Mitigation |
|------|------------|
| **Short-lived tokens** — Access tokens expire every ~30 minutes, refresh tokens every ~7 days. Users must periodically re-authenticate. | Implement robust auto-refresh for access tokens. Document the 7-day refresh token limitation clearly in the UI. |
| **Manual app approval** — Schwab requires manual review of developer apps, which can take days. | Document this in user-facing setup instructions. |
| **API rate limits** — Official rate limits are not published by Schwab. | Implement client-side rate limiting (120 req/min) as a safety measure. |
| **Option symbol format differences** — Schwab may use a different option symbol format than OCC. | Implement robust bidirectional symbol conversion and test thoroughly with real option symbols. |
| **Streaming API stability** — The Schwab streaming API is a carry-over from TDA and some streams may have undocumented behavioral changes. | Focus on confirmed-working streams (Level One equities/options, OHLCV charts, Account Activity). Add resilience with automatic reconnection. |
| **Account hash requirement** — Schwab uses account hashes instead of raw account numbers. | Require users to provide account hash in credentials. Provide instructions on how to obtain it. |

---

## 7. Data Source Mapping

This table maps each JuicyTrade Market Data section to the corresponding Schwab API data source:

| JuicyTrade Service | Schwab REST Endpoint | Schwab Streaming Service | Greeks Available? |
|---|---|---|---|
| `stock_quotes` | `GET /marketdata/v1/quotes?symbols=...` | `LEVELONE_EQUITIES` | N/A (equities) |
| `options_chain` | `GET /marketdata/v1/chains?symbol=...` | — | ✅ Yes (REST response includes delta, gamma, theta, vega, rho, IV) |
| `expiration_dates` | `GET /marketdata/v1/chains?symbol=...` (extract dates) | — | N/A |
| `greeks` | `GET /marketdata/v1/quotes?symbols={option_symbols}` | `LEVELONE_OPTIONS` (fields: DELTA, GAMMA, THETA, VEGA, RHO) | ✅ Yes (both REST and Streaming) |
| `streaming_quotes` | — | `LEVELONE_EQUITIES` + `LEVELONE_OPTIONS` | N/A / ✅ for options |
| `streaming_greeks` | — | `LEVELONE_OPTIONS` (DELTA, GAMMA, THETA, VEGA, RHO fields) | ✅ Yes |
| `trade_account` | `GET /trader/v1/accounts/{hash}`, `POST .../orders`, `DELETE .../orders/{id}` | `ACCT_ACTIVITY` | N/A |
| `symbol_lookup` | `GET /marketdata/v1/instruments?symbol=...&projection=symbol-search` | — | N/A |
| `historical_data` | `GET /marketdata/v1/pricehistory?symbol=...` | — | N/A |
| `market_calendar` | Business-day calculation (no dedicated Schwab endpoint) | — | N/A |
| `next_market_date` | Business-day calculation | — | N/A |

---

## 8. Implementation File Map

| File | Purpose |
|------|---------|
| `trade-backend-go/internal/providers/schwab/schwab.go` | Main provider implementation (all interface methods) |
| `trade-backend-go/internal/providers/schwab/schwab_test.go` | Unit tests |
| `trade-backend-go/internal/providers/provider_types.go` | Add `"schwab"` type definition and credential fields |
| `trade-backend-go/internal/providers/manager.go` | Add `"schwab"` factory case in `createProviderInstance()` |
| `trade-app/public/logos/schwab.svg` | (Optional) Provider logo |
| `trade-app/src/components/settings/ProvidersTab.vue` | (Optional) Add to `svgProviders` array |
| `trade-app/src/components/setup/WizardStep2Providers.vue` | (Optional) Add to `svgProviders` array |
| `trade-app/src/components/setup/WizardStep5Complete.vue` | (Optional) Add to `svgProviders` array |
| `trade-app/src/components/SettingsDialog.vue` | (Optional) Update About section |
