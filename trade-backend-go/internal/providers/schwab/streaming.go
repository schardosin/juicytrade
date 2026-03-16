package schwab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

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
// Stream Read Loop
// =============================================================================

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
			// Reconnection will be handled by step 4.3
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

// processStreamData processes a streaming data message.
// Placeholder — full implementation in step 4.2.
func (s *SchwabProvider) processStreamData(data schwabStreamDataItem) {
	// Will be implemented in step 4.2 with field decoding and MarketData dispatch
	s.logger.Debug("stream data received", "service", data.Service, "items", len(data.Content))
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
