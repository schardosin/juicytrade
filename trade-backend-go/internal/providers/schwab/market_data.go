package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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

// =============================================================================
// Expiration Dates
// =============================================================================

// GetExpirationDates gets available option expiration dates for a symbol.
// Uses GET /marketdata/v1/chains?symbol={sym} and extracts unique dates from
// the callExpDateMap keys (format "2025-01-17:180" → date:DTE).
func (s *SchwabProvider) GetExpirationDates(ctx context.Context, symbol string) ([]map[string]interface{}, error) {
	reqURL := s.buildMarketDataURL("/chains?symbol=" + url.QueryEscape(symbol))

	body, _, err := s.doAuthenticatedRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("schwab: GetExpirationDates failed for %s: %w", symbol, err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("schwab: failed to parse chains response: %w", err)
	}

	// Extract unique dates from callExpDateMap keys (preferred) or putExpDateMap
	dateMap := extractMap(response, "callExpDateMap")
	if dateMap == nil {
		dateMap = extractMap(response, "putExpDateMap")
	}
	if dateMap == nil {
		return []map[string]interface{}{}, nil
	}

	seen := make(map[string]bool)
	var result []map[string]interface{}

	for key := range dateMap {
		// Key format: "2025-01-17:180" (date:DTE)
		parts := strings.SplitN(key, ":", 2)
		date := parts[0]
		if seen[date] {
			continue
		}
		seen[date] = true

		entry := map[string]interface{}{
			"date": date,
		}
		if len(parts) == 2 {
			if dte, err := strconv.Atoi(parts[1]); err == nil {
				entry["dte"] = dte
			}
		}
		result = append(result, entry)
	}

	s.logger.Debug("GetExpirationDates completed", "symbol", symbol, "count", len(result))
	return result, nil
}

// =============================================================================
// Options Chain
// =============================================================================

// GetOptionsChainBasic gets an options chain without emphasis on Greeks.
// Uses GET /marketdata/v1/chains with date filtering and optional strike/type params.
func (s *SchwabProvider) GetOptionsChainBasic(ctx context.Context, symbol, expiry string, underlyingPrice *float64, strikeCount int, optionType, underlyingSymbol *string) ([]*models.OptionContract, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	if expiry != "" {
		params.Set("fromDate", expiry)
		params.Set("toDate", expiry)
	}
	if strikeCount > 0 {
		params.Set("strikeCount", strconv.Itoa(strikeCount))
	}
	if optionType != nil && *optionType != "" {
		params.Set("optionType", strings.ToUpper(*optionType))
	}

	reqURL := s.buildMarketDataURL("/chains?" + params.Encode())

	body, _, err := s.doAuthenticatedRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("schwab: GetOptionsChainBasic failed for %s: %w", symbol, err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("schwab: failed to parse chains response: %w", err)
	}

	contracts := transformSchwabOptionsChain(response, symbol, false)

	s.logger.Debug("GetOptionsChainBasic completed", "symbol", symbol, "contracts", len(contracts))
	return contracts, nil
}

// GetOptionsChainSmart gets an options chain with configurable Greek inclusion.
// Uses the same chains endpoint with additional parameters for ATM filtering.
func (s *SchwabProvider) GetOptionsChainSmart(ctx context.Context, symbol, expiry string, underlyingPrice *float64, atmRange int, includeGreeks, strikesOnly bool) ([]*models.OptionContract, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	if expiry != "" {
		params.Set("fromDate", expiry)
		params.Set("toDate", expiry)
	}
	if strikesOnly && atmRange > 0 {
		params.Set("strikeCount", strconv.Itoa(atmRange))
	}
	params.Set("includeUnderlyingQuote", "true")

	reqURL := s.buildMarketDataURL("/chains?" + params.Encode())

	body, _, err := s.doAuthenticatedRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("schwab: GetOptionsChainSmart failed for %s: %w", symbol, err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("schwab: failed to parse chains response: %w", err)
	}

	contracts := transformSchwabOptionsChain(response, symbol, includeGreeks)

	s.logger.Debug("GetOptionsChainSmart completed",
		"symbol", symbol, "contracts", len(contracts), "includeGreeks", includeGreeks)
	return contracts, nil
}

