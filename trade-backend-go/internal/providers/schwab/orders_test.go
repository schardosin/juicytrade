package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// =============================================================================
// Schwab orders response fixtures
// =============================================================================

const schwabOrdersResponse = `[
	{
		"orderId": 100001,
		"session": "NORMAL",
		"duration": "DAY",
		"orderType": "LIMIT",
		"price": 150.00,
		"status": "WORKING",
		"quantity": 100,
		"filledQuantity": 0,
		"enteredTime": "2024-05-16T10:30:00+0000",
		"orderLegCollection": [{
			"instruction": "BUY",
			"quantity": 100,
			"instrument": {"symbol": "AAPL", "assetType": "EQUITY"}
		}]
	},
	{
		"orderId": 100002,
		"session": "NORMAL",
		"duration": "GOOD_TILL_CANCEL",
		"orderType": "MARKET",
		"status": "FILLED",
		"quantity": 50,
		"filledQuantity": 50,
		"enteredTime": "2024-05-15T09:30:00+0000",
		"closeTime": "2024-05-15T09:30:05+0000",
		"orderLegCollection": [{
			"instruction": "SELL",
			"quantity": 50,
			"instrument": {"symbol": "MSFT", "assetType": "EQUITY"}
		}]
	}
]`

const schwabMultiLegOrderResponse = `[
	{
		"orderId": 100003,
		"session": "NORMAL",
		"duration": "DAY",
		"orderType": "NET_DEBIT",
		"price": 2.50,
		"status": "WORKING",
		"quantity": 10,
		"filledQuantity": 0,
		"enteredTime": "2024-05-16T11:00:00+0000",
		"orderLegCollection": [
			{
				"instruction": "BUY_TO_OPEN",
				"quantity": 10,
				"instrument": {"symbol": "AAPL  250117C00150000", "assetType": "OPTION"}
			},
			{
				"instruction": "SELL_TO_OPEN",
				"quantity": 10,
				"instrument": {"symbol": "AAPL  250117C00160000", "assetType": "OPTION"}
			}
		]
	}
]`

// =============================================================================
// GetOrders tests
// =============================================================================

func TestGetOrders_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/orders"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabOrdersResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	orders, err := p.GetOrders(context.Background(), "all")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(orders))
	}

	// Verify first order (working limit buy)
	o1 := orders[0]
	if o1.ID != "100001" {
		t.Errorf("expected ID '100001', got %s", o1.ID)
	}
	if o1.Status != "open" {
		t.Errorf("expected status 'open', got %s", o1.Status)
	}
	if o1.OrderType != "limit" {
		t.Errorf("expected order type 'limit', got %s", o1.OrderType)
	}
	if o1.TimeInForce != "day" {
		t.Errorf("expected TIF 'day', got %s", o1.TimeInForce)
	}
	if o1.Symbol != "AAPL" {
		t.Errorf("expected symbol 'AAPL', got %s", o1.Symbol)
	}
	if o1.Side != "buy" {
		t.Errorf("expected side 'buy', got %s", o1.Side)
	}
	if o1.Qty != 100 {
		t.Errorf("expected qty 100, got %f", o1.Qty)
	}
	if o1.LimitPrice == nil || *o1.LimitPrice != 150.00 {
		t.Errorf("expected limit price 150.00, got %v", o1.LimitPrice)
	}
	if o1.AssetClass != "us_equity" {
		t.Errorf("expected asset class 'us_equity', got %s", o1.AssetClass)
	}

	// Verify second order (filled market sell)
	o2 := orders[1]
	if o2.ID != "100002" {
		t.Errorf("expected ID '100002', got %s", o2.ID)
	}
	if o2.Status != "filled" {
		t.Errorf("expected status 'filled', got %s", o2.Status)
	}
	if o2.TimeInForce != "gtc" {
		t.Errorf("expected TIF 'gtc', got %s", o2.TimeInForce)
	}
	if o2.FilledQty != 50 {
		t.Errorf("expected filled qty 50, got %f", o2.FilledQty)
	}
	if o2.FilledAt == nil || *o2.FilledAt == "" {
		t.Error("expected filled_at to be set for filled order")
	}
}

func TestGetOrders_OpenFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/orders"):
			// Verify NO status filter is sent — we fetch all and filter client-side
			status := r.URL.Query().Get("status")
			if status != "" {
				t.Errorf("expected no status filter for open request, got %q", status)
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `[]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	_, err := p.GetOrders(context.Background(), "open")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestGetOrders_MultiLegOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/orders"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabMultiLegOrderResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	orders, err := p.GetOrders(context.Background(), "all")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}

	order := orders[0]
	if len(order.Legs) != 2 {
		t.Fatalf("expected 2 legs, got %d", len(order.Legs))
	}

	// Verify option symbols are in OCC format (no spaces)
	for _, leg := range order.Legs {
		if strings.Contains(leg.Symbol, " ") {
			t.Errorf("expected OCC format (no spaces), got %q", leg.Symbol)
		}
	}

	// Verify first leg (buy call)
	if order.Legs[0].Side != "buy" {
		t.Errorf("expected first leg side 'buy', got %s", order.Legs[0].Side)
	}
	if order.Legs[0].Symbol != "AAPL250117C00150000" {
		t.Errorf("expected first leg OCC symbol, got %s", order.Legs[0].Symbol)
	}

	// Verify second leg (sell call)
	if order.Legs[1].Side != "sell" {
		t.Errorf("expected second leg side 'sell', got %s", order.Legs[1].Side)
	}

	// Primary asset class from first leg
	if order.AssetClass != "us_option" {
		t.Errorf("expected asset class 'us_option', got %s", order.AssetClass)
	}
}

func TestGetOrders_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/orders"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `[]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	orders, err := p.GetOrders(context.Background(), "all")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(orders) != 0 {
		t.Fatalf("expected 0 orders, got %d", len(orders))
	}
}

