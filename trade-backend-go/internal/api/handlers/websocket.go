package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"trade-backend-go/internal/models"
	"trade-backend-go/internal/providers"
	"trade-backend-go/internal/providers/base"
	"trade-backend-go/internal/services/ivx"
	"trade-backend-go/internal/streaming"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// WebSocketHandler handles WebSocket connections for real-time streaming
// Exact conversion of Python WebSocket streaming functionality
type WebSocketHandler struct {
	upgrader     websocket.Upgrader
	clients      map[*websocket.Conn]*WebSocketClient
	clientsMutex sync.RWMutex
	streamingMgr *streaming.StreamingManager
	ivxService   *ivx.Service
	shutdownChan chan struct{}
}

// WebSocketClient represents a connected WebSocket client
type WebSocketClient struct {
	conn             *websocket.Conn
	subscriptions    map[string]bool
	ivxSubscriptions map[string]context.CancelFunc // Tracks IVx streaming goroutines
	lastPing         time.Time
	mutex            sync.RWMutex
	writeMutex       sync.Mutex // Protects concurrent writes to WebSocket connection
}

// WebSocketMessage represents incoming WebSocket messages
type WebSocketMessage struct {
	Type          string                 `json:"type"`
	Data          map[string]interface{} `json:"data,omitempty"`
	Symbols       []string               `json:"symbols,omitempty"`
	StockSymbols  []string               `json:"stock_symbols,omitempty"`
	OptionSymbols []string               `json:"option_symbols,omitempty"`
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
func NewWebSocketHandler(ivxService *ivx.Service) *WebSocketHandler {
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
		ivxService:   ivxService,
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
		conn:             conn,
		subscriptions:    make(map[string]bool),
		ivxSubscriptions: make(map[string]context.CancelFunc),
		lastPing:         time.Now(),
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
			"status":    "connected",
			"server":    "juicytrade-go",
			"streaming": "enabled",
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

		// Cancel all IVx subscriptions for this client
		client.mutex.Lock()
		for symbol, cancelFunc := range client.ivxSubscriptions {
			cancelFunc()
			slog.Info("Canceling IVx subscription on disconnect", "symbol", symbol)
		}
		client.ivxSubscriptions = make(map[string]context.CancelFunc) // Clear map
		client.mutex.Unlock()

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

	case "subscribe_ivx":
		// Handle IVx subscription request (continuous streaming)
		h.handleSubscribeIVx(client, msg.Symbols)

	case "get_ivx_data":
		// Handle IVx on-demand request (single snapshot)
		h.handleGetIVxData(client, msg.Symbols)

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

// handleSubscribeIVx handles IVx subscription requests (continuous streaming)
// Follows REPLACE pattern like regular subscriptions - cancels existing IVx, starts new ones
func (h *WebSocketHandler) handleSubscribeIVx(client *WebSocketClient, symbols []string) {
	if len(symbols) == 0 {
		return
	}

	// 1. Cancel ALL existing IVx subscriptions (replace pattern)
	client.mutex.Lock()
	for existingSymbol, cancelFunc := range client.ivxSubscriptions {
		cancelFunc()
		slog.Info("Canceling existing IVx subscription", "symbol", existingSymbol, "reason", "replace")
		// Also remove from regular subscriptions
		delete(client.subscriptions, existingSymbol)
	}
	// Clear the IVx subscriptions map
	client.ivxSubscriptions = make(map[string]context.CancelFunc)
	client.mutex.Unlock()

	// 2. Start new IVx subscriptions for requested symbols
	for _, symbol := range symbols {
		// Create context for this IVx subscription
		ctx, cancel := context.WithCancel(context.Background())

		// Add to regular subscriptions (same as regular subscribe)
		client.mutex.Lock()
		client.subscriptions[symbol] = true      // Unified subscription tracking
		client.ivxSubscriptions[symbol] = cancel // Track cancel function for IVx goroutine
		client.mutex.Unlock()

		// Start IVx streaming goroutine
		go func(sym string, cancelFunc context.CancelFunc) {
			defer func() {
				// Cleanup on exit - remove ONLY the IVx cancel function
				// The subscription itself is managed by unsubscribe/keepalive
				client.mutex.Lock()
				delete(client.ivxSubscriptions, sym)
				client.mutex.Unlock()
				slog.Info("IVx subscription stopped", "symbol", sym)
			}()

			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()

			// Run immediately once, then on ticker
			for {
				updates := make(chan ivx.StreamUpdate)
				streamCtx, streamCancel := context.WithCancel(ctx)

				// Start streaming in background
				go h.ivxService.GetIVxStream(streamCtx, sym, updates)

				// Forward updates to client
				for update := range updates {
					// Check if client is still connected
					h.clientsMutex.RLock()
					_, exists := h.clients[client.conn]
					h.clientsMutex.RUnlock()
					if !exists {
						streamCancel()
						return
					}

					// Only send status and updates, NOT the final complete data for streaming
					var msgType string
					switch update.Type {
					case "status":
						msgType = "ivx_status"
					case "data":
						msgType = "ivx_update"
					case "complete":
						continue // Skip complete message for streaming
					case "error":
						msgType = "error"
					default:
						msgType = "unknown"
					}

					response := map[string]interface{}{
						"type":      msgType,
						"symbol":    sym,
						"data":      update.Payload,
						"timestamp": time.Now().UnixMilli(),
					}

					if err := h.sendToClientRaw(client.conn, response); err != nil {
						slog.Error("Failed to send IVx update", "error", err, "symbol", sym)
						streamCancel()
						return
					}
				}
				streamCancel()

				// Wait for next tick, shutdown signal, or cancellation
				select {
				case <-ticker.C:
					continue
				case <-ctx.Done():
					return
				case <-h.shutdownChan:
					return
				}
			}
		}(symbol, cancel)
	}

	slog.Info("IVx subscriptions registered", "symbols", symbols, "client", client.conn.RemoteAddr())
}

// handleGetIVxData handles IVx on-demand requests (single snapshot)
func (h *WebSocketHandler) handleGetIVxData(client *WebSocketClient, symbols []string) {
	if len(symbols) == 0 {
		return
	}

	for _, symbol := range symbols {
		go func(sym string) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			response, err := h.ivxService.GetIVxSnapshot(ctx, sym)
			if err != nil {
				errResponse := map[string]interface{}{
					"type":      "error",
					"symbol":    sym,
					"message":   err.Error(),
					"timestamp": time.Now().UnixMilli(),
				}
				h.sendToClientRaw(client.conn, errResponse)
				return
			}

			successResponse := map[string]interface{}{
				"type":      "ivx_data",
				"symbol":    sym,
				"data":      response,
				"timestamp": time.Now().UnixMilli(),
			}
			h.sendToClientRaw(client.conn, successResponse)
		}(symbol)
	}
}

// ... (rest of the file remains same)

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

	// Replace all client subscriptions (smart replace) - does NOT affect IVx
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
	// Create a set of keepalive symbols for quick lookup
	keepaliveSet := make(map[string]bool)
	for _, symbol := range symbols {
		keepaliveSet[symbol] = true
	}

	// Update client subscriptions with keepalive symbols AND cancel IVx for removed symbols
	client.mutex.Lock()

	// Find symbols that were subscribed but are NOT in the keepalive list
	for existingSymbol := range client.subscriptions {
		if !keepaliveSet[existingSymbol] {
			// Symbol was removed - cancel IVx goroutine if running
			if cancelFunc, exists := client.ivxSubscriptions[existingSymbol]; exists {
				cancelFunc()
				slog.Info("Canceling IVx goroutine", "symbol", existingSymbol, "reason", "not_in_keepalive")
			}
		}
	}

	// Replace subscriptions with keepalive list (Python backend behavior)
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

	// Remove symbols from subscriptions AND cancel any IVx goroutines
	client.mutex.Lock()
	for _, symbol := range symbols {
		delete(client.subscriptions, symbol)
		// Also cancel IVx goroutine if running for this symbol
		if cancelFunc, exists := client.ivxSubscriptions[symbol]; exists {
			cancelFunc()
			slog.Info("Canceling IVx goroutine", "symbol", symbol, "reason", "unsubscribe")
		}
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
	totalIvxSubscriptions := 0
	clientSubscriptions := make(map[string][]string)
	clientIvxSubscriptions := make(map[string][]string)

	for conn, client := range h.clients {
		clientAddr := conn.RemoteAddr().String()

		client.mutex.RLock()
		// Regular subscriptions
		if len(client.subscriptions) > 0 {
			symbols := make([]string, 0, len(client.subscriptions))
			for symbol := range client.subscriptions {
				symbols = append(symbols, symbol)
				totalSubscriptions++
			}
			clientSubscriptions[clientAddr] = symbols
		}

		// IVx subscriptions
		if len(client.ivxSubscriptions) > 0 {
			ivxSymbols := make([]string, 0, len(client.ivxSubscriptions))
			for symbol := range client.ivxSubscriptions {
				ivxSymbols = append(ivxSymbols, symbol)
				totalIvxSubscriptions++
			}
			clientIvxSubscriptions[clientAddr] = ivxSymbols
		}
		client.mutex.RUnlock()
	}

	return map[string]interface{}{
		"connected_clients":        len(h.clients),
		"total_subscriptions":      totalSubscriptions,
		"total_ivx_subscriptions":  totalIvxSubscriptions,
		"client_subscriptions":     clientSubscriptions,
		"client_ivx_subscriptions": clientIvxSubscriptions,
		"streaming_status":         h.streamingMgr.GetSubscriptionStatus(),
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

	// Stop account stream if running
	tradeProvider := providers.GlobalProviderManager.GetProviderByService("trade_account")
	if tradeProvider != nil {
		tradeProvider.StopAccountStream()
	}
}

// StopAccountStream stops the account events stream for the current provider
func (h *WebSocketHandler) StopAccountStream() {
	tradeProvider := providers.GlobalProviderManager.GetProviderByService("trade_account")
	if tradeProvider != nil {
		slog.Info("Stopping account stream for provider", "provider", tradeProvider.GetName())
		tradeProvider.StopAccountStream()
	}
}

// RestartAccountStream restarts the account stream with a new provider
func (h *WebSocketHandler) RestartAccountStream(provider base.Provider) error {
	if provider == nil {
		slog.Error("Cannot restart account stream: provider is nil")
		return nil
	}

	slog.Info("Restarting account stream with new provider", "provider", provider.GetName())

	// Set the callback first
	provider.SetOrderEventCallback(func(event *models.OrderEvent) {
		h.BroadcastOrderEvent(event)
	})

	// Start the stream
	return provider.StartAccountStream(context.Background())
}

// StartAccountStream initializes the account events WebSocket stream for the provider
func (h *WebSocketHandler) StartAccountStream(provider base.Provider) error {
	if provider == nil {
		slog.Error("Cannot start account stream: provider is nil")
		return nil
	}

	provider.SetOrderEventCallback(func(event *models.OrderEvent) {
		h.BroadcastOrderEvent(event)
	})

	return provider.StartAccountStream(context.Background())
}

// BroadcastOrderEvent broadcasts an order event to all connected clients
func (h *WebSocketHandler) BroadcastOrderEvent(event *models.OrderEvent) {
	if event == nil {
		slog.Warn("[ORDER-EVENT] BroadcastOrderEvent called with nil event")
		return
	}

	orderID := event.GetIDAsString()

	slog.Info("[ORDER-EVENT] Broadcasting order event to clients",
		"orderID", orderID,
		"status", event.Status,
		"symbol", event.Symbol,
		"account", event.Account,
		"connectedClients", len(h.clients))

	response := map[string]interface{}{
		"type":      "order_event",
		"data":      event,
		"timestamp": time.Now().UnixMilli(),
	}

	h.clientsMutex.RLock()
	defer h.clientsMutex.RUnlock()

	sentCount := 0
	for conn := range h.clients {
		if err := h.sendToClientRaw(conn, response); err != nil {
			slog.Error("[ORDER-EVENT] Failed to send order event to client", "error", err, "orderID", event.ID, "client", conn.RemoteAddr())
		} else {
			sentCount++
			slog.Info("[ORDER-EVENT] Sent order event to client", "orderID", event.ID, "client", conn.RemoteAddr())
		}
	}

	slog.Info("[ORDER-EVENT] Order event broadcast complete", "orderID", event.ID, "sentTo", sentCount, "totalClients", len(h.clients))
}

// GetAccountStreamStatus returns the status of the account stream
func (h *WebSocketHandler) GetAccountStreamStatus() map[string]interface{} {
	status := map[string]interface{}{
		"running": false,
	}

	tradeProvider := providers.GlobalProviderManager.GetProviderByService("trade_account")
	if tradeProvider != nil {
		status["running"] = tradeProvider.IsAccountStreamConnected()
	}

	return status
}
