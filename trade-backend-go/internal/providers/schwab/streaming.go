package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"trade-backend-go/internal/models"

	"github.com/gorilla/websocket"
)

// =============================================================================
// Streaming Types
// =============================================================================

// schwabStreamerInfo holds connection details from the user preferences endpoint.
type schwabStreamerInfo struct {
	StreamerSocketURL      string `json:"streamerSocketUrl"`
	SchwabClientCustomerID string `json:"schwabClientCustomerId"`
	SchwabClientCorrelID   string `json:"schwabClientCorrelId"`
	SchwabClientChannel    string `json:"schwabClientChannel"`
	SchwabClientFunctionID string `json:"schwabClientFunctionId"`
}

// schwabStreamRequest is the top-level request envelope for the Schwab streaming API.
type schwabStreamRequest struct {
	Requests []schwabStreamRequestItem `json:"requests"`
}

// schwabStreamRequestItem is a single request within a streaming request.
type schwabStreamRequestItem struct {
	RequestID              string                 `json:"requestid"`
	Service                string                 `json:"service"`
	Command                string                 `json:"command"`
	SchwabClientCustomerID string                 `json:"SchwabClientCustomerId"`
	SchwabClientCorrelID   string                 `json:"SchwabClientCorrelId"`
	Parameters             map[string]interface{} `json:"parameters"`
}

// schwabStreamResponse is the top-level response envelope from the Schwab streaming API.
type schwabStreamResponse struct {
	Response []schwabStreamResponseItem `json:"response,omitempty"`
	Notify   []schwabStreamNotifyItem   `json:"notify,omitempty"`
	Data     []schwabStreamDataItem     `json:"data,omitempty"`
}

// schwabStreamResponseItem represents a response to a request (e.g., LOGIN, SUBS confirmation).
type schwabStreamResponseItem struct {
	Service   string                 `json:"service"`
	RequestID string                 `json:"requestid"`
	Command   string                 `json:"command"`
	Timestamp int64                  `json:"timestamp"`
	Content   map[string]interface{} `json:"content"`
}

// schwabStreamDataItem represents a data message (market data updates).
type schwabStreamDataItem struct {
	Service   string                   `json:"service"`
	Timestamp int64                    `json:"timestamp"`
	Command   string                   `json:"command"`
	Content   []map[string]interface{} `json:"content"`
}

// schwabStreamNotifyItem represents a notification (heartbeats).
type schwabStreamNotifyItem struct {
	Heartbeat int64 `json:"heartbeat,omitempty"`
}

// streamRequestCounter is an atomic counter for generating unique request IDs.
var streamRequestCounter int64

// =============================================================================
// Connect Streaming
// =============================================================================

