package schwab

import (
	"context"
	"encoding/json"
	"fmt"
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
// WebSocket test helpers
// =============================================================================

// mockStreamServer creates a mock HTTP+WebSocket server for testing.
// tokenHandler handles /v1/oauth/token requests.
// prefHandler handles /trader/v1/userPreference requests (returns streamer info pointing at the WS server).
// wsHandler handles WebSocket connections.
// Returns the httptest.Server and a function to create providers pointing at it.
func mockStreamServer(t *testing.T, wsHandler func(*websocket.Conn)) (*httptest.Server, func() *SchwabProvider) {
	t.Helper()

	var wsURL string

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)

		case strings.HasSuffix(r.URL.Path, "/userPreference"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{
				"streamerInfo": [{
					"streamerSocketUrl": %q,
					"schwabClientCustomerId": "test-customer-id",
					"schwabClientCorrelId": "test-correl-id",
					"schwabClientChannel": "IO",
					"schwabClientFunctionId": "Tradeticket"
				}]
			}`, wsURL)

		case r.URL.Path == "/ws":
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Logf("WebSocket upgrade failed: %v", err)
				return
			}
			defer conn.Close()
			wsHandler(conn)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	// Convert http URL to ws URL for the WebSocket endpoint
	wsURL = "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	makeProvider := func() *SchwabProvider {
		return newTestProvider(srv.URL)
	}

	return srv, makeProvider
}

// =============================================================================
// ConnectStreaming tests
// =============================================================================

func TestConnectStreaming_Success(t *testing.T) {
	srv, makeProvider := mockStreamServer(t, func(conn *websocket.Conn) {
		// Read LOGIN request
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Logf("read error: %v", err)
			return
		}

		// Verify LOGIN request structure
		var req schwabStreamRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			t.Errorf("failed to parse LOGIN request: %v", err)
			return
		}

		if len(req.Requests) != 1 {
			t.Errorf("expected 1 request, got %d", len(req.Requests))
			return
		}
		if req.Requests[0].Service != "ADMIN" {
			t.Errorf("expected service ADMIN, got %s", req.Requests[0].Service)
		}
		if req.Requests[0].Command != "LOGIN" {
			t.Errorf("expected command LOGIN, got %s", req.Requests[0].Command)
		}
		if req.Requests[0].SchwabClientCustomerID != "test-customer-id" {
			t.Errorf("expected customer ID 'test-customer-id', got %s", req.Requests[0].SchwabClientCustomerID)
		}

		// Send success response
		loginResp := schwabStreamResponse{
			Response: []schwabStreamResponseItem{
				{
					Service: "ADMIN",
					Command: "LOGIN",
					Content: map[string]interface{}{
						"code": 0.0,
						"msg":  "LOGIN successful",
					},
				},
			},
		}
		respData, _ := json.Marshal(loginResp)
		conn.WriteMessage(websocket.TextMessage, respData)

		// Keep connection alive until client disconnects
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	p := makeProvider()
	ctx := context.Background()

	ok, err := p.ConnectStreaming(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !ok {
		t.Error("expected true for successful connect")
	}
	if !p.IsConnected {
		t.Error("expected IsConnected to be true")
	}
	if p.streamCustomerID != "test-customer-id" {
		t.Errorf("expected customer ID 'test-customer-id', got %s", p.streamCustomerID)
	}

	// Clean up
	p.DisconnectStreaming(ctx)
}

func TestConnectStreaming_LoginFailure(t *testing.T) {
	srv, makeProvider := mockStreamServer(t, func(conn *websocket.Conn) {
		// Read LOGIN request
		conn.ReadMessage()

		// Send failure response
		loginResp := schwabStreamResponse{
			Response: []schwabStreamResponseItem{
				{
					Service: "ADMIN",
					Command: "LOGIN",
					Content: map[string]interface{}{
						"code": 3.0,
						"msg":  "Login denied",
					},
				},
			},
		}
		respData, _ := json.Marshal(loginResp)
		conn.WriteMessage(websocket.TextMessage, respData)
	})
	defer srv.Close()

	p := makeProvider()
	ctx := context.Background()

	ok, err := p.ConnectStreaming(ctx)
	if err == nil {
		t.Fatal("expected error for LOGIN failure, got nil")
	}
	if ok {
		t.Error("expected false for failed connect")
	}
	if !strings.Contains(err.Error(), "LOGIN failed") {
		t.Errorf("expected LOGIN failed error, got: %v", err)
	}
	if p.IsConnected {
		t.Error("expected IsConnected to be false after login failure")
	}
}

func TestConnectStreaming_AlreadyConnected(t *testing.T) {
	p := newTestProvider("http://unused")
	p.accessToken = "token"
	p.tokenExpiry = time.Now().Add(30 * time.Minute)
	p.IsConnected = true

	ok, err := p.ConnectStreaming(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !ok {
		t.Error("expected true for already connected")
	}
}

// =============================================================================
// DisconnectStreaming tests
// =============================================================================

func TestDisconnectStreaming_Success(t *testing.T) {
	srv, makeProvider := mockStreamServer(t, func(conn *websocket.Conn) {
		// Handle LOGIN
		conn.ReadMessage()
		loginResp := schwabStreamResponse{
			Response: []schwabStreamResponseItem{
				{Service: "ADMIN", Command: "LOGIN", Content: map[string]interface{}{"code": 0.0}},
			},
		}
		respData, _ := json.Marshal(loginResp)
		conn.WriteMessage(websocket.TextMessage, respData)

		// Keep alive until client disconnects
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	p := makeProvider()
	ctx := context.Background()

	// Connect first
	p.ConnectStreaming(ctx)
	if !p.IsConnected {
		t.Fatal("expected to be connected before disconnect test")
	}

	// Disconnect
	ok, err := p.DisconnectStreaming(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !ok {
		t.Error("expected true for successful disconnect")
	}
	if p.IsConnected {
		t.Error("expected IsConnected to be false after disconnect")
	}
}

func TestDisconnectStreaming_NotConnected(t *testing.T) {
	p := newTestProvider("http://unused")

	ok, err := p.DisconnectStreaming(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !ok {
		t.Error("expected true for already disconnected")
	}
}

// =============================================================================
// streamReadLoop tests
// =============================================================================

func TestStreamReadLoop_Heartbeat(t *testing.T) {
	srv, makeProvider := mockStreamServer(t, func(conn *websocket.Conn) {
		// Handle LOGIN
		conn.ReadMessage()
		loginResp := schwabStreamResponse{
			Response: []schwabStreamResponseItem{
				{Service: "ADMIN", Command: "LOGIN", Content: map[string]interface{}{"code": 0.0}},
			},
		}
		respData, _ := json.Marshal(loginResp)
		conn.WriteMessage(websocket.TextMessage, respData)

		// Send heartbeat
		heartbeat := schwabStreamResponse{
			Notify: []schwabStreamNotifyItem{
				{Heartbeat: time.Now().UnixMilli()},
			},
		}
		hbData, _ := json.Marshal(heartbeat)
		conn.WriteMessage(websocket.TextMessage, hbData)

		// Wait a moment then close (triggers read error in loop)
		time.Sleep(200 * time.Millisecond)
	})
	defer srv.Close()

	p := makeProvider()
	ctx := context.Background()

	p.ConnectStreaming(ctx)
	if !p.IsConnected {
		t.Fatal("expected connected")
	}

	// Wait briefly for heartbeat to be processed
	time.Sleep(300 * time.Millisecond)

	// If we get here without a crash or panic, the heartbeat was handled correctly.
	// The read loop may have exited due to the server closing, which is expected.

	p.DisconnectStreaming(ctx)
}

// =============================================================================
// Helper tests
// =============================================================================

func TestNextRequestID(t *testing.T) {
	p := newTestProvider("http://unused")

	id1 := p.nextRequestID()
	id2 := p.nextRequestID()
	id3 := p.nextRequestID()

	if id1 == id2 || id2 == id3 {
		t.Errorf("expected unique IDs, got %s, %s, %s", id1, id2, id3)
	}

	// IDs should be parseable as integers
	for _, id := range []string{id1, id2, id3} {
		if _, err := fmt.Sscanf(id, "%d", new(int)); err != nil {
			t.Errorf("expected numeric ID, got %q: %v", id, err)
		}
	}
}

func TestFetchStreamerInfo_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/userPreference"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{
				"streamerInfo": [{
					"streamerSocketUrl": "wss://stream.example.com",
					"schwabClientCustomerId": "cust-123",
					"schwabClientCorrelId": "corr-456",
					"schwabClientChannel": "IO",
					"schwabClientFunctionId": "Tradeticket"
				}]
			}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)
	info, err := p.fetchStreamerInfo(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if info.StreamerSocketURL != "wss://stream.example.com" {
		t.Errorf("expected socket URL 'wss://stream.example.com', got %s", info.StreamerSocketURL)
	}
	if info.SchwabClientCustomerID != "cust-123" {
		t.Errorf("expected customer ID 'cust-123', got %s", info.SchwabClientCustomerID)
	}
	if info.SchwabClientCorrelID != "corr-456" {
		t.Errorf("expected correl ID 'corr-456', got %s", info.SchwabClientCorrelID)
	}
}

func TestFetchStreamerInfo_MissingURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)
		case strings.HasSuffix(r.URL.Path, "/userPreference"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := newTestProvider(srv.URL)
	_, err := p.fetchStreamerInfo(context.Background())
	if err == nil {
		t.Fatal("expected error for missing streamer URL, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// =============================================================================
// SubscribeToSymbols tests
// =============================================================================

func TestSubscribeToSymbols_NotConnected(t *testing.T) {
	p := newTestProvider("http://unused")
	// IsConnected is false by default

	_, err := p.SubscribeToSymbols(context.Background(), []string{"AAPL"}, nil)
	if err == nil {
		t.Fatal("expected error when not connected, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestSubscribeToSymbols_EmptySymbols(t *testing.T) {
	p := newTestProvider("http://unused")
	p.IsConnected = true

	ok, err := p.SubscribeToSymbols(context.Background(), []string{}, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !ok {
		t.Error("expected true for empty symbols")
	}
}

func TestSubscribeToSymbols_ClassifiesAndSendsCorrectServices(t *testing.T) {
	// Track all SUBS messages received by the WebSocket server
	var mu sync.Mutex
	var receivedRequests []schwabStreamRequest

	srv, makeProvider := mockStreamServer(t, func(conn *websocket.Conn) {
		// Handle LOGIN
		conn.ReadMessage()
		loginResp := schwabStreamResponse{
			Response: []schwabStreamResponseItem{
				{Service: "ADMIN", Command: "LOGIN", Content: map[string]interface{}{"code": 0.0}},
			},
		}
		respData, _ := json.Marshal(loginResp)
		conn.WriteMessage(websocket.TextMessage, respData)

		// Read subscription messages
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var req schwabStreamRequest
			if err := json.Unmarshal(msg, &req); err == nil {
				mu.Lock()
				receivedRequests = append(receivedRequests, req)
				mu.Unlock()
			}
		}
	})
	defer srv.Close()

	p := makeProvider()
	ctx := context.Background()

	p.ConnectStreaming(ctx)
	defer p.DisconnectStreaming(ctx)

	// Mix of equities and options (OCC format)
	symbols := []string{
		"AAPL",                // equity
		"MSFT",                // equity
		"AAPL250117C00150000", // option (OCC)
		"SPY250321P00500000",  // option (OCC)
	}

	ok, err := p.SubscribeToSymbols(ctx, symbols, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !ok {
		t.Error("expected true for successful subscribe")
	}

	// Wait for messages to be received
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Should have received 2 SUBS requests: one for equities, one for options
	if len(receivedRequests) < 2 {
		t.Fatalf("expected at least 2 SUBS requests, got %d", len(receivedRequests))
	}

	// Find equity and option subscription requests
	var equitySubs, optionSubs *schwabStreamRequestItem
	for _, req := range receivedRequests {
		for i := range req.Requests {
			item := &req.Requests[i]
			if item.Command != "SUBS" {
				continue
			}
			switch item.Service {
			case "LEVELONE_EQUITIES":
				equitySubs = item
			case "LEVELONE_OPTIONS":
				optionSubs = item
			}
		}
	}

	// Verify equity subscription
	if equitySubs == nil {
		t.Fatal("expected LEVELONE_EQUITIES subscription")
	}
	keys := equitySubs.Parameters["keys"].(string)
	if !strings.Contains(keys, "AAPL") || !strings.Contains(keys, "MSFT") {
		t.Errorf("expected equities AAPL,MSFT in keys, got: %s", keys)
	}
	fields := equitySubs.Parameters["fields"].(string)
	if fields != equitySubscriptionFields {
		t.Errorf("expected equity fields %q, got %q", equitySubscriptionFields, fields)
	}

	// Verify option subscription
	if optionSubs == nil {
		t.Fatal("expected LEVELONE_OPTIONS subscription")
	}
	optKeys := optionSubs.Parameters["keys"].(string)
	// Options should be converted to Schwab space-padded format
	if !strings.Contains(optKeys, "AAPL  250117C00150000") {
		t.Errorf("expected Schwab-formatted AAPL option in keys, got: %s", optKeys)
	}
	if !strings.Contains(optKeys, "SPY   250321P00500000") {
		t.Errorf("expected Schwab-formatted SPY option in keys, got: %s", optKeys)
	}
	optFields := optionSubs.Parameters["fields"].(string)
	if optFields != optionSubscriptionFields {
		t.Errorf("expected option fields %q, got %q", optionSubscriptionFields, optFields)
	}

	// Verify symbols are tracked
	for _, sym := range symbols {
		if !p.SubscribedSymbols[sym] {
			t.Errorf("expected symbol %q to be tracked in SubscribedSymbols", sym)
		}
	}
}

func TestSubscribeToSymbols_Batching(t *testing.T) {
	// Generate 120 equity symbols (should produce 3 batches: 50, 50, 20)
	symbols := make([]string, 120)
	for i := 0; i < 120; i++ {
		symbols[i] = fmt.Sprintf("SYM%d", i)
	}

	var mu sync.Mutex
	var subsCount int

	srv, makeProvider := mockStreamServer(t, func(conn *websocket.Conn) {
		// Handle LOGIN
		conn.ReadMessage()
		loginResp := schwabStreamResponse{
			Response: []schwabStreamResponseItem{
				{Service: "ADMIN", Command: "LOGIN", Content: map[string]interface{}{"code": 0.0}},
			},
		}
		respData, _ := json.Marshal(loginResp)
		conn.WriteMessage(websocket.TextMessage, respData)

		// Count SUBS messages
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
						subsCount++
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

	ok, err := p.SubscribeToSymbols(ctx, symbols, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !ok {
		t.Error("expected true")
	}

	// Wait for all batches to be sent (100ms between batches × 2 delays + overhead)
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	count := subsCount
	mu.Unlock()

	// 120 equities → ceil(120/50) = 3 batches
	if count != 3 {
		t.Errorf("expected 3 SUBS batches for 120 symbols, got %d", count)
	}

	// All symbols should be tracked
	if len(p.SubscribedSymbols) != 120 {
		t.Errorf("expected 120 tracked symbols, got %d", len(p.SubscribedSymbols))
	}
}

// =============================================================================
// UnsubscribeFromSymbols tests
// =============================================================================

func TestUnsubscribeFromSymbols_RemovesTrackedSymbols(t *testing.T) {
	var mu sync.Mutex
	var unsubRequests []schwabStreamRequestItem

	srv, makeProvider := mockStreamServer(t, func(conn *websocket.Conn) {
		// Handle LOGIN
		conn.ReadMessage()
		loginResp := schwabStreamResponse{
			Response: []schwabStreamResponseItem{
				{Service: "ADMIN", Command: "LOGIN", Content: map[string]interface{}{"code": 0.0}},
			},
		}
		respData, _ := json.Marshal(loginResp)
		conn.WriteMessage(websocket.TextMessage, respData)

		// Read messages
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
						unsubRequests = append(unsubRequests, item)
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

	// Pre-populate subscribed symbols
	p.SubscribedSymbols["AAPL"] = true
	p.SubscribedSymbols["MSFT"] = true
	p.SubscribedSymbols["AAPL250117C00150000"] = true

	// Unsubscribe from a subset
	ok, err := p.UnsubscribeFromSymbols(ctx, []string{"AAPL", "AAPL250117C00150000"}, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !ok {
		t.Error("expected true for successful unsubscribe")
	}

	time.Sleep(200 * time.Millisecond)

	// AAPL should be removed, MSFT should remain
	if p.SubscribedSymbols["AAPL"] {
		t.Error("expected AAPL to be removed from SubscribedSymbols")
	}
	if p.SubscribedSymbols["AAPL250117C00150000"] {
		t.Error("expected option to be removed from SubscribedSymbols")
	}
	if !p.SubscribedSymbols["MSFT"] {
		t.Error("expected MSFT to remain in SubscribedSymbols")
	}

	// Check UNSUBS messages
	mu.Lock()
	defer mu.Unlock()

	if len(unsubRequests) != 2 {
		t.Fatalf("expected 2 UNSUBS requests (equities + options), got %d", len(unsubRequests))
	}

	var foundEquity, foundOption bool
	for _, item := range unsubRequests {
		switch item.Service {
		case "LEVELONE_EQUITIES":
			foundEquity = true
			keys := item.Parameters["keys"].(string)
			if keys != "AAPL" {
				t.Errorf("expected equity key 'AAPL', got %q", keys)
			}
		case "LEVELONE_OPTIONS":
			foundOption = true
		}
	}
	if !foundEquity {
		t.Error("expected LEVELONE_EQUITIES UNSUBS request")
	}
	if !foundOption {
		t.Error("expected LEVELONE_OPTIONS UNSUBS request")
	}
}

func TestUnsubscribeFromSymbols_NotConnected(t *testing.T) {
	p := newTestProvider("http://unused")
	// Not connected — should be a no-op, not an error

	ok, err := p.UnsubscribeFromSymbols(context.Background(), []string{"AAPL"}, nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !ok {
		t.Error("expected true when not connected")
	}
}

// =============================================================================
// processStreamData tests
// =============================================================================

func TestProcessStreamData_EquityFields(t *testing.T) {
	p := newTestProvider("http://unused")
	p.IsConnected = true

	// Set up a streaming queue to capture dispatched data
	queue := make(chan *models.MarketData, 10)
	p.StreamingQueue = queue

	// Simulate Schwab equity data with numerical keys
	data := schwabStreamDataItem{
		Service:   "LEVELONE_EQUITIES",
		Timestamp: time.Now().UnixMilli(),
		Command:   "SUBS",
		Content: []map[string]interface{}{
			{
				"key": "AAPL",
				"0":   "AAPL",    // SYMBOL
				"1":   150.25,    // BID_PRICE
				"2":   150.30,    // ASK_PRICE
				"3":   150.27,    // LAST_PRICE
				"4":   100.0,     // BID_SIZE
				"5":   200.0,     // ASK_SIZE
				"8":   5000000.0, // TOTAL_VOLUME
				"10":  152.00,    // HIGH_PRICE
				"11":  149.50,    // LOW_PRICE
				"12":  149.80,    // CLOSE_PRICE
				"17":  150.00,    // OPEN_PRICE
				"18":  0.47,      // NET_CHANGE
				"33":  150.28,    // MARK
			},
		},
	}

	p.processStreamData(data)

	// Should have dispatched to queue
	select {
	case md := <-queue:
		if md.Symbol != "AAPL" {
			t.Errorf("expected symbol AAPL, got %s", md.Symbol)
		}
		if md.DataType != "quote" {
			t.Errorf("expected dataType 'quote', got %s", md.DataType)
		}

		// Check decoded fields
		if bid, ok := md.Data["bid"].(float64); !ok || bid != 150.25 {
			t.Errorf("expected bid 150.25, got %v", md.Data["bid"])
		}
		if ask, ok := md.Data["ask"].(float64); !ok || ask != 150.30 {
			t.Errorf("expected ask 150.30, got %v", md.Data["ask"])
		}
		if last, ok := md.Data["last"].(float64); !ok || last != 150.27 {
			t.Errorf("expected last 150.27, got %v", md.Data["last"])
		}
		if vol, ok := md.Data["volume"].(float64); !ok || vol != 5000000.0 {
			t.Errorf("expected volume 5000000, got %v", md.Data["volume"])
		}
		if mark, ok := md.Data["mark"].(float64); !ok || mark != 150.28 {
			t.Errorf("expected mark 150.28, got %v", md.Data["mark"])
		}
		if change, ok := md.Data["change"].(float64); !ok || change != 0.47 {
			t.Errorf("expected change 0.47, got %v", md.Data["change"])
		}

	case <-time.After(time.Second):
		t.Fatal("timed out waiting for market data on queue")
	}
}

func TestProcessStreamData_OptionFieldsWithGreeks(t *testing.T) {
	p := newTestProvider("http://unused")
	p.IsConnected = true

	queue := make(chan *models.MarketData, 10)
	p.StreamingQueue = queue

	// Simulate Schwab option data with numerical keys — uses space-padded symbol
	data := schwabStreamDataItem{
		Service:   "LEVELONE_OPTIONS",
		Timestamp: time.Now().UnixMilli(),
		Command:   "SUBS",
		Content: []map[string]interface{}{
			{
				"key": "AAPL  250117C00150000",
				"0":   "AAPL  250117C00150000", // SYMBOL (Schwab space-padded)
				"2":   5.25,                    // BID_PRICE
				"3":   5.40,                    // ASK_PRICE
				"4":   5.32,                    // LAST_PRICE
				"5":   6.00,                    // HIGH_PRICE
				"6":   4.80,                    // LOW_PRICE
				"7":   5.10,                    // CLOSE_PRICE
				"8":   12500.0,                 // TOTAL_VOLUME
				"9":   85000.0,                 // OPEN_INTEREST
				"10":  0.32,                    // VOLATILITY
				"19":  0.22,                    // NET_CHANGE
				"21":  "C",                     // CONTRACT_TYPE
				"22":  "AAPL",                  // UNDERLYING
				"28":  0.65,                    // DELTA
				"29":  0.04,                    // GAMMA
				"30":  -0.08,                   // THETA
				"31":  0.12,                    // VEGA
				"32":  0.03,                    // RHO
				"35":  150.27,                  // UNDERLYING_PRICE
				"37":  5.33,                    // MARK
			},
		},
	}

	p.processStreamData(data)

	select {
	case md := <-queue:
		// Symbol should be converted from Schwab format to OCC
		if md.Symbol != "AAPL250117C00150000" {
			t.Errorf("expected OCC symbol AAPL250117C00150000, got %s", md.Symbol)
		}
		if md.DataType != "quote" {
			t.Errorf("expected dataType 'quote', got %s", md.DataType)
		}

		// Check quote fields
		if bid, ok := md.Data["bid"].(float64); !ok || bid != 5.25 {
			t.Errorf("expected bid 5.25, got %v", md.Data["bid"])
		}
		if ask, ok := md.Data["ask"].(float64); !ok || ask != 5.40 {
			t.Errorf("expected ask 5.40, got %v", md.Data["ask"])
		}
		if mark, ok := md.Data["mark"].(float64); !ok || mark != 5.33 {
			t.Errorf("expected mark 5.33, got %v", md.Data["mark"])
		}

		// Check Greeks
		if delta, ok := md.Data["delta"].(float64); !ok || delta != 0.65 {
			t.Errorf("expected delta 0.65, got %v", md.Data["delta"])
		}
		if gamma, ok := md.Data["gamma"].(float64); !ok || gamma != 0.04 {
			t.Errorf("expected gamma 0.04, got %v", md.Data["gamma"])
		}
		if theta, ok := md.Data["theta"].(float64); !ok || theta != -0.08 {
			t.Errorf("expected theta -0.08, got %v", md.Data["theta"])
		}
		if vega, ok := md.Data["vega"].(float64); !ok || vega != 0.12 {
			t.Errorf("expected vega 0.12, got %v", md.Data["vega"])
		}
		if rho, ok := md.Data["rho"].(float64); !ok || rho != 0.03 {
			t.Errorf("expected rho 0.03, got %v", md.Data["rho"])
		}

		// Check underlying price
		if up, ok := md.Data["underlying_price"].(float64); !ok || up != 150.27 {
			t.Errorf("expected underlying_price 150.27, got %v", md.Data["underlying_price"])
		}

		// Check symbol in data was also converted
		if sym, ok := md.Data["symbol"].(string); !ok || sym != "AAPL250117C00150000" {
			t.Errorf("expected symbol in data to be OCC format, got %v", md.Data["symbol"])
		}

	case <-time.After(time.Second):
		t.Fatal("timed out waiting for market data on queue")
	}
}

func TestProcessStreamData_MultipleItems(t *testing.T) {
	p := newTestProvider("http://unused")
	p.IsConnected = true

	queue := make(chan *models.MarketData, 10)
	p.StreamingQueue = queue

	// Two symbols in one data message
	data := schwabStreamDataItem{
		Service:   "LEVELONE_EQUITIES",
		Timestamp: time.Now().UnixMilli(),
		Command:   "SUBS",
		Content: []map[string]interface{}{
			{"0": "AAPL", "1": 150.25, "2": 150.30, "3": 150.27},
			{"0": "MSFT", "1": 380.00, "2": 380.10, "3": 380.05},
		},
	}

	p.processStreamData(data)

	// Should get 2 MarketData items
	received := make(map[string]*models.MarketData)
	for i := 0; i < 2; i++ {
		select {
		case md := <-queue:
			received[md.Symbol] = md
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for market data item %d", i+1)
		}
	}

	if _, ok := received["AAPL"]; !ok {
		t.Error("expected AAPL in received data")
	}
	if _, ok := received["MSFT"]; !ok {
		t.Error("expected MSFT in received data")
	}
}

func TestProcessStreamData_UnhandledService(t *testing.T) {
	p := newTestProvider("http://unused")
	p.IsConnected = true

	queue := make(chan *models.MarketData, 10)
	p.StreamingQueue = queue

	// Unknown service — should be silently ignored
	data := schwabStreamDataItem{
		Service:   "ACCT_ACTIVITY",
		Timestamp: time.Now().UnixMilli(),
		Content: []map[string]interface{}{
			{"1": "some activity data"},
		},
	}

	p.processStreamData(data)

	// Nothing should be dispatched
	select {
	case <-queue:
		t.Error("expected no data for unhandled service")
	case <-time.After(100 * time.Millisecond):
		// Expected: no data dispatched
	}
}

func TestProcessStreamData_MissingSymbol(t *testing.T) {
	p := newTestProvider("http://unused")
	p.IsConnected = true

	queue := make(chan *models.MarketData, 10)
	p.StreamingQueue = queue

	// Equity data without field "0" (SYMBOL)
	data := schwabStreamDataItem{
		Service:   "LEVELONE_EQUITIES",
		Timestamp: time.Now().UnixMilli(),
		Content: []map[string]interface{}{
			{"1": 150.25, "2": 150.30}, // No "0" key
		},
	}

	p.processStreamData(data)

	select {
	case <-queue:
		t.Error("expected no data dispatched for missing symbol")
	case <-time.After(100 * time.Millisecond):
		// Expected: skipped
	}
}

// =============================================================================
// dispatchMarketData tests
// =============================================================================

// mockStreamingCache implements base.StreamingCache for testing.
type mockStreamingCache struct {
	mu      sync.Mutex
	updates []*models.MarketData
}

func (m *mockStreamingCache) Update(marketData *models.MarketData) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updates = append(m.updates, marketData)
	return nil
}

func (m *mockStreamingCache) AddUpdateCallback(callback func(*models.MarketData)) {
	// No-op for testing
}

func (m *mockStreamingCache) getUpdates() []*models.MarketData {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*models.MarketData, len(m.updates))
	copy(result, m.updates)
	return result
}

func TestDispatchMarketData_ToQueue(t *testing.T) {
	p := newTestProvider("http://unused")

	queue := make(chan *models.MarketData, 10)
	p.StreamingQueue = queue

	decoded := map[string]interface{}{
		"SYMBOL":    "AAPL",
		"BID_PRICE": 150.25,
		"ASK_PRICE": 150.30,
	}

	p.dispatchMarketData("LEVELONE_EQUITIES", "AAPL", decoded)

	select {
	case md := <-queue:
		if md.Symbol != "AAPL" {
			t.Errorf("expected symbol AAPL, got %s", md.Symbol)
		}
		if md.DataType != "quote" {
			t.Errorf("expected dataType 'quote', got %s", md.DataType)
		}
		if md.Data["bid"] != 150.25 {
			t.Errorf("expected bid 150.25, got %v", md.Data["bid"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for data on queue")
	}
}

func TestDispatchMarketData_ToCache(t *testing.T) {
	p := newTestProvider("http://unused")

	cache := &mockStreamingCache{}
	p.StreamingCache = cache

	// Also set a queue to verify cache is preferred
	queue := make(chan *models.MarketData, 10)
	p.StreamingQueue = queue

	decoded := map[string]interface{}{
		"SYMBOL":     "AAPL",
		"BID_PRICE":  150.25,
		"ASK_PRICE":  150.30,
		"LAST_PRICE": 150.27,
	}

	p.dispatchMarketData("LEVELONE_EQUITIES", "AAPL", decoded)

	// Cache update is dispatched in a goroutine, wait briefly
	time.Sleep(100 * time.Millisecond)

	updates := cache.getUpdates()
	if len(updates) != 1 {
		t.Fatalf("expected 1 cache update, got %d", len(updates))
	}
	if updates[0].Symbol != "AAPL" {
		t.Errorf("expected symbol AAPL, got %s", updates[0].Symbol)
	}

	// Queue should NOT have received data (cache is preferred)
	select {
	case <-queue:
		t.Error("expected no data on queue when cache is available")
	default:
		// Expected: cache was used instead
	}
}

func TestDispatchMarketData_OptionFields(t *testing.T) {
	p := newTestProvider("http://unused")

	queue := make(chan *models.MarketData, 10)
	p.StreamingQueue = queue

	decoded := map[string]interface{}{
		"SYMBOL":           "AAPL250117C00150000",
		"BID_PRICE":        5.25,
		"ASK_PRICE":        5.40,
		"DELTA":            0.65,
		"GAMMA":            0.04,
		"THETA":            -0.08,
		"VEGA":             0.12,
		"RHO":              0.03,
		"UNDERLYING_PRICE": 150.27,
		"CONTRACT_TYPE":    "C",
	}

	p.dispatchMarketData("LEVELONE_OPTIONS", "AAPL250117C00150000", decoded)

	select {
	case md := <-queue:
		if md.Data["delta"] != 0.65 {
			t.Errorf("expected delta 0.65, got %v", md.Data["delta"])
		}
		if md.Data["theta"] != -0.08 {
			t.Errorf("expected theta -0.08, got %v", md.Data["theta"])
		}
		if md.Data["underlying_price"] != 150.27 {
			t.Errorf("expected underlying_price 150.27, got %v", md.Data["underlying_price"])
		}
		if md.Data["contract_type"] != "C" {
			t.Errorf("expected contract_type 'C', got %v", md.Data["contract_type"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for data on queue")
	}
}

// =============================================================================
// streamReadLoop data dispatch integration test
// =============================================================================

func TestStreamReadLoop_DataDispatch(t *testing.T) {
	srv, makeProvider := mockStreamServer(t, func(conn *websocket.Conn) {
		// Handle LOGIN
		conn.ReadMessage()
		loginResp := schwabStreamResponse{
			Response: []schwabStreamResponseItem{
				{Service: "ADMIN", Command: "LOGIN", Content: map[string]interface{}{"code": 0.0}},
			},
		}
		respData, _ := json.Marshal(loginResp)
		conn.WriteMessage(websocket.TextMessage, respData)

		// Send equity data through the WebSocket
		dataMsg := schwabStreamResponse{
			Data: []schwabStreamDataItem{
				{
					Service:   "LEVELONE_EQUITIES",
					Timestamp: time.Now().UnixMilli(),
					Command:   "SUBS",
					Content: []map[string]interface{}{
						{
							"0": "AAPL",
							"1": 150.25,
							"2": 150.30,
							"3": 150.27,
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

	// Set up queue to capture dispatched data
	queue := make(chan *models.MarketData, 10)
	p.StreamingQueue = queue

	p.ConnectStreaming(ctx)
	defer p.DisconnectStreaming(ctx)

	// Wait for data to be dispatched through the read loop
	select {
	case md := <-queue:
		if md.Symbol != "AAPL" {
			t.Errorf("expected symbol AAPL, got %s", md.Symbol)
		}
		if md.DataType != "quote" {
			t.Errorf("expected dataType 'quote', got %s", md.DataType)
		}
		if md.Data["bid"] != 150.25 {
			t.Errorf("expected bid 150.25, got %v", md.Data["bid"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for data dispatch through read loop")
	}
}

// =============================================================================
// Reconnection tests
// =============================================================================

// withFastBackoffs temporarily sets reconnectBackoffs to very short durations
// for testing, and restores the originals when the test completes.
func withFastBackoffs(t *testing.T) {
	t.Helper()
	original := reconnectBackoffs
	reconnectBackoffs = []time.Duration{
		50 * time.Millisecond,
		100 * time.Millisecond,
		150 * time.Millisecond,
		200 * time.Millisecond,
		250 * time.Millisecond,
	}
	t.Cleanup(func() { reconnectBackoffs = original })
}

// mockMultiConnStreamServer creates a mock server that tracks the number of
// WebSocket connections and invokes wsHandler for each one with a connection
// counter. This allows testing reconnection behavior.
func mockMultiConnStreamServer(t *testing.T, wsHandler func(conn *websocket.Conn, connNum int)) (*httptest.Server, func() *SchwabProvider) {
	t.Helper()

	var wsURL string
	var connCount int64

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/oauth/token"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, validTokenBody)

		case strings.HasSuffix(r.URL.Path, "/userPreference"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{
				"streamerInfo": [{
					"streamerSocketUrl": %q,
					"schwabClientCustomerId": "test-customer-id",
					"schwabClientCorrelId": "test-correl-id",
					"schwabClientChannel": "IO",
					"schwabClientFunctionId": "Tradeticket"
				}]
			}`, wsURL)

		case r.URL.Path == "/ws":
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Logf("WebSocket upgrade failed: %v", err)
				return
			}
			defer conn.Close()
			num := int(atomic.AddInt64(&connCount, 1))
			wsHandler(conn, num)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	wsURL = "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	makeProvider := func() *SchwabProvider {
		return newTestProvider(srv.URL)
	}

	return srv, makeProvider
}