// =============================================================================
// Greeks Batch
// =============================================================================

// GetOptionsGreeksBatch gets Greeks for multiple option symbols via the quotes endpoint.
// Converts OCC symbols to Schwab format, fetches quotes, and extracts Greek values.
func (s *SchwabProvider) GetOptionsGreeksBatch(ctx context.Context, optionSymbols []string) (map[string]map[string]interface{}, error) {
	if len(optionSymbols) == 0 {
		return make(map[string]map[string]interface{}), nil
	}

	// Convert OCC symbols to Schwab format for the API request
	schwabSymbols := make([]string, len(optionSymbols))
	schwabToOCC := make(map[string]string, len(optionSymbols))
	for i, occ := range optionSymbols {
		schwab := convertOCCToSchwab(occ)
		schwabSymbols[i] = schwab
		schwabToOCC[schwab] = occ
	}

	reqURL := s.buildMarketDataURL("/quotes?symbols=" + url.QueryEscape(strings.Join(schwabSymbols, ",")))

	body, _, err := s.doAuthenticatedRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("schwab: GetOptionsGreeksBatch failed: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("schwab: failed to parse option quotes response: %w", err)
	}

	result := make(map[string]map[string]interface{}, len(optionSymbols))

	for schwabSym, occSym := range schwabToOCC {
		symbolData := findSymbolData(response, schwabSym)
		if symbolData == nil {
			continue
		}
		quoteData := extractMap(symbolData, "quote")
		if quoteData == nil {
			quoteData = symbolData
		}

		greeks := make(map[string]interface{})
		if v, ok := extractFloat64(quoteData, "delta"); ok {
			greeks["delta"] = v
		}
		if v, ok := extractFloat64(quoteData, "gamma"); ok {
			greeks["gamma"] = v
		}
		if v, ok := extractFloat64(quoteData, "theta"); ok {
			greeks["theta"] = v
		}
		if v, ok := extractFloat64(quoteData, "vega"); ok {
			greeks["vega"] = v
		}
		if v, ok := extractFloat64(quoteData, "rho"); ok {
			greeks["rho"] = v
		}
		if v, ok := extractFloat64(quoteData, "impliedVolatility"); ok {
			greeks["implied_volatility"] = v
		} else if v, ok := extractFloat64(quoteData, "volatility"); ok {
			greeks["implied_volatility"] = v
		}
		if v, ok := extractFloat64(quoteData, "bidPrice"); ok {
			greeks["bid"] = v
		}
		if v, ok := extractFloat64(quoteData, "askPrice"); ok {
			greeks["ask"] = v
		}
		if v, ok := extractFloat64(quoteData, "lastPrice"); ok {
			greeks["last"] = v
		}

		result[occSym] = greeks
	}

	s.logger.Debug("GetOptionsGreeksBatch completed",
		"requested", len(optionSymbols), "returned", len(result))
	return result, nil
}

// =============================================================================
// Options Chain Transformation
// =============================================================================

