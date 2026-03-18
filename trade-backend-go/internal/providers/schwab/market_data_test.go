package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Schwab quote response fixtures
// =============================================================================

const schwabQuoteResponseAAPL = `{
	"AAPL": {
		"assetMainType": "EQUITY",
		"assetSubType": "COE",
		"ssid": 1973757747,
		"symbol": "AAPL",
		"quote": {
			"bidPrice": 150.0,
			"askPrice": 150.5,
			"lastPrice": 150.25,
			"openPrice": 149.0,
			"highPrice": 151.0,
			"lowPrice": 148.5,
			"closePrice": 149.5,
			"totalVolume": 50000000,
			"mark": 150.25,
			"netChange": 0.75,
			"netPercentChange": 0.502,
			"quoteTimeInLong": 1715908546000,
			"tradeTimeInLong": 1715908545000,
			"52WeekHigh": 199.62,
			"52WeekLow": 164.08
		},
		"reference": {
			"description": "Apple Inc",
			"exchange": "Q",
			"exchangeName": "NASDAQ"
		}
	}
}`

const schwabQuoteResponseMultiple = `{
	"AAPL": {
		"assetMainType": "EQUITY",
		"quote": {
			"bidPrice": 150.0,
			"askPrice": 150.5,
			"lastPrice": 150.25,
			"quoteTimeInLong": 1715908546000
		}
	},
	"MSFT": {
		"assetMainType": "EQUITY",
		"quote": {
			"bidPrice": 420.0,
			"askPrice": 420.5,
			"lastPrice": 420.25,
			"quoteTimeInLong": 1715908546000
		}
	},
	"GOOGL": {
		"assetMainType": "EQUITY",
		"quote": {
			"bidPrice": 175.0,
			"askPrice": 175.5,
			"lastPrice": 175.25,
			"quoteTimeInLong": 1715908546000
		}
	}
}`

// =============================================================================
// GetStockQuote tests
// =============================================================================

func TestGetStockQuote_Success(t *testing.T) {
	srv := newMultiEndpointServer(testServerOptions{
		tokenStatus: http.StatusOK,
		tokenBody:   validTokenBody,
	})
	// Override the handler to also serve quote requests
	srv.Close()

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/quotes"):
			// Verify symbols query param
			symbols := r.URL.Query().Get("symbols")
			if symbols != "AAPL" {
				t.Errorf("expected symbols=AAPL, got %s", symbols)
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabQuoteResponseAAPL)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	quote, err := p.GetStockQuote(context.Background(), "AAPL")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if quote.Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %s", quote.Symbol)
	}
	if quote.Bid == nil || *quote.Bid != 150.0 {
		t.Errorf("expected bid 150.0, got %v", quote.Bid)
	}
	if quote.Ask == nil || *quote.Ask != 150.5 {
		t.Errorf("expected ask 150.5, got %v", quote.Ask)
	}
	if quote.Last == nil || *quote.Last != 150.25 {
		t.Errorf("expected last 150.25, got %v", quote.Last)
	}
	if quote.Timestamp == "" {
		t.Error("expected timestamp to be set")
	}
	// Verify timestamp was parsed from quoteTimeInLong
	parsed, err := time.Parse(time.RFC3339, quote.Timestamp)
	if err != nil {
		t.Errorf("timestamp is not valid RFC3339: %s", quote.Timestamp)
	}
	if parsed.Year() != 2024 {
		t.Errorf("expected year 2024 from epoch 1715908546000, got %d", parsed.Year())
	}
}

func TestGetStockQuote_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/quotes"):
			// Return empty response (no matching symbol)
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	_, err := p.GetStockQuote(context.Background(), "INVALID")
	if err == nil {
		t.Fatal("expected error for missing symbol, got nil")
	}
	if !strings.Contains(err.Error(), "no quote data returned") {
		t.Fatalf("expected 'no quote data' error, got: %v", err)
	}
}

func TestGetStockQuote_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/quotes"):
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"message": "Internal error"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	_, err := p.GetStockQuote(context.Background(), "AAPL")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

// =============================================================================
// GetStockQuotes tests
// =============================================================================

func TestGetStockQuotes_MultipleSymbols(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/quotes"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabQuoteResponseMultiple)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	quotes, err := p.GetStockQuotes(context.Background(), []string{"AAPL", "MSFT", "GOOGL"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(quotes) != 3 {
		t.Fatalf("expected 3 quotes, got %d", len(quotes))
	}

	for _, sym := range []string{"AAPL", "MSFT", "GOOGL"} {
		q, ok := quotes[sym]
		if !ok {
			t.Errorf("missing quote for %s", sym)
			continue
		}
		if q.Symbol != sym {
			t.Errorf("expected symbol %s, got %s", sym, q.Symbol)
		}
		if q.Bid == nil {
			t.Errorf("expected bid for %s, got nil", sym)
		}
		if q.Ask == nil {
			t.Errorf("expected ask for %s, got nil", sym)
		}
		if q.Last == nil {
			t.Errorf("expected last for %s, got nil", sym)
		}
	}

	// Verify specific values
	if *quotes["MSFT"].Bid != 420.0 {
		t.Errorf("expected MSFT bid 420.0, got %v", *quotes["MSFT"].Bid)
	}
	if *quotes["GOOGL"].Last != 175.25 {
		t.Errorf("expected GOOGL last 175.25, got %v", *quotes["GOOGL"].Last)
	}
}

