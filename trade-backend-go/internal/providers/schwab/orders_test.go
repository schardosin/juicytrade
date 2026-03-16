package schwab

import (
	"context"
	"fmt"
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
