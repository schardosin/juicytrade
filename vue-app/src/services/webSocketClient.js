class WebSocketStreamingClient {
  constructor(baseUrl = "ws://localhost:8008") {
    this.baseUrl = baseUrl;
    this.ws = null;
    this.callbacks = new Map();
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
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
  }

  async connect() {
    if (this.connectionPromise) {
      return this.connectionPromise;
    }

    this.connectionPromise = new Promise((resolve, reject) => {
      try {
        console.log(`Connecting to WebSocket: ${this.baseUrl}/ws`);
        this.ws = new WebSocket(`${this.baseUrl}/ws`);

        this.ws.onopen = () => {
          console.log("WebSocket connected successfully");
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

      //console.log("Sending WebSocket subscription:", message);
      this.ws.send(JSON.stringify(message));
    } else {
      console.log(
        "WebSocket not connected, symbols will be subscribed on connection"
      );
    }
  }

  onPriceUpdate(callback) {
    this.callbacks.set("price_update", callback);
  }

  onSubscriptionConfirmed(callback) {
    this.callbacks.set("subscription_confirmed", callback);
  }

  onPositionsUpdate(callback) {
    this.callbacks.set("positions_update", callback);
  }

  onPositionsError(callback) {
    this.callbacks.set("positions_error", callback);
  }

  onOpenOrdersUpdate(callback) {
    this.callbacks.set("open_orders_update", callback);
  }

  onAdjustmentsUpdate(callback) {
    this.callbacks.set("adjustments_update", callback);
  }

  onAdjustmentsError(callback) {
    this.callbacks.set("adjustments_error", callback);
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
    //console.log("WebSocket message received:", message);

    switch (message.type) {
      case "price_update":
        // Queue price updates for throttled processing
        this.queuePriceUpdate({
          symbol: message.symbol,
          price: message.data.ask || message.data.bid,
          data: message.data,
        });
        break;

      case "subscription_confirmed":
        // console.log("Subscription confirmed:", message);
        const subCallback = this.callbacks.get("subscription_confirmed");
        if (subCallback) {
          subCallback(message);
        }
        break;

      case "positions_update":
        //console.log("Positions update received:", message);
        const posCallback = this.callbacks.get("positions_update");
        if (posCallback) {
          posCallback(message.data);
        }
        break;

      case "positions_error":
        console.error("Positions error received:", message);
        const posErrorCallback = this.callbacks.get("positions_error");
        if (posErrorCallback) {
          posErrorCallback(message.error);
        }
        break;

      case "open_orders_update":
        console.log("Open orders update received:", message);
        const ordersCallback = this.callbacks.get("open_orders_update");
        if (ordersCallback) {
          ordersCallback(message.data);
        }
        break;

      case "open_orders_error":
        console.error("Open orders error received:", message);
        const ordersErrorCallback = this.callbacks.get("open_orders_error");
        if (ordersErrorCallback) {
          ordersErrorCallback(message.error);
        }
        break;

      case "orders_update":
        console.log("Orders update received:", message);
        const allOrdersCallback = this.callbacks.get("open_orders_update");
        if (allOrdersCallback) {
          allOrdersCallback(message.data);
        }
        break;

      case "orders_error":
        console.error("Orders error received:", message);
        const allOrdersErrorCallback = this.callbacks.get("open_orders_error");
        if (allOrdersErrorCallback) {
          allOrdersErrorCallback(message.error);
        }
        break;

      case "adjustments_update":
        console.log("Adjustments update received:", message);
        const adjustmentsCallback = this.callbacks.get("adjustments_update");
        if (adjustmentsCallback) {
          adjustmentsCallback(message.data);
        }
        break;

      case "adjustments_error":
        console.error("Adjustments error received:", message);
        const adjustmentsErrorCallback =
          this.callbacks.get("adjustments_error");
        if (adjustmentsErrorCallback) {
          adjustmentsErrorCallback(message.error);
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

    const priceCallback = this.callbacks.get("price_update");
    if (!priceCallback || this.priceUpdateQueue.size === 0) {
      return;
    }

    // Process all queued updates in a batch
    const updates = Array.from(this.priceUpdateQueue.values());
    this.priceUpdateQueue.clear();
    this.lastUpdateTime = now;

    // Use requestAnimationFrame for smooth UI updates
    requestAnimationFrame(() => {
      updates.forEach((priceData) => {
        try {
          priceCallback(priceData);
        } catch (error) {
          console.error("Error in price update callback:", error);
        }
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

  getConnectionStatus() {
    return {
      isConnected: this.isConnected,
      subscribedSymbols: Array.from(this.subscribedSymbols),
      reconnectAttempts: this.reconnectAttempts,
    };
  }
}

export default new WebSocketStreamingClient();
