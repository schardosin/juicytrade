# Provider Implementation Guide

This document explains how to add a new brokerage/data provider to the JuicyTrade system.

## Table of Contents

1. [Overview](#1-overview)
2. [Quick Start Checklist](#2-quick-start-checklist)
3. [Provider Interface Reference](#3-provider-interface-reference)
4. [Step-by-Step Implementation Guide](#4-step-by-step-implementation-guide)
5. [Registering the Provider](#5-registering-the-provider)
6. [Configuration System](#6-configuration-system)
7. [Data Models Reference](#7-data-models-reference)
8. [Testing Your Provider](#8-testing-your-provider)
9. [Common Patterns & Examples](#9-common-patterns--examples)

---

## 1. Overview

### Architecture

The provider system uses a layered architecture that decouples API endpoints from broker-specific implementations:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Frontend (Vue.js)                                  │
└─────────────────────────────────────────┬───────────────────────────────────┘
                                          │ HTTP/WebSocket
                                          ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                         API Handlers (Gin)                                   │
│                    trade-backend-go/internal/api/                            │
└─────────────────────────────────────────┬───────────────────────────────────┘
                                          │ Method calls
                                          ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        ProviderManager                                       │
│                 trade-backend-go/internal/providers/manager.go               │
│  ┌────────────────────────────────────────────────────────────────────────┐ │
│  │  ConfigManager ← provider_config.json                                   │ │
│  │  Routes operations to providers based on service type                   │ │
│  │                                                                          │ │
│  │  Example routing:                                                        │ │
│  │    "stock_quotes"     → tastytrade_live_tasty                           │ │
│  │    "options_chain"    → tastytrade_live_tasty                           │ │
│  │    "trade_account"    → tradier_paper_tradier                           │ │
│  │    "streaming_quotes" → tradier_live_tradier                            │ │
│  └────────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────┬───────────────────────────────────┘
                                          │
         ┌────────────────────────────────┼────────────────────────────────┐
         ▼                                ▼                                ▼
┌─────────────────┐              ┌─────────────────┐              ┌─────────────────┐
│     Alpaca      │              │    Tradier      │              │   TastyTrade    │
│    Provider     │              │    Provider     │              │    Provider     │
│                 │              │                 │              │                 │
│ alpaca/         │              │ tradier/        │              │ tastytrade/     │
│ alpaca.go       │              │ tradier.go      │              │ tastytrade.go   │
└────────┬────────┘              └────────┬────────┘              └────────┬────────┘
         │                                │                                │
         └────────────────────────────────┼────────────────────────────────┘
                                          │
                                          ▼
                              base.Provider Interface
                        trade-backend-go/internal/providers/base/provider.go
```

### Key Concepts

| Concept | Description |
|---------|-------------|
| **Provider** | A Go struct that implements the `base.Provider` interface to communicate with a specific broker's API |
| **Provider Instance** | A configured provider with credentials, identified by an instance ID (e.g., `tradier_live_MyAccount`) |
| **Service Type** | A category of functionality (e.g., `stock_quotes`, `trade_account`, `streaming_quotes`) |
| **Capability** | What services a provider type supports (REST vs streaming) |
| **ProviderManager** | The abstraction layer that routes API calls to the correct provider instance |
| **ConfigManager** | Manages `provider_config.json` which maps service types to provider instances |
| **CredentialStore** | Stores provider instance credentials in `provider_credentials.json` |

### Data Flow

1. **API Handler** receives a request (e.g., "get stock quote for AAPL")
2. **ProviderManager** looks up which provider handles `stock_quotes` in config
3. **ConfigManager** returns the provider instance ID (e.g., `tastytrade_live_tasty`)
4. **ProviderManager** calls the method on that provider instance
5. **Provider** makes the broker API call and transforms the response
6. **Provider** returns standardized data models (e.g., `*models.StockQuote`)
7. **API Handler** sends the response to the frontend

---

## 2. Quick Start Checklist

Follow these steps to add a new provider:

| Step | Action | File Location |
|------|--------|---------------|
| 1 | Create provider directory and file | `trade-backend-go/internal/providers/<name>/<name>.go` |
| 2 | Define provider struct embedding `*base.BaseProviderImpl` | Your new provider file |
| 3 | Implement constructor function `New<Name>Provider(...)` | Your new provider file |
| 4 | Implement all `base.Provider` interface methods | Your new provider file |
| 5 | Add provider type definition with capabilities | `trade-backend-go/internal/providers/provider_types.go` |
| 6 | Add credential field definitions | `trade-backend-go/internal/providers/provider_types.go` |
| 7 | Add factory case in `createProviderInstance()` | `trade-backend-go/internal/providers/manager.go` |
| 8 | (Optional) Add legacy provider capabilities | `trade-backend-go/internal/providers/provider_types.go` |

After completing these steps:
- Create a provider instance via the API or manually in `provider_credentials.json`
- Configure which services use your provider in `provider_config.json`

---

## 3. Provider Interface Reference

All providers must implement the `base.Provider` interface defined in `trade-backend-go/internal/providers/base/provider.go`.

### 3.1 Market Data Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `GetStockQuote` | `(ctx context.Context, symbol string) (*models.StockQuote, error)` | Get latest quote for a single symbol |
| `GetStockQuotes` | `(ctx context.Context, symbols []string) (map[string]*models.StockQuote, error)` | Get quotes for multiple symbols |
| `GetExpirationDates` | `(ctx context.Context, symbol string) ([]map[string]interface{}, error)` | Get available option expiration dates for a symbol |
| `GetOptionsChainBasic` | `(ctx context.Context, symbol, expiry string, underlyingPrice *float64, strikeCount int, optionType, underlyingSymbol *string) ([]*models.OptionContract, error)` | Get options chain without Greeks (fast loading) |
| `GetOptionsChainSmart` | `(ctx context.Context, symbol, expiry string, underlyingPrice *float64, atmRange int, includeGreeks, strikesOnly bool) ([]*models.OptionContract, error)` | Get options chain with configurable Greeks inclusion |
| `GetOptionsGreeksBatch` | `(ctx context.Context, optionSymbols []string) (map[string]map[string]interface{}, error)` | Get Greeks for multiple option symbols |
| `GetNextMarketDate` | `(ctx context.Context) (string, error)` | Get the next trading date (YYYY-MM-DD format) |
| `LookupSymbols` | `(ctx context.Context, query string) ([]*models.SymbolSearchResult, error)` | Search for symbols matching a query string |
| `GetHistoricalBars` | `(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error)` | Get historical OHLCV bars for charting |

### 3.2 Account & Portfolio Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `GetPositions` | `(ctx context.Context) ([]*models.Position, error)` | Get all current positions in the account |
| `GetPositionsEnhanced` | `(ctx context.Context) (*models.EnhancedPositionsResponse, error)` | Get positions grouped by underlying and strategy |
| `GetOrders` | `(ctx context.Context, status string) ([]*models.Order, error)` | Get orders filtered by status ("all", "open", "filled", "canceled") |
| `GetAccount` | `(ctx context.Context) (*models.Account, error)` | Get account information (balance, buying power, etc.) |

### 3.3 Order Management Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `PlaceOrder` | `(ctx context.Context, orderData map[string]interface{}) (*models.Order, error)` | Place a single-leg order (equity or option) |
| `PlaceMultiLegOrder` | `(ctx context.Context, orderData map[string]interface{}) (*models.Order, error)` | Place a multi-leg options order (spreads, etc.) |
| `PreviewOrder` | `(ctx context.Context, orderData map[string]interface{}) (map[string]interface{}, error)` | Preview order for cost estimates without placing |
| `CancelOrder` | `(ctx context.Context, orderID string) (bool, error)` | Cancel an existing order by ID |

### 3.4 Streaming Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `ConnectStreaming` | `(ctx context.Context) (bool, error)` | Connect to the provider's streaming service |
| `DisconnectStreaming` | `(ctx context.Context) (bool, error)` | Disconnect from the streaming service |
| `SubscribeToSymbols` | `(ctx context.Context, symbols []string, dataTypes []string) (bool, error)` | Subscribe to real-time data for symbols |
| `UnsubscribeFromSymbols` | `(ctx context.Context, symbols []string, dataTypes []string) (bool, error)` | Unsubscribe from real-time data |

### 3.5 Account Event Streaming Methods

These methods are for real-time order event notifications. The base implementation provides no-op defaults.

| Method | Signature | Description |
|--------|-----------|-------------|
| `StartAccountStream` | `(ctx context.Context) error` | Start WebSocket stream for order events |
| `StopAccountStream` | `()` | Stop the account event stream |
| `SetOrderEventCallback` | `(callback func(*models.OrderEvent))` | Set callback for order event notifications |
| `IsAccountStreamConnected` | `() bool` | Check if account stream is connected |

### 3.6 Utility Methods

| Method | Signature | Description | Base Implementation |
|--------|-----------|-------------|---------------------|
| `GetSubscribedSymbols` | `() map[string]bool` | Get currently subscribed symbols | ✅ Provided |
| `IsStreamingConnected` | `() bool` | Check if streaming is connected | ✅ Provided |
| `Ping` | `(ctx context.Context) error` | Heartbeat to verify connection | ✅ Provided |
| `HealthCheck` | `(ctx context.Context) (map[string]interface{}, error)` | Perform health check | ✅ Provided |
| `TestCredentials` | `(ctx context.Context) (map[string]interface{}, error)` | Test credentials with real API call | ❌ Must implement |
| `GetName` | `() string` | Return provider name | ✅ Provided |
| `SetStreamingQueue` | `(queue chan *models.MarketData)` | Set queue for streaming data | ✅ Provided |
| `SetStreamingCache` | `(cache StreamingCache)` | Set streaming cache | ✅ Provided |

---

## 4. Step-by-Step Implementation Guide

### 4.1 Create Provider Directory & File

Create a new directory and file for your provider:

```
trade-backend-go/internal/providers/
├── alpaca/
│   └── alpaca.go
├── tradier/
│   └── tradier.go
├── tastytrade/
│   └── tastytrade.go
├── <your-provider>/          ← Create this directory
│   └── <your-provider>.go    ← Create this file
├── base/
│   └── provider.go
├── manager.go
├── config_manager.go
├── credential_store.go
└── provider_types.go
```

### 4.2 Define Provider Struct

Your provider struct should embed `*base.BaseProviderImpl` to inherit base functionality:

```go
package yourprovider

import (
    "context"
    "trade-backend-go/internal/models"
    "trade-backend-go/internal/providers/base"
    "trade-backend-go/internal/utils"
)

// YourProvider implements the Provider interface for Your Brokerage API.
type YourProvider struct {
    *base.BaseProviderImpl           // Embed base implementation
    
    // Credential fields
    accountID   string
    apiKey      string
    apiSecret   string
    baseURL     string
    
    // HTTP client
    client      *utils.HTTPClient
    
    // Streaming fields (if supporting streaming)
    streamConn  *websocket.Conn
    streamMutex sync.RWMutex
}
```

### 4.3 Implement Constructor

Create a constructor function that matches the pattern used in `manager.go`:

```go
// NewYourProvider creates a new Your provider instance.
func NewYourProvider(accountID, apiKey, apiSecret, baseURL string) *YourProvider {
    return &YourProvider{
        BaseProviderImpl: base.NewBaseProvider("YourProvider"),
        accountID:        accountID,
        apiKey:           apiKey,
        apiSecret:        apiSecret,
        baseURL:          baseURL,
        client:           utils.NewHTTPClient(),
    }
}
```

### 4.4 Implement Required Methods

#### TestCredentials (Critical)

This method validates credentials by making a real API call. It's called when testing provider configuration:

```go
func (p *YourProvider) TestCredentials(ctx context.Context) (map[string]interface{}, error) {
    slog.Info("Testing YourProvider credentials...")
    
    // Make a simple API call to verify credentials
    account, err := p.GetAccount(ctx)
    if err != nil {
        slog.Error("YourProvider credentials validation failed", "error", err)
        return map[string]interface{}{
            "success": false,
            "message": fmt.Sprintf("Authentication failed: %v", err),
        }, nil
    }
    
    if account != nil && account.AccountID != "" {
        slog.Info("YourProvider credentials are valid")
        return map[string]interface{}{
            "success": true,
            "message": "Successfully connected to YourProvider.",
        }, nil
    }
    
    return map[string]interface{}{
        "success": false,
        "message": "Invalid credentials or no account info found.",
    }, nil
}
```

#### Market Data Methods

Example implementation for `GetStockQuote`:

```go
func (p *YourProvider) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
    quotes, err := p.GetStockQuotes(ctx, []string{symbol})
    if err != nil {
        return nil, err
    }
    
    if quote, exists := quotes[symbol]; exists {
        return quote, nil
    }
    
    return nil, fmt.Errorf("quote not found for symbol %s", symbol)
}

func (p *YourProvider) GetStockQuotes(ctx context.Context, symbols []string) (map[string]*models.StockQuote, error) {
    endpoint := fmt.Sprintf("%s/v1/quotes", p.baseURL)
    
    headers := map[string]string{
        "Authorization": fmt.Sprintf("Bearer %s", p.apiKey),
        "Accept":        "application/json",
    }
    
    params := map[string]string{
        "symbols": strings.Join(symbols, ","),
    }
    
    resp, err := p.client.Get(ctx, endpoint, headers, params)
    if err != nil {
        return nil, fmt.Errorf("failed to get stock quotes: %w", err)
    }
    
    // Parse and transform response
    var response YourAPIQuoteResponse
    if err := json.Unmarshal(resp.Body, &response); err != nil {
        return nil, fmt.Errorf("failed to parse quotes response: %w", err)
    }
    
    // Transform to standard models
    result := make(map[string]*models.StockQuote)
    for _, rawQuote := range response.Quotes {
        transformed := p.transformStockQuote(rawQuote)
        if transformed != nil {
            result[transformed.Symbol] = transformed
        }
    }
    
    return result, nil
}
```

#### Account Methods

Example implementation for `GetAccount`:

```go
func (p *YourProvider) GetAccount(ctx context.Context) (*models.Account, error) {
    endpoint := fmt.Sprintf("%s/v1/accounts/%s", p.baseURL, p.accountID)
    
    headers := map[string]string{
        "Authorization": fmt.Sprintf("Bearer %s", p.apiKey),
        "Accept":        "application/json",
    }
    
    resp, err := p.client.Get(ctx, endpoint, headers, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to get account: %w", err)
    }
    
    var response YourAPIAccountResponse
    if err := json.Unmarshal(resp.Body, &response); err != nil {
        return nil, fmt.Errorf("failed to parse account response: %w", err)
    }
    
    return p.transformAccount(response), nil
}
```

### 4.5 Data Transformation Patterns

Each provider's API returns data in its own format. You must transform it to standard models:

```go
// transformStockQuote transforms your broker's quote format to the standard model.
func (p *YourProvider) transformStockQuote(rawQuote map[string]interface{}) *models.StockQuote {
    symbol, _ := rawQuote["symbol"].(string)
    
    var ask, bid, last *float64
    if askVal, ok := rawQuote["ask_price"].(float64); ok && askVal > 0 {
        ask = &askVal
    }
    if bidVal, ok := rawQuote["bid_price"].(float64); ok && bidVal > 0 {
        bid = &bidVal
    }
    if lastVal, ok := rawQuote["last_price"].(float64); ok && lastVal > 0 {
        last = &lastVal
    }
    
    timestamp := time.Now().Format(time.RFC3339)
    if ts, ok := rawQuote["timestamp"].(string); ok {
        timestamp = ts
    }
    
    return &models.StockQuote{
        Symbol:    symbol,
        Ask:       ask,
        Bid:       bid,
        Last:      last,
        Timestamp: timestamp,
    }
}

// transformAccount transforms your broker's account format to the standard model.
func (p *YourProvider) transformAccount(raw YourAPIAccountResponse) *models.Account {
    var buyingPower, cash, portfolioValue *float64
    
    if raw.BuyingPower > 0 {
        buyingPower = &raw.BuyingPower
    }
    if raw.CashBalance > 0 {
        cash = &raw.CashBalance
    }
    if raw.TotalEquity > 0 {
        portfolioValue = &raw.TotalEquity
    }
    
    return &models.Account{
        AccountID:      p.accountID,
        Status:         "active",
        Currency:       "USD",
        BuyingPower:    buyingPower,
        Cash:           cash,
        PortfolioValue: portfolioValue,
    }
}
```

---

## 5. Registering the Provider

### 5.1 Add to provider_types.go

Add your provider type definition to the `ProviderTypes` map in `trade-backend-go/internal/providers/provider_types.go`:

```go
var ProviderTypes = map[string]ProviderType{
    // ... existing providers ...
    
    "yourprovider": {
        Name:                 "YourProvider",
        Description:          "Your Brokerage API",
        SupportsAccountTypes: []string{"live", "paper"},  // Supported account types
        Capabilities: ProviderCapabilities{
            Rest: []string{
                "stock_quotes",
                "options_chain",
                "trade_account",
                "symbol_lookup",
                "historical_data",
                "market_calendar",
                "greeks",
                "expiration_dates",
                "next_market_date",
            },
            Streaming: []string{
                "streaming_quotes",
                // "streaming_greeks",  // Include if supported
                // "trade_account",     // Include if real-time order events supported
            },
        },
        CredentialFields: map[string][]CredentialField{
            "live": {
                {Name: "api_key", Label: "API Key", Type: "password", Required: true, Placeholder: "Your Live API Key"},
                {Name: "api_secret", Label: "API Secret", Type: "password", Required: true, Placeholder: "Your Live API Secret"},
                {Name: "account_id", Label: "Account ID", Type: "text", Required: true, Placeholder: "Your Account ID"},
                {Name: "base_url", Label: "Base URL", Type: "text", Required: false, Default: "https://api.yourprovider.com"},
            },
            "paper": {
                {Name: "api_key", Label: "API Key", Type: "password", Required: true, Placeholder: "Your Sandbox API Key"},
                {Name: "api_secret", Label: "API Secret", Type: "password", Required: true, Placeholder: "Your Sandbox API Secret"},
                {Name: "account_id", Label: "Account ID", Type: "text", Required: true, Placeholder: "Your Sandbox Account ID"},
                {Name: "base_url", Label: "Base URL", Type: "text", Required: false, Default: "https://sandbox.yourprovider.com"},
            },
        },
    },
}
```

#### Credential Field Types

| Type | Description |
|------|-------------|
| `text` | Standard text input (visible) |
| `password` | Masked input for sensitive data |

#### Capability Reference

**REST Capabilities:**

| Capability | Description | Required Methods |
|------------|-------------|------------------|
| `stock_quotes` | Real-time stock quotes | `GetStockQuote`, `GetStockQuotes` |
| `options_chain` | Options chain data | `GetOptionsChainBasic`, `GetOptionsChainSmart`, `GetExpirationDates` |
| `trade_account` | Account/position/order management | `GetAccount`, `GetPositions`, `GetOrders`, `PlaceOrder`, `CancelOrder` |
| `symbol_lookup` | Symbol search | `LookupSymbols` |
| `historical_data` | Historical OHLCV bars | `GetHistoricalBars` |
| `market_calendar` | Market calendar/dates | `GetNextMarketDate` |
| `greeks` | Option Greeks via REST API | `GetOptionsGreeksBatch` |
| `expiration_dates` | Option expiration dates | `GetExpirationDates` |
| `next_market_date` | Next trading date | `GetNextMarketDate` |

**Streaming Capabilities:**

| Capability | Description | Required Methods |
|------------|-------------|------------------|
| `streaming_quotes` | Real-time quote streaming | `ConnectStreaming`, `SubscribeToSymbols`, etc. |
| `streaming_greeks` | Real-time Greeks streaming | `ConnectStreaming`, `SubscribeToSymbols`, etc. |
| `trade_account` | Real-time order events | `StartAccountStream`, `SetOrderEventCallback`, etc. |

### 5.2 Update manager.go Factory

Add your provider to the factory method in `trade-backend-go/internal/providers/manager.go`:

```go
// createProviderInstance creates a provider instance based on type and credentials.
func (pm *ProviderManager) createProviderInstance(providerType, accountType string, credentials map[string]interface{}) base.Provider {
    switch providerType {
    case "alpaca":
        // ... existing code ...
        
    case "tradier":
        // ... existing code ...
        
    case "tastytrade":
        // ... existing code ...
        
    case "yourprovider":
        accountID, _ := credentials["account_id"].(string)
        apiKey, _ := credentials["api_key"].(string)
        apiSecret, _ := credentials["api_secret"].(string)
        baseURL, _ := credentials["base_url"].(string)
        
        return yourprovider.NewYourProvider(accountID, apiKey, apiSecret, baseURL)
        
    default:
        slog.Error(fmt.Sprintf("Unknown provider type: %s", providerType))
        return nil
    }
}
```

Don't forget to add the import:

```go
import (
    // ... existing imports ...
    "trade-backend-go/internal/providers/yourprovider"
)
```

### 5.3 Update Legacy Provider Capabilities (Optional)

For backward compatibility, add legacy provider entries in the `LegacyProviderCapabilities` map:

```go
var LegacyProviderCapabilities = map[string]map[string]interface{}{
    // ... existing providers ...
    
    "yourprovider": {
        "capabilities": map[string]interface{}{
            "rest":      []string{"stock_quotes", "options_chain", "trade_account", ...},
            "streaming": []string{"streaming_quotes"},
        },
        "paper":        false,
        "display_name": "YourProvider",
    },
    "yourprovider_paper": {
        "capabilities": map[string]interface{}{
            "rest":      []string{"stock_quotes", "options_chain", "trade_account", ...},
            "streaming": []string{"streaming_quotes"},
        },
        "paper":        true,
        "display_name": "YourProvider",
    },
}
```

---

## 6. Configuration System

### 6.1 Service Types

The system uses these service types to route operations:

| Service Type | Description | Example Operations |
|--------------|-------------|-------------------|
| `stock_quotes` | Stock quote data | `GetStockQuote`, `GetStockQuotes` |
| `options_chain` | Options chain and expiration data | `GetOptionsChainBasic`, `GetOptionsChainSmart`, `GetExpirationDates` |
| `trade_account` | Account, positions, orders | `GetAccount`, `GetPositions`, `GetOrders`, `PlaceOrder`, `CancelOrder` |
| `symbol_lookup` | Symbol search | `LookupSymbols` |
| `historical_data` | Historical bars | `GetHistoricalBars` |
| `market_calendar` | Market dates | `GetNextMarketDate` |
| `greeks` | Options Greeks (REST) | `GetOptionsGreeksBatch` |
| `streaming_quotes` | Real-time quote streaming | `ConnectStreaming`, `SubscribeToSymbols` |
| `streaming_greeks` | Real-time Greeks streaming | `ConnectStreaming`, `SubscribeToSymbols` |

### 6.2 provider_config.json Format

The configuration file maps service types to provider instance IDs:

```json
{
  "stock_quotes": "tastytrade_live_tasty",
  "options_chain": "tastytrade_live_tasty",
  "trade_account": "tradier_paper_tradier",
  "symbol_lookup": "tastytrade_live_tasty",
  "historical_data": "tastytrade_live_tasty",
  "market_calendar": "tastytrade_live_tasty",
  "greeks": "tastytrade_live_tasty",
  "streaming_quotes": "tradier_live_tradier",
  "streaming_greeks": "tastytrade_live_tasty"
}
```

**Key Points:**
- Different services can use different providers
- This allows mixing providers based on their strengths
- Empty string `""` means no provider for that service

### 6.3 Credential Store

Provider instances and credentials are stored in `provider_credentials.json`:

```json
{
  "tradier_live_tradier": {
    "active": true,
    "provider_type": "tradier",
    "account_type": "live",
    "display_name": "Tradier Live",
    "credentials": {
      "api_key": "your-api-key",
      "account_id": "your-account-id",
      "base_url": "https://api.tradier.com",
      "stream_url": "wss://ws.tradier.com/v1/markets/events"
    },
    "created_at": 1704067200,
    "updated_at": 1704067200
  }
}
```

### 6.4 Instance ID Convention

Instance IDs follow this naming convention:

```
{provider_type}_{account_type}_{display_name}
```

Examples:
- `tradier_live_MyMainAccount`
- `alpaca_paper_Testing`
- `tastytrade_live_tasty`

---

## 7. Data Models Reference

All providers must return standardized data models defined in `trade-backend-go/internal/models/models.go`.

### StockQuote

```go
type StockQuote struct {
    Symbol    string   `json:"symbol"`
    Ask       *float64 `json:"ask"`
    Bid       *float64 `json:"bid"`
    Last      *float64 `json:"last"`
    Timestamp string   `json:"timestamp"`
}
```

### OptionContract

```go
type OptionContract struct {
    Symbol            string   `json:"symbol"`              // OCC format: AAPL240119C00150000
    UnderlyingSymbol  string   `json:"underlying_symbol"`
    ExpirationDate    string   `json:"expiration_date"`     // YYYY-MM-DD
    StrikePrice       float64  `json:"strike_price"`
    Type              string   `json:"type"`                // "call" or "put"
    RootSymbol        *string  `json:"root_symbol"`
    Bid               *float64 `json:"bid"`
    Ask               *float64 `json:"ask"`
    ClosePrice        *float64 `json:"close_price"`
    Volume            *int     `json:"volume"`
    OpenInterest      *int     `json:"open_interest"`
    ImpliedVolatility *float64 `json:"implied_volatility"`
    Delta             *float64 `json:"delta"`
    Gamma             *float64 `json:"gamma"`
    Theta             *float64 `json:"theta"`
    Vega              *float64 `json:"vega"`
}
```

### Position

```go
type Position struct {
    Symbol           string   `json:"symbol"`
    Qty              float64  `json:"qty"`
    Side             string   `json:"side"`               // "long" or "short"
    MarketValue      float64  `json:"market_value"`
    CostBasis        float64  `json:"cost_basis"`
    UnrealizedPL     float64  `json:"unrealized_pl"`
    UnrealizedPLPC   *float64 `json:"unrealized_plpc"`
    CurrentPrice     float64  `json:"current_price"`
    AvgEntryPrice    float64  `json:"avg_entry_price"`
    AssetClass       string   `json:"asset_class"`        // "us_equity", "us_option"
    LastdayPrice     *float64 `json:"lastday_price"`
    DateAcquired     *string  `json:"date_acquired"`
    
    // Option-specific fields
    UnderlyingSymbol *string  `json:"underlying_symbol"`
    OptionType       *string  `json:"option_type"`        // "call" or "put"
    StrikePrice      *float64 `json:"strike_price"`
    ExpiryDate       *string  `json:"expiry_date"`
}
```

### Order

```go
type Order struct {
    ID           string     `json:"id"`
    Symbol       string     `json:"symbol"`
    AssetClass   string     `json:"asset_class"`
    Side         string     `json:"side"`               // "buy" or "sell"
    Action       *string    `json:"action"`
    OrderType    string     `json:"order_type"`         // "market", "limit", "stop", "stop_limit"
    Qty          float64    `json:"qty"`
    FilledQty    float64    `json:"filled_qty"`
    LimitPrice   *float64   `json:"limit_price"`
    StopPrice    *float64   `json:"stop_price"`
    AvgFillPrice *float64   `json:"avg_fill_price"`
    Status       string     `json:"status"`             // "new", "filled", "canceled", "rejected"
    TimeInForce  string     `json:"time_in_force"`      // "day", "gtc", "ioc", "fok"
    SubmittedAt  string     `json:"submitted_at"`
    FilledAt     *string    `json:"filled_at"`
    Legs         []OrderLeg `json:"legs,omitempty"`     // For multi-leg orders
}

type OrderLeg struct {
    Symbol string  `json:"symbol"`
    Side   string  `json:"side"`
    Qty    float64 `json:"qty"`
}
```

### Account

```go
type Account struct {
    AccountID             string   `json:"account_id"`
    AccountNumber         *string  `json:"account_number"`
    Status                string   `json:"status"`
    Currency              string   `json:"currency"`
    BuyingPower           *float64 `json:"buying_power"`
    Cash                  *float64 `json:"cash"`
    PortfolioValue        *float64 `json:"portfolio_value"`
    Equity                *float64 `json:"equity"`
    DayTradingBuyingPower *float64 `json:"day_trading_buying_power"`
    RegtBuyingPower       *float64 `json:"regt_buying_power"`
    OptionsBuyingPower    *float64 `json:"options_buying_power"`
    PatternDayTrader      *bool    `json:"pattern_day_trader"`
    TradingBlocked        *bool    `json:"trading_blocked"`
    TransfersBlocked      *bool    `json:"transfers_blocked"`
    AccountBlocked        *bool    `json:"account_blocked"`
    CreatedAt             *string  `json:"created_at"`
    Multiplier            *string  `json:"multiplier"`
    LongMarketValue       *float64 `json:"long_market_value"`
    ShortMarketValue      *float64 `json:"short_market_value"`
    InitialMargin         *float64 `json:"initial_margin"`
    MaintenanceMargin     *float64 `json:"maintenance_margin"`
    DaytradeCount         *int     `json:"daytrade_count"`
    OptionsApprovedLevel  *int     `json:"options_approved_level"`
    OptionsTradingLevel   *int     `json:"options_trading_level"`
}
```

### MarketData (Streaming)

```go
type MarketData struct {
    Symbol      string                 `json:"symbol"`
    DataType    string                 `json:"data_type"`   // "quote", "trade", "greeks"
    Timestamp   string                 `json:"timestamp"`
    TimestampMs *int64                 `json:"timestamp_ms"`
    Data        map[string]interface{} `json:"data"`
}
```

### SymbolSearchResult

```go
type SymbolSearchResult struct {
    Symbol      string `json:"symbol"`
    Description string `json:"description"`
    Exchange    string `json:"exchange"`
    Type        string `json:"type"`   // "stock", "etf", "index", "option"
}
```

---

## 8. Testing Your Provider

### 8.1 Unit Testing Pattern

Create tests in `trade-backend-go/internal/providers/yourprovider/yourprovider_test.go`:

```go
package yourprovider

import (
    "context"
    "testing"
)

func TestNewYourProvider(t *testing.T) {
    provider := NewYourProvider("account123", "apikey", "secret", "https://api.test.com")
    
    if provider == nil {
        t.Fatal("expected provider to be created")
    }
    
    if provider.GetName() != "YourProvider" {
        t.Errorf("expected name 'YourProvider', got '%s'", provider.GetName())
    }
}

func TestTransformStockQuote(t *testing.T) {
    provider := NewYourProvider("", "", "", "")
    
    rawQuote := map[string]interface{}{
        "symbol":    "AAPL",
        "ask_price": 150.50,
        "bid_price": 150.25,
        "last_price": 150.30,
    }
    
    quote := provider.transformStockQuote(rawQuote)
    
    if quote.Symbol != "AAPL" {
        t.Errorf("expected symbol 'AAPL', got '%s'", quote.Symbol)
    }
    if *quote.Ask != 150.50 {
        t.Errorf("expected ask 150.50, got %f", *quote.Ask)
    }
}
```

### 8.2 Integration Testing

Test against the real API (use paper/sandbox accounts):

```go
func TestIntegrationGetAccount(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    
    // Use environment variables for credentials
    apiKey := os.Getenv("YOURPROVIDER_API_KEY")
    accountID := os.Getenv("YOURPROVIDER_ACCOUNT_ID")
    
    if apiKey == "" || accountID == "" {
        t.Skip("credentials not set")
    }
    
    provider := NewYourProvider(accountID, apiKey, "", "https://sandbox.yourprovider.com")
    
    ctx := context.Background()
    account, err := provider.GetAccount(ctx)
    
    if err != nil {
        t.Fatalf("GetAccount failed: %v", err)
    }
    
    if account.AccountID == "" {
        t.Error("expected account ID to be set")
    }
}
```

Run tests:

```bash
# Run unit tests
cd trade-backend-go
go test ./internal/providers/yourprovider/

# Run integration tests (requires credentials)
go test -v ./internal/providers/yourprovider/ -run Integration

# Run all provider tests
go test ./internal/providers/...
```

### 8.3 TestCredentials Method

The `TestCredentials` method is called by the frontend when users add/test provider instances. Ensure it:

1. Makes a real API call (typically `GetAccount`)
2. Returns `{"success": true, "message": "..."}` on success
3. Returns `{"success": false, "message": "..."}` on failure
4. Handles network errors gracefully

---

## 9. Common Patterns & Examples

### 9.1 HTTP Client Usage

Use the shared HTTP client in `trade-backend-go/internal/utils/http_client.go`:

```go
import "trade-backend-go/internal/utils"

// In constructor
client := utils.NewHTTPClient()

// GET request
resp, err := p.client.Get(ctx, endpoint, headers, params)
if err != nil {
    return nil, fmt.Errorf("API request failed: %w", err)
}

// POST request (JSON body)
resp, err := p.client.Post(ctx, endpoint, headers, body)

// POST request (form data)
resp, err := p.client.PostForm(ctx, endpoint, headers, formData)
```

### 9.2 Error Handling

Follow consistent error handling patterns:

```go
func (p *YourProvider) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
    // 1. Make API call
    resp, err := p.client.Get(ctx, endpoint, headers, params)
    if err != nil {
        p.LogError("GetStockQuote", err)  // Use base method
        return nil, fmt.Errorf("failed to get stock quote: %w", err)
    }
    
    // 2. Parse response
    var response APIResponse
    if err := json.Unmarshal(resp.Body, &response); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }
    
    // 3. Check for API errors
    if response.Error != "" {
        return nil, fmt.Errorf("API error: %s", response.Error)
    }
    
    // 4. Transform and return
    return p.transformStockQuote(response.Data), nil
}
```

### 9.3 Option Symbol Handling (OCC Format)

Options use the OCC (Options Clearing Corporation) symbol format:

```
AAPL240119C00150000
│   │     │└── Strike price × 1000 (8 digits)
│   │     └── Option type (C=call, P=put)
│   └── Expiration date (YYMMDD)
└── Root symbol (underlying, variable length)
```

Helper function to parse option symbols:

```go
type ParsedOptionSymbol struct {
    Underlying string
    Expiry     string  // YYYY-MM-DD
    Type       string  // "call" or "put"
    Strike     float64
}

func (p *YourProvider) parseOptionSymbol(symbol string) *ParsedOptionSymbol {
    if len(symbol) < 15 {
        return nil
    }
    
    // Extract components from right to left
    strikeStr := symbol[len(symbol)-8:]
    optionType := symbol[len(symbol)-9 : len(symbol)-8]
    dateStr := symbol[len(symbol)-15 : len(symbol)-9]
    underlying := symbol[:len(symbol)-15]
    
    // Parse strike (divide by 1000)
    strike, _ := strconv.ParseFloat(strikeStr, 64)
    strike = strike / 1000.0
    
    // Parse date (YYMMDD -> YYYY-MM-DD)
    year := "20" + dateStr[0:2]
    month := dateStr[2:4]
    day := dateStr[4:6]
    expiry := fmt.Sprintf("%s-%s-%s", year, month, day)
    
    // Map option type
    typeStr := "call"
    if optionType == "P" {
        typeStr = "put"
    }
    
    return &ParsedOptionSymbol{
        Underlying: underlying,
        Expiry:     expiry,
        Type:       typeStr,
        Strike:     strike,
    }
}

func (p *YourProvider) isOptionSymbol(symbol string) bool {
    // Option symbols are typically > 10 chars and contain C or P
    if len(symbol) <= 10 {
        return false
    }
    
    // Check for call/put indicator
    hasCP := strings.Contains(symbol, "C") || strings.Contains(symbol, "P")
    if !hasCP {
        return false
    }
    
    // Check for digits in strike portion
    lastEight := symbol[len(symbol)-8:]
    for _, c := range lastEight {
        if c >= '0' && c <= '9' {
            return true
        }
    }
    
    return false
}
```

### 9.4 Streaming WebSocket Pattern

For providers that support streaming:

```go
func (p *YourProvider) ConnectStreaming(ctx context.Context) (bool, error) {
    p.streamMutex.Lock()
    defer p.streamMutex.Unlock()
    
    if p.IsConnected {
        return true, nil
    }
    
    // Connect to WebSocket
    dialer := websocket.Dialer{
        HandshakeTimeout: 10 * time.Second,
    }
    
    conn, _, err := dialer.DialContext(ctx, p.streamURL, nil)
    if err != nil {
        return false, fmt.Errorf("failed to connect: %w", err)
    }
    
    p.streamConn = conn
    p.IsConnected = true
    
    // Start message reader goroutine
    go p.readMessages(ctx)
    
    p.LogInfo("Streaming connected")
    return true, nil
}

func (p *YourProvider) readMessages(ctx context.Context) {
    defer func() {
        p.streamMutex.Lock()
        p.IsConnected = false
        p.streamMutex.Unlock()
    }()
    
    for {
        select {
        case <-ctx.Done():
            return
        default:
            _, message, err := p.streamConn.ReadMessage()
            if err != nil {
                p.LogError("readMessages", err)
                return
            }
            
            // Parse and process message
            p.processStreamMessage(message)
        }
    }
}

func (p *YourProvider) processStreamMessage(message []byte) {
    var data map[string]interface{}
    if err := json.Unmarshal(message, &data); err != nil {
        return
    }
    
    // Transform to MarketData and send to cache/queue
    marketData := p.transformStreamData(data)
    if marketData != nil && p.StreamingCache != nil {
        p.StreamingCache.Update(marketData)
    }
}
```

### 9.5 GetPositionsEnhanced Pattern

Use the base implementation's helper for converting positions to enhanced format:

```go
func (p *YourProvider) GetPositionsEnhanced(ctx context.Context) (*models.EnhancedPositionsResponse, error) {
    // 1. Get current positions
    positions, err := p.GetPositions(ctx)
    if err != nil {
        return models.NewEnhancedPositionsResponse(), nil
    }
    
    if len(positions) == 0 {
        return models.NewEnhancedPositionsResponse(), nil
    }
    
    // 2. Use base provider's conversion logic
    return p.BaseProviderImpl.ConvertPositionsToEnhanced(positions), nil
}
```

---

## 10. Frontend UI Integration (Optional)

The frontend UI **automatically discovers** provider types from the backend via the `/api/providers/types` endpoint. When you add a provider to `provider_types.go`, it will automatically appear in:

- Provider selection grid (Add Provider dialog)
- Account type selection (live/paper)
- Dynamic credential form generation
- Service routing dropdowns

However, there are some **optional cosmetic updates** for full branding integration.

### 10.1 What Works Automatically

| Feature | Source | Automatic? |
|---------|--------|------------|
| Provider appears in selection grid | `provider_types.go` → API | ✅ Yes |
| Account type options (live/paper) | `supports_account_types` field | ✅ Yes |
| Credential form fields | `credential_fields` definition | ✅ Yes |
| Capabilities for service routing | `capabilities` REST/streaming arrays | ✅ Yes |
| Test connection functionality | `TestCredentials()` method | ✅ Yes |

### 10.2 What Requires Frontend Updates (Optional)

| Feature | What's Needed | Without It |
|---------|---------------|------------|
| Provider logo | SVG file + whitelist entry | Shows generic server icon |
| About page listing | Update hardcoded list | Provider not mentioned |

### 10.3 Adding Provider Logo

**Step 1: Create the logo file**

Add an SVG logo to the public logos directory:

```
trade-app/public/logos/{yourprovider}.svg
```

The logo should be:
- SVG format (for scalability)
- Reasonable size (around 100x100 or similar aspect ratio)
- Work well on both light and dark backgrounds

**Step 2: Add to SVG whitelist**

The frontend has a whitelist of providers with SVG logos. Add your provider key to the `svgProviders` array in these files:

**File 1:** `trade-app/src/components/settings/ProvidersTab.vue` (around line 639)
```javascript
const svgProviders = ['alpaca', 'tradier', 'public', 'tastytrade', 'yourprovider'];
```

**File 2:** `trade-app/src/components/setup/WizardStep2Providers.vue` (around line 504)
```javascript
const svgProviders = ['alpaca', 'tradier', 'public', 'tastytrade', 'yourprovider'];
```

**File 3:** `trade-app/src/components/setup/WizardStep5Complete.vue` (around line 297)
```javascript
const svgProviders = ['alpaca', 'tradier', 'public', 'tastytrade', 'yourprovider'];
```

### 10.4 Updating the About Section (Optional)

The Settings dialog has a hardcoded "Supported Providers" section for marketing purposes.

**File:** `trade-app/src/components/SettingsDialog.vue` (around lines 125-152)

Add your provider to the list:

```vue
<div class="about-section">
  <h4>Supported Providers</h4>
  <ul>
    <li>Alpaca - Commission-free trading API</li>
    <li>Tradier - Brokerage API with streaming</li>
    <li>TastyTrade - Options-focused platform</li>
    <li>Public.com - Social investing platform</li>
    <li>YourProvider - Your provider description</li>  <!-- Add this -->
  </ul>
</div>
```

### 10.5 Frontend Files Summary

Here's a complete checklist of frontend files to update for full UI integration:

| File | Change | Required? |
|------|--------|-----------|
| `trade-app/public/logos/{provider}.svg` | Add logo file | Optional |
| `trade-app/src/components/settings/ProvidersTab.vue` | Add to `svgProviders` array | Optional |
| `trade-app/src/components/setup/WizardStep2Providers.vue` | Add to `svgProviders` array | Optional |
| `trade-app/src/components/setup/WizardStep5Complete.vue` | Add to `svgProviders` array | Optional |
| `trade-app/src/components/SettingsDialog.vue` | Update About section | Optional |

### 10.6 How the UI Discovers Providers

The frontend dynamically loads provider information via API:

```
┌─────────────────────────────────────────────────────────────┐
│                        BACKEND                               │
│                                                              │
│  provider_types.go                                           │
│  └── ProviderTypes map (source of truth)                    │
│      ├── name, description                                   │
│      ├── supports_account_types                              │
│      ├── capabilities (rest/streaming)                       │
│      └── credential_fields (per account_type)                │
│                                                              │
│  GET /api/providers/types → returns ProviderTypes            │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                        FRONTEND                              │
│                                                              │
│  api.js                                                      │
│  └── getProviderTypes() → fetches provider definitions      │
│                                                              │
│  ProvidersTab.vue / WizardStep2Providers.vue                │
│  └── Renders UI dynamically:                                │
│      ├── Provider cards from providerTypes keys             │
│      ├── Account type buttons from supports_account_types   │
│      └── Form fields from credential_fields[account_type]   │
└─────────────────────────────────────────────────────────────┘
```

**API Endpoints Used:**

| Endpoint | Purpose |
|----------|---------|
| `GET /api/providers/types` | Get all provider type definitions |
| `GET /api/providers/instances` | Get configured provider instances |
| `GET /api/providers/available` | Get active instances with capabilities |
| `GET /api/providers/config` | Get current service routing config |
| `POST /api/providers/instances` | Create new provider instance |
| `POST /api/providers/instances/test` | Test provider credentials |

---

## Summary

Adding a new provider involves:

1. **Create provider file** implementing all `base.Provider` interface methods
2. **Transform data** from broker format to standard models
3. **Register provider type** in `provider_types.go` with capabilities and credential fields
4. **Add factory case** in `manager.go` to instantiate your provider
5. **Test thoroughly** using unit and integration tests
6. **Configure usage** by adding provider instance and updating `provider_config.json`

The key principle is that **providers are never called directly** - they are always accessed through the `ProviderManager` which routes based on service type configuration. This allows flexible mixing of providers based on their strengths (e.g., one broker for trading, another for data).

For reference implementations, see:
- `trade-backend-go/internal/providers/tradier/tradier.go` - Full-featured provider with streaming
- `trade-backend-go/internal/providers/tastytrade/tastytrade.go` - Provider with Greeks streaming
- `trade-backend-go/internal/providers/alpaca/alpaca.go` - Provider with paper trading support