// ConnectStreaming connects to the Schwab streaming service via WebSocket.
//
// Steps:
// 1. Fetch streamer info from /trader/v1/userPreference
// 2. Dial WebSocket with retry
// 3. Send LOGIN request
// 4. Verify LOGIN response
// 5. Start read loop goroutine
func (s *SchwabProvider) ConnectStreaming(ctx context.Context) (bool, error) {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()

	if s.IsConnected {
		return true, nil // Already connected
	}

	// Step 1: Fetch streamer info
	info, err := s.fetchStreamerInfo(ctx)
	if err != nil {
		return false, fmt.Errorf("schwab: failed to fetch streamer info: %w", err)
	}

	s.streamCustomerID = info.SchwabClientCustomerID
	s.streamCorrelID = info.SchwabClientCorrelID
	s.streamSocketURL = info.StreamerSocketURL

	s.logger.Info("streamer info fetched",
		"socketURL", s.streamSocketURL,
		"customerID", s.streamCustomerID,
	)

	// Step 2: Dial WebSocket with retry (3 attempts, exponential backoff)
	var conn *websocket.Conn
	backoffs := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	for attempt := 0; attempt <= len(backoffs); attempt++ {
		dialer := websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		}
		conn, _, err = dialer.DialContext(ctx, s.streamSocketURL, nil)
		if err == nil {
			break
		}

		if attempt < len(backoffs) {
			s.logger.Warn("WebSocket dial failed, retrying",
				"attempt", attempt+1, "error", err, "backoff", backoffs[attempt])
			time.Sleep(backoffs[attempt])
		}
	}
	if err != nil {
		return false, fmt.Errorf("schwab: failed to connect WebSocket after retries: %w", err)
	}

	s.streamConn = conn

	// Step 3: Send LOGIN request
	s.tokenMu.Lock()
	token := s.accessToken
	s.tokenMu.Unlock()

	loginReq := schwabStreamRequest{
		Requests: []schwabStreamRequestItem{
			{
				RequestID:              "0",
				Service:                "ADMIN",
				Command:                "LOGIN",
				SchwabClientCustomerID: s.streamCustomerID,
				SchwabClientCorrelID:   s.streamCorrelID,
				Parameters: map[string]interface{}{
					"Authorization":          token,
					"SchwabClientChannel":    info.SchwabClientChannel,
					"SchwabClientFunctionId": info.SchwabClientFunctionID,
				},
			},
		},
	}

	if err := s.sendStreamMessageLocked(loginReq); err != nil {
		conn.Close()
		s.streamConn = nil
		return false, fmt.Errorf("schwab: failed to send LOGIN: %w", err)
	}

	// Step 4: Read LOGIN response with timeout
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		s.streamConn = nil
		return false, fmt.Errorf("schwab: failed to read LOGIN response: %w", err)
	}

	var loginResp schwabStreamResponse
	if err := json.Unmarshal(message, &loginResp); err != nil {
		conn.Close()
		s.streamConn = nil
		return false, fmt.Errorf("schwab: failed to parse LOGIN response: %w", err)
	}

	if len(loginResp.Response) == 0 {
		conn.Close()
		s.streamConn = nil
		return false, fmt.Errorf("schwab: empty LOGIN response")
	}

	content := loginResp.Response[0].Content
	code, _ := content["code"].(float64)
	if code != 0 {
		msg, _ := content["msg"].(string)
		conn.Close()
		s.streamConn = nil
		return false, fmt.Errorf("schwab: LOGIN failed with code %.0f: %s", code, msg)
	}

	// Step 5: Initialize channels and start read loop
	s.streamStopChan = make(chan struct{})
	s.streamDoneChan = make(chan struct{})
	s.IsConnected = true
	now := time.Now()
	s.LastDataTime = &now

	go s.streamReadLoop()

	s.logger.Info("streaming connected successfully")
	return true, nil
}

// =============================================================================
// Disconnect Streaming
// =============================================================================

// DisconnectStreaming disconnects from the Schwab streaming service.
func (s *SchwabProvider) DisconnectStreaming(ctx context.Context) (bool, error) {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()

	if !s.IsConnected || s.streamConn == nil {
		return true, nil // Already disconnected
	}

	// Signal read loop to stop
	close(s.streamStopChan)

	// Wait for read loop to exit (with timeout)
	select {
	case <-s.streamDoneChan:
		// Read loop exited cleanly
	case <-time.After(5 * time.Second):
		s.logger.Warn("timed out waiting for stream read loop to exit")
	}

	// Send LOGOUT (best-effort)
	logoutReq := schwabStreamRequest{
		Requests: []schwabStreamRequestItem{
			{
				RequestID:              s.nextRequestID(),
				Service:                "ADMIN",
				Command:                "LOGOUT",
				SchwabClientCustomerID: s.streamCustomerID,
				SchwabClientCorrelID:   s.streamCorrelID,
			},
		},
	}
	_ = s.sendStreamMessageLocked(logoutReq) // ignore error

	// Close WebSocket
	s.streamConn.Close()
	s.streamConn = nil

	// Reset state
	s.IsConnected = false
	s.SubscribedSymbols = make(map[string]bool)

	s.logger.Info("streaming disconnected")
	return true, nil
}

// =============================================================================
// Subscribe / Unsubscribe
// =============================================================================

// subscribeBatchSize is the maximum number of symbols to send in a single
// SUBS message. Schwab's streaming API may reject excessively large payloads.
const subscribeBatchSize = 50

