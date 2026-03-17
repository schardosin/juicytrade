package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"trade-backend-go/internal/models"

	"github.com/gorilla/websocket"
)

// =============================================================================
// StartAccountStream tests
// =============================================================================

func TestStartAccountStream_SubscribesACCT_ACTIVITY(t *testing.T) {
	var mu sync.Mutex
	var receivedSubs []schwabStreamRequestItem

	srv, makeProvider := mockStreamServer(t, func(conn *websocket.Conn) {
		sendLoginSuccess(conn)

		// Read messages
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var req schwabStreamRequest
			if err := json.Unmarshal(msg, &req); err == nil {
				for _, item := range req.Requests {
					if item.Command == "SUBS" {
						mu.Lock()
						receivedSubs = append(receivedSubs, item)
						mu.Unlock()
					}
				}
			}
		}
	})
	defer srv.Close()

	p := makeProvider()
	ctx := context.Background()

	// Connect first
	p.ConnectStreaming(ctx)
	defer p.DisconnectStreaming(ctx)

	// Start account stream
	err := p.StartAccountStream(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Wait for message to arrive
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have received an ACCT_ACTIVITY SUBS request
	var found bool
	for _, item := range receivedSubs {
		if item.Service == "ACCT_ACTIVITY" {
			found = true
			if item.SchwabClientCustomerID != "test-customer-id" {
				t.Errorf("expected customer ID 'test-customer-id', got %s", item.SchwabClientCustomerID)
			}
			break
		}
	}
	if !found {
		t.Error("expected ACCT_ACTIVITY SUBS request")
	}

	// acctStreamActive should be true
	if !p.IsAccountStreamConnected() {
		t.Error("expected IsAccountStreamConnected to return true")
	}
}

