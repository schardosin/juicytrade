package schwab

import (
	"context"
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