// SubscribeToSymbols subscribes to real-time data for the given symbols.
//
// Symbols are classified into equities and options via classifySymbols().
// Option symbols in OCC format are converted to Schwab space-padded format
// before subscribing. Symbols are batched in groups of 50 with a 100ms delay
// between batches to avoid overwhelming the WebSocket connection.
func (s *SchwabProvider) SubscribeToSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	if !s.IsConnected {
		return false, fmt.Errorf("schwab: cannot subscribe — streaming not connected")
	}

	if len(symbols) == 0 {
		return true, nil
	}

	// Classify symbols into equities and options
	equities, options := classifySymbols(symbols)

	// Convert option symbols from OCC to Schwab format for subscription
	schwabOptions := make([]string, len(options))
	for i, opt := range options {
		schwabOptions[i] = convertOCCToSchwab(opt)
	}

	// Subscribe to equities (LEVELONE_EQUITIES service)
	if len(equities) > 0 {
		if err := s.subscribeBatched(ctx, "LEVELONE_EQUITIES", equities, equitySubscriptionFields); err != nil {
			return false, fmt.Errorf("schwab: failed to subscribe equities: %w", err)
		}
	}

	// Subscribe to options (LEVELONE_OPTIONS service)
	if len(schwabOptions) > 0 {
		if err := s.subscribeBatched(ctx, "LEVELONE_OPTIONS", schwabOptions, optionSubscriptionFields); err != nil {
			return false, fmt.Errorf("schwab: failed to subscribe options: %w", err)
		}
	}

	// Track subscribed symbols using the original (OCC/equity) symbols
	for _, sym := range symbols {
		s.SubscribedSymbols[sym] = true
	}

	s.logger.Info("subscribed to symbols",
		"equities", len(equities), "options", len(options), "total", len(symbols))

	return true, nil
}

// subscribeBatched sends SUBS messages for a list of symbols in batches of
// subscribeBatchSize, with a 100ms delay between batches.
func (s *SchwabProvider) subscribeBatched(ctx context.Context, service string, symbols []string, fields string) error {
	for i := 0; i < len(symbols); i += subscribeBatchSize {
		end := i + subscribeBatchSize
		if end > len(symbols) {
			end = len(symbols)
		}
		batch := symbols[i:end]

		req := schwabStreamRequest{
			Requests: []schwabStreamRequestItem{
				{
					RequestID:              s.nextRequestID(),
					Service:                service,
					Command:                "SUBS",
					SchwabClientCustomerID: s.streamCustomerID,
					SchwabClientCorrelID:   s.streamCorrelID,
					Parameters: map[string]interface{}{
						"keys":   strings.Join(batch, ","),
						"fields": fields,
					},
				},
			},
		}

		if err := s.sendStreamMessage(req); err != nil {
			return err
		}

		// Delay between batches to avoid overwhelming the connection
		if end < len(symbols) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(100 * time.Millisecond):
			}
		}
	}

	return nil
}

// UnsubscribeFromSymbols unsubscribes from real-time data for the given symbols.
func (s *SchwabProvider) UnsubscribeFromSymbols(ctx context.Context, symbols []string, dataTypes []string) (bool, error) {
	if !s.IsConnected {
		return true, nil // Already disconnected, nothing to unsubscribe
	}

	if len(symbols) == 0 {
		return true, nil
	}

	// Classify symbols into equities and options
	equities, options := classifySymbols(symbols)

	// Convert option symbols from OCC to Schwab format for unsubscription
	schwabOptions := make([]string, len(options))
	for i, opt := range options {
		schwabOptions[i] = convertOCCToSchwab(opt)
	}

	// Unsubscribe equities
	if len(equities) > 0 {
		req := schwabStreamRequest{
			Requests: []schwabStreamRequestItem{
				{
					RequestID:              s.nextRequestID(),
					Service:                "LEVELONE_EQUITIES",
					Command:                "UNSUBS",
					SchwabClientCustomerID: s.streamCustomerID,
					SchwabClientCorrelID:   s.streamCorrelID,
					Parameters: map[string]interface{}{
						"keys": strings.Join(equities, ","),
					},
				},
			},
		}
		if err := s.sendStreamMessage(req); err != nil {
			return false, fmt.Errorf("schwab: failed to unsubscribe equities: %w", err)
		}
	}

	// Unsubscribe options
	if len(schwabOptions) > 0 {
		req := schwabStreamRequest{
			Requests: []schwabStreamRequestItem{
				{
					RequestID:              s.nextRequestID(),
					Service:                "LEVELONE_OPTIONS",
					Command:                "UNSUBS",
					SchwabClientCustomerID: s.streamCustomerID,
					SchwabClientCorrelID:   s.streamCorrelID,
					Parameters: map[string]interface{}{
						"keys": strings.Join(schwabOptions, ","),
					},
				},
			},
		}
		if err := s.sendStreamMessage(req); err != nil {
			return false, fmt.Errorf("schwab: failed to unsubscribe options: %w", err)
		}
	}

	// Remove from tracked subscriptions
	for _, sym := range symbols {
		delete(s.SubscribedSymbols, sym)
	}

	s.logger.Info("unsubscribed from symbols",
		"equities", len(equities), "options", len(options), "total", len(symbols))

	return true, nil
}

