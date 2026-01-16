package models

import (
	"fmt"
	"time"
)

// StockQuote represents a standardized stock quote model.
// Exact conversion of Python StockQuote class.
type StockQuote struct {
	Symbol    string   `json:"symbol"`
	Ask       *float64 `json:"ask"`
	Bid       *float64 `json:"bid"`
	Last      *float64 `json:"last"`
	Timestamp string   `json:"timestamp"`
}

// OptionContract represents a standardized option contract model.
// Exact conversion of Python OptionContract class.
type OptionContract struct {
	Symbol            string   `json:"symbol"`
	UnderlyingSymbol  string   `json:"underlying_symbol"`
	ExpirationDate    string   `json:"expiration_date"`
	StrikePrice       float64  `json:"strike_price"`
	Type              string   `json:"type"` // "call" or "put"
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

// Position represents a standardized position model.
// Exact conversion of Python Position class.
type Position struct {
	Symbol         string   `json:"symbol"`
	Qty            float64  `json:"qty"`
	Side           string   `json:"side"` // "long" or "short"
	MarketValue    float64  `json:"market_value"`
	CostBasis      float64  `json:"cost_basis"`
	UnrealizedPL   float64  `json:"unrealized_pl"`
	UnrealizedPLPC *float64 `json:"unrealized_plpc"`
	CurrentPrice   float64  `json:"current_price"`
	AvgEntryPrice  float64  `json:"avg_entry_price"`
	AssetClass     string   `json:"asset_class"` // "us_equity", "us_option", etc.
	LastdayPrice   *float64 `json:"lastday_price"`
	DateAcquired   *string  `json:"date_acquired"`

	// Option-specific fields
	UnderlyingSymbol *string  `json:"underlying_symbol"`
	OptionType       *string  `json:"option_type"` // "call" or "put"
	StrikePrice      *float64 `json:"strike_price"`
	ExpiryDate       *string  `json:"expiry_date"`
}

// OrderLeg represents a single leg in a multi-leg order
type OrderLeg struct {
	Symbol string  `json:"symbol"`
	Side   string  `json:"side"`
	Qty    float64 `json:"qty"`
}

// Order represents a standardized order model.
// Exact conversion of Python Order class.
type Order struct {
	ID           string   `json:"id"`
	Symbol       string   `json:"symbol"`
	AssetClass   string   `json:"asset_class"`
	Side         string   `json:"side"` // "buy" or "sell"
	Action       *string  `json:"action"`
	OrderType    string   `json:"order_type"` // "market", "limit", etc.
	Qty          float64  `json:"qty"`
	FilledQty    float64  `json:"filled_qty"`
	LimitPrice   *float64 `json:"limit_price"`
	StopPrice    *float64 `json:"stop_price"`
	AvgFillPrice *float64 `json:"avg_fill_price"`
	Status       string   `json:"status"`        // "new", "filled", "canceled", etc.
	TimeInForce  string   `json:"time_in_force"` // "day", "gtc", etc.
	SubmittedAt  string   `json:"submitted_at"`
	FilledAt     *string  `json:"filled_at"`

	// Multi-leg order support
	Legs []OrderLeg `json:"legs,omitempty"`
}

// ExpirationDate represents a standardized expiration date model.
// Exact conversion of Python ExpirationDate class.
type ExpirationDate struct {
	Date         string `json:"date"`
	DaysToExpiry *int   `json:"days_to_expiry"`
}

// MarketData represents standardized market data model for streaming.
// Exact conversion of Python MarketData class.
type MarketData struct {
	Symbol      string                 `json:"symbol"`
	DataType    string                 `json:"data_type"` // "quote", "trade", etc.
	Timestamp   string                 `json:"timestamp"`
	TimestampMs *int64                 `json:"timestamp_ms"`
	Data        map[string]interface{} `json:"data"`
}

// NewMarketData creates a new MarketData instance with auto-generated timestamp_ms
// Replicates Python's __init__ behavior
func NewMarketData(symbol, dataType, timestamp string, data map[string]interface{}) *MarketData {
	md := &MarketData{
		Symbol:    symbol,
		DataType:  dataType,
		Timestamp: timestamp,
		Data:      data,
	}

	// Auto-generate timestamp_ms if not provided (same as Python)
	now := time.Now().UnixMilli()
	md.TimestampMs = &now

	return md
}

// AgeSeconds returns the age of this data in seconds.
// Exact conversion of Python age_seconds property.
func (md *MarketData) AgeSeconds() float64 {
	if md.TimestampMs != nil {
		now := time.Now().UnixMilli()
		return float64(now-*md.TimestampMs) / 1000.0
	}
	return 0
}

// IsFresh checks if data is fresh (not older than maxAgeSeconds).
// Exact conversion of Python is_fresh property.
func (md *MarketData) IsFresh(maxAgeSeconds float64) bool {
	if maxAgeSeconds == 0 {
		maxAgeSeconds = 30.0 // Default same as Python
	}
	return md.AgeSeconds() <= maxAgeSeconds
}

// SymbolSearchResult represents a standardized symbol search result model.
// Exact conversion of Python SymbolSearchResult class.
type SymbolSearchResult struct {
	Symbol      string `json:"symbol"`
	Description string `json:"description"`
	Exchange    string `json:"exchange"`
	Type        string `json:"type"` // "stock", "etf", "index", "option"
}

// Account represents standardized account information model.
// Exact conversion of Python Account class.
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

// NewAccount creates a new Account with default currency
func NewAccount() *Account {
	return &Account{
		Currency: "USD", // Default same as Python
	}
}

// ApiResponse represents standardized API response wrapper.
// Exact conversion of Python ApiResponse class.
type ApiResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data"`
	Error     *string     `json:"error"`
	Message   *string     `json:"message"`
	Timestamp string      `json:"timestamp"`
}

