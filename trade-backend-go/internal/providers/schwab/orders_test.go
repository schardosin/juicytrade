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
			// Verify status filter
			status := r.URL.Query().Get("status")
			if status != "WORKING" {
				t.Errorf("expected status=WORKING for open filter, got %s", status)
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
