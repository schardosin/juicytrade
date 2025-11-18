package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"trade-backend-go/internal/models"
	"trade-backend-go/internal/streaming"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocketHandler handles WebSocket connections for real-time streaming
// Exact conversion of Python WebSocket streaming functionality
type WebSocketHandler struct {
	upgrader        websocket.Upgrader
	clients         map[*websocket.Conn]*WebSocketClient
	clientsMutex    sync.RWMutex
	streamingMgr    *streaming.StreamingManager
	shutdownChan    chan struct{}
}

// WebSocketClient represents a connected WebSocket client
type WebSocketClient struct {
	conn          *websocket.Conn
	subscriptions map[string]bool
	lastPing      time.Time
	mutex         sync.RWMutex
	writeMutex    sync.Mutex // Protects concurrent writes to WebSocket connection
}

// WebSocketMessage represents incoming WebSocket messages
type WebSocketMessage struct {
	Type          string   `json:"type"`
	Data          map[string]interface{} `json:"data,omitempty"`
	Symbols       []string `json:"symbols,omitempty"`
	StockSymbols  []string `json:"stock_symbols,omitempty"`
	OptionSymbols []string `json:"option_symbols,omitempty"`
}

// WebSocketResponse represents outgoing WebSocket messages
type WebSocketResponse struct {
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Symbol    string                 `json:"symbol,omitempty"`
	Timestamp int64                  `json:"timestamp"`
	Success   bool                   `json:"success"`
	Message   string                 `json:"message,omitempty"`
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler() *WebSocketHandler {
	handler := &WebSocketHandler{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins for development - same as Python CORS policy
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		clients:      make(map[*websocket.Conn]*WebSocketClient),
		streamingMgr: streaming.GetStreamingManager(),
		shutdownChan: make(chan struct{}),
	}

	// Set up streaming cache callback to broadcast data to clients
	handler.streamingMgr.GetLatestCache().AddUpdateCallback(func(marketData *models.MarketData) {
		if err := handler.broadcastMarketData(marketData); err != nil {
			slog.Error("Failed to broadcast market data", "error", err)
		}
	})

	// Start connection manager
	go handler.connectionManager()

	return handler
}

// HandleWebSocket handles WebSocket upgrade and connection management
// Exact conversion of Python WebSocket endpoint functionality
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("Failed to upgrade WebSocket connection", "error", err)
		return
	}

	// Create client
	client := &WebSocketClient{
		conn:          conn,
		subscriptions: make(map[string]bool),
		lastPing:      time.Now(),
	}

	// Register client
	h.clientsMutex.Lock()
	h.clients[conn] = client
	h.clientsMutex.Unlock()

	slog.Info("WebSocket client connected", "remote_addr", conn.RemoteAddr())

	// Send welcome message
	welcomeMsg := WebSocketResponse{
		Type:      "connection",
		Success:   true,
		Message:   "WebSocket connected successfully",
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"status":     "connected",
			"server":     "juicytrade-go",
			"streaming":  "enabled",
		},
	}
	h.sendToClient(conn, welcomeMsg)

	// Handle client messages
	go h.handleClient(client)
}

// handleClient handles messages from a WebSocket client
// Exact conversion of Python WebSocket message handling
func (h *WebSocketHandler) handleClient(client *WebSocketClient) {
	defer func() {
		// Cleanup on disconnect
		h.clientsMutex.Lock()
		delete(h.clients, client.conn)
		h.clientsMutex.Unlock()

		client.conn.Close()
		slog.Info("WebSocket client disconnected", "remote_addr", client.conn.RemoteAddr())

		// Update global subscriptions after client disconnect
		h.updateGlobalSubscriptions()
	}()

	// Set read deadline and pong handler
	client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.conn.SetPongHandler(func(string) error {
		client.mutex.Lock()
		client.lastPing = time.Now()
		client.mutex.Unlock()
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg WebSocketMessage
		err := client.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Error("WebSocket read error", "error", err)
			}
			break
		}

		// Handle message based on type
		h.handleMessage(client, msg)
	}
}