// sendLoginSuccess is a test helper that reads the LOGIN request and sends a
// success response.
func sendLoginSuccess(conn *websocket.Conn) {
	conn.ReadMessage() // consume LOGIN request
	loginResp := schwabStreamResponse{
		Response: []schwabStreamResponseItem{
			{Service: "ADMIN", Command: "LOGIN", Content: map[string]interface{}{"code": 0.0}},
		},
	}
	respData, _ := json.Marshal(loginResp)
	conn.WriteMessage(websocket.TextMessage, respData)
}

func TestReconnect_TriggeredAfterReadError(t *testing.T) {
	withFastBackoffs(t)

	var connCount int64

	srv, makeProvider := mockMultiConnStreamServer(t, func(conn *websocket.Conn, connNum int) {
		atomic.StoreInt64(&connCount, int64(connNum))
		sendLoginSuccess(conn)

		if connNum == 1 {
			// First connection: send one heartbeat then close abruptly
			time.Sleep(50 * time.Millisecond)
			conn.Close()
			return
		}

		// Second connection (reconnect): keep alive
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	p := makeProvider()
	ctx := context.Background()

	ok, err := p.ConnectStreaming(ctx)
	if err != nil || !ok {
		t.Fatalf("initial connect failed: %v", err)
	}

	// Wait for the first connection to be closed and reconnection to happen.
	// With fast backoffs, reconnection should complete within ~200ms.
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for reconnection")
		default:
		}

		if atomic.LoadInt64(&connCount) >= 2 && p.IsConnected {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !p.IsConnected {
		t.Error("expected IsConnected to be true after reconnect")
	}

	p.DisconnectStreaming(ctx)
}

func TestReconnect_ResubscribesExistingSymbols(t *testing.T) {
	withFastBackoffs(t)

	var mu sync.Mutex
	var subsAfterReconnect []schwabStreamRequestItem

	srv, makeProvider := mockMultiConnStreamServer(t, func(conn *websocket.Conn, connNum int) {
		sendLoginSuccess(conn)

		if connNum == 1 {
			// First connection: read any messages for a moment, then close
			go func() {
				for {
					if _, _, err := conn.ReadMessage(); err != nil {
						return
					}
				}
			}()
			time.Sleep(200 * time.Millisecond)
			conn.Close()
			return
		}

		// Second connection (reconnect): capture SUBS messages
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
						subsAfterReconnect = append(subsAfterReconnect, item)
						mu.Unlock()
					}
				}
			}
		}
	})
	defer srv.Close()

	p := makeProvider()
	ctx := context.Background()

	ok, err := p.ConnectStreaming(ctx)
	if err != nil || !ok {
		t.Fatalf("initial connect failed: %v", err)
	}

	// Subscribe to some symbols on the first connection
	p.SubscribedSymbols["AAPL"] = true
	p.SubscribedSymbols["MSFT"] = true
	p.SubscribedSymbols["SPY250321P00500000"] = true

	// Wait for reconnection to complete and subscriptions to be restored
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for re-subscription after reconnect")
		default:
		}

		mu.Lock()
		count := len(subsAfterReconnect)
		mu.Unlock()

		// We expect at least 2 SUBS messages (equities + options)
		if count >= 2 && p.IsConnected {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()

	// Verify both equity and option SUBS messages were sent
	var hasEquities, hasOptions bool
	for _, item := range subsAfterReconnect {
		switch item.Service {
		case "LEVELONE_EQUITIES":
			hasEquities = true
			keys := item.Parameters["keys"].(string)
			if !strings.Contains(keys, "AAPL") || !strings.Contains(keys, "MSFT") {
				t.Errorf("expected AAPL and MSFT in equity keys, got: %s", keys)
			}
		case "LEVELONE_OPTIONS":
			hasOptions = true
		}
	}

	if !hasEquities {
		t.Error("expected equity re-subscription after reconnect")
	}
	if !hasOptions {
		t.Error("expected option re-subscription after reconnect")
	}

	p.DisconnectStreaming(ctx)
}

