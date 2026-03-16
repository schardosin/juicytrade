package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"trade-backend-go/internal/models"
)

// =============================================================================
// Stock Quote Methods
// =============================================================================

// GetStockQuote gets the latest stock quote for a single symbol.
// Uses the Schwab Market Data API: GET /marketdata/v1/quotes?symbols={symbol}
func (s *SchwabProvider) GetStockQuote(ctx context.Context, symbol string) (*models.StockQuote, error) {
	reqURL := s.buildMarketDataURL("/quotes?symbols=" + url.QueryEscape(symbol))

	body, _, err := s.doAuthenticatedRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("schwab: GetStockQuote failed for %s: %w", symbol, err)
	}

	// Parse response — keyed by symbol
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("schwab: failed to parse quote response: %w", err)
	}

	// Look up the symbol in the response (case-insensitive — Schwab returns uppercase keys)
	symbolData := findSymbolData(response, symbol)
	if symbolData == nil {
		return nil, fmt.Errorf("schwab: no quote data returned for %s", symbol)
	}

	quote := transformSchwabQuote(symbolData, symbol)

	s.logger.Debug("GetStockQuote completed",
		"symbol", symbol,
		"hasBid", quote.Bid != nil,
		"hasAsk", quote.Ask != nil,
		"hasLast", quote.Last != nil,
	)

	return quote, nil
}

// GetStockQuotes gets stock quotes for multiple symbols in a single request.
// Uses the Schwab Market Data API: GET /marketdata/v1/quotes?symbols={csv}
func (s *SchwabProvider) GetStockQuotes(ctx context.Context, symbols []string) (map[string]*models.StockQuote, error) {
	if len(symbols) == 0 {
		return make(map[string]*models.StockQuote), nil
	}

	reqURL := s.buildMarketDataURL("/quotes?symbols=" + url.QueryEscape(strings.Join(symbols, ",")))

	body, _, err := s.doAuthenticatedRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("schwab: GetStockQuotes failed: %w", err)
	}

	// Parse response — keyed by symbol
	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("schwab: failed to parse quotes response: %w", err)
	}

	result := make(map[string]*models.StockQuote, len(symbols))
	for _, symbol := range symbols {
		symbolData := findSymbolData(response, symbol)
		if symbolData == nil {
			continue // Symbol not in response — skip
		}
		result[symbol] = transformSchwabQuote(symbolData, symbol)
	}

	s.logger.Debug("GetStockQuotes completed",
		"requested", len(symbols),
		"returned", len(result),
	)

	return result, nil
}

// =============================================================================
// Quote Transformation Helpers
// =============================================================================

// findSymbolData finds the data for a symbol in the Schwab response map.
// Schwab returns uppercase symbol keys, so we do a case-insensitive lookup.
func findSymbolData(response map[string]interface{}, symbol string) map[string]interface{} {
	// Try exact match first (most common case)
	if data, ok := response[symbol]; ok {
		if m, ok := data.(map[string]interface{}); ok {
			return m
		}
	}

	// Try uppercase
	upper := strings.ToUpper(symbol)
	if data, ok := response[upper]; ok {
		if m, ok := data.(map[string]interface{}); ok {
			return m
		}
	}

	return nil
}

// transformSchwabQuote transforms a Schwab quote response into a StockQuote model.
//
// Schwab response structure per symbol:
//
//	{
//	  "assetMainType": "EQUITY",
//	  "quote": {
//	    "bidPrice": 150.0,
//	    "askPrice": 150.5,
//	    "lastPrice": 150.25,
//	    "openPrice": 149.0,
//	    "highPrice": 151.0,
//	    "lowPrice": 148.5,
//	    "closePrice": 149.5,
//	    "totalVolume": 50000000,
//	    "mark": 150.25,
//	    "netChange": 0.75,
//	    "quoteTimeInLong": 1715908546000,
//	    "tradeTimeInLong": 1715908545000
//	  }
//	}
func transformSchwabQuote(data map[string]interface{}, symbol string) *models.StockQuote {
	// Extract the "quote" sub-object
	quoteData := extractMap(data, "quote")

	// If no "quote" sub-object, try using the data directly
	// (some endpoints may return a flat structure)
	if quoteData == nil {
		quoteData = data
	}

	bid := extractFloat64Ptr(quoteData, "bidPrice")
	ask := extractFloat64Ptr(quoteData, "askPrice")
	last := extractFloat64Ptr(quoteData, "lastPrice")

	// Build timestamp from quoteTimeInLong (epoch millis) or fall back to now
	timestamp := time.Now().Format(time.RFC3339)
	if quoteTimeMs, ok := extractFloat64(quoteData, "quoteTimeInLong"); ok && quoteTimeMs > 0 {
		t := time.UnixMilli(int64(quoteTimeMs))
		timestamp = t.Format(time.RFC3339)
	} else if tradeTimeMs, ok := extractFloat64(quoteData, "tradeTimeInLong"); ok && tradeTimeMs > 0 {
		t := time.UnixMilli(int64(tradeTimeMs))
		timestamp = t.Format(time.RFC3339)
	}

	return &models.StockQuote{
		Symbol:    symbol,
		Bid:       bid,
		Ask:       ask,
		Last:      last,
		Timestamp: timestamp,
	}
}

// =============================================================================
// JSON Extraction Helpers
//
// These helpers safely extract typed values from map[string]interface{},
// handling type mismatches and missing keys gracefully.
// =============================================================================

// extractMap extracts a nested map from a parent map.
func extractMap(data map[string]interface{}, key string) map[string]interface{} {
	if data == nil {
		return nil
	}
	if v, ok := data[key]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

// extractFloat64 extracts a float64 value from a map, returning (value, true) or (0, false).
// Handles both float64 and json.Number types.
func extractFloat64(data map[string]interface{}, key string) (float64, bool) {
	if data == nil {
		return 0, false
	}
	v, ok := data[key]
	if !ok || v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case json.Number:
		f, err := val.Float64()
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// extractFloat64Ptr extracts a *float64 from a map. Returns nil if key is missing or zero.
func extractFloat64Ptr(data map[string]interface{}, key string) *float64 {
	v, ok := extractFloat64(data, key)
	if !ok {
		return nil
	}
	return &v
}

// extractString extracts a string value from a map.
func extractString(data map[string]interface{}, key string) (string, bool) {
	if data == nil {
		return "", false
	}
	v, ok := data[key]
	if !ok || v == nil {
		return "", false
	}
	if s, ok := v.(string); ok {
		return s, true
	}
	return "", false
}

// extractInt extracts an int value from a map.
func extractInt(data map[string]interface{}, key string) (int, bool) {
	if data == nil {
		return 0, false
	}
	v, ok := data[key]
	if !ok || v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case float64:
		return int(val), true
	case int:
		return val, true
	case int64:
		return int(val), true
	case json.Number:
		i, err := val.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}

// extractIntPtr extracts an *int from a map. Returns nil if key is missing.
func extractIntPtr(data map[string]interface{}, key string) *int {
	v, ok := extractInt(data, key)
	if !ok {
		return nil
	}
	return &v
}
