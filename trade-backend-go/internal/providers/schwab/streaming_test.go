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