// streamReadLoop reads messages from the WebSocket in a loop.
// It runs in its own goroutine and exits when streamStopChan is closed
// or a read error occurs.
func (s *SchwabProvider) streamReadLoop() {
	defer close(s.streamDoneChan)

	for {
		select {
		case <-s.streamStopChan:
			return
		default:
		}

		// Set read deadline to detect stale connections (120 seconds)
		s.streamMu.RLock()
		conn := s.streamConn
		s.streamMu.RUnlock()

		if conn == nil {
			return
		}

		conn.SetReadDeadline(time.Now().Add(120 * time.Second))

		_, message, err := conn.ReadMessage()
		if err != nil {
			// Check if we were asked to stop
			select {
			case <-s.streamStopChan:
				return
			default:
			}

			s.logger.Error("stream read error", "error", err)
			s.handleStreamDisconnect()
			return
		}

		// Update last data time for health monitoring
		s.UpdateLastDataTime()

		// Parse message
		var response schwabStreamResponse
		if err := json.Unmarshal(message, &response); err != nil {
			s.logger.Warn("failed to parse stream message", "error", err)
			continue
		}

		// Handle heartbeats (notify)
		for _, notify := range response.Notify {
			if notify.Heartbeat > 0 {
				s.logger.Debug("heartbeat received", "timestamp", notify.Heartbeat)
			}
		}

		// Handle data messages (market data)
		for _, data := range response.Data {
			s.processStreamData(data)
		}

		// Handle responses (subscription confirmations, errors)
		for _, resp := range response.Response {
			s.processStreamResponse(resp)
		}
	}
}

// processStreamData processes a streaming data message by decoding numerical
// field keys into named fields using the appropriate field map, then
// dispatching the decoded data as models.MarketData.
func (s *SchwabProvider) processStreamData(data schwabStreamDataItem) {
	service := data.Service

	// Select the appropriate field map based on the service
	var fieldMap map[string]string
	switch service {
	case "LEVELONE_EQUITIES":
		fieldMap = equityFieldMap
	case "LEVELONE_OPTIONS":
		fieldMap = optionFieldMap
	default:
		s.logger.Debug("unhandled stream data service", "service", service, "items", len(data.Content))
		return
	}

	for _, item := range data.Content {
		// Decode numerical field keys → named fields
		decoded := make(map[string]interface{}, len(item))
		var symbol string

		for key, value := range item {
			if fieldName, ok := fieldMap[key]; ok {
				decoded[fieldName] = value
				if key == "0" { // Field 0 is always SYMBOL
					if sym, ok := value.(string); ok {
						symbol = sym
					}
				}
			} else {
				// Keep unknown fields with their original numerical key
				decoded[key] = value
			}
		}

		if symbol == "" {
			s.logger.Warn("stream data missing symbol", "service", service)
			continue
		}

		// For options, convert Schwab symbol back to OCC format
		if service == "LEVELONE_OPTIONS" {
			occSymbol := convertSchwabOptionToOCC(symbol)
			decoded["SYMBOL"] = occSymbol
			symbol = occSymbol
		}

		s.dispatchMarketData(service, symbol, decoded)
	}
}

// processStreamResponse handles subscription confirmations and other responses.
func (s *SchwabProvider) processStreamResponse(resp schwabStreamResponseItem) {
	code, _ := resp.Content["code"].(float64)
	msg, _ := resp.Content["msg"].(string)

	if code == 0 {
		s.logger.Debug("stream response OK",
			"service", resp.Service, "command", resp.Command, "msg", msg)
	} else {
		s.logger.Warn("stream response error",
			"service", resp.Service, "command", resp.Command,
			"code", code, "msg", msg)
	}
}

// =============================================================================
// Market Data Dispatch
// =============================================================================

