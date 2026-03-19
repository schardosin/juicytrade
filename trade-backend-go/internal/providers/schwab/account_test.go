package schwab

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"trade-backend-go/internal/models"
)

// =============================================================================
// Schwab account response fixtures
// =============================================================================

const schwabAccountResponse = `{
	"securitiesAccount": {
		"type": "MARGIN",
		"accountNumber": "12345678",
		"roundTrips": 0,
		"isDayTrader": false,
		"isClosingOnlyRestricted": false,
		"positions": [
			{
				"instrument": {"symbol": "AAPL", "assetType": "EQUITY", "cusip": "037833100"},
				"longQuantity": 100.0,
				"shortQuantity": 0.0,
				"averagePrice": 145.00,
				"marketValue": 15025.00,
				"currentDayProfitLoss": 25.00,
				"currentDayProfitLossPercentage": 0.17,
				"currentDayCost": 14500.00
			},
			{
				"instrument": {"symbol": "MSFT", "assetType": "EQUITY", "cusip": "594918104"},
				"longQuantity": 50.0,
				"shortQuantity": 0.0,
				"averagePrice": 400.00,
				"marketValue": 21050.00,
				"currentDayProfitLoss": 50.00,
				"currentDayProfitLossPercentage": 0.24,
				"currentDayCost": 20000.00
			},
			{
				"instrument": {"symbol": "AAPL  250117C00150000", "assetType": "OPTION", "cusip": "0AAPL.AH50117"},
				"longQuantity": 5.0,
				"shortQuantity": 0.0,
				"averagePrice": 4.50,
				"marketValue": 2250.00,
				"currentDayProfitLoss": -100.00,
				"currentDayProfitLossPercentage": -4.26,
				"currentDayCost": 2250.00
			}
		],
		"currentBalances": {
			"liquidationValue": 50000.00,
			"buyingPower": 25000.00,
			"cashBalance": 10000.00,
			"equity": 40000.00,
			"longMarketValue": 38325.00,
			"shortMarketValue": 0.00,
			"dayTradingBuyingPower": 100000.00,
			"availableFunds": 25000.00,
			"maintenanceRequirement": 15000.00
		}
	}
}`

// =============================================================================
// GetAccount tests
// =============================================================================