// handleMessage processes incoming WebSocket messages
// Exact conversion of Python WebSocket message handling logic
func (h *WebSocketHandler) handleMessage(client *WebSocketClient, msg WebSocketMessage) {
	switch msg.Type {
	case "ping":
		// Handle ping message
		response := WebSocketResponse{
			Type:      "pong",
			Success:   true,
			Timestamp: time.Now().UnixMilli(),
		}
		h.sendToClient(client.conn, response)

	case "subscribe_smart_replace_all":
		// Handle smart subscription request (Python backend compatible)
		h.handleSmartSubscription(client, msg.StockSymbols, msg.OptionSymbols)

	case "keepalive":
		// Handle keepalive request
		h.handleKeepalive(client, msg.Symbols)

	case "subscribe":
		// Handle basic subscription request (legacy support)
		h.handleSubscribe(client, msg.Symbols)

	case "unsubscribe":
		// Handle unsubscription request
		h.handleUnsubscribe(client, msg.Symbols)

	case "get_subscriptions":
		// Handle get subscriptions request
		h.handleGetSubscriptions(client)

	case "get_latest":
		// Handle get latest data request
		h.handleGetLatest(client, msg.Symbols)

	default:
		// Unknown message type
		response := WebSocketResponse{
			Type:      "error",
			Success:   false,
			Message:   "Unknown message type: " + msg.Type,
			Timestamp: time.Now().UnixMilli(),
		}
		h.sendToClient(client.conn, response)
	}
}

// handleSubscribe handles symbol subscription requests
// Exact conversion of Python subscription logic
func (h *WebSocketHandler) handleSubscribe(client *WebSocketClient, symbols []string) {
	if len(symbols) == 0 {
		response := WebSocketResponse{
			Type:      "error",
			Success:   false,
			Message:   "No symbols provided for subscription",
			Timestamp: time.Now().UnixMilli(),
		}
		h.sendToClient(client.conn, response)
		return
	}

	// Add symbols to client subscriptions
	client.mutex.Lock()
	for _, symbol := range symbols {
		client.subscriptions[symbol] = true
	}
	client.mutex.Unlock()

	// Update global subscriptions
	h.updateGlobalSubscriptions()

	// Send confirmation
	response := WebSocketResponse{
		Type:      "subscribed",
		Success:   true,
		Message:   "Successfully subscribed to symbols",
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"symbols": symbols,
			"count":   len(symbols),
		},
	}
	h.sendToClient(client.conn, response)

	slog.Info("Client subscribed to symbols", "symbols", symbols, "client", client.conn.RemoteAddr())
}

// handleSmartSubscription handles smart subscription requests (Python backend compatible)
// Exact conversion of Python _handle_smart_subscription method
func (h *WebSocketHandler) handleSmartSubscription(client *WebSocketClient, stockSymbols, optionSymbols []string) {
	// Combine all symbols
	allSymbols := make([]string, 0, len(stockSymbols)+len(optionSymbols))
	allSymbols = append(allSymbols, stockSymbols...)
	allSymbols = append(allSymbols, optionSymbols...)

	// Replace all client subscriptions (smart replace)
	client.mutex.Lock()
	client.subscriptions = make(map[string]bool)
	for _, symbol := range allSymbols {
		client.subscriptions[symbol] = true
	}
	client.mutex.Unlock()

	// Update global subscriptions
	h.updateGlobalSubscriptions()

	// Send confirmation (exact Python format)
	response := WebSocketResponse{
		Type:      "subscription_confirmed",
		Success:   true,
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"stock_symbols":  stockSymbols,
			"option_symbols": optionSymbols,
		},
	}
	h.sendToClient(client.conn, response)

	slog.Info("Client smart subscription", "stocks", len(stockSymbols), "options", len(optionSymbols), "total", len(allSymbols), "client", client.conn.RemoteAddr())
}

// handleKeepalive handles keepalive requests
// Exact conversion of Python _handle_keepalive method
func (h *WebSocketHandler) handleKeepalive(client *WebSocketClient, symbols []string) {
	// Update client subscriptions with keepalive symbols
	client.mutex.Lock()
	client.subscriptions = make(map[string]bool)
	for _, symbol := range symbols {
		client.subscriptions[symbol] = true
	}
	client.lastPing = time.Now()
	client.mutex.Unlock()

	// Update global subscriptions
	h.updateGlobalSubscriptions()

	// Send pong response
	response := WebSocketResponse{
		Type:      "pong",
		Success:   true,
		Timestamp: time.Now().UnixMilli(),
	}
	h.sendToClient(client.conn, response)

	slog.Debug("Client keepalive", "symbols", len(symbols), "client", client.conn.RemoteAddr())
}