// dispatchMarketData converts decoded streaming fields to a models.MarketData
// and sends it to the StreamingCache (preferred) or StreamingQueue (fallback).
//
// For equities the DataType is "quote". For options the DataType is "quote"
// with Greeks fields embedded in the Data map alongside quote fields.
func (s *SchwabProvider) dispatchMarketData(service, symbol string, decoded map[string]interface{}) {
	// Build the data map for models.MarketData using standardized field names
	data := make(map[string]interface{}, len(decoded))

	switch service {
	case "LEVELONE_EQUITIES":
		mapEquityFields(decoded, data)
	case "LEVELONE_OPTIONS":
		mapOptionFields(decoded, data)
	}

	// Determine the data type — options with Greeks get "quote" with Greeks embedded
	dataType := "quote"

	timestamp := time.Now().Format(time.RFC3339)
	marketData := models.NewMarketData(symbol, dataType, timestamp, data)

	// Dispatch: prefer StreamingCache, fall back to StreamingQueue
	if s.StreamingCache != nil {
		go func() {
			if err := s.StreamingCache.Update(marketData); err != nil {
				s.logger.Error("failed to update streaming cache",
					"symbol", symbol, "error", err)
			}
		}()
	} else if s.StreamingQueue != nil {
		select {
		case s.StreamingQueue <- marketData:
		default:
			s.logger.Warn("streaming queue full, dropping data", "symbol", symbol)
		}
	}
}

// mapEquityFields maps decoded equity streaming fields to standardized names
// for the MarketData.Data map.
func mapEquityFields(decoded, data map[string]interface{}) {
	fieldMapping := map[string]string{
		"BID_PRICE":    "bid",
		"ASK_PRICE":    "ask",
		"LAST_PRICE":   "last",
		"BID_SIZE":     "bidsize",
		"ASK_SIZE":     "asksize",
		"TOTAL_VOLUME": "volume",
		"HIGH_PRICE":   "high",
		"LOW_PRICE":    "low",
		"CLOSE_PRICE":  "close",
		"OPEN_PRICE":   "open",
		"NET_CHANGE":   "change",
		"MARK":         "mark",
	}

	for schwabField, standardField := range fieldMapping {
		if val, ok := decoded[schwabField]; ok {
			data[standardField] = val
		}
	}

	// Keep the symbol
	if sym, ok := decoded["SYMBOL"]; ok {
		data["symbol"] = sym
	}
}

// mapOptionFields maps decoded option streaming fields to standardized names
// for the MarketData.Data map, including Greeks.
func mapOptionFields(decoded, data map[string]interface{}) {
	fieldMapping := map[string]string{
		"BID_PRICE":        "bid",
		"ASK_PRICE":        "ask",
		"LAST_PRICE":       "last",
		"HIGH_PRICE":       "high",
		"LOW_PRICE":        "low",
		"CLOSE_PRICE":      "close",
		"OPEN_PRICE":       "open",
		"TOTAL_VOLUME":     "volume",
		"OPEN_INTEREST":    "open_interest",
		"VOLATILITY":       "volatility",
		"NET_CHANGE":       "change",
		"MARK":             "mark",
		"UNDERLYING_PRICE": "underlying_price",
		"CONTRACT_TYPE":    "contract_type",
		"UNDERLYING":       "underlying",
		"EXPIRATION_YEAR":  "expiration_year",
		"EXPIRATION_MONTH": "expiration_month",
		"EXPIRATION_DAY":   "expiration_day",
		// Greeks
		"DELTA": "delta",
		"GAMMA": "gamma",
		"THETA": "theta",
		"VEGA":  "vega",
		"RHO":   "rho",
	}

	for schwabField, standardField := range fieldMapping {
		if val, ok := decoded[schwabField]; ok {
			data[standardField] = val
		}
	}

	// Keep the symbol
	if sym, ok := decoded["SYMBOL"]; ok {
		data["symbol"] = sym
	}
}

// =============================================================================
// Reconnection Logic
// =============================================================================

// reconnectMaxRetries is the maximum number of reconnection attempts before
// giving up.
const reconnectMaxRetries = 5

// reconnectBackoffs defines the exponential backoff durations for reconnection
// attempts. The series is: 5s, 10s, 20s, 40s, 60s (capped).
var reconnectBackoffs = []time.Duration{
	5 * time.Second,
	10 * time.Second,
	20 * time.Second,
	40 * time.Second,
	60 * time.Second,
}

