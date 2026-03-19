package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"trade-backend-go/internal/models"

	"github.com/gorilla/websocket"
)

// =============================================================================
// HIGH PRIORITY: Malformed JSON Response Tests
// =============================================================================

// TestGetStockQuote_MalformedJSON verifies that the provider returns a
// graceful error (not a panic) when the API returns 200 OK with invalid JSON.
func TestGetStockQuote_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/v1/quotes"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{this is not valid JSON!!!`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	_, err := p.GetStockQuote(context.Background(), "AAPL")
	if err == nil {
		t.Fatal("expected error for malformed JSON response, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("expected 'failed to parse' error, got: %v", err)
	}
}

// TestGetAccount_MalformedJSON verifies that GetAccount returns a graceful
// error when the account endpoint returns corrupted JSON.
func TestGetAccount_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.Contains(r.URL.Path, "/accounts/"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `<<<corrupted json response>>>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	_, err := p.GetAccount(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed JSON account response, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("expected 'failed to parse' error, got: %v", err)
	}
}

// TestGetOrders_MalformedJSON verifies that GetOrders returns a graceful
// error when the orders endpoint returns corrupted JSON.
func TestGetOrders_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/orders"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `not json at all {{{`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	_, err := p.GetOrders(context.Background(), "all")
	if err == nil {
		t.Fatal("expected error for malformed JSON orders response, got nil")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("expected 'failed to parse' error, got: %v", err)
	}
}

// =============================================================================
// HIGH PRIORITY: Concurrent Token Refresh
// =============================================================================

// TestEnsureValidToken_ConcurrentRefresh launches 20 goroutines that all call
// ensureValidToken() with an expired token. The mutex should serialize access
// so that only 1 actual HTTP refresh call is made (the rest see the updated
// token after acquiring the lock).
func TestEnsureValidToken_ConcurrentRefresh(t *testing.T) {
	var refreshCalls int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/v1/oauth/token") {
			atomic.AddInt64(&refreshCalls, 1)
			// Simulate a tiny delay to give other goroutines time to contend
			time.Sleep(10 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"access_token":"concurrent-token","expires_in":1800,"token_type":"Bearer"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)
	// Token is empty → expired → will trigger refresh
	p.accessToken = ""
	p.tokenExpiry = time.Time{}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			errs[idx] = p.ensureValidToken()
		}(i)
	}

	wg.Wait()

	// All goroutines should succeed
	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d failed: %v", i, err)
		}
	}

	// The mutex serializes access. The first goroutine to acquire the lock
	// will refresh. Subsequent goroutines will see the fresh token (expiry >
	// 5 minutes from now) and return without refreshing. We expect exactly 1
	// refresh call.
	calls := atomic.LoadInt64(&refreshCalls)
	if calls != 1 {
		t.Errorf("expected exactly 1 refresh call (mutex serialization), got %d", calls)
	}

	// Verify the token was set
	p.tokenMu.Lock()
	token := p.accessToken
	p.tokenMu.Unlock()

	if token != "concurrent-token" {
		t.Errorf("expected accessToken 'concurrent-token', got %q", token)
	}
}

// =============================================================================
// HIGH PRIORITY: WebSocket Malformed JSON
// =============================================================================

// TestStreamReadLoop_MalformedJSON verifies that the streaming read loop
// handles a non-JSON WebSocket message gracefully (logs a warning, does not
// panic, continues processing).
func TestStreamReadLoop_MalformedJSON(t *testing.T) {
	srv, makeProvider := mockStreamServer(t, func(conn *websocket.Conn) {
		sendLoginSuccess(conn)

		// Send a non-JSON message — should be logged and skipped
		conn.WriteMessage(websocket.TextMessage, []byte(`this is not JSON {{{`))

		// Then send a valid equity data message
		dataMsg := schwabStreamResponse{
			Data: []schwabStreamDataItem{
				{
					Service:   "LEVELONE_EQUITIES",
					Timestamp: time.Now().UnixMilli(),
					Command:   "SUBS",
					Content: []map[string]interface{}{
						{"0": "AAPL", "1": 155.0, "2": 155.5, "3": 155.25},
					},
				},
			},
		}
		dataBytes, _ := json.Marshal(dataMsg)
		conn.WriteMessage(websocket.TextMessage, dataBytes)

		// Keep alive
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	p := makeProvider()
	ctx := context.Background()

	queue := make(chan *models.MarketData, 10)
	p.StreamingQueue = queue

	p.ConnectStreaming(ctx)
	defer p.DisconnectStreaming(ctx)

	// The read loop should skip the bad message and still deliver the valid one
	select {
	case md := <-queue:
		if md.Symbol != "AAPL" {
			t.Errorf("expected symbol AAPL, got %s", md.Symbol)
		}
		if bid, ok := md.Data["bid"].(float64); !ok || bid != 155.0 {
			t.Errorf("expected bid 155.0, got %v", md.Data["bid"])
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for data after malformed JSON — read loop may have crashed")
	}
}

// =============================================================================
// HIGH PRIORITY: PreviewOrder
// =============================================================================

// TestPreviewOrder_SingleLegEquity verifies that PreviewOrder calls the real
// Schwab previewOrder API endpoint and correctly parses commission, fees,
// order cost, and buying power from the response.
func TestPreviewOrder_SingleLegEquity(t *testing.T) {
	const previewResponse = `{
		"orderId": 0,
		"orderStrategy": {
			"accountNumber": "12345678",
			"orderBalance": {
				"orderValue": 15000.00,
				"projectedAvailableFund": 85000.00,
				"projectedBuyingPower": 85000.00,
				"projectedCommission": 0.00
			},
			"orderStrategyType": "SINGLE",
			"session": "NORMAL",
			"duration": "DAY",
			"orderType": "LIMIT",
			"price": 150.00,
			"quantity": 100,
			"orderLegs": [
				{
					"askPrice": 150.50,
					"bidPrice": 149.80,
					"lastPrice": 150.25,
					"markPrice": 150.15,
					"projectedCommission": 0.00,
					"quantity": 100,
					"finalSymbol": "AAPL",
					"legId": 1,
					"assetType": "EQUITY",
					"instruction": "BUY"
				}
			]
		},
		"orderValidationResult": {
			"alerts": [],
			"accepts": [{"validationRuleName": "OrderAccepted", "message": "Order is valid"}],
			"rejects": [],
			"reviews": [],
			"warns": []
		},
		"commissionAndFee": {
			"commission": {
				"commissionLegs": [
					{
						"commissionValues": [
							{"value": 0.00, "type": "COMMISSION"}
						]
					}
				]
			},
			"fee": {
				"feeLegs": [
					{
						"feeValues": [
							{"value": 0.03, "type": "TAF_FEE"},
							{"value": 0.01, "type": "REG_FEE"}
						]
					}
				]
			},
			"trueCommission": {
				"commissionLegs": [
					{
						"commissionValues": [
							{"value": 0.00, "type": "COMMISSION"}
						]
					}
				]
			}
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/previewOrder") && r.Method == http.MethodPost:
			// Verify it's a POST with a JSON body
			bodyBytes, _ := io.ReadAll(r.Body)
			var orderReq map[string]interface{}
			if err := json.Unmarshal(bodyBytes, &orderReq); err != nil {
				t.Errorf("failed to parse request body: %v", err)
			}
			// Verify order fields are present
			if orderReq["orderType"] != "LIMIT" {
				t.Errorf("expected orderType LIMIT, got %v", orderReq["orderType"])
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, previewResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	result, err := p.PreviewOrder(context.Background(), map[string]interface{}{
		"symbol":        "AAPL",
		"side":          "buy",
		"qty":           100.0,
		"type":          "limit",
		"price":         150.0,
		"time_in_force": "day",
	})

	if err != nil {
		t.Fatalf("expected no Go error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Verify status is ok
	if status, ok := result["status"].(string); !ok || status != "ok" {
		t.Errorf("expected status 'ok', got %v", result["status"])
	}

	// Verify NO preview_not_available field (this is a real API response)
	if _, exists := result["preview_not_available"]; exists {
		t.Error("expected no preview_not_available field for real API response")
	}

	// Verify commission is 0.00
	if commission, ok := result["commission"].(float64); !ok || commission != 0.00 {
		t.Errorf("expected commission 0.00, got %v", result["commission"])
	}

	// Verify fees = 0.03 + 0.01 = 0.04
	if fees, ok := result["fees"].(float64); !ok || fmt.Sprintf("%.2f", fees) != "0.04" {
		t.Errorf("expected fees 0.04, got %v", result["fees"])
	}

	// Verify order cost
	if orderCost, ok := result["order_cost"].(float64); !ok || orderCost != 15000.00 {
		t.Errorf("expected order_cost 15000.00, got %v", result["order_cost"])
	}

	// Verify estimated total = orderCost + commission + fees = 15000.00 + 0.00 + 0.04
	if total, ok := result["estimated_total"].(float64); !ok || fmt.Sprintf("%.2f", total) != "15000.04" {
		t.Errorf("expected estimated_total 15000.04, got %v", result["estimated_total"])
	}

	// Verify buying power effect
	if bp, ok := result["buying_power_effect"].(float64); !ok || bp != 85000.00 {
		t.Errorf("expected buying_power_effect 85000.00, got %v", result["buying_power_effect"])
	}

	// Verify all standardized fields are present
	requiredFields := []string{
		"status", "commission", "cost", "fees", "order_cost",
		"margin_change", "buying_power_effect", "day_trades",
		"estimated_total", "validation_errors",
	}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("missing required field %q in preview result", field)
		}
	}
}

// TestPreviewOrder_MultiLegOption verifies that PreviewOrder correctly handles
// a multi-leg option order (vertical spread) via the real Schwab preview API.
func TestPreviewOrder_MultiLegOption(t *testing.T) {
	const previewResponse = `{
		"orderId": 0,
		"orderStrategy": {
			"accountNumber": "12345678",
			"orderBalance": {
				"orderValue": 250.00,
				"projectedAvailableFund": 99750.00,
				"projectedBuyingPower": 99750.00,
				"projectedCommission": 1.30
			},
			"orderStrategyType": "SINGLE",
			"session": "NORMAL",
			"duration": "DAY",
			"orderType": "NET_DEBIT",
			"price": 2.50,
			"orderLegs": [
				{
					"projectedCommission": 0.65,
					"quantity": 1,
					"finalSymbol": "AAPL  251219C00200000",
					"legId": 1,
					"assetType": "OPTION",
					"instruction": "BUY_TO_OPEN"
				},
				{
					"projectedCommission": 0.65,
					"quantity": 1,
					"finalSymbol": "AAPL  251219C00210000",
					"legId": 2,
					"assetType": "OPTION",
					"instruction": "SELL_TO_OPEN"
				}
			]
		},
		"orderValidationResult": {
			"alerts": [],
			"accepts": [{"validationRuleName": "OrderAccepted", "message": "Order is valid"}],
			"rejects": [],
			"reviews": [],
			"warns": []
		},
		"commissionAndFee": {
			"commission": {
				"commissionLegs": [
					{"commissionValues": [{"value": 0.65, "type": "COMMISSION"}]},
					{"commissionValues": [{"value": 0.65, "type": "COMMISSION"}]}
				]
			},
			"fee": {
				"feeLegs": [
					{"feeValues": [{"value": 0.04, "type": "ORF_FEE"}]},
					{"feeValues": [{"value": 0.04, "type": "ORF_FEE"}]}
				]
			},
			"trueCommission": {
				"commissionLegs": [
					{"commissionValues": [{"value": 0.65, "type": "COMMISSION"}]},
					{"commissionValues": [{"value": 0.65, "type": "COMMISSION"}]}
				]
			}
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/previewOrder") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, previewResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	result, err := p.PreviewOrder(context.Background(), map[string]interface{}{
		"type":          "net_debit",
		"price":         2.50,
		"time_in_force": "day",
		"legs": []interface{}{
			map[string]interface{}{
				"symbol":      "AAPL251219C00200000",
				"side":        "buy_to_open",
				"qty":         1.0,
				"asset_class": "option",
			},
			map[string]interface{}{
				"symbol":      "AAPL251219C00210000",
				"side":        "sell_to_open",
				"qty":         1.0,
				"asset_class": "option",
			},
		},
	})

	if err != nil {
		t.Fatalf("expected no Go error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if status, ok := result["status"].(string); !ok || status != "ok" {
		t.Errorf("expected status 'ok', got %v", result["status"])
	}

	// Verify NO preview_not_available field
	if _, exists := result["preview_not_available"]; exists {
		t.Error("expected no preview_not_available field for real API response")
	}

	// Verify commission = 0.65 + 0.65 = 1.30
	if commission, ok := result["commission"].(float64); !ok || fmt.Sprintf("%.2f", commission) != "1.30" {
		t.Errorf("expected commission 1.30, got %v", result["commission"])
	}

	// Verify fees = 0.04 + 0.04 = 0.08
	if fees, ok := result["fees"].(float64); !ok || fmt.Sprintf("%.2f", fees) != "0.08" {
		t.Errorf("expected fees 0.08, got %v", result["fees"])
	}

	// Verify order cost
	if orderCost, ok := result["order_cost"].(float64); !ok || orderCost != 250.00 {
		t.Errorf("expected order_cost 250.00, got %v", result["order_cost"])
	}

	// Verify estimated total = 250.00 + 1.30 + 0.08 = 251.38
	if total, ok := result["estimated_total"].(float64); !ok || fmt.Sprintf("%.2f", total) != "251.38" {
		t.Errorf("expected estimated_total 251.38, got %v", result["estimated_total"])
	}

	// Verify buying power effect
	if bp, ok := result["buying_power_effect"].(float64); !ok || bp != 99750.00 {
		t.Errorf("expected buying_power_effect 99750.00, got %v", result["buying_power_effect"])
	}
}

// TestPreviewOrder_ValidationRejects verifies that PreviewOrder correctly handles
// a response where orderValidationResult.rejects contains error messages.
func TestPreviewOrder_ValidationRejects(t *testing.T) {
	const rejectResponse = `{
		"orderId": 0,
		"orderStrategy": {
			"orderBalance": {
				"orderValue": 0,
				"projectedBuyingPower": 0
			}
		},
		"orderValidationResult": {
			"alerts": [],
			"accepts": [],
			"rejects": [
				{
					"validationRuleName": "InsufficientFunds",
					"message": "Insufficient buying power to place this order",
					"activityMessage": "Order rejected due to insufficient funds",
					"originalSeverity": "REJECT"
				},
				{
					"validationRuleName": "MarketClosed",
					"message": "Market is currently closed for this security",
					"activityMessage": "Cannot place order outside market hours",
					"originalSeverity": "REJECT"
				}
			],
			"reviews": [],
			"warns": []
		},
		"commissionAndFee": {
			"commission": {"commissionLegs": []},
			"fee": {"feeLegs": []}
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/previewOrder") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, rejectResponse)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	result, err := p.PreviewOrder(context.Background(), map[string]interface{}{
		"symbol":        "AAPL",
		"side":          "buy",
		"qty":           10000.0,
		"type":          "market",
		"time_in_force": "day",
	})

	// Must NOT return a Go error — structured error result instead
	if err != nil {
		t.Fatalf("expected no Go error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Status must be "error"
	if status, ok := result["status"].(string); !ok || status != "error" {
		t.Errorf("expected status 'error', got %v", result["status"])
	}

	// validation_errors must contain both reject messages
	validationErrors, ok := result["validation_errors"].([]string)
	if !ok {
		t.Fatalf("expected validation_errors to be []string, got %T", result["validation_errors"])
	}

	if len(validationErrors) != 2 {
		t.Fatalf("expected 2 validation errors, got %d: %v", len(validationErrors), validationErrors)
	}

	if !strings.Contains(validationErrors[0], "Order rejected due to insufficient funds") {
		t.Errorf("expected first error to mention 'Order rejected due to insufficient funds', got: %s", validationErrors[0])
	}
	if !strings.Contains(validationErrors[1], "Cannot place order outside market hours") {
		t.Errorf("expected second error to mention 'Cannot place order outside market hours', got: %s", validationErrors[1])
	}
}

// TestPreviewOrder_InvalidOrderData_ReturnsStructuredError verifies that
// invalid order data (missing required fields) returns a structured error
// result, not a Go error.
func TestPreviewOrder_InvalidOrderData_ReturnsStructuredError(t *testing.T) {
	// No mock server needed — the build step fails before any HTTP call
	p := newTestProvider("http://unused")

	result, err := p.PreviewOrder(context.Background(), map[string]interface{}{
		"symbol": "",
		"side":   "buy",
	})

	// Must NOT return a Go error
	if err != nil {
		t.Fatalf("expected no Go error for invalid order data, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for invalid input")
	}

	// Status must be "error"
	if status, ok := result["status"].(string); !ok || status != "error" {
		t.Errorf("expected status 'error', got %v", result["status"])
	}

	// validation_errors must explain the issue
	if validationErrors, ok := result["validation_errors"].([]string); ok {
		if len(validationErrors) == 0 {
			t.Error("expected non-empty validation_errors for invalid input")
		}
	} else {
		t.Errorf("expected validation_errors to be []string, got %T", result["validation_errors"])
	}
}

// =============================================================================
// MEDIUM PRIORITY: Stop & Stop-Limit Orders
// =============================================================================

// TestPlaceOrder_StopOrder verifies that a stop order correctly sets the
// Schwab order type to STOP and includes the stop price.
func TestPlaceOrder_StopOrder(t *testing.T) {
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
			fmt.Fprint(w, `{"orderId": 300001}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	order, err := p.PlaceOrder(context.Background(), map[string]interface{}{
		"symbol":        "AAPL",
		"side":          "sell",
		"type":          "stop",
		"qty":           50.0,
		"stop_price":    140.0,
		"time_in_force": "day",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if order == nil {
		t.Fatal("expected non-nil order")
	}

	// Verify request body
	if receivedBody["orderType"] != "STOP" {
		t.Errorf("expected orderType STOP, got %v", receivedBody["orderType"])
	}
	if receivedBody["stopPrice"] != "140.00" {
		t.Errorf("expected stopPrice \"140.00\", got %v", receivedBody["stopPrice"])
	}

	// Verify returned order
	if order.StopPrice == nil || *order.StopPrice != 140.0 {
		t.Errorf("expected stop price 140.0 on returned order, got %v", order.StopPrice)
	}
	if order.OrderType != "stop" {
		t.Errorf("expected order type 'stop', got %s", order.OrderType)
	}
}

// TestPlaceOrder_StopLimitOrder verifies that a stop-limit order correctly
// sets both the stop price and the limit price.
func TestPlaceOrder_StopLimitOrder(t *testing.T) {
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
			fmt.Fprint(w, `{"orderId": 300002}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	order, err := p.PlaceOrder(context.Background(), map[string]interface{}{
		"symbol":        "MSFT",
		"side":          "buy",
		"type":          "stop_limit",
		"qty":           25.0,
		"price":         380.0,
		"stop_price":    375.0,
		"time_in_force": "gtc",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if order == nil {
		t.Fatal("expected non-nil order")
	}

	// Verify request body
	if receivedBody["orderType"] != "STOP_LIMIT" {
		t.Errorf("expected orderType STOP_LIMIT, got %v", receivedBody["orderType"])
	}
	if receivedBody["price"] != "380.00" {
		t.Errorf("expected price \"380.00\", got %v", receivedBody["price"])
	}
	if receivedBody["stopPrice"] != "375.00" {
		t.Errorf("expected stopPrice \"375.00\", got %v", receivedBody["stopPrice"])
	}
	if receivedBody["duration"] != "GOOD_TILL_CANCEL" {
		t.Errorf("expected duration GOOD_TILL_CANCEL, got %v", receivedBody["duration"])
	}

	// Verify returned order
	if order.LimitPrice == nil || *order.LimitPrice != 380.0 {
		t.Errorf("expected limit price 380.0, got %v", order.LimitPrice)
	}
	if order.StopPrice == nil || *order.StopPrice != 375.0 {
		t.Errorf("expected stop price 375.0, got %v", order.StopPrice)
	}
	if order.OrderType != "stop_limit" {
		t.Errorf("expected order type 'stop_limit', got %s", order.OrderType)
	}
	if order.TimeInForce != "gtc" {
		t.Errorf("expected TIF 'gtc', got %s", order.TimeInForce)
	}
}

// =============================================================================
// MEDIUM PRIORITY: Short Position
// =============================================================================

// TestGetPositions_ShortPosition verifies that when shortQuantity exceeds
// longQuantity, the position is correctly marked as short with the absolute
// quantity.
func TestGetPositions_ShortPosition(t *testing.T) {
	const shortPositionResponse = `{
		"securitiesAccount": {
			"type": "MARGIN",
			"accountNumber": "12345678",
			"positions": [
				{
					"instrument": {"symbol": "TSLA", "assetType": "EQUITY"},
					"longQuantity": 0.0,
					"shortQuantity": 50.0,
					"averagePrice": 250.00,
					"marketValue": -12500.00,
					"currentDayProfitLoss": -200.00,
					"currentDayProfitLossPercentage": -1.58
				}
			],
			"currentBalances": {}
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.Contains(r.URL.Path, "/accounts/"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, shortPositionResponse)
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

	if len(positions) != 1 {
		t.Fatalf("expected 1 position, got %d", len(positions))
	}

	pos := positions[0]
	if pos.Symbol != "TSLA" {
		t.Errorf("expected symbol TSLA, got %s", pos.Symbol)
	}
	if pos.Side != "short" {
		t.Errorf("expected side 'short', got %s", pos.Side)
	}
	if pos.Qty != 50.0 {
		t.Errorf("expected qty 50.0 (absolute), got %f", pos.Qty)
	}
	if pos.AssetClass != "us_equity" {
		t.Errorf("expected asset class 'us_equity', got %s", pos.AssetClass)
	}
	if pos.AvgEntryPrice != 250.00 {
		t.Errorf("expected avg entry price 250.00, got %f", pos.AvgEntryPrice)
	}
}

// =============================================================================
// MEDIUM PRIORITY: Order Filters
// =============================================================================

// TestGetOrders_FilledFilter verifies that passing "filled" status correctly
// translates to Schwab's "FILLED" status parameter.
func TestGetOrders_FilledFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/orders"):
			status := r.URL.Query().Get("status")
			if status != "FILLED" {
				t.Errorf("expected status=FILLED for filled filter, got %q", status)
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `[]`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	orders, err := p.GetOrders(context.Background(), "filled")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(orders) != 0 {
		t.Fatalf("expected 0 orders, got %d", len(orders))
	}
}

// =============================================================================
// MEDIUM PRIORITY: Account Activity — Canceled Order
// =============================================================================

// TestProcessAccountActivity_CanceledOrder verifies that a CANCELED order
// status is correctly mapped through processAccountActivity and the callback
// is invoked with the normalized event.
func TestProcessAccountActivity_CanceledOrder(t *testing.T) {
	p := newTestProvider("http://unused")

	var mu sync.Mutex
	var receivedEvents []*models.OrderEvent

	p.SetOrderEventCallback(func(event *models.OrderEvent) {
		mu.Lock()
		receivedEvents = append(receivedEvents, event)
		mu.Unlock()
	})

	data := schwabStreamDataItem{
		Service:   "ACCT_ACTIVITY",
		Timestamp: time.Now().UnixMilli(),
		Command:   "SUBS",
		Content: []map[string]interface{}{
			{
				"orderId":   "88888",
				"status":    "CANCELED",
				"symbol":    "GOOGL",
				"side":      "BUY_TO_OPEN",
				"orderType": "LIMIT",
				"quantity":  20.0,
				"price":     180.50,
			},
		},
	}

	p.processAccountActivity(data)

	mu.Lock()
	defer mu.Unlock()

	if len(receivedEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(receivedEvents))
	}

	event := receivedEvents[0]
	if event.GetIDAsString() != "88888" {
		t.Errorf("expected order ID '88888', got %s", event.GetIDAsString())
	}
	if event.Status != "canceled" {
		t.Errorf("expected status 'canceled', got %s", event.Status)
	}
	if event.Symbol != "GOOGL" {
		t.Errorf("expected symbol 'GOOGL', got %s", event.Symbol)
	}
	if event.Side != "buy" {
		t.Errorf("expected side 'buy' (mapped from BUY_TO_OPEN), got %s", event.Side)
	}
	if event.Type != "limit" {
		t.Errorf("expected type 'limit', got %s", event.Type)
	}
	if event.NormalizedEvent != models.NormalizedEventCancelled {
		t.Errorf("expected normalized event '%s', got '%s'",
			models.NormalizedEventCancelled, event.NormalizedEvent)
	}
}