func TestGetStockQuotes_PartialResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/quotes"):
			// Only return AAPL, not INVALID
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabQuoteResponseAAPL)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	quotes, err := p.GetStockQuotes(context.Background(), []string{"AAPL", "INVALID"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(quotes) != 1 {
		t.Fatalf("expected 1 quote (partial), got %d", len(quotes))
	}
	if _, ok := quotes["AAPL"]; !ok {
		t.Error("expected AAPL in results")
	}
	if _, ok := quotes["INVALID"]; ok {
		t.Error("expected INVALID to be absent from results")
	}
}

func TestGetStockQuotes_EmptyInput(t *testing.T) {
	p := newTestProvider("http://unused")
	p.accessToken = "token"
	p.tokenExpiry = time.Now().Add(30 * time.Minute)

	quotes, err := p.GetStockQuotes(context.Background(), []string{})
	if err != nil {
		t.Fatalf("expected no error for empty input, got: %v", err)
	}
	if len(quotes) != 0 {
		t.Fatalf("expected empty map for empty input, got %d entries", len(quotes))
	}
}

// =============================================================================
// transformSchwabQuote tests (pure function)
// =============================================================================

func TestTransformSchwabQuote_FullData(t *testing.T) {
	data := map[string]interface{}{
		"quote": map[string]interface{}{
			"bidPrice":        150.0,
			"askPrice":        150.5,
			"lastPrice":       150.25,
			"openPrice":       149.0,
			"highPrice":       151.0,
			"lowPrice":        148.5,
			"closePrice":      149.5,
			"totalVolume":     50000000.0,
			"mark":            150.25,
			"netChange":       0.75,
			"quoteTimeInLong": 1715908546000.0,
			"tradeTimeInLong": 1715908545000.0,
		},
	}

	quote := transformSchwabQuote(data, "AAPL")

	if quote.Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %s", quote.Symbol)
	}
	if quote.Bid == nil || *quote.Bid != 150.0 {
		t.Errorf("expected bid 150.0, got %v", quote.Bid)
	}
	if quote.Ask == nil || *quote.Ask != 150.5 {
		t.Errorf("expected ask 150.5, got %v", quote.Ask)
	}
	if quote.Last == nil || *quote.Last != 150.25 {
		t.Errorf("expected last 150.25, got %v", quote.Last)
	}

	// Verify timestamp from quoteTimeInLong
	parsed, err := time.Parse(time.RFC3339, quote.Timestamp)
	if err != nil {
		t.Fatalf("failed to parse timestamp: %v", err)
	}
	expected := time.UnixMilli(1715908546000)
	if !parsed.Equal(expected) {
		t.Errorf("expected timestamp %v, got %v", expected, parsed)
	}
}

func TestTransformSchwabQuote_MissingFields(t *testing.T) {
	// Minimal data — only lastPrice
	data := map[string]interface{}{
		"quote": map[string]interface{}{
			"lastPrice": 100.0,
		},
	}

	quote := transformSchwabQuote(data, "SPY")

	if quote.Symbol != "SPY" {
		t.Errorf("expected symbol SPY, got %s", quote.Symbol)
	}
	if quote.Bid != nil {
		t.Errorf("expected nil bid, got %v", *quote.Bid)
	}
	if quote.Ask != nil {
		t.Errorf("expected nil ask, got %v", *quote.Ask)
	}
	if quote.Last == nil || *quote.Last != 100.0 {
		t.Errorf("expected last 100.0, got %v", quote.Last)
	}
	// Timestamp should fall back to current time
	if quote.Timestamp == "" {
		t.Error("expected timestamp to be set (fallback to now)")
	}
}

func TestTransformSchwabQuote_NoQuoteSubObject(t *testing.T) {
	// Flat structure (no "quote" sub-object)
	data := map[string]interface{}{
		"bidPrice":  200.0,
		"askPrice":  200.5,
		"lastPrice": 200.25,
	}

	quote := transformSchwabQuote(data, "FLAT")

	if quote.Bid == nil || *quote.Bid != 200.0 {
		t.Errorf("expected bid 200.0, got %v", quote.Bid)
	}
	if quote.Ask == nil || *quote.Ask != 200.5 {
		t.Errorf("expected ask 200.5, got %v", quote.Ask)
	}
	if quote.Last == nil || *quote.Last != 200.25 {
		t.Errorf("expected last 200.25, got %v", quote.Last)
	}
}

func TestTransformSchwabQuote_EmptyData(t *testing.T) {
	quote := transformSchwabQuote(map[string]interface{}{}, "EMPTY")

	if quote.Symbol != "EMPTY" {
		t.Errorf("expected symbol EMPTY, got %s", quote.Symbol)
	}
	if quote.Bid != nil {
		t.Errorf("expected nil bid, got %v", *quote.Bid)
	}
	if quote.Ask != nil {
		t.Errorf("expected nil ask, got %v", *quote.Ask)
	}
	if quote.Last != nil {
		t.Errorf("expected nil last, got %v", *quote.Last)
	}
}