// handleStreamDisconnect is called when the read loop encounters a read error.
// It marks the provider as disconnected, cleans up the old connection, and
// launches an asynchronous reconnection attempt.
//
// This method does NOT acquire streamMu — it is called from the read loop
// goroutine which has already exited its read cycle.
func (s *SchwabProvider) handleStreamDisconnect() {
	s.streamMu.Lock()

	// Check if this is a user-initiated disconnect (streamStopChan closed)
	select {
	case <-s.streamStopChan:
		// User called DisconnectStreaming — do NOT reconnect
		s.streamMu.Unlock()
		return
	default:
	}

	s.logger.Warn("stream disconnected unexpectedly, will attempt reconnection")

	// Mark as disconnected and close the old connection
	s.IsConnected = false
	if s.streamConn != nil {
		s.streamConn.Close()
		s.streamConn = nil
	}

	// Snapshot the currently subscribed symbols so we can re-subscribe
	// after reconnecting. Use a copy to avoid races.
	symbolSnapshot := make([]string, 0, len(s.SubscribedSymbols))
	for sym := range s.SubscribedSymbols {
		symbolSnapshot = append(symbolSnapshot, sym)
	}

	// Capture the stop channel reference for the reconnection goroutine.
	stopChan := s.streamStopChan

	s.streamMu.Unlock()

	// Launch reconnection in a new goroutine so the read loop can exit
	go s.reconnectLoop(stopChan, symbolSnapshot)
}

// reconnectLoop attempts to re-establish the streaming connection with
// exponential backoff. On success it re-subscribes all previously subscribed
// symbols. It gives up after reconnectMaxRetries consecutive failures.
func (s *SchwabProvider) reconnectLoop(stopChan chan struct{}, symbols []string) {
	for attempt := 0; attempt < reconnectMaxRetries; attempt++ {
		// Check if we should stop (user called DisconnectStreaming)
		select {
		case <-stopChan:
			s.logger.Info("reconnection cancelled — disconnect was requested")
			return
		default:
		}

		// Wait with backoff before attempting (except on first attempt)
		if attempt > 0 {
			backoff := reconnectBackoffs[attempt-1]
			s.logger.Info("waiting before reconnect attempt",
				"attempt", attempt+1, "backoff", backoff)
			select {
			case <-stopChan:
				s.logger.Info("reconnection cancelled during backoff")
				return
			case <-time.After(backoff):
			}
		} else {
			// First attempt gets the first backoff delay too
			backoff := reconnectBackoffs[0]
			s.logger.Info("waiting before first reconnect attempt",
				"backoff", backoff)
			select {
			case <-stopChan:
				s.logger.Info("reconnection cancelled during initial backoff")
				return
			case <-time.After(backoff):
			}
		}

		s.logger.Info("attempting streaming reconnection",
			"attempt", attempt+1, "max_retries", reconnectMaxRetries)

		// Use a background context so the reconnection is not bound to any
		// particular caller's context.
		ctx := context.Background()

		// ConnectStreaming acquires streamMu internally
		ok, err := s.ConnectStreaming(ctx)
		if err != nil || !ok {
			s.logger.Warn("reconnection attempt failed",
				"attempt", attempt+1, "error", err)
			continue
		}

		s.logger.Info("streaming reconnected successfully", "attempt", attempt+1)

		// Re-subscribe to previously subscribed symbols
		if len(symbols) > 0 {
			if _, err := s.SubscribeToSymbols(ctx, symbols, nil); err != nil {
				s.logger.Error("failed to re-subscribe after reconnect",
					"error", err, "symbols_count", len(symbols))
				// The connection itself is fine; subscription failure is not
				// fatal. The caller or health manager can retry.
			} else {
				s.logger.Info("re-subscribed after reconnect",
					"symbols_count", len(symbols))
			}
		}

		return // Success — exit the reconnection loop
	}

	// Exhausted all retries
	s.logger.Error("streaming reconnection failed after max retries",
		"max_retries", reconnectMaxRetries)
}