// =============================================================================
// CancelOrder tests
// =============================================================================

func TestCancelOrder_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.Contains(r.URL.Path, "/orders/100001"):
			if r.Method != http.MethodDelete {
				t.Errorf("expected DELETE method, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	ok, err := p.CancelOrder(context.Background(), "100001")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !ok {
		t.Error("expected true for successful cancellation")
	}
}

func TestCancelOrder_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.Contains(r.URL.Path, "/orders/"):
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message": "Order not found"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	ok, err := p.CancelOrder(context.Background(), "999999")
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
	if ok {
		t.Error("expected false for failed cancellation")
	}
}

// =============================================================================
// transformSchwabOrder tests (pure function)
// =============================================================================

func TestTransformSchwabOrder_Basic(t *testing.T) {
	data := map[string]interface{}{
		"orderId":        100001.0,
		"status":         "WORKING",
		"orderType":      "LIMIT",
		"duration":       "DAY",
		"price":          150.0,
		"quantity":       100.0,
		"filledQuantity": 0.0,
		"enteredTime":    "2024-05-16T10:30:00+0000",
		"orderLegCollection": []interface{}{
			map[string]interface{}{
				"instruction": "BUY",
				"quantity":    100.0,
				"instrument": map[string]interface{}{
					"symbol":    "AAPL",
					"assetType": "EQUITY",
				},
			},
		},
	}

	order := transformSchwabOrder(data)
	if order == nil {
		t.Fatal("expected non-nil order")
	}

	if order.ID != "100001" {
		t.Errorf("expected ID '100001', got %s", order.ID)
	}
	if order.Status != "open" {
		t.Errorf("expected status 'open', got %s", order.Status)
	}
	if order.OrderType != "limit" {
		t.Errorf("expected order type 'limit', got %s", order.OrderType)
	}
	if order.Symbol != "AAPL" {
		t.Errorf("expected symbol 'AAPL', got %s", order.Symbol)
	}
}

func TestTransformSchwabOrder_NoOrderID(t *testing.T) {
	data := map[string]interface{}{
		"status": "WORKING",
	}

	order := transformSchwabOrder(data)
	if order != nil {
		t.Error("expected nil order when orderId is missing")
	}
}

// =============================================================================
// Status mapping tests
// =============================================================================

func TestMapSchwabOrderStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"WORKING", "open"},
		{"AWAITING_PARENT_ORDER", "open"},
		{"AWAITING_CONDITION", "open"},
		{"AWAITING_MANUAL_REVIEW", "open"},
		{"FILLED", "filled"},
		{"CANCELED", "canceled"},
		{"REJECTED", "rejected"},
		{"EXPIRED", "expired"},
		{"PENDING_ACTIVATION", "pending"},
		{"QUEUED", "pending"},
		{"ACCEPTED", "pending"},
		{"REPLACED", "replaced"},
		{"", "unknown"},
		{"SOME_NEW_STATUS", "some_new_status"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapSchwabOrderStatus(tt.input)
			if got != tt.expected {
				t.Errorf("mapSchwabOrderStatus(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMapSchwabInstruction(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BUY", "buy"},
		{"SELL", "sell"},
		{"BUY_TO_OPEN", "buy"},
		{"SELL_TO_CLOSE", "sell"},
		{"BUY_TO_COVER", "buy"},
		{"SELL_SHORT", "sell"},
		{"SELL_TO_OPEN", "sell"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapSchwabInstruction(tt.input)
			if got != tt.expected {
				t.Errorf("mapSchwabInstruction(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMapSchwabDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"DAY", "day"},
		{"GOOD_TILL_CANCEL", "gtc"},
		{"FILL_OR_KILL", "fok"},
		{"", "day"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapSchwabDuration(tt.input)
			if got != tt.expected {
				t.Errorf("mapSchwabDuration(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// =============================================================================
// PlaceOrder tests
// =============================================================================

func TestPlaceOrder_EquityBuy(t *testing.T) {
	var receivedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/orders") && r.Method == http.MethodPost:
			// Capture request body
			bodyBytes, _ := io.ReadAll(r.Body)
			json.Unmarshal(bodyBytes, &receivedBody)

			w.Header().Set("Location", "/v1/accounts/hash/orders/200001")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"orderId": 200001}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	order, err := p.PlaceOrder(context.Background(), map[string]interface{}{
		"symbol":        "AAPL",
		"side":          "buy",
		"type":          "market",
		"qty":           100.0,
		"time_in_force": "day",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if order == nil {
		t.Fatal("expected non-nil order")
	}
	if order.OrderType != "market" {
		t.Errorf("expected order type 'market', got %s", order.OrderType)
	}
	if order.Side != "buy" {
		t.Errorf("expected side 'buy', got %s", order.Side)
	}
	if order.Symbol != "AAPL" {
		t.Errorf("expected symbol 'AAPL', got %s", order.Symbol)
	}

	// Verify request body structure
	if receivedBody["session"] != "NORMAL" {
		t.Errorf("expected session NORMAL, got %v", receivedBody["session"])
	}
	if receivedBody["orderType"] != "MARKET" {
		t.Errorf("expected orderType MARKET, got %v", receivedBody["orderType"])
	}
	if receivedBody["duration"] != "DAY" {
		t.Errorf("expected duration DAY, got %v", receivedBody["duration"])
	}
	legs, _ := receivedBody["orderLegCollection"].([]interface{})
	if len(legs) != 1 {
		t.Fatalf("expected 1 leg, got %d", len(legs))
	}
	leg, _ := legs[0].(map[string]interface{})
	if leg["instruction"] != "BUY" {
		t.Errorf("expected instruction BUY, got %v", leg["instruction"])
	}

	// Verify complexOrderStrategyType is NOT present for equity orders (omitempty)
	if _, exists := receivedBody["complexOrderStrategyType"]; exists {
		t.Errorf("expected complexOrderStrategyType to be absent for equity order, got %v", receivedBody["complexOrderStrategyType"])
	}
}

func TestPlaceOrder_OptionSell(t *testing.T) {
	var receivedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/orders") && r.Method == http.MethodPost:
			bodyBytes, _ := io.ReadAll(r.Body)
			json.Unmarshal(bodyBytes, &receivedBody)
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"orderId": 200002}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	order, err := p.PlaceOrder(context.Background(), map[string]interface{}{
		"symbol":      "AAPL250117C00150000",
		"side":        "sell",
		"type":        "limit",
		"qty":         5.0,
		"price":       4.50,
		"asset_class": "us_option",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if order == nil {
		t.Fatal("expected non-nil order")
	}
	if order.AssetClass != "us_option" {
		t.Errorf("expected asset class 'us_option', got %s", order.AssetClass)
	}

	// Verify instruction is SELL_TO_CLOSE for option sell
	legs, _ := receivedBody["orderLegCollection"].([]interface{})
	if len(legs) != 1 {
		t.Fatalf("expected 1 leg, got %d", len(legs))
	}
	leg, _ := legs[0].(map[string]interface{})
	if leg["instruction"] != "SELL_TO_CLOSE" {
		t.Errorf("expected instruction SELL_TO_CLOSE, got %v", leg["instruction"])
	}

	// Verify symbol is Schwab format (space-padded)
	inst, _ := leg["instrument"].(map[string]interface{})
	symbol, _ := inst["symbol"].(string)
	if !strings.Contains(symbol, " ") {
		t.Errorf("expected Schwab format (with spaces), got %q", symbol)
	}
	if inst["assetType"] != "OPTION" {
		t.Errorf("expected assetType OPTION, got %v", inst["assetType"])
	}

	// Verify complexOrderStrategyType is set to NONE for single-leg option orders
	if receivedBody["complexOrderStrategyType"] != "NONE" {
		t.Errorf("expected complexOrderStrategyType NONE for option order, got %v", receivedBody["complexOrderStrategyType"])
	}
}

func TestPlaceOrder_LimitOrder(t *testing.T) {
	var receivedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/orders") && r.Method == http.MethodPost:
			bodyBytes, _ := io.ReadAll(r.Body)
			json.Unmarshal(bodyBytes, &receivedBody)
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"orderId": 200003}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	order, err := p.PlaceOrder(context.Background(), map[string]interface{}{
		"symbol":        "AAPL",
		"side":          "buy",
		"type":          "limit",
		"qty":           50.0,
		"price":         150.00,
		"time_in_force": "gtc",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if order.LimitPrice == nil || *order.LimitPrice != 150.00 {
		t.Errorf("expected limit price 150.00, got %v", order.LimitPrice)
	}
	if order.TimeInForce != "gtc" {
		t.Errorf("expected TIF 'gtc', got %s", order.TimeInForce)
	}

	// Verify request body
	if receivedBody["orderType"] != "LIMIT" {
		t.Errorf("expected orderType LIMIT, got %v", receivedBody["orderType"])
	}
	if receivedBody["price"] != "150.00" {
		t.Errorf("expected price \"150.00\", got %v", receivedBody["price"])
	}
	if receivedBody["duration"] != "GOOD_TILL_CANCEL" {
		t.Errorf("expected duration GOOD_TILL_CANCEL, got %v", receivedBody["duration"])
	}
}

func TestPlaceMultiLegOrder_VerticalSpread(t *testing.T) {
	var receivedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/orders") && r.Method == http.MethodPost:
			bodyBytes, _ := io.ReadAll(r.Body)
			json.Unmarshal(bodyBytes, &receivedBody)
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"orderId": 200004}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	order, err := p.PlaceMultiLegOrder(context.Background(), map[string]interface{}{
		"type":          "net_debit",
		"price":         2.50,
		"time_in_force": "day",
		"legs": []interface{}{
			map[string]interface{}{
				"symbol":      "AAPL250117C00150000",
				"side":        "buy",
				"qty":         10.0,
				"action":      "BUY_TO_OPEN",
				"asset_class": "us_option",
			},
			map[string]interface{}{
				"symbol":      "AAPL250117C00160000",
				"side":        "sell",
				"qty":         10.0,
				"action":      "SELL_TO_OPEN",
				"asset_class": "us_option",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if order == nil {
		t.Fatal("expected non-nil order")
	}

	// Verify request body has 2 legs
	legs, _ := receivedBody["orderLegCollection"].([]interface{})
	if len(legs) != 2 {
		t.Fatalf("expected 2 legs, got %d", len(legs))
	}

	// Verify first leg
	leg1, _ := legs[0].(map[string]interface{})
	if leg1["instruction"] != "BUY_TO_OPEN" {
		t.Errorf("expected first leg instruction BUY_TO_OPEN, got %v", leg1["instruction"])
	}

	// Verify second leg
	leg2, _ := legs[1].(map[string]interface{})
	if leg2["instruction"] != "SELL_TO_OPEN" {
		t.Errorf("expected second leg instruction SELL_TO_OPEN, got %v", leg2["instruction"])
	}

	// Verify order type
	if receivedBody["orderType"] != "NET_DEBIT" {
		t.Errorf("expected orderType NET_DEBIT, got %v", receivedBody["orderType"])
	}

	// Verify complexOrderStrategyType is set to CUSTOM for multi-leg orders
	if receivedBody["complexOrderStrategyType"] != "CUSTOM" {
		t.Errorf("expected complexOrderStrategyType CUSTOM for multi-leg order, got %v", receivedBody["complexOrderStrategyType"])
	}
}

func TestPlaceOrder_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/orders"):
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"message": "Invalid order"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	_, err := p.PlaceOrder(context.Background(), map[string]interface{}{
		"symbol": "AAPL",
		"side":   "buy",
		"type":   "market",
		"qty":    100.0,
	})
	if err == nil {
		t.Fatal("expected error for 400 response, got nil")
	}
}

func TestPlaceOrder_MissingSymbol(t *testing.T) {
	p := newTestProvider("http://unused")

	_, err := p.PlaceOrder(context.Background(), map[string]interface{}{
		"side": "buy",
		"qty":  100.0,
	})
	if err == nil {
		t.Fatal("expected error for missing symbol, got nil")
	}
	if !strings.Contains(err.Error(), "missing symbol") {
		t.Errorf("expected 'missing symbol' error, got: %v", err)
	}
}

// =============================================================================
// Order mapping helper tests
// =============================================================================

func TestMapSideToInstruction(t *testing.T) {
	tests := []struct {
		side     string
		isOption bool
		expected string
	}{
		{"buy", false, "BUY"},
		{"sell", false, "SELL"},
		{"buy", true, "BUY_TO_OPEN"},
		{"sell", true, "SELL_TO_CLOSE"},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%s_option=%v", tt.side, tt.isOption)
		t.Run(name, func(t *testing.T) {
			got := mapSideToInstruction(tt.side, tt.isOption)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestMapOrderTypeToSchwab(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"market", "MARKET"},
		{"limit", "LIMIT"},
		{"stop", "STOP"},
		{"stop_limit", "STOP_LIMIT"},
		{"net_debit", "NET_DEBIT"},
		{"net_credit", "NET_CREDIT"},
		{"", "MARKET"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapOrderTypeToSchwab(tt.input)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestMapDurationToSchwab(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"day", "DAY"},
		{"gtc", "GOOD_TILL_CANCEL"},
		{"fok", "FILL_OR_KILL"},
		{"", "DAY"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapDurationToSchwab(tt.input)
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestBuildSchwabOrderRequest(t *testing.T) {
	req, err := buildSchwabOrderRequest(map[string]interface{}{
		"symbol":        "AAPL",
		"side":          "buy",
		"type":          "limit",
		"qty":           100.0,
		"price":         150.0,
		"time_in_force": "gtc",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if req.OrderType != "LIMIT" {
		t.Errorf("expected LIMIT, got %s", req.OrderType)
	}
	if req.Duration != "GOOD_TILL_CANCEL" {
		t.Errorf("expected GOOD_TILL_CANCEL, got %s", req.Duration)
	}
	if req.Price != "150.00" {
		t.Errorf("expected price \"150.00\", got %s", req.Price)
	}
	if len(req.OrderLegCollection) != 1 {
		t.Fatalf("expected 1 leg, got %d", len(req.OrderLegCollection))
	}
	if req.OrderLegCollection[0].Instruction != "BUY" {
		t.Errorf("expected BUY, got %s", req.OrderLegCollection[0].Instruction)
	}
	if req.OrderLegCollection[0].Instrument.Symbol != "AAPL" {
		t.Errorf("expected AAPL, got %s", req.OrderLegCollection[0].Instrument.Symbol)
	}
	if req.OrderLegCollection[0].Instrument.AssetType != "EQUITY" {
		t.Errorf("expected EQUITY, got %s", req.OrderLegCollection[0].Instrument.AssetType)
	}

	// Equity orders should NOT have complexOrderStrategyType set
	if req.ComplexOrderStrategyType != "" {
		t.Errorf("expected empty ComplexOrderStrategyType for equity order, got %q", req.ComplexOrderStrategyType)
	}
}

func TestBuildSchwabOrderRequest_OptionOrder(t *testing.T) {
	req, err := buildSchwabOrderRequest(map[string]interface{}{
		"symbol":      "AAPL250117C00150000",
		"side":        "buy",
		"type":        "limit",
		"qty":         5.0,
		"price":       4.50,
		"asset_class": "us_option",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Option orders must have complexOrderStrategyType set to NONE
	if req.ComplexOrderStrategyType != "NONE" {
		t.Errorf("expected ComplexOrderStrategyType NONE for option order, got %q", req.ComplexOrderStrategyType)
	}
	if req.OrderLegCollection[0].Instrument.AssetType != "OPTION" {
		t.Errorf("expected OPTION, got %s", req.OrderLegCollection[0].Instrument.AssetType)
	}
	if req.Price != "4.50" {
		t.Errorf("expected price \"4.50\", got %s", req.Price)
	}

	// Verify the JSON serialization includes the field
	jsonBytes, _ := json.Marshal(req)
	jsonStr := string(jsonBytes)
	if !strings.Contains(jsonStr, `"complexOrderStrategyType":"NONE"`) {
		t.Errorf("expected JSON to contain complexOrderStrategyType:NONE, got: %s", jsonStr)
	}
}

func TestBuildSchwabMultiLegOrderRequest_ComplexOrderStrategyType(t *testing.T) {
	req, err := buildSchwabMultiLegOrderRequest(map[string]interface{}{
		"type":          "net_debit",
		"price":         2.50,
		"time_in_force": "day",
		"legs": []interface{}{
			map[string]interface{}{
				"symbol":      "AAPL250117C00150000",
				"side":        "buy",
				"qty":         10.0,
				"action":      "BUY_TO_OPEN",
				"asset_class": "us_option",
			},
			map[string]interface{}{
				"symbol":      "AAPL250117C00160000",
				"side":        "sell",
				"qty":         10.0,
				"action":      "SELL_TO_OPEN",
				"asset_class": "us_option",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Multi-leg orders must have complexOrderStrategyType set to CUSTOM
	if req.ComplexOrderStrategyType != "CUSTOM" {
		t.Errorf("expected ComplexOrderStrategyType CUSTOM for multi-leg order, got %q", req.ComplexOrderStrategyType)
	}

	// Verify the JSON serialization includes the field
	jsonBytes, _ := json.Marshal(req)
	jsonStr := string(jsonBytes)
	if !strings.Contains(jsonStr, `"complexOrderStrategyType":"CUSTOM"`) {
		t.Errorf("expected JSON to contain complexOrderStrategyType:CUSTOM, got: %s", jsonStr)
	}
}

// =============================================================================
// Credit/Debit Spread and limit_price Tests
// =============================================================================

// TestBuildSchwabMultiLegOrderRequest_CreditSpread tests that a multi-leg credit
// spread (negative limit_price) correctly produces NET_CREDIT order type with a
// positive price. This is the exact scenario the customer reported failing.
func TestBuildSchwabMultiLegOrderRequest_CreditSpread(t *testing.T) {
	// Exact payload from the customer's bug report
	req, err := buildSchwabMultiLegOrderRequest(map[string]interface{}{
		"order_type":    "limit",
		"limit_price":   -0.41,
		"time_in_force": "day",
		"legs": []interface{}{
			map[string]interface{}{
				"symbol": "SPY260326P00659000",
				"side":   "sell_to_open",
				"qty":    1.0,
			},
			map[string]interface{}{
				"symbol": "SPY260326P00658000",
				"side":   "buy_to_open",
				"qty":    1.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if req.OrderType != "NET_CREDIT" {
		t.Errorf("expected OrderType NET_CREDIT for credit spread, got %q", req.OrderType)
	}
	if req.Price != "0.41" {
		t.Errorf("expected Price \"0.41\" (absolute value), got %q", req.Price)
	}
	if req.ComplexOrderStrategyType != "CUSTOM" {
		t.Errorf("expected ComplexOrderStrategyType CUSTOM, got %q", req.ComplexOrderStrategyType)
	}

	// Verify JSON serialization
	jsonBytes, _ := json.Marshal(req)
	jsonStr := string(jsonBytes)
	if !strings.Contains(jsonStr, `"orderType":"NET_CREDIT"`) {
		t.Errorf("expected JSON orderType:NET_CREDIT, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"price":"0.41"`) {
		t.Errorf("expected JSON price:\"0.41\", got: %s", jsonStr)
	}
}

// TestBuildSchwabMultiLegOrderRequest_DebitSpread tests that a multi-leg debit
// spread (positive limit_price) correctly produces NET_DEBIT order type.
func TestBuildSchwabMultiLegOrderRequest_DebitSpread(t *testing.T) {
	req, err := buildSchwabMultiLegOrderRequest(map[string]interface{}{
		"order_type":    "limit",
		"limit_price":   1.25,
		"time_in_force": "day",
		"legs": []interface{}{
			map[string]interface{}{
				"symbol": "SPY260326C00680000",
				"side":   "buy_to_open",
				"qty":    1.0,
			},
			map[string]interface{}{
				"symbol": "SPY260326C00685000",
				"side":   "sell_to_open",
				"qty":    1.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if req.OrderType != "NET_DEBIT" {
		t.Errorf("expected OrderType NET_DEBIT for debit spread, got %q", req.OrderType)
	}
	if req.Price != "1.25" {
		t.Errorf("expected Price \"1.25\", got %q", req.Price)
	}
}

// TestBuildSchwabMultiLegOrderRequest_LimitPriceFallbackToPrice tests that
// the builder reads "price" when "limit_price" is absent.
func TestBuildSchwabMultiLegOrderRequest_LimitPriceFallbackToPrice(t *testing.T) {
	req, err := buildSchwabMultiLegOrderRequest(map[string]interface{}{
		"order_type":    "limit",
		"price":         -0.55,
		"time_in_force": "day",
		"legs": []interface{}{
			map[string]interface{}{
				"symbol": "SPY260326P00659000",
				"side":   "sell_to_open",
				"qty":    1.0,
			},
			map[string]interface{}{
				"symbol": "SPY260326P00658000",
				"side":   "buy_to_open",
				"qty":    1.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if req.OrderType != "NET_CREDIT" {
		t.Errorf("expected OrderType NET_CREDIT when price is negative, got %q", req.OrderType)
	}
	if req.Price != "0.55" {
		t.Errorf("expected Price \"0.55\" (abs), got %q", req.Price)
	}
}

// TestBuildSchwabMultiLegOrderRequest_MarketOrder tests that a market order
// with no price keeps MARKET type and no price field.
func TestBuildSchwabMultiLegOrderRequest_MarketOrder(t *testing.T) {
	req, err := buildSchwabMultiLegOrderRequest(map[string]interface{}{
		"order_type":    "market",
		"time_in_force": "day",
		"legs": []interface{}{
			map[string]interface{}{
				"symbol": "SPY260326P00659000",
				"side":   "sell_to_open",
				"qty":    1.0,
			},
			map[string]interface{}{
				"symbol": "SPY260326P00658000",
				"side":   "buy_to_open",
				"qty":    1.0,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if req.OrderType != "MARKET" {
		t.Errorf("expected OrderType MARKET, got %q", req.OrderType)
	}
	if req.Price != "" {
		t.Errorf("expected empty Price for market order, got %q", req.Price)
	}
}

// TestBuildSchwabOrderRequest_LimitPriceNegative tests that single-leg orders
// correctly handle a negative limit_price (use absolute value).
func TestBuildSchwabOrderRequest_LimitPriceNegative(t *testing.T) {
	req, err := buildSchwabOrderRequest(map[string]interface{}{
		"symbol":        "SPY",
		"side":          "sell",
		"qty":           1.0,
		"order_type":    "limit",
		"limit_price":   -5.50,
		"time_in_force": "day",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if req.Price != "5.50" {
		t.Errorf("expected Price \"5.50\" (absolute value of -5.50), got %q", req.Price)
	}
	if req.OrderType != "LIMIT" {
		t.Errorf("expected OrderType LIMIT for single-leg, got %q", req.OrderType)
	}
}

// TestBuildSchwabOrderRequest_LimitPricePositive tests the normal positive price case.
func TestBuildSchwabOrderRequest_LimitPricePositive(t *testing.T) {
	req, err := buildSchwabOrderRequest(map[string]interface{}{
		"symbol":        "AAPL",
		"side":          "buy",
		"qty":           10.0,
		"order_type":    "limit",
		"limit_price":   150.00,
		"time_in_force": "day",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if req.Price != "150.00" {
		t.Errorf("expected Price \"150.00\", got %q", req.Price)
	}
}

// TestMapActionToInstruction_SideValues tests that mapActionToInstruction correctly
// handles side values like "sell_to_open" when action is empty (as sent by the frontend).
func TestMapActionToInstruction_SideValues(t *testing.T) {
	tests := []struct {
		action   string
		side     string
		isOption bool
		expected string
	}{
		// Action takes priority
		{"BUY_TO_OPEN", "sell_to_open", true, "BUY_TO_OPEN"},
		{"SELL_TO_CLOSE", "buy_to_open", true, "SELL_TO_CLOSE"},
		// Empty action, side contains specific instruction (frontend pattern)
		{"", "sell_to_open", true, "SELL_TO_OPEN"},
		{"", "buy_to_open", true, "BUY_TO_OPEN"},
		{"", "sell_to_close", true, "SELL_TO_CLOSE"},
		{"", "buy_to_close", true, "BUY_TO_CLOSE"},
		// Simple buy/sell fallback
		{"", "buy", true, "BUY_TO_OPEN"},
		{"", "sell", true, "SELL_TO_CLOSE"},
		{"", "buy", false, "BUY"},
		{"", "sell", false, "SELL"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("action=%q_side=%q_option=%v", tt.action, tt.side, tt.isOption), func(t *testing.T) {
			result := mapActionToInstruction(tt.action, tt.side, tt.isOption)
			if result != tt.expected {
				t.Errorf("mapActionToInstruction(%q, %q, %v) = %q, want %q",
					tt.action, tt.side, tt.isOption, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Bug Fix Tests: Working Orders Filter + Credit/Debit Display
// =============================================================================

// TestMapOrderStatusFilter_PendingReturnsEmpty verifies that the "pending"
// status filter returns "" (no server-side filter) so we can fetch all orders
// and filter client-side for both "open" and "pending" normalized statuses.
func TestMapOrderStatusFilter_PendingReturnsEmpty(t *testing.T) {
	got := mapOrderStatusFilter("pending")
	if got != "" {
		t.Errorf("mapOrderStatusFilter(\"pending\") = %q, want \"\"", got)
	}
}

// TestMapOrderStatusFilter_OpenAndWorkingReturnEmpty verifies that "open" and
// "working" also return "" (no server-side filter).
func TestMapOrderStatusFilter_OpenAndWorkingReturnEmpty(t *testing.T) {
	for _, input := range []string{"open", "working"} {
		got := mapOrderStatusFilter(input)
		if got != "" {
			t.Errorf("mapOrderStatusFilter(%q) = %q, want \"\"", input, got)
		}
	}
}

// TestIsActiveOrderStatus verifies the helper correctly identifies active statuses.
func TestIsActiveOrderStatus(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{"open", true},
		{"pending", true},
		{"Open", true},
		{"Pending", true},
		{"OPEN", true},
		{"PENDING", true},
		{"filled", false},
		{"canceled", false},
		{"rejected", false},
		{"expired", false},
		{"replaced", false},
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := isActiveOrderStatus(tt.status)
			if got != tt.expected {
				t.Errorf("isActiveOrderStatus(%q) = %v, want %v", tt.status, got, tt.expected)
			}
		})
	}
}

// schwabMixedStatusOrdersResponse contains orders in various Schwab statuses
// for testing client-side filtering. The expected normalized statuses are:
//   - WORKING       → "open"
//   - PENDING_ACTIVATION → "pending"
//   - FILLED        → "filled"
//   - CANCELED      → "canceled"
const schwabMixedStatusOrdersResponse = `[
	{
		"orderId": 300001,
		"session": "NORMAL",
		"duration": "DAY",
		"orderType": "LIMIT",
		"price": 150.00,
		"status": "WORKING",
		"quantity": 100,
		"filledQuantity": 0,
		"enteredTime": "2024-05-16T10:30:00+0000",
		"orderLegCollection": [{
			"instruction": "BUY",
			"quantity": 100,
			"instrument": {"symbol": "AAPL", "assetType": "EQUITY"}
		}]
	},
	{
		"orderId": 300002,
		"session": "NORMAL",
		"duration": "DAY",
		"orderType": "LIMIT",
		"price": 50.00,
		"status": "PENDING_ACTIVATION",
		"quantity": 50,
		"filledQuantity": 0,
		"enteredTime": "2024-05-16T11:00:00+0000",
		"orderLegCollection": [{
			"instruction": "BUY",
			"quantity": 50,
			"instrument": {"symbol": "MSFT", "assetType": "EQUITY"}
		}]
	},
	{
		"orderId": 300003,
		"session": "NORMAL",
		"duration": "GOOD_TILL_CANCEL",
		"orderType": "MARKET",
		"status": "FILLED",
		"quantity": 25,
		"filledQuantity": 25,
		"enteredTime": "2024-05-15T09:30:00+0000",
		"closeTime": "2024-05-15T09:30:05+0000",
		"orderLegCollection": [{
			"instruction": "SELL",
			"quantity": 25,
			"instrument": {"symbol": "GOOG", "assetType": "EQUITY"}
		}]
	},
	{
		"orderId": 300004,
		"session": "NORMAL",
		"duration": "DAY",
		"orderType": "LIMIT",
		"price": 200.00,
		"status": "CANCELED",
		"quantity": 10,
		"filledQuantity": 0,
		"enteredTime": "2024-05-14T10:00:00+0000",
		"orderLegCollection": [{
			"instruction": "BUY",
			"quantity": 10,
			"instrument": {"symbol": "AMZN", "assetType": "EQUITY"}
		}]
	}
]`

// TestGetOrders_PendingFilter_ClientSideFiltering verifies that when the UI
// requests status="pending", we fetch all orders from Schwab (no status param)
// and return only those whose normalized status is "open" or "pending".
func TestGetOrders_PendingFilter_ClientSideFiltering(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/orders"):
			// No status filter should be sent
			status := r.URL.Query().Get("status")
			if status != "" {
				t.Errorf("expected no status filter for pending request, got %q", status)
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabMixedStatusOrdersResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	orders, err := p.GetOrders(context.Background(), "pending")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should only return WORKING (→ "open") and PENDING_ACTIVATION (→ "pending")
	if len(orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(orders))
	}

	// Verify the returned orders are the active ones
	ids := map[string]bool{}
	for _, o := range orders {
		ids[o.ID] = true
		if !isActiveOrderStatus(o.Status) {
			t.Errorf("unexpected non-active order status %q for order %s", o.Status, o.ID)
		}
	}
	if !ids["300001"] {
		t.Error("expected WORKING order 300001 to be included")
	}
	if !ids["300002"] {
		t.Error("expected PENDING_ACTIVATION order 300002 to be included")
	}
}

// TestGetOrders_OpenFilter_IncludesBothStatuses verifies that status="open"
// also returns both "open" and "pending" normalized orders via client-side
// filtering.
func TestGetOrders_OpenFilter_IncludesBothStatuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/orders"):
			// No status filter should be sent
			status := r.URL.Query().Get("status")
			if status != "" {
				t.Errorf("expected no status filter for open request, got %q", status)
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, schwabMixedStatusOrdersResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	orders, err := p.GetOrders(context.Background(), "open")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should only return WORKING (→ "open") and PENDING_ACTIVATION (→ "pending")
	if len(orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(orders))
	}

	// Verify no terminal orders snuck through
	for _, o := range orders {
		if !isActiveOrderStatus(o.Status) {
			t.Errorf("unexpected non-active order status %q for order %s", o.Status, o.ID)
		}
	}
}

// TestTransformSchwabOrder_NetCreditNegatesPrice verifies that a NET_CREDIT
// order has its limit_price negated (so the UI correctly shows "CR") and that
// the order_type is normalized to "limit".
func TestTransformSchwabOrder_NetCreditNegatesPrice(t *testing.T) {
	data := map[string]interface{}{
		"orderId":        200001.0,
		"status":         "WORKING",
		"orderType":      "NET_CREDIT",
		"duration":       "DAY",
		"price":          0.41,
		"quantity":       1.0,
		"filledQuantity": 0.0,
		"enteredTime":    "2026-03-18T10:30:00+0000",
		"orderLegCollection": []interface{}{
			map[string]interface{}{
				"instruction": "SELL_TO_OPEN",
				"quantity":    1.0,
				"instrument": map[string]interface{}{
					"symbol":    "SPY   260326P00659000",
					"assetType": "OPTION",
				},
			},
			map[string]interface{}{
				"instruction": "BUY_TO_OPEN",
				"quantity":    1.0,
				"instrument": map[string]interface{}{
					"symbol":    "SPY   260326P00658000",
					"assetType": "OPTION",
				},
			},
		},
	}

	order := transformSchwabOrder(data)
	if order == nil {
		t.Fatal("expected non-nil order")
	}

	// order_type must be normalized to "limit" (not "net_credit")
	if order.OrderType != "limit" {
		t.Errorf("expected order_type \"limit\", got %q", order.OrderType)
	}

	// limit_price must be negative for credit orders
	if order.LimitPrice == nil {
		t.Fatal("expected non-nil LimitPrice")
	}
	if *order.LimitPrice >= 0 {
		t.Errorf("expected negative LimitPrice for NET_CREDIT, got %f", *order.LimitPrice)
	}
	if fmt.Sprintf("%.2f", *order.LimitPrice) != "-0.41" {
		t.Errorf("expected LimitPrice -0.41, got %.2f", *order.LimitPrice)
	}
}

// TestTransformSchwabOrder_NetDebitPositivePrice verifies that a NET_DEBIT
// order keeps its limit_price positive and that order_type is normalized to
// "limit".
func TestTransformSchwabOrder_NetDebitPositivePrice(t *testing.T) {
	data := map[string]interface{}{
		"orderId":        200002.0,
		"status":         "WORKING",
		"orderType":      "NET_DEBIT",
		"duration":       "DAY",
		"price":          2.50,
		"quantity":       1.0,
		"filledQuantity": 0.0,
		"enteredTime":    "2026-03-18T11:00:00+0000",
		"orderLegCollection": []interface{}{
			map[string]interface{}{
				"instruction": "BUY_TO_OPEN",
				"quantity":    1.0,
				"instrument": map[string]interface{}{
					"symbol":    "SPY   260326C00680000",
					"assetType": "OPTION",
				},
			},
			map[string]interface{}{
				"instruction": "SELL_TO_OPEN",
				"quantity":    1.0,
				"instrument": map[string]interface{}{
					"symbol":    "SPY   260326C00685000",
					"assetType": "OPTION",
				},
			},
		},
	}

	order := transformSchwabOrder(data)
	if order == nil {
		t.Fatal("expected non-nil order")
	}

	// order_type must be normalized to "limit" (not "net_debit")
	if order.OrderType != "limit" {
		t.Errorf("expected order_type \"limit\", got %q", order.OrderType)
	}

	// limit_price must remain positive for debit orders
	if order.LimitPrice == nil {
		t.Fatal("expected non-nil LimitPrice")
	}
	if *order.LimitPrice <= 0 {
		t.Errorf("expected positive LimitPrice for NET_DEBIT, got %f", *order.LimitPrice)
	}
	if fmt.Sprintf("%.2f", *order.LimitPrice) != "2.50" {
		t.Errorf("expected LimitPrice 2.50, got %.2f", *order.LimitPrice)
	}
}