func TestTransformSchwabQuote_TradeTimeFallback(t *testing.T) {
	// No quoteTimeInLong, but has tradeTimeInLong
	data := map[string]interface{}{
		"quote": map[string]interface{}{
			"lastPrice":       50.0,
			"tradeTimeInLong": 1715908545000.0,
		},
	}

	quote := transformSchwabQuote(data, "F")

	parsed, err := time.Parse(time.RFC3339, quote.Timestamp)
	if err != nil {
		t.Fatalf("failed to parse timestamp: %v", err)
	}
	expected := time.UnixMilli(1715908545000)
	if !parsed.Equal(expected) {
		t.Errorf("expected tradeTimeInLong fallback %v, got %v", expected, parsed)
	}
}

// =============================================================================
// JSON extraction helper tests
// =============================================================================

func TestExtractFloat64(t *testing.T) {
	data := map[string]interface{}{
		"float":   150.5,
		"int":     42,
		"string":  "not a number",
		"nil_val": nil,
	}

	if v, ok := extractFloat64(data, "float"); !ok || v != 150.5 {
		t.Errorf("expected 150.5, got %v (ok=%v)", v, ok)
	}
	if v, ok := extractFloat64(data, "int"); !ok || v != 42.0 {
		t.Errorf("expected 42.0, got %v (ok=%v)", v, ok)
	}
	if _, ok := extractFloat64(data, "string"); ok {
		t.Error("expected string extraction to fail")
	}
	if _, ok := extractFloat64(data, "nil_val"); ok {
		t.Error("expected nil extraction to fail")
	}
	if _, ok := extractFloat64(data, "missing"); ok {
		t.Error("expected missing key extraction to fail")
	}
	if _, ok := extractFloat64(nil, "key"); ok {
		t.Error("expected nil map extraction to fail")
	}
}

func TestExtractMap(t *testing.T) {
	data := map[string]interface{}{
		"nested": map[string]interface{}{"key": "value"},
		"string": "not a map",
		"nil":    nil,
	}

	if m := extractMap(data, "nested"); m == nil || m["key"] != "value" {
		t.Errorf("expected nested map, got %v", m)
	}
	if m := extractMap(data, "string"); m != nil {
		t.Errorf("expected nil for string, got %v", m)
	}
	if m := extractMap(data, "missing"); m != nil {
		t.Errorf("expected nil for missing key, got %v", m)
	}
	if m := extractMap(nil, "key"); m != nil {
		t.Errorf("expected nil for nil map, got %v", m)
	}
}

func TestFindSymbolData_CaseInsensitive(t *testing.T) {
	response := map[string]interface{}{
		"AAPL": map[string]interface{}{"quote": map[string]interface{}{"lastPrice": 150.0}},
	}

	// Exact match
	if data := findSymbolData(response, "AAPL"); data == nil {
		t.Error("expected to find AAPL with exact match")
	}

	// Lowercase → uppercase fallback
	if data := findSymbolData(response, "aapl"); data == nil {
		t.Error("expected to find AAPL via uppercase fallback")
	}

	// Missing symbol
	if data := findSymbolData(response, "MSFT"); data != nil {
		t.Error("expected nil for missing symbol")
	}
}

// =============================================================================
// Schwab chain response fixtures
// =============================================================================

const schwabChainResponse = `{
	"symbol": "AAPL",
	"status": "SUCCESS",
	"underlying": {"symbol": "AAPL", "last": 150.25},
	"callExpDateMap": {
		"2025-01-17:180": {
			"145.0": [{
				"putCall": "CALL",
				"symbol": "AAPL  250117C00145000",
				"strikePrice": 145.0,
				"bid": 7.50,
				"ask": 7.80,
				"closePrice": 7.60,
				"totalVolume": 1200,
				"openInterest": 5000,
				"delta": 0.65,
				"gamma": 0.03,
				"theta": -0.05,
				"vega": 0.25,
				"volatility": 28.5
			}],
			"150.0": [{
				"putCall": "CALL",
				"symbol": "AAPL  250117C00150000",
				"strikePrice": 150.0,
				"bid": 4.20,
				"ask": 4.50,
				"closePrice": 4.30,
				"totalVolume": 3500,
				"openInterest": 12000,
				"delta": 0.50,
				"gamma": 0.04,
				"theta": -0.06,
				"vega": 0.30,
				"volatility": 27.0
			}]
		},
		"2025-02-21:215": {
			"150.0": [{
				"putCall": "CALL",
				"symbol": "AAPL  250221C00150000",
				"strikePrice": 150.0,
				"bid": 6.00,
				"ask": 6.30,
				"totalVolume": 800,
				"openInterest": 4000,
				"delta": 0.52,
				"gamma": 0.025,
				"theta": -0.04,
				"vega": 0.35,
				"volatility": 26.0
			}]
		}
	},
	"putExpDateMap": {
		"2025-01-17:180": {
			"150.0": [{
				"putCall": "PUT",
				"symbol": "AAPL  250117P00150000",
				"strikePrice": 150.0,
				"bid": 3.80,
				"ask": 4.10,
				"closePrice": 3.90,
				"totalVolume": 2800,
				"openInterest": 9500,
				"delta": -0.50,
				"gamma": 0.04,
				"theta": -0.06,
				"vega": 0.30,
				"volatility": 27.5
			}]
		}
	}
}`

// =============================================================================
// GetExpirationDates tests
// =============================================================================