// NewApiResponse creates a new ApiResponse with current timestamp
// Replicates Python's default timestamp behavior
func NewApiResponse(success bool, data interface{}, error, message *string) *ApiResponse {
	return &ApiResponse{
		Success:   success,
		Data:      data,
		Error:     error,
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// Request models for API endpoints - exact conversions

// SymbolRequest represents a request for multiple symbols
type SymbolRequest struct {
	Symbols []string `json:"symbols"`
}

// OrderRequest represents a single order request
type OrderRequest struct {
	Symbol      string   `json:"symbol"`
	Side        string   `json:"side"`
	Qty         float64  `json:"qty"`
	OrderType   string   `json:"order_type"`
	TimeInForce string   `json:"time_in_force"`
	LimitPrice  *float64 `json:"limit_price"`
	StopPrice   *float64 `json:"stop_price"`
	IsShortSell *bool    `json:"is_short_sell"`
}

// NewOrderRequest creates a new OrderRequest with defaults
func NewOrderRequest() *OrderRequest {
	return &OrderRequest{
		OrderType:   "market", // Default same as Python
		TimeInForce: "day",    // Default same as Python
	}
}

// MultiLegOrderRequest represents a multi-leg order request
type MultiLegOrderRequest struct {
	Legs        []map[string]interface{} `json:"legs"`
	Action      *string                  `json:"action"`
	Qty         int                      `json:"qty"`
	OrderType   string                   `json:"order_type"`
	TimeInForce string                   `json:"time_in_force"`
	LimitPrice  *float64                 `json:"limit_price"`
	StopPrice   *float64                 `json:"stop_price"`

	// Equity order fields
	Symbol      *string `json:"symbol"`
	Side        *string `json:"side"`
	IsShortSell *bool   `json:"is_short_sell"`
}

// NewMultiLegOrderRequest creates a new MultiLegOrderRequest with defaults
func NewMultiLegOrderRequest() *MultiLegOrderRequest {
	return &MultiLegOrderRequest{
		Qty:         1,       // Default same as Python
		OrderType:   "limit", // Default same as Python
		TimeInForce: "day",   // Default same as Python
	}
}

// PositionGroup represents position group model for order chain grouping
type PositionGroup struct {
	ID                  string     `json:"id"`
	Symbol              string     `json:"symbol"`
	Strategy            string     `json:"strategy"`
	AssetClass          string     `json:"asset_class"`
	TotalQty            float64    `json:"total_qty"`
	TotalCostBasis      float64    `json:"total_cost_basis"`
	TotalMarketValue    float64    `json:"total_market_value"`
	TotalUnrealizedPL   float64    `json:"total_unrealized_pl"`
	TotalUnrealizedPLPC float64    `json:"total_unrealized_plpc"`
	PLDay               float64    `json:"pl_day"`
	PLOpen              float64    `json:"pl_open"`
	Legs                []Position `json:"legs"`
	OrderDate           *string    `json:"order_date"`
	ExpirationDate      *string    `json:"expiration_date"`
	DTE                 *int       `json:"dte"`
	OrderChainID        *string    `json:"order_chain_id"`
}

// HistoricalTrade represents historical trade data from broker history API
type HistoricalTrade struct {
	ID         string   `json:"id"`
	Symbol     string   `json:"symbol"`
	Side       string   `json:"side"`
	Qty        float64  `json:"qty"`
	Price      float64  `json:"price"`
	Date       string   `json:"date"`
	OrderID    *string  `json:"order_id"`
	Commission *float64 `json:"commission"`
	AssetClass *string  `json:"asset_class"`
}

// Provider Instance Management Models - exact conversions

// CreateProviderInstanceRequest represents a request for creating a new provider instance
type CreateProviderInstanceRequest struct {
	ProviderType string            `json:"provider_type"`
	AccountType  string            `json:"account_type"`
	DisplayName  string            `json:"display_name"`
	Credentials  map[string]string `json:"credentials"`
}

// UpdateProviderInstanceRequest represents a request for updating a provider instance
type UpdateProviderInstanceRequest struct {
	DisplayName *string            `json:"display_name"`
	Credentials *map[string]string `json:"credentials"`
}

// ProviderInstanceResponse represents response model for provider instance data
type ProviderInstanceResponse struct {
	InstanceID   string `json:"instance_id"`
	Active       bool   `json:"active"`
	ProviderType string `json:"provider_type"`
	AccountType  string `json:"account_type"`
	DisplayName  string `json:"display_name"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	// Note: credentials are not included in response for security
}

// TestProviderConnectionRequest represents a request for testing provider connection
type TestProviderConnectionRequest struct {
	ProviderType string            `json:"provider_type"`
	AccountType  string            `json:"account_type"`
	Credentials  map[string]string `json:"credentials"`
}

// TestProviderConnectionResponse represents response model for provider connection test
type TestProviderConnectionResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// IVxExpiration represents individual expiration data for IVx calculations
type IVxExpiration struct {
	ExpirationDate      string  `json:"expiration_date"`
	DaysToExpiration    float64 `json:"days_to_expiration"`
	IVxPercent          float64 `json:"ivx_percent"`
	ExpectedMoveDollars float64 `json:"expected_move_dollars"`
	CalculationMethod   string  `json:"calculation_method"`
	OptionsCount        int     `json:"options_count"`
	IsExpired           bool    `json:"is_expired"`
}

// IVxResponse represents the complete IVx API response data
type IVxResponse struct {
	Symbol          string          `json:"symbol"`
	Expirations     []IVxExpiration `json:"expirations"`
	Cached          bool            `json:"cached"`
	CalculationTime *float64        `json:"calculation_time,omitempty"`
}

// OrderEvent represents a real-time order event from broker WebSocket streams
type OrderEvent struct {
	ID                interface{} `json:"id"`                         // Can be string or number depending on API version
	Event             string      `json:"event"`                      // "order"
	Status            string      `json:"status"`                     // "open", "pending", "filled", "canceled", "rejected", etc.
	NormalizedEvent   string      `json:"normalized_event,omitempty"` // "order_submitted", "order_filled", "order_partially_filled", "order_cancelled"
	Type              string      `json:"type"`                       // "market", "limit", etc.
	Symbol            string      `json:"symbol,omitempty"`
	Side              string      `json:"side,omitempty"`
	Quantity          float64     `json:"quantity,omitempty"`
	Price             float64     `json:"price,omitempty"`
	StopPrice         float64     `json:"stop_price,omitempty"`
	AvgFillPrice      float64     `json:"avg_fill_price,omitempty"`
	ExecutedQuantity  float64     `json:"executed_quantity,omitempty"`
	LastFillQuantity  float64     `json:"last_fill_quantity,omitempty"`
	LastFillPrice     float64     `json:"last_fill_price,omitempty"`
	RemainingQuantity float64     `json:"remaining_quantity,omitempty"`
	TransactionDate   string      `json:"transaction_date"`
	CreateDate        string      `json:"create_date"`
	Account           string      `json:"account"`
	ParentID          interface{} `json:"parent_id,omitempty"` // For child orders in multi-leg orders
}

// GetIDAsString returns the order ID as a string
func (e *OrderEvent) GetIDAsString() string {
	if e.ID == nil {
		return ""
	}
	switch v := e.ID.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// OrderEventMessage represents the WebSocket message format for order events
type OrderEventMessage struct {
	Type string     `json:"type"` // "order_event"
	Data OrderEvent `json:"data"`
}