// transformSchwabOptionsChain transforms a Schwab chains response into OptionContract models.
//
// Schwab chain response structure:
//
//	{
//	  "callExpDateMap": {
//	    "2025-01-17:180": {
//	      "150.0": [{"putCall":"CALL", "symbol":"AAPL  250117C00150000", ...}]
//	    }
//	  },
//	  "putExpDateMap": { ... same structure ... }
//	}
func transformSchwabOptionsChain(response map[string]interface{}, underlyingSymbol string, includeGreeks bool) []*models.OptionContract {
	var contracts []*models.OptionContract

	for _, mapKey := range []string{"callExpDateMap", "putExpDateMap"} {
		expDateMap := extractMap(response, mapKey)
		if expDateMap == nil {
			continue
		}

		for dateKey, strikesRaw := range expDateMap {
			// dateKey format: "2025-01-17:180"
			dateParts := strings.SplitN(dateKey, ":", 2)
			expirationDate := dateParts[0]

			strikesMap, ok := strikesRaw.(map[string]interface{})
			if !ok {
				continue
			}

			for _, contractsRaw := range strikesMap {
				contractList, ok := contractsRaw.([]interface{})
				if !ok {
					continue
				}

				for _, contractRaw := range contractList {
					contractData, ok := contractRaw.(map[string]interface{})
					if !ok {
						continue
					}

					contract := transformSchwabOptionContract(contractData, underlyingSymbol, expirationDate, includeGreeks)
					if contract != nil {
						contracts = append(contracts, contract)
					}
				}
			}
		}
	}

	return contracts
}

// transformSchwabOptionContract transforms a single Schwab option contract into an OptionContract model.
func transformSchwabOptionContract(data map[string]interface{}, underlyingSymbol, expirationDate string, includeGreeks bool) *models.OptionContract {
	// Extract and convert symbol from Schwab to OCC format
	schwabSymbol, _ := extractString(data, "symbol")
	occSymbol := convertSchwabOptionToOCC(schwabSymbol)

	// Determine option type
	putCall, _ := extractString(data, "putCall")
	optionType := strings.ToLower(putCall)
	if optionType != "call" && optionType != "put" {
		optionType = "call" // default
	}

	strikePrice, _ := extractFloat64(data, "strikePrice")

	contract := &models.OptionContract{
		Symbol:           occSymbol,
		UnderlyingSymbol: underlyingSymbol,
		ExpirationDate:   expirationDate,
		StrikePrice:      strikePrice,
		Type:             optionType,
		Bid:              extractFloat64Ptr(data, "bid"),
		Ask:              extractFloat64Ptr(data, "ask"),
		ClosePrice:       extractFloat64Ptr(data, "closePrice"),
		Volume:           extractIntPtr(data, "totalVolume"),
		OpenInterest:     extractIntPtr(data, "openInterest"),
	}

	if includeGreeks {
		contract.Delta = extractFloat64Ptr(data, "delta")
		contract.Gamma = extractFloat64Ptr(data, "gamma")
		contract.Theta = extractFloat64Ptr(data, "theta")
		contract.Vega = extractFloat64Ptr(data, "vega")
		contract.ImpliedVolatility = extractFloat64Ptr(data, "volatility")
	}

	return contract
}

// =============================================================================
// Historical Bars
// =============================================================================

// schwabTimeframeParams holds the Schwab API parameters for a given timeframe.
type schwabTimeframeParams struct {
	periodType    string
	frequencyType string
	frequency     string
}

// mapTimeframe maps a JuicyTrade timeframe string to Schwab API parameters.
func mapTimeframe(timeframe string) schwabTimeframeParams {
	switch strings.ToLower(timeframe) {
	case "1min":
		return schwabTimeframeParams{"day", "minute", "1"}
	case "5min":
		return schwabTimeframeParams{"day", "minute", "5"}
	case "15min":
		return schwabTimeframeParams{"day", "minute", "15"}
	case "30min":
		return schwabTimeframeParams{"day", "minute", "30"}
	case "1hour", "1h", "60min":
		// Schwab doesn't have hourly — approximate with 30-min
		return schwabTimeframeParams{"day", "minute", "30"}
	case "1d", "daily":
		return schwabTimeframeParams{"year", "daily", "1"}
	case "1w", "weekly":
		return schwabTimeframeParams{"year", "weekly", "1"}
	default:
		// Default to daily
		return schwabTimeframeParams{"year", "daily", "1"}
	}
}