func TestGetExpirationDates_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/expirationchain"):
			if r.URL.Query().Get("symbol") != "AAPL" {
				t.Errorf("expected symbol=AAPL, got %s", r.URL.Query().Get("symbol"))
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{
				"expirationList": [
					{
						"expirationDate": "2025-01-17",
						"daysToExpiration": 180,
						"expirationType": "S",
						"standard": true
					},
					{
						"expirationDate": "2025-02-21",
						"daysToExpiration": 215,
						"expirationType": "S",
						"standard": true
					}
				]
			}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	dates, err := p.GetExpirationDates(context.Background(), "AAPL")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(dates) != 2 {
		t.Fatalf("expected 2 expiration dates, got %d: %v", len(dates), dates)
	}

	// Results should be sorted by date
	if dates[0]["date"] != "2025-01-17" {
		t.Errorf("expected first date 2025-01-17, got %s", dates[0]["date"])
	}
	if dates[1]["date"] != "2025-02-21" {
		t.Errorf("expected second date 2025-02-21, got %s", dates[1]["date"])
	}

	// Verify DTE fields
	if dates[0]["dte"] != 180 {
		t.Errorf("expected dte 180 for first date, got %v", dates[0]["dte"])
	}
	if dates[1]["dte"] != 215 {
		t.Errorf("expected dte 215 for second date, got %v", dates[1]["dte"])
	}

	// Verify symbol field (must be present for frontend options chain expansion)
	if dates[0]["symbol"] != "AAPL" {
		t.Errorf("expected symbol AAPL for first date, got %v", dates[0]["symbol"])
	}
	if dates[1]["symbol"] != "AAPL" {
		t.Errorf("expected symbol AAPL for second date, got %v", dates[1]["symbol"])
	}

	// Verify type field (standard=true + expirationType="S" → "monthly")
	if dates[0]["type"] != "monthly" {
		t.Errorf("expected type monthly for first date, got %v", dates[0]["type"])
	}
	if dates[1]["type"] != "monthly" {
		t.Errorf("expected type monthly for second date, got %v", dates[1]["type"])
	}
}

func TestGetExpirationDates_EmptyChain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/expirationchain"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"expirationList": []}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	dates, err := p.GetExpirationDates(context.Background(), "AAPL")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(dates) != 0 {
		t.Fatalf("expected 0 dates for empty chain, got %d", len(dates))
	}
}

func TestGetExpirationDates_MissingExpirationList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/expirationchain"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	dates, err := p.GetExpirationDates(context.Background(), "AAPL")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(dates) != 0 {
		t.Fatalf("expected 0 dates for missing expirationList, got %d", len(dates))
	}
}