func TestReconnect_MaxRetryExhaustion(t *testing.T) {
	withFastBackoffs(t)

	// Server that always rejects LOGIN — every connection attempt will fail
	var connAttempts int64

	srv, makeProvider := mockMultiConnStreamServer(t, func(conn *websocket.Conn, connNum int) {
		atomic.AddInt64(&connAttempts, 1)
		conn.ReadMessage() // consume LOGIN
		loginResp := schwabStreamResponse{
			Response: []schwabStreamResponseItem{
				{Service: "ADMIN", Command: "LOGIN", Content: map[string]interface{}{
					"code": 3.0, "msg": "Login denied",
				}},
			},
		}
		respData, _ := json.Marshal(loginResp)
		conn.WriteMessage(websocket.TextMessage, respData)
	})
	defer srv.Close()

	p := makeProvider()

	// Directly call reconnectLoop instead of going through the full connect
	// flow. This tests the retry logic in isolation.
	stopChan := make(chan struct{})
	symbols := []string{"AAPL"}

	p.reconnectLoop(stopChan, symbols)

	// Should have attempted reconnectMaxRetries times
	attempts := atomic.LoadInt64(&connAttempts)
	if attempts != int64(reconnectMaxRetries) {
		t.Errorf("expected %d reconnection attempts, got %d", reconnectMaxRetries, attempts)
	}

	// Should still be disconnected
	if p.IsConnected {
		t.Error("expected IsConnected to be false after exhausting retries")
	}
}