// EnsureHealthyConnection verifies the streaming connection is healthy.
// If the connection has gone stale or is disconnected, it attempts to
// reconnect. This method is duck-typed by the streaming manager — it will
// be detected via type assertion:
//
//	if hc, ok := provider.(interface{ EnsureHealthyConnection(context.Context) error }); ok { ... }
func (s *SchwabProvider) EnsureHealthyConnection(ctx context.Context) error {
	s.streamMu.RLock()
	connected := s.IsConnected
	conn := s.streamConn
	lastData := s.LastDataTime
	s.streamMu.RUnlock()

	if connected && conn != nil {
		// Connection exists — check if data is stale (>120s without data)
		if lastData != nil && time.Since(*lastData) > 120*time.Second {
			s.logger.Warn("streaming connection stale, reconnecting",
				"last_data_age", time.Since(*lastData))
			// Disconnect and reconnect
			s.DisconnectStreaming(ctx)
			_, err := s.ConnectStreaming(ctx)
			return err
		}
		return nil // Connection is healthy
	}

	// Not connected — attempt to connect
	s.logger.Info("streaming not connected, attempting connection")
	_, err := s.ConnectStreaming(ctx)
	return err
}

// =============================================================================
// Streaming Helpers
// =============================================================================

// fetchStreamerInfo fetches the streaming connection info from the Schwab API.
func (s *SchwabProvider) fetchStreamerInfo(ctx context.Context) (*schwabStreamerInfo, error) {
	reqURL := s.buildTraderURL("/userPreference")

	body, _, err := s.doAuthenticatedRequest(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse userPreference response: %w", err)
	}

	// Try multiple locations for streamer info
	info := &schwabStreamerInfo{}

	// Try top-level streamerInfo array
	if siArr, ok := response["streamerInfo"].([]interface{}); ok && len(siArr) > 0 {
		if siMap, ok := siArr[0].(map[string]interface{}); ok {
			parseStreamerInfo(siMap, info)
		}
	}

	// Try top-level streamerInfo as object
	if info.StreamerSocketURL == "" {
		if siMap, ok := response["streamerInfo"].(map[string]interface{}); ok {
			parseStreamerInfo(siMap, info)
		}
	}

	// Try accounts[0].streamerInfo
	if info.StreamerSocketURL == "" {
		if accounts, ok := response["accounts"].([]interface{}); ok && len(accounts) > 0 {
			if acct, ok := accounts[0].(map[string]interface{}); ok {
				if siArr, ok := acct["streamerInfo"].([]interface{}); ok && len(siArr) > 0 {
					if siMap, ok := siArr[0].(map[string]interface{}); ok {
						parseStreamerInfo(siMap, info)
					}
				}
			}
		}
	}

	if info.StreamerSocketURL == "" {
		return nil, fmt.Errorf("streamer socket URL not found in user preferences")
	}

	// Apply defaults
	if info.SchwabClientChannel == "" {
		info.SchwabClientChannel = "IO"
	}
	if info.SchwabClientFunctionID == "" {
		info.SchwabClientFunctionID = "Tradeticket"
	}

	return info, nil
}

// parseStreamerInfo extracts streamer info fields from a map.
func parseStreamerInfo(data map[string]interface{}, info *schwabStreamerInfo) {
	if v, ok := data["streamerSocketUrl"].(string); ok {
		info.StreamerSocketURL = v
	}
	if v, ok := data["schwabClientCustomerId"].(string); ok {
		info.SchwabClientCustomerID = v
	}
	if v, ok := data["schwabClientCorrelId"].(string); ok {
		info.SchwabClientCorrelID = v
	}
	if v, ok := data["schwabClientChannel"].(string); ok {
		info.SchwabClientChannel = v
	}
	if v, ok := data["schwabClientFunctionId"].(string); ok {
		info.SchwabClientFunctionID = v
	}
}

// nextRequestID returns the next unique request ID for stream messages.
func (s *SchwabProvider) nextRequestID() string {
	return strconv.FormatInt(atomic.AddInt64(&streamRequestCounter, 1), 10)
}

// sendStreamMessage sends a JSON message to the WebSocket.
// Acquires streamMu write lock.
func (s *SchwabProvider) sendStreamMessage(msg schwabStreamRequest) error {
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	return s.sendStreamMessageLocked(msg)
}

// sendStreamMessageLocked sends a JSON message to the WebSocket.
// Must be called with streamMu held.
func (s *SchwabProvider) sendStreamMessageLocked(msg schwabStreamRequest) error {
	if s.streamConn == nil {
		return fmt.Errorf("schwab: streaming connection not established")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("schwab: failed to marshal stream message: %w", err)
	}

	s.streamConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := s.streamConn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("schwab: failed to write stream message: %w", err)
	}

	return nil
}