func TestGetExpirationDates_ManyExpirations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/expirationchain"):
			w.WriteHeader(http.StatusOK)
			// Return dates deliberately out of order
			fmt.Fprint(w, `{
				"expirationList": [
					{"expirationDate": "2025-06-20", "daysToExpiration": 90, "expirationType": "S", "standard": true},
					{"expirationDate": "2025-01-10", "daysToExpiration": 5, "expirationType": "W", "standard": true},
					{"expirationDate": "2025-12-19", "daysToExpiration": 270, "expirationType": "S", "standard": true},
					{"expirationDate": "2025-03-21", "daysToExpiration": 45, "expirationType": "S", "standard": true},
					{"expirationDate": "2025-01-17", "daysToExpiration": 12, "expirationType": "W", "standard": true}
				]
			}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	dates, err := p.GetExpirationDates(context.Background(), "SPY")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(dates) != 5 {
		t.Fatalf("expected 5 expiration dates, got %d", len(dates))
	}

	// Verify dates are sorted ascending
	expectedOrder := []string{"2025-01-10", "2025-01-17", "2025-03-21", "2025-06-20", "2025-12-19"}
	for i, expected := range expectedOrder {
		got, _ := dates[i]["date"].(string)
		if got != expected {
			t.Errorf("dates[%d]: expected %s, got %s", i, expected, got)
		}
	}

	// Verify DTE values are preserved correctly after sorting
	expectedDTEs := []int{5, 12, 45, 90, 270}
	for i, expected := range expectedDTEs {
		got, _ := dates[i]["dte"].(int)
		if got != expected {
			t.Errorf("dates[%d] dte: expected %d, got %d", i, expected, got)
		}
	}

	// Verify symbol field is present on all entries
	for i, d := range dates {
		if d["symbol"] != "SPY" {
			t.Errorf("dates[%d]: expected symbol SPY, got %v", i, d["symbol"])
		}
	}

	// Verify type field: W→weekly, S+standard=true→monthly (sorted order)
	expectedTypes := []string{"weekly", "weekly", "monthly", "monthly", "monthly"}
	for i, expected := range expectedTypes {
		got, _ := dates[i]["type"].(string)
		if got != expected {
			t.Errorf("dates[%d] type: expected %s, got %s", i, expected, got)
		}
	}
}

func TestGetExpirationDates_TypeMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/expirationchain"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{
				"expirationList": [
					{"expirationDate": "2025-01-17", "daysToExpiration": 10, "expirationType": "S", "standard": true},
					{"expirationDate": "2025-01-24", "daysToExpiration": 17, "expirationType": "W", "standard": false},
					{"expirationDate": "2025-01-31", "daysToExpiration": 24, "expirationType": "W", "standard": true},
					{"expirationDate": "2025-03-31", "daysToExpiration": 83, "expirationType": "Q", "standard": false},
					{"expirationDate": "2025-02-07", "daysToExpiration": 31, "expirationType": "S", "standard": false},
					{"expirationDate": "2025-04-17", "daysToExpiration": 100, "expirationType": "R", "standard": false}
				]
			}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	dates, err := p.GetExpirationDates(context.Background(), "TEST")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(dates) != 6 {
		t.Fatalf("expected 6 expiration dates, got %d", len(dates))
	}

	// After sorting by date, expected order and types:
	// 2025-01-17: S + standard=true  → "monthly"
	// 2025-01-24: W + standard=false → "weekly"
	// 2025-01-31: W + standard=true  → "weekly" (W takes precedence over standard)
	// 2025-02-07: S + standard=false → "weekly" (S but not standard)
	// 2025-03-31: Q + standard=false → "quarterly"
	// 2025-04-17: R + standard=false → "monthly" (unknown type defaults to monthly)
	expectedTypes := []string{"monthly", "weekly", "weekly", "weekly", "quarterly", "monthly"}
	expectedDates := []string{"2025-01-17", "2025-01-24", "2025-01-31", "2025-02-07", "2025-03-31", "2025-04-17"}

	for i := range dates {
		gotDate, _ := dates[i]["date"].(string)
		gotType, _ := dates[i]["type"].(string)
		gotSymbol, _ := dates[i]["symbol"].(string)

		if gotDate != expectedDates[i] {
			t.Errorf("dates[%d] date: expected %s, got %s", i, expectedDates[i], gotDate)
		}
		if gotType != expectedTypes[i] {
			t.Errorf("dates[%d] type: expected %s, got %s (date=%s)", i, expectedTypes[i], gotType, gotDate)
		}
		if gotSymbol != "TEST" {
			t.Errorf("dates[%d] symbol: expected TEST, got %s", i, gotSymbol)
		}
	}
}

// =============================================================================
// GetOptionsChainBasic tests
// =============================================================================

func TestGetOptionsChainBasic_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/chains"):
			// Verify params
			if r.URL.Query().Get("symbol") != "AAPL" {
				t.Errorf("expected symbol=AAPL, got %s", r.URL.Query().Get("symbol"))
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabChainResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	contracts, err := p.GetOptionsChainBasic(context.Background(), "AAPL", "2025-01-17", nil, 0, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// 2 calls at 2025-01-17, 1 call at 2025-02-21, 1 put at 2025-01-17 = 4 total
	if len(contracts) != 4 {
		t.Fatalf("expected 4 contracts, got %d", len(contracts))
	}

	// Verify contracts have correct structure
	for _, c := range contracts {
		if c.UnderlyingSymbol != "AAPL" {
			t.Errorf("expected underlying AAPL, got %s", c.UnderlyingSymbol)
		}
		if c.Symbol == "" {
			t.Error("expected non-empty symbol")
		}
		// Should be OCC format (no spaces)
		if strings.Contains(c.Symbol, " ") {
			t.Errorf("expected OCC format symbol (no spaces), got %q", c.Symbol)
		}
		if c.Type != "call" && c.Type != "put" {
			t.Errorf("expected type call or put, got %s", c.Type)
		}
		// No Greeks since includeGreeks=false
		if c.Delta != nil {
			t.Errorf("expected nil delta (no Greeks), got %v", *c.Delta)
		}
	}
}

// =============================================================================
// GetOptionsChainSmart tests
// =============================================================================

func TestGetOptionsChainSmart_WithGreeks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/chains"):
			if r.URL.Query().Get("includeUnderlyingQuote") != "true" {
				t.Error("expected includeUnderlyingQuote=true")
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabChainResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	contracts, err := p.GetOptionsChainSmart(context.Background(), "AAPL", "2025-01-17", nil, 0, true, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(contracts) != 4 {
		t.Fatalf("expected 4 contracts, got %d", len(contracts))
	}

	// Find the AAPL 150 call and verify Greeks
	for _, c := range contracts {
		if c.StrikePrice == 150.0 && c.Type == "call" && c.ExpirationDate == "2025-01-17" {
			if c.Delta == nil || *c.Delta != 0.50 {
				t.Errorf("expected delta 0.50, got %v", c.Delta)
			}
			if c.Gamma == nil || *c.Gamma != 0.04 {
				t.Errorf("expected gamma 0.04, got %v", c.Gamma)
			}
			if c.Theta == nil || *c.Theta != -0.06 {
				t.Errorf("expected theta -0.06, got %v", c.Theta)
			}
			if c.Vega == nil || *c.Vega != 0.30 {
				t.Errorf("expected vega 0.30, got %v", c.Vega)
			}
			if c.ImpliedVolatility == nil || *c.ImpliedVolatility != 27.0 {
				t.Errorf("expected IV 27.0, got %v", c.ImpliedVolatility)
			}
			return
		}
	}
	t.Fatal("AAPL 150 call at 2025-01-17 not found in contracts")
}

// =============================================================================
// GetOptionsGreeksBatch tests
// =============================================================================

func TestGetOptionsGreeksBatch_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/quotes"):
			w.WriteHeader(http.StatusOK)
			// Return option quote with Greeks (Schwab space-padded symbol keys)
			fmt.Fprint(w, `{
				"AAPL  250117C00150000": {
					"assetMainType": "OPTION",
					"quote": {
						"bidPrice": 4.20,
						"askPrice": 4.50,
						"lastPrice": 4.35,
						"delta": 0.50,
						"gamma": 0.04,
						"theta": -0.06,
						"vega": 0.30,
						"rho": 0.02,
						"impliedVolatility": 27.0
					}
				}
			}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	// Request with OCC format symbols
	greeks, err := p.GetOptionsGreeksBatch(context.Background(), []string{"AAPL250117C00150000"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(greeks) != 1 {
		t.Fatalf("expected 1 result, got %d", len(greeks))
	}

	// Result should be keyed by OCC symbol
	g, ok := greeks["AAPL250117C00150000"]
	if !ok {
		t.Fatal("expected result keyed by OCC symbol AAPL250117C00150000")
	}

	if g["delta"] != 0.50 {
		t.Errorf("expected delta 0.50, got %v", g["delta"])
	}
	if g["gamma"] != 0.04 {
		t.Errorf("expected gamma 0.04, got %v", g["gamma"])
	}
	if g["theta"] != -0.06 {
		t.Errorf("expected theta -0.06, got %v", g["theta"])
	}
	if g["vega"] != 0.30 {
		t.Errorf("expected vega 0.30, got %v", g["vega"])
	}
	if g["rho"] != 0.02 {
		t.Errorf("expected rho 0.02, got %v", g["rho"])
	}
	if g["implied_volatility"] != 27.0 {
		t.Errorf("expected IV 27.0, got %v", g["implied_volatility"])
	}
}

func TestGetOptionsGreeksBatch_Empty(t *testing.T) {
	p := newTestProvider("http://unused")
	p.accessToken = "token"
	p.tokenExpiry = time.Now().Add(30 * time.Minute)

	greeks, err := p.GetOptionsGreeksBatch(context.Background(), []string{})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(greeks) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(greeks))
	}
}

