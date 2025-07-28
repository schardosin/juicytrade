class WebSocketStreamingClient {
  constructor(baseUrl = "ws://localhost:8008") {
    this.baseUrl = baseUrl;
    this.ws = null;
    this.callbacks = new Map(); // Map of event type -> Set of callbacks
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 10; // Increased for better resilience
    this.reconnectDelay = 1000;
    this.isConnected = false;
    this.subscribedSymbols = new Set();
    this.connectionPromise = null;

    // Performance optimization: throttling and batching
    this.priceUpdateQueue = new Map();
    this.throttleDelay = 150; // ms
    this.throttleTimer = null;
    this.lastUpdateTime = 0;

    // Cleanup tracking
    this.activeCallbacks = new Set();

    // Debouncing for subscription calls
    this.stockSubscriptionDebounce = null;
    this.optionsSubscriptionDebounce = null;
    this.debounceDelay = 300; // ms

    // Enhanced connection monitoring
    this.heartbeatInterval = null;
    this.heartbeatTimeout = null;
    this.lastHeartbeat = null;
    this.heartbeatIntervalMs = 30000; // 30 seconds
    this.heartbeatTimeoutMs = 10000; // 10 seconds

    // Page visibility handling
    this.isPageVisible = !document.hidden;
    this.setupPageVisibilityHandling();

    // Store current subscriptions for recovery
    this.currentStockSubscription = null;
    this.currentOptionsSubscriptions = [];
  }

  async connect() {
    if (this.connectionPromise) {
      return this.connectionPromise;
    }

    this.connectionPromise = new Promise((resolve, reject) => {
      try {
        this.ws = new WebSocket(`${this.baseUrl}/ws`);

        this.ws.onopen = () => {
          this.isConnected = true;
          this.reconnectAttempts = 0;

          // Re-subscribe to symbols after reconnection
          if (this.subscribedSymbols.size > 0) {
            this.subscribe(Array.from(this.subscribedSymbols));
          }

          resolve();
        };

        this.ws.onmessage = (event) => {
          try {
            //console.log("Raw WebSocket message received:", event.data);
            const message = JSON.parse(event.data);
            this.handleMessage(message);
          } catch (error) {
            console.error("Error parsing WebSocket message:", error);
          }
        };

        this.ws.onclose = (event) => {
          console.log("WebSocket disconnected:", event.code, event.reason);
          this.isConnected = false;
          this.connectionPromise = null;

          // Only attempt reconnect if it wasn't a clean close
          if (event.code !== 1000) {
            this.attemptReconnect();
          }
        };

        this.ws.onerror = (error) => {
          console.error("WebSocket error:", error);
          this.connectionPromise = null;
          reject(error);
        };
      } catch (error) {
        this.connectionPromise = null;
        reject(error);
      }
    });

    return this.connectionPromise;
  }

  subscribe(symbols) {
    if (!Array.isArray(symbols)) {
      symbols = [symbols];
    }

    // Add to our local set
    symbols.forEach((symbol) => this.subscribedSymbols.add(symbol));

    if (this.isConnected && this.ws) {
      const message = {
        type: "subscribe",
        symbols: symbols,
      };

      console.log("Sending WebSocket subscription:", message);
      this.ws.send(JSON.stringify(message));
    } else {
      console.log(
        "WebSocket not connected, symbols will be subscribed on connection"
      );
    }
  }

  onPriceUpdate(callback) {
    if (!this.callbacks.has("price_update")) {
      this.callbacks.set("price_update", new Set());
    }
    this.callbacks.get("price_update").add(callback);
  }

  // Clear all price update callbacks - useful when switching symbols
  clearPriceUpdateCallbacks() {
    if (this.callbacks.has("price_update")) {
      this.callbacks.get("price_update").clear();
      console.log("🧹 Cleared all price update callbacks");
    }
  }

  // Remove a specific callback
  removePriceUpdateCallback(callback) {
    if (this.callbacks.has("price_update")) {
      this.callbacks.get("price_update").delete(callback);
    }
  }

  onSubscriptionConfirmed(callback) {
    if (!this.callbacks.has("subscription_confirmed")) {
      this.callbacks.set("subscription_confirmed", new Set());
    }
    this.callbacks.get("subscription_confirmed").add(callback);
  }

  onPositionsUpdate(callback) {
    if (!this.callbacks.has("positions_update")) {
      this.callbacks.set("positions_update", new Set());
    }
    this.callbacks.get("positions_update").add(callback);
  }

  onPositionsError(callback) {
    if (!this.callbacks.has("positions_error")) {
      this.callbacks.set("positions_error", new Set());
    }
    this.callbacks.get("positions_error").add(callback);
  }

  onOpenOrdersUpdate(callback) {
    if (!this.callbacks.has("open_orders_update")) {
      this.callbacks.set("open_orders_update", new Set());
    }
    this.callbacks.get("open_orders_update").add(callback);
  }

  onAdjustmentsUpdate(callback) {
    if (!this.callbacks.has("adjustments_update")) {
      this.callbacks.set("adjustments_update", new Set());
    }
    this.callbacks.get("adjustments_update").add(callback);
  }

  onAdjustmentsError(callback) {
    if (!this.callbacks.has("adjustments_error")) {
      this.callbacks.set("adjustments_error", new Set());
    }
    this.callbacks.get("adjustments_error").add(callback);
  }

  requestPositions() {
    if (this.isConnected && this.ws) {
      const message = {
        type: "get_positions",
      };
      //console.log("Requesting positions via WebSocket:", message);
      this.ws.send(JSON.stringify(message));
    } else {
      console.error("Cannot request positions: WebSocket not connected");
    }
  }

  requestOpenOrders() {
    if (this.isConnected && this.ws) {
      const message = {
        type: "get_open_orders",
      };
      console.log("Requesting open orders via WebSocket:", message);
      this.ws.send(JSON.stringify(message));
    } else {
      console.error("Cannot request open orders: WebSocket not connected");
    }
  }

  requestOrders(statusFilter = "open", dateFilter = "today") {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      const message = {
        type: "get_orders",
        filter: statusFilter,
        date_filter: dateFilter,
      };
      console.log("Requesting orders via WebSocket:", message);
      this.ws.send(JSON.stringify(message));
    } else {
      console.warn("WebSocket not connected, cannot request orders");
    }
  }

  requestAdjustments(requestData) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      const message = {
        type: "calculate_adjustments",
        data: requestData,
      };
      console.log("Requesting adjustments via WebSocket:", message);
      this.ws.send(JSON.stringify(message));
    } else {
      console.warn("WebSocket not connected, cannot request adjustments");
    }
  }

  handleMessage(message) {
    switch (message.type) {
      case "price_update":
        // For index symbols like SPX, prefer 'last' price over bid/ask
        let price;
        if (message.data.last && message.data.last > 0) {
          price = message.data.last;
        } else if (message.data.ask && message.data.bid) {
          price = (message.data.ask + message.data.bid) / 2;
        } else {
          price = message.data.ask || message.data.bid;
        }

        // Queue price updates for throttled processing
        this.queuePriceUpdate({
          symbol: message.symbol,
          price: price,
          data: message.data,
        });
        break;

      case "subscription_confirmed":
        // console.log("Subscription confirmed:", message);
        const subCallbacks = this.callbacks.get("subscription_confirmed");
        if (subCallbacks && subCallbacks.size > 0) {
          subCallbacks.forEach((callback) => callback(message));
        }
        break;

      case "positions_update":
        //console.log("Positions update received:", message);
        const posCallbacks = this.callbacks.get("positions_update");
        if (posCallbacks && posCallbacks.size > 0) {
          posCallbacks.forEach((callback) => callback(message.data));
        }
        break;

      case "positions_error":
        console.error("Positions error received:", message);
        const posErrorCallbacks = this.callbacks.get("positions_error");
        if (posErrorCallbacks && posErrorCallbacks.size > 0) {
          posErrorCallbacks.forEach((callback) => callback(message.error));
        }
        break;

      case "open_orders_update":
        console.log("Open orders update received:", message);
        const ordersCallbacks = this.callbacks.get("open_orders_update");
        if (ordersCallbacks && ordersCallbacks.size > 0) {
          ordersCallbacks.forEach((callback) => callback(message.data));
        }
        break;

      case "open_orders_error":
        console.error("Open orders error received:", message);
        const ordersErrorCallbacks = this.callbacks.get("open_orders_error");
        if (ordersErrorCallbacks && ordersErrorCallbacks.size > 0) {
          ordersErrorCallbacks.forEach((callback) => callback(message.error));
        }
        break;

      case "orders_update":
        console.log("Orders update received:", message);
        const allOrdersCallbacks = this.callbacks.get("open_orders_update");
        if (allOrdersCallbacks && allOrdersCallbacks.size > 0) {
          allOrdersCallbacks.forEach((callback) => callback(message.data));
        }
        break;

      case "orders_error":
        console.error("Orders error received:", message);
        const allOrdersErrorCallbacks = this.callbacks.get("open_orders_error");
        if (allOrdersErrorCallbacks && allOrdersErrorCallbacks.size > 0) {
          allOrdersErrorCallbacks.forEach((callback) =>
            callback(message.error)
          );
        }
        break;

      case "adjustments_update":
        console.log("Adjustments update received:", message);
        const adjustmentsCallbacks = this.callbacks.get("adjustments_update");
        if (adjustmentsCallbacks && adjustmentsCallbacks.size > 0) {
          adjustmentsCallbacks.forEach((callback) => callback(message.data));
        }
        break;

      case "adjustments_error":
        console.error("Adjustments error received:", message);
        const adjustmentsErrorCallbacks =
          this.callbacks.get("adjustments_error");
        if (adjustmentsErrorCallbacks && adjustmentsErrorCallbacks.size > 0) {
          adjustmentsErrorCallbacks.forEach((callback) =>
            callback(message.error)
          );
        }
        break;

      default:
        console.log("Unknown message type:", message.type);
    }
  }

  async attemptReconnect() {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++;
      const delay =
        this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1); // Exponential backoff

      console.log(
        `Attempting to reconnect... (${this.reconnectAttempts}/${this.maxReconnectAttempts}) in ${delay}ms`
      );

      setTimeout(async () => {
        try {
          await this.connect();
        } catch (error) {
          console.error("Reconnection failed:", error);
        }
      }, delay);
    } else {
      console.error(
        "Max reconnection attempts reached. WebSocket connection failed."
      );
    }
  }

  // Performance optimization: Queue and throttle price updates
  queuePriceUpdate(priceData) {
    // Store the latest price data for each symbol
    this.priceUpdateQueue.set(priceData.symbol, priceData);

    // Throttle the processing
    if (!this.throttleTimer) {
      this.throttleTimer = setTimeout(() => {
        this.processPriceUpdateQueue();
        this.throttleTimer = null;
      }, this.throttleDelay);
    }
  }

  processPriceUpdateQueue() {
    const now = Date.now();

    // Skip if we updated too recently
    if (now - this.lastUpdateTime < this.throttleDelay) {
      return;
    }

    const priceCallbacks = this.callbacks.get("price_update");
    if (
      !priceCallbacks ||
      priceCallbacks.size === 0 ||
      this.priceUpdateQueue.size === 0
    ) {
      return;
    }

    // Process all queued updates in a batch
    const updates = Array.from(this.priceUpdateQueue.values());
    this.priceUpdateQueue.clear();
    this.lastUpdateTime = now;

    // Use requestAnimationFrame for smooth UI updates
    requestAnimationFrame(() => {
      updates.forEach((priceData) => {
        // Call all registered callbacks
        priceCallbacks.forEach((callback) => {
          try {
            callback(priceData);
          } catch (error) {
            console.error("Error in price update callback:", error);
          }
        });
      });
    });
  }

  // Clean up resources and prevent memory leaks
  cleanup() {
    if (this.throttleTimer) {
      clearTimeout(this.throttleTimer);
      this.throttleTimer = null;
    }

    this.priceUpdateQueue.clear();
    this.activeCallbacks.clear();
  }

  disconnect() {
    console.log("Disconnecting WebSocket");

    // Clean up timers and queues
    this.cleanup();

    if (this.ws) {
      this.ws.close(1000, "Client disconnect");
      this.ws = null;
    }

    this.isConnected = false;
    this.subscribedSymbols.clear();
    this.callbacks.clear();
    this.connectionPromise = null;
    this.reconnectAttempts = 0;
  }

  // Unified subscription replacement method
  replaceAllSubscriptions(underlyingSymbol, optionSymbols = []) {
    // Handle both old format (underlyingSymbol, optionSymbols) and new format (array of all symbols)
    if (Array.isArray(underlyingSymbol)) {
      // New format: single array of all symbols
      return this.replaceAllSubscriptionsWithSymbols(underlyingSymbol);
    }

    // Old format: separate underlying and options
    if (!Array.isArray(optionSymbols)) {
      optionSymbols = [optionSymbols];
    }

    // Store current subscriptions for recovery
    this.currentStockSubscription = underlyingSymbol;
    this.currentOptionsSubscriptions = [...optionSymbols];

    // Clear existing debounce timers
    if (this.stockSubscriptionDebounce) {
      clearTimeout(this.stockSubscriptionDebounce);
    }
    if (this.optionsSubscriptionDebounce) {
      clearTimeout(this.optionsSubscriptionDebounce);
    }

    // Use a single debounce timer for unified subscription
    this.unifiedSubscriptionDebounce = setTimeout(() => {
      if (this.isConnected && this.ws) {
        const message = {
          type: "subscribe_replace_all",
          underlying_symbol: underlyingSymbol,
          option_symbols: optionSymbols,
        };

        this.ws.send(JSON.stringify(message));
      } else {
        console.warn(
          "⚠️ WebSocket not connected, unified subscription will be replaced on connection"
        );
      }
    }, this.debounceDelay);
  }

  // New method for Smart Market Data Store - replaces all subscriptions with a simple array
  replaceAllSubscriptionsWithSymbols(allSymbols = []) {
    if (!Array.isArray(allSymbols)) {
      allSymbols = [allSymbols];
    }

    // Separate stocks and options for backend compatibility
    const stockSymbols = [];
    const optionSymbols = [];

    allSymbols.forEach((symbol) => {
      if (this.isOptionSymbol(symbol)) {
        optionSymbols.push(symbol);
      } else {
        stockSymbols.push(symbol);
      }
    });

    // Store for recovery
    this.currentStockSubscription = stockSymbols[0] || null; // Use first stock as primary
    this.currentOptionsSubscriptions = [...optionSymbols];

    // Clear existing debounce timers
    if (this.stockSubscriptionDebounce) {
      clearTimeout(this.stockSubscriptionDebounce);
    }
    if (this.optionsSubscriptionDebounce) {
      clearTimeout(this.optionsSubscriptionDebounce);
    }
    if (this.unifiedSubscriptionDebounce) {
      clearTimeout(this.unifiedSubscriptionDebounce);
    }

    // Use a single debounce timer for smart subscription
    this.smartSubscriptionDebounce = setTimeout(() => {
      if (this.isConnected && this.ws) {
        const message = {
          type: "subscribe_smart_replace_all",
          stock_symbols: stockSymbols,
          option_symbols: optionSymbols,
          all_symbols: allSymbols, // Send all symbols for backend flexibility
        };

        this.ws.send(JSON.stringify(message));
      } else {
        console.warn(
          "⚠️ WebSocket not connected, smart subscription will be applied on connection"
        );
      }
    }, this.debounceDelay);

    return Promise.resolve(); // Return promise for async compatibility
  }

  // Legacy methods - deprecated but kept for backward compatibility
  replaceStockSubscription(symbol) {
    console.warn(
      "replaceStockSubscription is deprecated - use replaceAllSubscriptions"
    );
    this.replaceAllSubscriptions(symbol, []);
  }

  replaceOptionsSubscriptions(symbols) {
    console.warn(
      "replaceOptionsSubscriptions is deprecated - use replaceAllSubscriptions"
    );
    const currentStock = this.currentStockSubscription || "SPY"; // fallback
    this.replaceAllSubscriptions(currentStock, symbols);
  }

  ensurePersistentSubscriptions(dataTypes = ["orders", "positions"]) {
    if (this.isConnected && this.ws) {
      const message = {
        type: "subscribe_persistent",
        data_types: dataTypes,
      };

      this.ws.send(JSON.stringify(message));
    } else {
      console.log(
        "WebSocket not connected, persistent subscriptions will be ensured on connection"
      );
    }
  }

  getSubscriptionStatus() {
    if (this.isConnected && this.ws) {
      const message = {
        type: "get_subscription_status",
      };

      console.log("Requesting subscription status");
      this.ws.send(JSON.stringify(message));
    } else {
      console.warn("WebSocket not connected, cannot get subscription status");
    }
  }

  getConnectionStatus() {
    return {
      isConnected: this.isConnected,
      subscribedSymbols: Array.from(this.subscribedSymbols),
      reconnectAttempts: this.reconnectAttempts,
    };
  }

  // Enhanced page visibility handling for sleep/wake scenarios
  setupPageVisibilityHandling() {
    const handleVisibilityChange = () => {
      const wasVisible = this.isPageVisible;
      this.isPageVisible = !document.hidden;

      console.log(
        `Page visibility changed: ${this.isPageVisible ? "visible" : "hidden"}`
      );

      if (!wasVisible && this.isPageVisible) {
        // Page became visible (wake up from sleep)
        console.log("🌅 Page became visible - performing comprehensive recovery");
        this.performWakeupRecovery();
      }
    };

    document.addEventListener("visibilitychange", handleVisibilityChange);

    // Enhanced focus handling
    window.addEventListener("focus", () => {
      if (this.isPageVisible) {
        console.log("🔍 Window focused - checking connection health");
        this.checkConnectionHealth();
      }
    });

    // Network status monitoring
    if ('onLine' in navigator) {
      window.addEventListener('online', () => {
        console.log("🌐 Network came online - performing recovery");
        this.performNetworkRecovery();
      });

      window.addEventListener('offline', () => {
        console.log("📴 Network went offline");
        this.isConnected = false;
      });
    }

    // Detect system resume from sleep (additional method)
    let lastActivity = Date.now();
    const checkForSleep = () => {
      const now = Date.now();
      const timeDiff = now - lastActivity;
      
      // If more than 2 minutes have passed since last check, likely resumed from sleep
      if (timeDiff > 120000) {
        console.log("😴 System likely resumed from sleep - performing recovery");
        this.performWakeupRecovery();
      }
      
      lastActivity = now;
    };

    // Check every 30 seconds
    setInterval(checkForSleep, 30000);
  }
  // Comprehensive wakeup recovery
  async performWakeupRecovery() {
    console.log("🔄 Starting comprehensive wakeup recovery...");
    
    try {
      // Step 1: Force disconnect if connection exists but is stale
      if (this.ws && this.ws.readyState !== WebSocket.CLOSED) {
        console.log("🔌 Closing stale WebSocket connection");
        this.ws.close();
        this.ws = null;
      }

      // Step 2: Reset connection state
      this.isConnected = false;
      this.connectionPromise = null;

      // Step 3: Wait a moment for network to stabilize
      await new Promise(resolve => setTimeout(resolve, 1000));

      // Step 4: Attempt reconnection
      console.log("🔄 Attempting to reconnect after wakeup...");
      await this.connect();

      // Step 5: Restore all subscriptions
      await this.restoreSubscriptions();

      console.log("✅ Wakeup recovery completed successfully");
      
      // Notify any listeners about successful recovery
      this.notifyRecoverySuccess();

    } catch (error) {
      console.error("❌ Wakeup recovery failed:", error);
      // Try again in a few seconds
      setTimeout(() => this.performWakeupRecovery(), 5000);
    }
  }

  // Network-specific recovery
  async performNetworkRecovery() {
    console.log("🌐 Starting network recovery...");
    
    try {
      // Check if we think we're connected but actually aren't
      if (this.isConnected && (!this.ws || this.ws.readyState !== WebSocket.OPEN)) {
        console.log("🚨 Detected stale connection during network recovery");
        this.isConnected = false;
        this.connectionPromise = null;
      }

      // If not connected, attempt to reconnect
      if (!this.isConnected) {
        await this.connect();
        await this.restoreSubscriptions();
        console.log("✅ Network recovery completed");
      } else {
        // If we think we're connected, verify with a ping
        this.sendPing();
      }

    } catch (error) {
      console.error("❌ Network recovery failed:", error);
      // Retry network recovery
      setTimeout(() => this.performNetworkRecovery(), 3000);
    }
  }

  // Notify recovery success (can be extended to update UI)
  notifyRecoverySuccess() {
    // Dispatch custom event for UI components to listen to
    window.dispatchEvent(new CustomEvent('websocket-recovered', {
      detail: {
        timestamp: Date.now(),
        subscriptions: Array.from(this.subscribedSymbols)
      }
    }));
  }

  // Check connection health and reconnect if needed
  async checkConnectionHealth() {
    // If we think we're connected but WebSocket is not in OPEN state
    if (
      this.isConnected &&
      (!this.ws || this.ws.readyState !== WebSocket.OPEN)
    ) {
      console.log("🚨 Connection health check failed - forcing reconnection");
      this.isConnected = false;
      this.connectionPromise = null;

      try {
        await this.connect();
        await this.restoreSubscriptions();
      } catch (error) {
        console.error("Failed to restore connection:", error);
      }
    } else if (!this.isConnected) {
      console.log("🔄 Not connected - attempting to reconnect");
      try {
        await this.connect();
        await this.restoreSubscriptions();
      } catch (error) {
        console.error("Failed to reconnect:", error);
      }
    } else {
      // Send a ping to verify the connection is actually working
      this.sendPing();
    }
  }

  // Send ping to verify connection
  sendPing() {
    if (this.isConnected && this.ws && this.ws.readyState === WebSocket.OPEN) {
      try {
        this.ws.send(JSON.stringify({ type: "ping", timestamp: Date.now() }));
      } catch (error) {
        console.error("Failed to send ping:", error);
        this.checkConnectionHealth();
      }
    }
  }

  // Restore subscriptions after reconnection
  async restoreSubscriptions() {
    console.log("🔄 Restoring subscriptions after reconnection");

    // Restore stock subscription
    if (this.currentStockSubscription) {
      console.log(
        "📈 Restoring stock subscription:",
        this.currentStockSubscription
      );
      this.replaceStockSubscription(this.currentStockSubscription);
    }

    // Restore options subscriptions
    if (this.currentOptionsSubscriptions.length > 0) {
      console.log(
        "📊 Restoring options subscriptions:",
        this.currentOptionsSubscriptions.length,
        "symbols"
      );
      this.replaceOptionsSubscriptions(this.currentOptionsSubscriptions);
    }

    // Restore persistent subscriptions
    this.ensurePersistentSubscriptions(["orders", "positions"]);
  }

  // Get human-readable WebSocket ready state
  getReadyStateText() {
    if (!this.ws) return "null";
    switch (this.ws.readyState) {
      case WebSocket.CONNECTING:
        return "CONNECTING";
      case WebSocket.OPEN:
        return "OPEN";
      case WebSocket.CLOSING:
        return "CLOSING";
      case WebSocket.CLOSED:
        return "CLOSED";
      default:
        return "UNKNOWN";
    }
  }

  // Enhanced disconnect with cleanup
  disconnect() {
    console.log("Disconnecting WebSocket");

    // Clean up timers and queues
    this.cleanup();

    // Clear heartbeat timers
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }
    if (this.heartbeatTimeout) {
      clearTimeout(this.heartbeatTimeout);
      this.heartbeatTimeout = null;
    }

    if (this.ws) {
      this.ws.close(1000, "Client disconnect");
      this.ws = null;
    }

    this.isConnected = false;
    this.subscribedSymbols.clear();
    this.callbacks.clear();
    this.connectionPromise = null;
    this.reconnectAttempts = 0;
  }

  // Helper method to check if symbol is an option symbol
  isOptionSymbol(symbol) {
    return (
      symbol &&
      symbol.length > 10 &&
      (symbol.includes("C") || symbol.includes("P")) &&
      /\d{6}[CP]\d{8}/.test(symbol)
    );
  }
}

export default new WebSocketStreamingClient();