// handleUnsubscribe handles symbol unsubscription requests
// Exact conversion of Python unsubscription logic
func (h *WebSocketHandler) handleUnsubscribe(client *WebSocketClient, symbols []string) {
	if len(symbols) == 0 {
		response := WebSocketResponse{
			Type:      "error",
			Success:   false,
			Message:   "No symbols provided for unsubscription",
			Timestamp: time.Now().UnixMilli(),
		}
		h.sendToClient(client.conn, response)
		return
	}

	// Remove symbols from client subscriptions
	client.mutex.Lock()
	for _, symbol := range symbols {
		delete(client.subscriptions, symbol)
	}
	client.mutex.Unlock()

	// Update global subscriptions
	h.updateGlobalSubscriptions()

	// Send confirmation
	response := WebSocketResponse{
		Type:      "unsubscribed",
		Success:   true,
		Message:   "Successfully unsubscribed from symbols",
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"symbols": symbols,
			"count":   len(symbols),
		},
	}
	h.sendToClient(client.conn, response)

	slog.Info("Client unsubscribed from symbols", "symbols", symbols, "client", client.conn.RemoteAddr())
}

// handleGetSubscriptions returns current client subscriptions
func (h *WebSocketHandler) handleGetSubscriptions(client *WebSocketClient) {
	client.mutex.RLock()
	symbols := make([]string, 0, len(client.subscriptions))
	for symbol := range client.subscriptions {
		symbols = append(symbols, symbol)
	}
	client.mutex.RUnlock()

	response := WebSocketResponse{
		Type:      "subscriptions",
		Success:   true,
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"symbols": symbols,
			"count":   len(symbols),
		},
	}
	h.sendToClient(client.conn, response)
}

// handleGetLatest returns latest data for requested symbols
func (h *WebSocketHandler) handleGetLatest(client *WebSocketClient, symbols []string) {
	cache := h.streamingMgr.GetLatestCache()
	latestData := make(map[string]*models.MarketData)

	for _, symbol := range symbols {
		if data := cache.GetLatest(symbol); data != nil {
			latestData[symbol] = data
		}
	}

	response := WebSocketResponse{
		Type:      "latest_data",
		Success:   true,
		Timestamp: time.Now().UnixMilli(),
		Data: map[string]interface{}{
			"data":  latestData,
			"count": len(latestData),
		},
	}
	h.sendToClient(client.conn, response)
}

// updateGlobalSubscriptions aggregates all client subscriptions and updates streaming manager
// Exact conversion of Python global subscription management
func (h *WebSocketHandler) updateGlobalSubscriptions() {
	h.clientsMutex.RLock()
	defer h.clientsMutex.RUnlock()

	// Aggregate all client subscriptions
	clientSubscriptions := make(map[interface{}]map[string]bool)
	for conn, client := range h.clients {
		client.mutex.RLock()
		if len(client.subscriptions) > 0 {
			clientSubscriptions[conn] = make(map[string]bool)
			for symbol := range client.subscriptions {
				clientSubscriptions[conn][symbol] = true
			}
		}
		client.mutex.RUnlock()
	}

	// Update streaming manager with aggregated subscriptions
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.streamingMgr.UpdateGlobalSubscriptions(ctx, clientSubscriptions); err != nil {
		slog.Error("Failed to update global subscriptions", "error", err)
	}
}

// broadcastMarketData broadcasts market data to subscribed clients
// Exact conversion of Python market data broadcasting
func (h *WebSocketHandler) broadcastMarketData(marketData *models.MarketData) error {
	if marketData == nil || marketData.Symbol == "" {
		return nil
	}

	// Determine message type based on data type (exact Python logic)
	messageType := "price_update"
	if marketData.DataType == "greeks" {
		messageType = "greeks_update"
	}

	// Create broadcast message in exact Python format
	response := map[string]interface{}{
		"type":      messageType,
		"symbol":    marketData.Symbol,
		"data":      marketData.Data,
		"timestamp": marketData.Timestamp,
	}

	// Broadcast to subscribed clients
	h.clientsMutex.RLock()
	defer h.clientsMutex.RUnlock()

	for conn, client := range h.clients {
		client.mutex.RLock()
		isSubscribed := client.subscriptions[marketData.Symbol]
		client.mutex.RUnlock()

		if isSubscribed {
			if err := h.sendToClientRaw(conn, response); err != nil {
				slog.Error("Failed to send market data to client", "error", err, "symbol", marketData.Symbol)
			}
		}
	}

	return nil
}