// =============================================================================
// transformSchwabOptionsChain tests (pure function)
// =============================================================================

func TestTransformSchwabOptionsChain(t *testing.T) {
	var response map[string]interface{}
	if err := json.Unmarshal([]byte(schwabChainResponse), &response); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	contracts := transformSchwabOptionsChain(response, "AAPL", true)

	if len(contracts) != 4 {
		t.Fatalf("expected 4 contracts, got %d", len(contracts))
	}

	// Count calls vs puts
	calls, puts := 0, 0
	for _, c := range contracts {
		switch c.Type {
		case "call":
			calls++
		case "put":
			puts++
		}
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
	if puts != 1 {
		t.Errorf("expected 1 put, got %d", puts)
	}

	// Verify a specific contract
	for _, c := range contracts {
		if c.Symbol == "AAPL250117P00150000" {
			if c.Type != "put" {
				t.Errorf("expected put, got %s", c.Type)
			}
			if c.StrikePrice != 150.0 {
				t.Errorf("expected strike 150.0, got %f", c.StrikePrice)
			}
			if c.ExpirationDate != "2025-01-17" {
				t.Errorf("expected expiry 2025-01-17, got %s", c.ExpirationDate)
			}
			if c.Bid == nil || *c.Bid != 3.80 {
				t.Errorf("expected bid 3.80, got %v", c.Bid)
			}
			if c.Delta == nil || *c.Delta != -0.50 {
				t.Errorf("expected delta -0.50, got %v", c.Delta)
			}
			return
		}
	}
	t.Error("AAPL250117P00150000 not found in contracts")
}

func TestTransformSchwabOptionsChain_EmptyMap(t *testing.T) {
	response := map[string]interface{}{}

	contracts := transformSchwabOptionsChain(response, "AAPL", false)

	if len(contracts) != 0 {
		t.Fatalf("expected 0 contracts for empty response, got %d", len(contracts))
	}
}

func TestTransformSchwabOptionsChain_NoGreeks(t *testing.T) {
	var response map[string]interface{}
	if err := json.Unmarshal([]byte(schwabChainResponse), &response); err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	contracts := transformSchwabOptionsChain(response, "AAPL", false)

	for _, c := range contracts {
		if c.Delta != nil {
			t.Errorf("expected nil delta when includeGreeks=false, got %v for %s", *c.Delta, c.Symbol)
		}
		if c.Gamma != nil {
			t.Errorf("expected nil gamma when includeGreeks=false, got %v for %s", *c.Gamma, c.Symbol)
		}
	}
}

// =============================================================================
// Historical bars fixtures and tests
// =============================================================================

const schwabPriceHistoryResponse = `{
	"candles": [
		{"open": 149.0, "high": 150.0, "low": 148.0, "close": 149.5, "volume": 10000000, "datetime": 1715731200000},
		{"open": 149.5, "high": 151.0, "low": 149.0, "close": 150.25, "volume": 12000000, "datetime": 1715817600000},
		{"open": 150.25, "high": 152.0, "low": 150.0, "close": 151.5, "volume": 11000000, "datetime": 1715904000000}
	],
	"symbol": "AAPL",
	"empty": false
}`

func TestGetHistoricalBars_Daily(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/pricehistory"):
			if r.URL.Query().Get("frequencyType") != "daily" {
				t.Errorf("expected frequencyType=daily, got %s", r.URL.Query().Get("frequencyType"))
			}
			if r.URL.Query().Get("frequency") != "1" {
				t.Errorf("expected frequency=1, got %s", r.URL.Query().Get("frequency"))
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabPriceHistoryResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	bars, err := p.GetHistoricalBars(context.Background(), "AAPL", "1D", nil, nil, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(bars) != 3 {
		t.Fatalf("expected 3 bars, got %d", len(bars))
	}

	// Verify first bar
	bar := bars[0]
	if bar["open"] != 149.0 {
		t.Errorf("expected open 149.0, got %v", bar["open"])
	}
	if bar["high"] != 150.0 {
		t.Errorf("expected high 150.0, got %v", bar["high"])
	}
	if bar["low"] != 148.0 {
		t.Errorf("expected low 148.0, got %v", bar["low"])
	}
	if bar["close"] != 149.5 {
		t.Errorf("expected close 149.5, got %v", bar["close"])
	}
	if bar["volume"] != 10000000.0 {
		t.Errorf("expected volume 10000000, got %v", bar["volume"])
	}
	// Verify timestamp is ISO 8601
	ts, ok := bar["timestamp"].(string)
	if !ok || ts == "" {
		t.Error("expected timestamp string")
	}
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Errorf("timestamp is not valid RFC3339: %s", ts)
	}
}

func TestGetHistoricalBars_Minute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/pricehistory"):
			if r.URL.Query().Get("frequencyType") != "minute" {
				t.Errorf("expected frequencyType=minute, got %s", r.URL.Query().Get("frequencyType"))
			}
			if r.URL.Query().Get("frequency") != "5" {
				t.Errorf("expected frequency=5, got %s", r.URL.Query().Get("frequency"))
			}
			if r.URL.Query().Get("periodType") != "day" {
				t.Errorf("expected periodType=day, got %s", r.URL.Query().Get("periodType"))
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabPriceHistoryResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	bars, err := p.GetHistoricalBars(context.Background(), "AAPL", "5min", nil, nil, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(bars) != 3 {
		t.Fatalf("expected 3 bars, got %d", len(bars))
	}
}

func TestGetHistoricalBars_WithDateRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/pricehistory"):
			// Verify date params are present as epoch millis
			startDate := r.URL.Query().Get("startDate")
			endDate := r.URL.Query().Get("endDate")
			if startDate == "" {
				t.Error("expected startDate param")
			}
			if endDate == "" {
				t.Error("expected endDate param")
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabPriceHistoryResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	start := "2024-05-01"
	end := "2024-05-15"
	bars, err := p.GetHistoricalBars(context.Background(), "AAPL", "1D", &start, &end, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(bars) != 3 {
		t.Fatalf("expected 3 bars, got %d", len(bars))
	}
}

func TestGetHistoricalBars_WithLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/pricehistory"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabPriceHistoryResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	// Request with limit=2 — should return last 2 of 3 candles
	bars, err := p.GetHistoricalBars(context.Background(), "AAPL", "1D", nil, nil, 2)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(bars) != 2 {
		t.Fatalf("expected 2 bars (limited), got %d", len(bars))
	}
	// Verify it's the last 2 bars (not first 2)
	if bars[0]["close"] != 150.25 {
		t.Errorf("expected second candle close 150.25, got %v", bars[0]["close"])
	}
	if bars[1]["close"] != 151.5 {
		t.Errorf("expected third candle close 151.5, got %v", bars[1]["close"])
	}
}

func TestGetHistoricalBars_EmptyCandles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/pricehistory"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"candles": [], "symbol": "AAPL", "empty": true}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	bars, err := p.GetHistoricalBars(context.Background(), "AAPL", "1D", nil, nil, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(bars) != 0 {
		t.Fatalf("expected 0 bars for empty candles, got %d", len(bars))
	}
}

// =============================================================================
// GetNextMarketDate tests
// =============================================================================

func TestGetNextMarketDate(t *testing.T) {
	p := newTestProvider("http://unused")
	p.accessToken = "token"
	p.tokenExpiry = time.Now().Add(30 * time.Minute)

	date, err := p.GetNextMarketDate(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if date == "" {
		t.Fatal("expected non-empty date")
	}

	// Verify it's a valid date
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		t.Fatalf("expected valid YYYY-MM-DD date, got %q: %v", date, err)
	}

	// Should be a weekday
	day := parsed.Weekday()
	if day == time.Saturday || day == time.Sunday {
		t.Errorf("expected weekday, got %s", day)
	}
}

// =============================================================================
// mapTimeframe tests
// =============================================================================

func TestMapTimeframe(t *testing.T) {
	tests := []struct {
		input         string
		frequencyType string
		frequency     string
		periodType    string
	}{
		{"1min", "minute", "1", "day"},
		{"5min", "minute", "5", "day"},
		{"15min", "minute", "15", "day"},
		{"30min", "minute", "30", "day"},
		{"1hour", "minute", "30", "day"},
		{"1H", "minute", "30", "day"},
		{"1D", "daily", "1", "year"},
		{"daily", "daily", "1", "year"},
		{"1W", "weekly", "1", "year"},
		{"weekly", "weekly", "1", "year"},
		{"unknown", "daily", "1", "year"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			params := mapTimeframe(tt.input)
			if params.frequencyType != tt.frequencyType {
				t.Errorf("frequencyType: expected %s, got %s", tt.frequencyType, params.frequencyType)
			}
			if params.frequency != tt.frequency {
				t.Errorf("frequency: expected %s, got %s", tt.frequency, params.frequency)
			}
			if params.periodType != tt.periodType {
				t.Errorf("periodType: expected %s, got %s", tt.periodType, params.periodType)
			}
		})
	}
}