func TestGetAccount_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.Contains(r.URL.Path, "/accounts/test-account-hash"):
			if r.URL.Query().Get("fields") != "positions" {
				t.Errorf("expected fields=positions, got %s", r.URL.Query().Get("fields"))
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabAccountResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	account, err := p.GetAccount(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if account.AccountID != "test-account-hash" {
		t.Errorf("expected account ID 'test-account-hash', got %s", account.AccountID)
	}
	if account.Status != "active" {
		t.Errorf("expected status 'active', got %s", account.Status)
	}
	if account.Currency != "USD" {
		t.Errorf("expected currency USD, got %s", account.Currency)
	}
	if account.AccountNumber == nil || *account.AccountNumber != "12345678" {
		t.Errorf("expected account number '12345678', got %v", account.AccountNumber)
	}
	if account.BuyingPower == nil || *account.BuyingPower != 25000.00 {
		t.Errorf("expected buying power 25000, got %v", account.BuyingPower)
	}
	if account.Cash == nil || *account.Cash != 10000.00 {
		t.Errorf("expected cash 10000, got %v", account.Cash)
	}
	if account.PortfolioValue == nil || *account.PortfolioValue != 50000.00 {
		t.Errorf("expected portfolio value 50000, got %v", account.PortfolioValue)
	}
	if account.Equity == nil || *account.Equity != 40000.00 {
		t.Errorf("expected equity 40000, got %v", account.Equity)
	}
	if account.DayTradingBuyingPower == nil || *account.DayTradingBuyingPower != 100000.00 {
		t.Errorf("expected DTBP 100000, got %v", account.DayTradingBuyingPower)
	}
	if account.LongMarketValue == nil || *account.LongMarketValue != 38325.00 {
		t.Errorf("expected long market value 38325, got %v", account.LongMarketValue)
	}
	if account.MaintenanceMargin == nil || *account.MaintenanceMargin != 15000.00 {
		t.Errorf("expected maintenance margin 15000, got %v", account.MaintenanceMargin)
	}
}

func TestGetAccount_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		default:
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"message": "Internal error"}`)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	_, err := p.GetAccount(context.Background())
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}

// =============================================================================
// GetPositions tests
// =============================================================================

func TestGetPositions_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.Contains(r.URL.Path, "/accounts/"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabAccountResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	positions, err := p.GetPositions(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(positions) != 3 {
		t.Fatalf("expected 3 positions, got %d", len(positions))
	}

	// Verify equity position
	aapl := positions[0]
	if aapl.Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %s", aapl.Symbol)
	}
	if aapl.Qty != 100 {
		t.Errorf("expected qty 100, got %f", aapl.Qty)
	}
	if aapl.Side != "long" {
		t.Errorf("expected side long, got %s", aapl.Side)
	}
	if aapl.AssetClass != "us_equity" {
		t.Errorf("expected asset class us_equity, got %s", aapl.AssetClass)
	}
	if aapl.AvgEntryPrice != 145.00 {
		t.Errorf("expected avg entry price 145.00, got %f", aapl.AvgEntryPrice)
	}
	if aapl.MarketValue != 15025.00 {
		t.Errorf("expected market value 15025.00, got %f", aapl.MarketValue)
	}
	if aapl.UnrealizedPL != 25.00 {
		t.Errorf("expected unrealized PL 25.00, got %f", aapl.UnrealizedPL)
	}
}

func TestGetPositions_OptionPositionParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.Contains(r.URL.Path, "/accounts/"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabAccountResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	positions, err := p.GetPositions(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Find the option position (3rd position)
	option := positions[2]

	if option.AssetClass != "us_option" {
		t.Errorf("expected asset class us_option, got %s", option.AssetClass)
	}
	// Symbol should be OCC format (no spaces)
	if option.Symbol != "AAPL250117C00150000" {
		t.Errorf("expected OCC symbol 'AAPL250117C00150000', got %s", option.Symbol)
	}
	if strings.Contains(option.Symbol, " ") {
		t.Errorf("expected no spaces in OCC symbol, got %q", option.Symbol)
	}

	// Verify option-specific fields parsed from OCC symbol
	if option.UnderlyingSymbol == nil || *option.UnderlyingSymbol != "AAPL" {
		t.Errorf("expected underlying AAPL, got %v", option.UnderlyingSymbol)
	}
	if option.OptionType == nil || *option.OptionType != "call" {
		t.Errorf("expected option type 'call', got %v", option.OptionType)
	}
	if option.StrikePrice == nil || *option.StrikePrice != 150.0 {
		t.Errorf("expected strike 150.0, got %v", option.StrikePrice)
	}
	if option.ExpiryDate == nil || *option.ExpiryDate != "2025-01-17" {
		t.Errorf("expected expiry 2025-01-17, got %v", option.ExpiryDate)
	}
	if option.Qty != 5 {
		t.Errorf("expected qty 5, got %f", option.Qty)
	}
}

func TestGetPositions_NoPositions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.Contains(r.URL.Path, "/accounts/"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"securitiesAccount": {"type": "MARGIN", "accountNumber": "12345678", "currentBalances": {}}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	positions, err := p.GetPositions(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(positions) != 0 {
		t.Fatalf("expected 0 positions, got %d", len(positions))
	}
}

// =============================================================================
// GetPositionsEnhanced tests
// =============================================================================

func TestGetPositionsEnhanced_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.Contains(r.URL.Path, "/accounts/"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabAccountResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	enhanced, err := p.GetPositionsEnhanced(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if enhanced == nil {
		t.Fatal("expected non-nil enhanced response")
	}
	// Should have groups (AAPL with equity+option, MSFT with equity)
	if len(enhanced.SymbolGroups) == 0 {
		t.Error("expected at least one group in enhanced positions")
	}
}

// =============================================================================
// parseOCCSymbolIntoPosition tests (pure function)
// =============================================================================

func TestParseOCCSymbolIntoPosition(t *testing.T) {
	tests := []struct {
		name       string
		symbol     string
		underlying string
		optType    string
		strike     float64
		expiry     string
	}{
		{"AAPL call", "AAPL250117C00150000", "AAPL", "call", 150.0, "2025-01-17"},
		{"SPY put", "SPY250321P00500000", "SPY", "put", 500.0, "2025-03-21"},
		{"F low strike", "F250620C00015000", "F", "call", 15.0, "2025-06-20"},
		{"GOOGL call", "GOOGL250117C00200000", "GOOGL", "call", 200.0, "2025-01-17"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := &models.Position{}
			parseOCCSymbolIntoPosition(pos, tt.symbol)

			if pos.UnderlyingSymbol == nil || *pos.UnderlyingSymbol != tt.underlying {
				t.Errorf("expected underlying %s, got %v", tt.underlying, pos.UnderlyingSymbol)
			}
			if pos.OptionType == nil || *pos.OptionType != tt.optType {
				t.Errorf("expected option type %s, got %v", tt.optType, pos.OptionType)
			}
			if pos.StrikePrice == nil || *pos.StrikePrice != tt.strike {
				t.Errorf("expected strike %f, got %v", tt.strike, pos.StrikePrice)
			}
			if pos.ExpiryDate == nil || *pos.ExpiryDate != tt.expiry {
				t.Errorf("expected expiry %s, got %v", tt.expiry, pos.ExpiryDate)
			}
		})
	}
}

func TestParseOCCSymbolIntoPosition_TooShort(t *testing.T) {
	pos := &models.Position{}
	parseOCCSymbolIntoPosition(pos, "SHORT")

	if pos.UnderlyingSymbol != nil {
		t.Errorf("expected nil underlying for short symbol, got %v", pos.UnderlyingSymbol)
	}
}