// sendToClientRaw sends a raw message to a specific WebSocket client
func (h *WebSocketHandler) sendToClientRaw(conn *websocket.Conn, message map[string]interface{}) error {
	// Get client to access write mutex
	h.clientsMutex.RLock()
	client, exists := h.clients[conn]
	h.clientsMutex.RUnlock()
	
	if !exists {
		return nil // Client disconnected
	}
	
	// Serialize writes to prevent concurrent write panic
	client.writeMutex.Lock()
	defer client.writeMutex.Unlock()
	
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(message)
}

// sendToClient sends a message to a specific WebSocket client
func (h *WebSocketHandler) sendToClient(conn *websocket.Conn, response WebSocketResponse) error {
	// Get client to access write mutex
	h.clientsMutex.RLock()
	client, exists := h.clients[conn]
	h.clientsMutex.RUnlock()
	
	if !exists {
		return nil // Client disconnected
	}
	
	// Serialize writes to prevent concurrent write panic
	client.writeMutex.Lock()
	defer client.writeMutex.Unlock()
	
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return conn.WriteJSON(response)
}

// connectionManager manages WebSocket connections and health checks
// Exact conversion of Python connection management
func (h *WebSocketHandler) connectionManager() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.healthCheck()
		case <-h.shutdownChan:
			return
		}
	}
}

// healthCheck performs periodic health checks on WebSocket connections
func (h *WebSocketHandler) healthCheck() {
	h.clientsMutex.Lock()
	defer h.clientsMutex.Unlock()

	now := time.Now()
	disconnectedClients := make([]*websocket.Conn, 0)

	for conn, client := range h.clients {
		client.mutex.RLock()
		lastPing := client.lastPing
		client.mutex.RUnlock()

		// Check if client is stale (no ping for 90 seconds)
		if now.Sub(lastPing) > 90*time.Second {
			slog.Info("Disconnecting stale WebSocket client", "remote_addr", conn.RemoteAddr())
			disconnectedClients = append(disconnectedClients, conn)
			continue
		}

		// Send ping to client with proper synchronization
		client.writeMutex.Lock()
		conn.SetWriteDeadline(now.Add(10 * time.Second))
		err := conn.WriteMessage(websocket.PingMessage, nil)
		client.writeMutex.Unlock()
		
		if err != nil {
			slog.Error("Failed to ping WebSocket client", "error", err)
			disconnectedClients = append(disconnectedClients, conn)
		}
	}

	// Clean up disconnected clients
	for _, conn := range disconnectedClients {
		delete(h.clients, conn)
		conn.Close()
	}

	if len(disconnectedClients) > 0 {
		h.updateGlobalSubscriptions()
	}
}

// GetConnectionStats returns WebSocket connection statistics
func (h *WebSocketHandler) GetConnectionStats() map[string]interface{} {
	h.clientsMutex.RLock()
	defer h.clientsMutex.RUnlock()

	totalSubscriptions := 0
	for _, client := range h.clients {
		client.mutex.RLock()
		totalSubscriptions += len(client.subscriptions)
		client.mutex.RUnlock()
	}

	return map[string]interface{}{
		"connected_clients":    len(h.clients),
		"total_subscriptions":  totalSubscriptions,
		"streaming_status":     h.streamingMgr.GetSubscriptionStatus(),
	}
}

// Shutdown gracefully shuts down the WebSocket handler
func (h *WebSocketHandler) Shutdown() {
	close(h.shutdownChan)

	h.clientsMutex.Lock()
	defer h.clientsMutex.Unlock()

	// Close all client connections
	for conn := range h.clients {
		conn.Close()
	}
	h.clients = make(map[*websocket.Conn]*WebSocketClient)
}