// =============================================================================
// dateToEpochMs tests
// =============================================================================

func TestDateToEpochMs(t *testing.T) {
	ms, err := dateToEpochMs("2024-05-15")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if ms <= 0 {
		t.Fatalf("expected positive epoch ms, got %d", ms)
	}

	// Verify round-trip
	roundTrip := time.UnixMilli(ms).UTC().Format("2006-01-02")
	if roundTrip != "2024-05-15" {
		t.Errorf("expected 2024-05-15, got %s", roundTrip)
	}
}

func TestDateToEpochMs_Invalid(t *testing.T) {
	_, err := dateToEpochMs("not-a-date")
	if err == nil {
		t.Fatal("expected error for invalid date, got nil")
	}
}

func TestIsWeekday(t *testing.T) {
	// Monday 2024-05-13
	mon := time.Date(2024, 5, 13, 12, 0, 0, 0, time.UTC)
	if !isWeekday(mon) {
		t.Error("expected Monday to be a weekday")
	}
	// Friday 2024-05-17
	fri := time.Date(2024, 5, 17, 12, 0, 0, 0, time.UTC)
	if !isWeekday(fri) {
		t.Error("expected Friday to be a weekday")
	}
	// Saturday 2024-05-18
	sat := time.Date(2024, 5, 18, 12, 0, 0, 0, time.UTC)
	if isWeekday(sat) {
		t.Error("expected Saturday to NOT be a weekday")
	}
	// Sunday 2024-05-19
	sun := time.Date(2024, 5, 19, 12, 0, 0, 0, time.UTC)
	if isWeekday(sun) {
		t.Error("expected Sunday to NOT be a weekday")
	}
}