func TestReconnect_NoReconnectOnGracefulDisconnect(t *testing.T) {
	withFastBackoffs(t)

	var connCount int64

	srv, makeProvider := mockMultiConnStreamServer(t, func(conn *websocket.Conn, connNum int) {
		atomic.AddInt64(&connCount, 1)
		sendLoginSuccess(conn)

		// Keep alive until connection is closed
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	p := makeProvider()
	ctx := context.Background()

	ok, err := p.ConnectStreaming(ctx)
	if err != nil || !ok {
		t.Fatalf("initial connect failed: %v", err)
	}

	if atomic.LoadInt64(&connCount) != 1 {
		t.Fatalf("expected 1 connection, got %d", atomic.LoadInt64(&connCount))
	}

	// Gracefully disconnect — this closes streamStopChan which should prevent
	// any reconnection attempts
	p.DisconnectStreaming(ctx)

	// Wait a bit to ensure no reconnection happens
	time.Sleep(500 * time.Millisecond)

	if atomic.LoadInt64(&connCount) != 1 {
		t.Errorf("expected no reconnection after graceful disconnect, but got %d connections", atomic.LoadInt64(&connCount))
	}
}

func TestHandleStreamDisconnect_StopChanClosed(t *testing.T) {
	// Verify handleStreamDisconnect is a no-op when streamStopChan is already
	// closed (i.e., user initiated disconnect).
	p := newTestProvider("http://unused")
	p.streamStopChan = make(chan struct{})
	close(p.streamStopChan) // simulate user disconnect
	p.IsConnected = true

	// This should return immediately without launching a reconnection goroutine
	p.handleStreamDisconnect()

	// IsConnected should NOT be changed by handleStreamDisconnect when
	// streamStopChan is closed — it returns early.
	if !p.IsConnected {
		t.Error("expected IsConnected to remain true when stopChan is closed")
	}
}

// =============================================================================
// EnsureHealthyConnection tests
// =============================================================================

func TestEnsureHealthyConnection_AlreadyHealthy(t *testing.T) {
	p := newTestProvider("http://unused")
	p.IsConnected = true
	p.streamConn = &websocket.Conn{} // non-nil placeholder
	now := time.Now()
	p.LastDataTime = &now

	err := p.EnsureHealthyConnection(context.Background())
	if err != nil {
		t.Fatalf("expected no error for healthy connection, got: %v", err)
	}
}

func TestEnsureHealthyConnection_NotConnected_Connects(t *testing.T) {
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

	// Not connected initially
	if p.IsConnected {
		t.Fatal("expected not connected initially")
	}

	err := p.EnsureHealthyConnection(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !p.IsConnected {
		t.Error("expected IsConnected after EnsureHealthyConnection")
	}

	p.DisconnectStreaming(ctx)
}

func TestEnsureHealthyConnection_StaleConnection(t *testing.T) {
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

	// Connect first
	p.ConnectStreaming(ctx)
	if !p.IsConnected {
		t.Fatal("expected connected")
	}

	// Simulate stale data (>120 seconds ago)
	staleTime := time.Now().Add(-130 * time.Second)
	p.LastDataTime = &staleTime

	// EnsureHealthyConnection should disconnect and reconnect
	err := p.EnsureHealthyConnection(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !p.IsConnected {
		t.Error("expected IsConnected after stale connection recovery")
	}

	p.DisconnectStreaming(ctx)
}

func TestEnsureHealthyConnection_TypeAssertion(t *testing.T) {
	// Verify the provider satisfies the duck-typed interface used by the
	// streaming manager.
	p := newTestProvider("http://unused")

	type healthChecker interface {
		EnsureHealthyConnection(context.Context) error
	}

	var _ healthChecker = p // compile-time check

	// Also verify with the exact type assertion pattern used in manager.go
	var provider interface{} = p
	if _, ok := provider.(interface{ EnsureHealthyConnection(context.Context) error }); !ok {
		t.Error("SchwabProvider does not satisfy EnsureHealthyConnection duck-type interface")
	}
}