// GetHistoricalBars gets historical OHLCV bars for charting.
// Uses GET /marketdata/v1/pricehistory with timeframe mapping.
func (s *SchwabProvider) GetHistoricalBars(ctx context.Context, symbol, timeframe string, startDate, endDate *string, limit int) ([]map[string]interface{}, error) {
	tf := mapTimeframe(timeframe)

	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("periodType", tf.periodType)
	params.Set("frequencyType", tf.frequencyType)
	params.Set("frequency", tf.frequency)

	// Add date range if provided (convert YYYY-MM-DD to epoch millis)
	if startDate != nil && *startDate != "" {
		if ms, err := dateToEpochMs(*startDate); err == nil {
			params.Set("startDate", strconv.FormatInt(ms, 10))
		}
	}
	if endDate != nil && *endDate != "" {
		if ms, err := dateToEpochMs(*endDate); err == nil {
			params.Set("endDate", strconv.FormatInt(ms, 10))
		}
	}

	reqURL := s.buildMarketDataURL("/pricehistory?" + params.Encode())

	body, _, err := s.doAuthenticatedRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("schwab: GetHistoricalBars failed for %s: %w", symbol, err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("schwab: failed to parse pricehistory response: %w", err)
	}

	// Extract candles array
	candlesRaw, ok := response["candles"]
	if !ok {
		return []map[string]interface{}{}, nil
	}
	candlesArr, ok := candlesRaw.([]interface{})
	if !ok {
		return []map[string]interface{}{}, nil
	}

	var bars []map[string]interface{}
	for _, c := range candlesArr {
		candle, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		bar := map[string]interface{}{}
		if v, ok := extractFloat64(candle, "open"); ok {
			bar["open"] = v
		}
		if v, ok := extractFloat64(candle, "high"); ok {
			bar["high"] = v
		}
		if v, ok := extractFloat64(candle, "low"); ok {
			bar["low"] = v
		}
		if v, ok := extractFloat64(candle, "close"); ok {
			bar["close"] = v
		}
		if v, ok := extractFloat64(candle, "volume"); ok {
			bar["volume"] = v
		}
		// Convert datetime epoch millis to ISO 8601
		if dtMs, ok := extractFloat64(candle, "datetime"); ok && dtMs > 0 {
			bar["timestamp"] = time.UnixMilli(int64(dtMs)).Format(time.RFC3339)
		}

		bars = append(bars, bar)
	}

	// Apply limit — return only the last N bars
	if limit > 0 && len(bars) > limit {
		bars = bars[len(bars)-limit:]
	}

	s.logger.Debug("GetHistoricalBars completed",
		"symbol", symbol, "timeframe", timeframe, "bars", len(bars))
	return bars, nil
}

// dateToEpochMs converts a "YYYY-MM-DD" date string to epoch milliseconds.
func dateToEpochMs(dateStr string) (int64, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}

// =============================================================================
// Market Calendar
// =============================================================================

// GetNextMarketDate returns the next US equity market trading date.
// Uses a simple business-day calculation: advances to the next weekday,
// skipping weekends. Does not account for market holidays.
func (s *SchwabProvider) GetNextMarketDate(ctx context.Context) (string, error) {
	// Use US Eastern time for market hours
	eastern, err := time.LoadLocation("America/New_York")
	if err != nil {
		// Fallback to UTC if timezone data not available
		eastern = time.UTC
	}

	now := time.Now().In(eastern)

	// If it's a weekday and before market close (4:00 PM ET), today is valid
	if isWeekday(now) && now.Hour() < 16 {
		return now.Format("2006-01-02"), nil
	}

	// Advance to next day and find next weekday
	next := now.AddDate(0, 0, 1)
	for !isWeekday(next) {
		next = next.AddDate(0, 0, 1)
	}

	return next.Format("2006-01-02"), nil
}

// isWeekday returns true if the given time falls on a weekday (Mon-Fri).
func isWeekday(t time.Time) bool {
	day := t.Weekday()
	return day != time.Saturday && day != time.Sunday
}