// =============================================================================
// LookupSymbols tests
// =============================================================================

func TestLookupSymbols_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/instruments"):
			if r.URL.Query().Get("projection") != "symbol-search" {
				t.Errorf("expected projection=symbol-search, got %s", r.URL.Query().Get("projection"))
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{
				"instruments": [
					{"symbol": "AAPL", "description": "Apple Inc", "exchange": "NASDAQ", "assetType": "EQUITY"},
					{"symbol": "AAPX", "description": "Apple ETF Example", "exchange": "NYSE", "assetType": "ETF"},
					{"symbol": "AAPL250117C00150000", "description": "AAPL Jan 17 2025 150 Call", "exchange": "CBOE", "assetType": "OPTION"}
				]
			}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	results, err := p.LookupSymbols(context.Background(), "AAP")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify first result
	if results[0].Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %s", results[0].Symbol)
	}
	if results[0].Description != "Apple Inc" {
		t.Errorf("expected description 'Apple Inc', got %s", results[0].Description)
	}
	if results[0].Exchange != "NASDAQ" {
		t.Errorf("expected exchange NASDAQ, got %s", results[0].Exchange)
	}
	if results[0].Type != "stock" {
		t.Errorf("expected type 'stock', got %s", results[0].Type)
	}

	// Verify ETF mapping
	if results[1].Type != "etf" {
		t.Errorf("expected type 'etf', got %s", results[1].Type)
	}

	// Verify OPTION mapping
	if results[2].Type != "option" {
		t.Errorf("expected type 'option', got %s", results[2].Type)
	}
}

func TestLookupSymbols_MapFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/instruments"):
			// Return map format (keyed by symbol)
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{
				"AAPL": {"description": "Apple Inc", "exchange": "NASDAQ", "assetType": "EQUITY"},
				"MSFT": {"description": "Microsoft Corp", "exchange": "NASDAQ", "assetType": "EQUITY"}
			}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	results, err := p.LookupSymbols(context.Background(), "A")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Verify symbols are populated from map keys
	symbols := make(map[string]bool)
	for _, r := range results {
		symbols[r.Symbol] = true
		if r.Type != "stock" {
			t.Errorf("expected type 'stock' for %s, got %s", r.Symbol, r.Type)
		}
	}
	if !symbols["AAPL"] {
		t.Error("expected AAPL in results")
	}
	if !symbols["MSFT"] {
		t.Error("expected MSFT in results")
	}
}

func TestLookupSymbols_NoResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/instruments"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"instruments": []}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	results, err := p.LookupSymbols(context.Background(), "ZZZZZ")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestLookupSymbols_EmptyQuery(t *testing.T) {
	p := newTestProvider("http://unused")
	p.accessToken = "token"
	p.tokenExpiry = time.Now().Add(30 * time.Minute)

	results, err := p.LookupSymbols(context.Background(), "")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty query, got %d", len(results))
	}
}

// =============================================================================
// mapSchwabAssetType tests
// =============================================================================

func TestMapSchwabAssetType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"EQUITY", "stock"},
		{"ETF", "etf"},
		{"INDEX", "index"},
		{"OPTION", "option"},
		{"MUTUAL_FUND", "fund"},
		{"BOND", "bond"},
		{"equity", "stock"},
		{"", ""},
		{"UNKNOWN_TYPE", "unknown_type"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapSchwabAssetType(tt.input)
			if got != tt.expected {
				t.Errorf("mapSchwabAssetType(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
