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

  handleMessage(message) {
    //console.log("WebSocket message received:", message);

    switch (message.type) {
      case "price_update":
        const priceCallback = this.callbacks.get("price_update");
        if (priceCallback) {
          priceCallback({
            symbol: message.symbol,
            price: message.data.ask || message.data.bid,
            data: message.data,
          });
        }
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

  disconnect() {
    console.log("Disconnecting WebSocket");

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