func TestStartAccountStream_ConnectsIfNotConnected(t *testing.T) {
	srv, makeProvider := mockStreamServer(t, func(conn *websocket.Conn) {
		sendLoginSuccess(conn)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	p := makeProvider()
	ctx := context.Background()
	defer p.DisconnectStreaming(ctx)

	// Not connected initially
	if p.IsConnected {
		t.Fatal("expected not connected initially")
	}

	// StartAccountStream should connect automatically
	err := p.StartAccountStream(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !p.IsConnected {
		t.Error("expected IsConnected after StartAccountStream auto-connect")
	}
	if !p.IsAccountStreamConnected() {
		t.Error("expected IsAccountStreamConnected to return true")
	}
}

func TestStartAccountStream_ConnectionFailure(t *testing.T) {
	// Server that returns token but nothing else — WebSocket will fail
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/userPreference"):
			w.WriteHeader(http.StatusOK)
			// Return a WebSocket URL that doesn't actually have a WS server
			fmt.Fprint(w, `{
				"streamerInfo": [{
					"streamerSocketUrl": "ws://127.0.0.1:1/ws",
					"schwabClientCustomerId": "test-customer-id",
					"schwabClientCorrelId": "test-correl-id"
				}]
			}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)

	err := p.StartAccountStream(context.Background())
	if err == nil {
		t.Fatal("expected error when connection fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to connect") {
		t.Errorf("expected 'failed to connect' error, got: %v", err)
	}
}

// =============================================================================
// StopAccountStream tests
// =============================================================================

func TestStopAccountStream_ClearsActiveState(t *testing.T) {
	var mu sync.Mutex
	var receivedUnsubs []schwabStreamRequestItem

	srv, makeProvider := mockStreamServer(t, func(conn *websocket.Conn) {
		sendLoginSuccess(conn)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var req schwabStreamRequest
			if err := json.Unmarshal(msg, &req); err == nil {
				for _, item := range req.Requests {
					if item.Command == "UNSUBS" {
						mu.Lock()
						receivedUnsubs = append(receivedUnsubs, item)
						mu.Unlock()
					}
				}
			}
		}
	})
	defer srv.Close()

	p := makeProvider()
	ctx := context.Background()

	p.ConnectStreaming(ctx)
	defer p.DisconnectStreaming(ctx)

	// Start then stop
	p.StartAccountStream(ctx)
	if !p.IsAccountStreamConnected() {
		t.Fatal("expected account stream to be connected")
	}

	p.StopAccountStream()

	// acctStreamActive should be false
	if p.IsAccountStreamConnected() {
		t.Error("expected IsAccountStreamConnected to return false after stop")
	}

	// Wait for message
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have sent UNSUBS for ACCT_ACTIVITY
	var found bool
	for _, item := range receivedUnsubs {
		if item.Service == "ACCT_ACTIVITY" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected ACCT_ACTIVITY UNSUBS request")
	}
}

func TestStopAccountStream_NoOpWhenNotActive(t *testing.T) {
	p := newTestProvider("http://unused")

	// Should not panic when stopping while never started
	p.StopAccountStream()

	if p.IsAccountStreamConnected() {
		t.Error("expected IsAccountStreamConnected to be false")
	}
}

// =============================================================================
// SetOrderEventCallback tests
// =============================================================================

func TestSetOrderEventCallback_StoresCallback(t *testing.T) {
	p := newTestProvider("http://unused")

	called := false
	p.SetOrderEventCallback(func(event *models.OrderEvent) {
		called = true
	})

	// Verify callback is stored
	p.acctStreamMu.RLock()
	cb := p.orderEventCallback
	p.acctStreamMu.RUnlock()

	if cb == nil {
		t.Fatal("expected callback to be stored")
	}

	// Invoke it
	cb(&models.OrderEvent{})
	if !called {
		t.Error("expected callback to be invoked")
	}
}

func TestSetOrderEventCallback_ReplacesExisting(t *testing.T) {
	p := newTestProvider("http://unused")

	firstCalled := false
	secondCalled := false

	p.SetOrderEventCallback(func(event *models.OrderEvent) {
		firstCalled = true
	})
	p.SetOrderEventCallback(func(event *models.OrderEvent) {
		secondCalled = true
	})

	p.acctStreamMu.RLock()
	cb := p.orderEventCallback
	p.acctStreamMu.RUnlock()

	cb(&models.OrderEvent{})

	if firstCalled {
		t.Error("expected first callback to NOT be called after replacement")
	}
	if !secondCalled {
		t.Error("expected second callback to be called")
	}
}

// =============================================================================
// IsAccountStreamConnected tests
// =============================================================================

func TestIsAccountStreamConnected_BothRequired(t *testing.T) {
	p := newTestProvider("http://unused")

	// Neither active nor connected
	if p.IsAccountStreamConnected() {
		t.Error("expected false when neither active nor connected")
	}

	// Active but not connected
	p.acctStreamMu.Lock()
	p.acctStreamActive = true
	p.acctStreamMu.Unlock()
	if p.IsAccountStreamConnected() {
		t.Error("expected false when active but not connected")
	}

	// Connected but not active
	p.acctStreamMu.Lock()
	p.acctStreamActive = false
	p.acctStreamMu.Unlock()
	p.IsConnected = true
	if p.IsAccountStreamConnected() {
		t.Error("expected false when connected but not active")
	}

	// Both active and connected
	p.acctStreamMu.Lock()
	p.acctStreamActive = true
	p.acctStreamMu.Unlock()
	if !p.IsAccountStreamConnected() {
		t.Error("expected true when both active and connected")
	}
}

// =============================================================================
// processAccountActivity tests
// =============================================================================

func TestProcessAccountActivity_InvokesCallback(t *testing.T) {
	p := newTestProvider("http://unused")

	var mu sync.Mutex
	var receivedEvents []*models.OrderEvent

	p.SetOrderEventCallback(func(event *models.OrderEvent) {
		mu.Lock()
		receivedEvents = append(receivedEvents, event)
		mu.Unlock()
	})

	// Simulate account activity data with order information
	data := schwabStreamDataItem{
		Service:   "ACCT_ACTIVITY",
		Timestamp: time.Now().UnixMilli(),
		Command:   "SUBS",
		Content: []map[string]interface{}{
			{
				"orderId":          "12345",
				"status":           "FILLED",
				"symbol":           "AAPL",
				"side":             "BUY",
				"orderType":        "LIMIT",
				"quantity":         10.0,
				"price":            150.25,
				"avgFillPrice":     150.20,
				"executedQuantity": 10.0,
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
	if event.GetIDAsString() != "12345" {
		t.Errorf("expected order ID '12345', got %s", event.GetIDAsString())
	}
	if event.Status != "filled" {
		t.Errorf("expected status 'filled', got %s", event.Status)
	}
	if event.Symbol != "AAPL" {
		t.Errorf("expected symbol 'AAPL', got %s", event.Symbol)
	}
	if event.Side != "buy" {
		t.Errorf("expected side 'buy', got %s", event.Side)
	}
	if event.Type != "limit" {
		t.Errorf("expected type 'limit', got %s", event.Type)
	}
	if event.Quantity != 10.0 {
		t.Errorf("expected quantity 10.0, got %f", event.Quantity)
	}
	if event.AvgFillPrice != 150.20 {
		t.Errorf("expected avgFillPrice 150.20, got %f", event.AvgFillPrice)
	}
	if event.NormalizedEvent != models.NormalizedEventFilled {
		t.Errorf("expected normalized event '%s', got '%s'",
			models.NormalizedEventFilled, event.NormalizedEvent)
	}
}

func TestProcessAccountActivity_PendingOrder(t *testing.T) {
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
		Content: []map[string]interface{}{
			{
				"orderId":  "67890",
				"status":   "PENDING_ACTIVATION",
				"symbol":   "MSFT",
				"side":     "SELL_TO_CLOSE",
				"quantity": 5.0,
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
	if event.Status != "pending" {
		t.Errorf("expected status 'pending', got %s", event.Status)
	}
	if event.Side != "sell" {
		t.Errorf("expected side 'sell', got %s", event.Side)
	}
	if event.NormalizedEvent != models.NormalizedEventSubmitted {
		t.Errorf("expected normalized event '%s', got '%s'",
			models.NormalizedEventSubmitted, event.NormalizedEvent)
	}
}

func TestProcessAccountActivity_NoCallback(t *testing.T) {
	p := newTestProvider("http://unused")
	// No callback set — should not panic

	data := schwabStreamDataItem{
		Service:   "ACCT_ACTIVITY",
		Timestamp: time.Now().UnixMilli(),
		Content: []map[string]interface{}{
			{
				"orderId": "99999",
				"status":  "FILLED",
				"symbol":  "SPY",
			},
		},
	}

	// Should not panic
	p.processAccountActivity(data)
}

func TestProcessAccountActivity_EmptyContent(t *testing.T) {
	p := newTestProvider("http://unused")

	called := false
	p.SetOrderEventCallback(func(event *models.OrderEvent) {
		called = true
	})

	data := schwabStreamDataItem{
		Service:   "ACCT_ACTIVITY",
		Timestamp: time.Now().UnixMilli(),
		Content:   []map[string]interface{}{},
	}

	p.processAccountActivity(data)

	if called {
		t.Error("expected callback to NOT be called for empty content")
	}
}

func TestProcessAccountActivity_UnparseableItem(t *testing.T) {
	p := newTestProvider("http://unused")

	called := false
	p.SetOrderEventCallback(func(event *models.OrderEvent) {
		called = true
	})

	// Item with no recognizable order fields
	data := schwabStreamDataItem{
		Service:   "ACCT_ACTIVITY",
		Timestamp: time.Now().UnixMilli(),
		Content: []map[string]interface{}{
			{"randomKey": "randomValue"},
		},
	}

	p.processAccountActivity(data)

	if called {
		t.Error("expected callback to NOT be called for unparseable item")
	}
}

func TestProcessAccountActivity_MultipleEvents(t *testing.T) {
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
		Content: []map[string]interface{}{
			{
				"orderId": "100",
				"status":  "FILLED",
				"symbol":  "AAPL",
			},
			{
				"orderId": "101",
				"status":  "PENDING_ACTIVATION",
				"symbol":  "MSFT",
			},
		},
	}

	p.processAccountActivity(data)

	mu.Lock()
	defer mu.Unlock()

	if len(receivedEvents) != 2 {
		t.Fatalf("expected 2 events, got %d", len(receivedEvents))
	}

	if receivedEvents[0].GetIDAsString() != "100" {
		t.Errorf("expected first event ID '100', got %s", receivedEvents[0].GetIDAsString())
	}
	if receivedEvents[1].GetIDAsString() != "101" {
		t.Errorf("expected second event ID '101', got %s", receivedEvents[1].GetIDAsString())
	}
}

// =============================================================================
// processStreamData routing test
// =============================================================================

func TestProcessStreamData_RoutesACCT_ACTIVITY(t *testing.T) {
	p := newTestProvider("http://unused")
	p.IsConnected = true

	var mu sync.Mutex
	var receivedEvents []*models.OrderEvent

	p.SetOrderEventCallback(func(event *models.OrderEvent) {
		mu.Lock()
		receivedEvents = append(receivedEvents, event)
		mu.Unlock()
	})

	// Also set up a queue to verify market data is NOT dispatched
	queue := make(chan *models.MarketData, 10)
	p.StreamingQueue = queue

	data := schwabStreamDataItem{
		Service:   "ACCT_ACTIVITY",
		Timestamp: time.Now().UnixMilli(),
		Content: []map[string]interface{}{
			{
				"orderId": "55555",
				"status":  "FILLED",
				"symbol":  "TSLA",
			},
		},
	}

	p.processStreamData(data)

	mu.Lock()
	eventCount := len(receivedEvents)
	mu.Unlock()

	if eventCount != 1 {
		t.Fatalf("expected 1 account event, got %d", eventCount)
	}

	// Market data queue should be empty
	select {
	case <-queue:
		t.Error("expected no market data for ACCT_ACTIVITY service")
	default:
		// Expected
	}
}

// =============================================================================
// Helper function tests
// =============================================================================

func TestExtractStringField(t *testing.T) {
	item := map[string]interface{}{
		"orderId": "12345",
		"Status":  "FILLED",
		"empty":   "",
		"numeric": 42,
	}

	// Found with first key
	val, ok := extractStringField(item, "orderId")
	if !ok || val != "12345" {
		t.Errorf("expected '12345', got %q, ok=%v", val, ok)
	}

	// Found with fallback key
	val, ok = extractStringField(item, "status", "Status")
	if !ok || val != "FILLED" {
		t.Errorf("expected 'FILLED', got %q, ok=%v", val, ok)
	}

	// Not found
	_, ok = extractStringField(item, "missing")
	if ok {
		t.Error("expected not found")
	}

	// Empty string is not returned
	_, ok = extractStringField(item, "empty")
	if ok {
		t.Error("expected not found for empty string")
	}

	// Non-string value
	_, ok = extractStringField(item, "numeric")
	if ok {
		t.Error("expected not found for non-string value")
	}
}

func TestExtractFloatField(t *testing.T) {
	item := map[string]interface{}{
		"price":    150.25,
		"quantity": 10,
		"bignum":   int64(1000000),
		"text":     "not a number",
	}

	// float64
	val, ok := extractFloatField(item, "price")
	if !ok || val != 150.25 {
		t.Errorf("expected 150.25, got %f, ok=%v", val, ok)
	}

	// int
	val, ok = extractFloatField(item, "quantity")
	if !ok || val != 10.0 {
		t.Errorf("expected 10.0, got %f, ok=%v", val, ok)
	}

	// int64
	val, ok = extractFloatField(item, "bignum")
	if !ok || val != 1000000.0 {
		t.Errorf("expected 1000000.0, got %f, ok=%v", val, ok)
	}

	// Not found
	_, ok = extractFloatField(item, "missing")
	if ok {
		t.Error("expected not found")
	}

	// Non-numeric
	_, ok = extractFloatField(item, "text")
	if ok {
		t.Error("expected not found for string value")
	}
}

func TestMapSchwabActivitySide(t *testing.T) {
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
		{"buy", "buy"},
		{"sell", "sell"},
		{"", ""},
	}

	for _, tt := range tests {
		got := mapSchwabActivitySide(tt.input)
		if got != tt.expected {
			t.Errorf("mapSchwabActivitySide(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// =============================================================================
// Integration test: full stream data flow
// =============================================================================

func TestStreamReadLoop_AccountActivityDispatch(t *testing.T) {
	srv, makeProvider := mockStreamServer(t, func(conn *websocket.Conn) {
		sendLoginSuccess(conn)

		// Send account activity data through the WebSocket
		dataMsg := schwabStreamResponse{
			Data: []schwabStreamDataItem{
				{
					Service:   "ACCT_ACTIVITY",
					Timestamp: time.Now().UnixMilli(),
					Command:   "SUBS",
					Content: []map[string]interface{}{
						{
							"orderId":  "77777",
							"status":   "FILLED",
							"symbol":   "NVDA",
							"side":     "BUY",
							"quantity": 50.0,
						},
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

	var mu sync.Mutex
	var receivedEvent *models.OrderEvent

	p.SetOrderEventCallback(func(event *models.OrderEvent) {
		mu.Lock()
		receivedEvent = event
		mu.Unlock()
	})

	// Need to set acctStreamActive for the callback to fire via processAccountActivity
	p.acctStreamMu.Lock()
	p.acctStreamActive = true
	p.acctStreamMu.Unlock()

	p.ConnectStreaming(ctx)
	defer p.DisconnectStreaming(ctx)

	// Wait for event to be dispatched through the read loop
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for account activity dispatch")
		default:
		}

		mu.Lock()
		got := receivedEvent
		mu.Unlock()

		if got != nil {
			if got.GetIDAsString() != "77777" {
				t.Errorf("expected order ID '77777', got %s", got.GetIDAsString())
			}
			if got.Symbol != "NVDA" {
				t.Errorf("expected symbol 'NVDA', got %s", got.Symbol)
			}
			if got.Side != "buy" {
				t.Errorf("expected side 'buy', got %s", got.Side)
			}
			return // Success
		}

		time.Sleep(50 * time.Millisecond)
	}
}
