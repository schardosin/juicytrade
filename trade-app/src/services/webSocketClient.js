// trade-app/src/services/webSocketClient.js
import { ref } from 'vue';

class WebSocketStreamingClient {
  constructor(baseUrl = "ws://localhost:8008") {
    this.baseUrl = baseUrl;
    this.worker = null;
    this.callbacks = new Map();
    this.isConnected = ref(false); // Make this reactive
    this.subscribedSymbols = new Set();
    this.connectionPromise = null;

    this.initializeWorker();
  }

  initializeWorker() {
    // Ensure this runs only in the browser
    if (typeof Worker === "undefined") {
      console.error("Web Workers are not supported in this environment.");
      return;
    }

    this.worker = new Worker(new URL('./streaming-worker.js', import.meta.url), { type: 'module' });

    this.worker.onmessage = (event) => {
      const { type, message, payload } = event.data;
      if (type === 'status') {
        this.handleStatusUpdate(message);
      } else if (type === 'data') {
        this.handleMessage(payload);
      }
    };

    this.worker.onerror = (error) => {
      console.error("Error in WebSocket worker:", error);
    };
  }

  handleStatusUpdate(status) {
    const wasConnected = this.isConnected.value;
    this.isConnected.value = (status === 'connected');
    
    if (this.isConnected.value) {
      // Clear connection timeout
      if (this.connectionTimeout) {
        clearTimeout(this.connectionTimeout);
        this.connectionTimeout = null;
      }
      
      if (this.resolveConnection) {
        this.resolveConnection();
        this.resolveConnection = null;
        this.rejectConnection = null;
      }
      
      // If we just connected, trigger immediate system health update
      if (!wasConnected) {
        // Import dynamically to avoid circular dependency
        import('./smartMarketDataStore.js').then(({ smartMarketDataStore }) => {
          // Update system health immediately when connection is established
          smartMarketDataStore.systemState.isHealthy = true;
          smartMarketDataStore.systemState.failedComponents.clear();
        }).catch(error => {
          console.error('❌ Failed to update system health:', error);
        });
      }
    } else {
      // Connection lost
      this.isConnected.value = false;
    }
  }

  connect() {
    if (this.connectionPromise) {
      return this.connectionPromise;
    }

    // If already connected, resolve immediately
    if (this.isConnected.value) {
      return Promise.resolve();
    }

    // Ensure worker exists before attempting connection
    if (!this.worker) {
      console.log("🔄 Worker not found, reinitializing...");
      this.initializeWorker();
    }

    this.connectionPromise = new Promise((resolve, reject) => {
      this.resolveConnection = resolve;
      this.rejectConnection = reject;
      
      // Add timeout to prevent hanging promises
      const timeout = setTimeout(() => {
        if (this.rejectConnection) {
          console.warn("⏰ Connection timeout - clearing promise and retrying");
          this.rejectConnection(new Error("Connection timeout"));
          this.rejectConnection = null;
          this.resolveConnection = null;
          this.connectionPromise = null;
        }
      }, 10000); // 10 second timeout

      // Store timeout reference to clear it on success
      this.connectionTimeout = timeout;

      try {
        this.worker.postMessage({ command: 'connect', url: `${this.baseUrl}/ws` });
      } catch (error) {
        clearTimeout(timeout);
        console.error("❌ Failed to send connect message to worker:", error);
        if (this.rejectConnection) {
          this.rejectConnection(error);
          this.rejectConnection = null;
          this.resolveConnection = null;
          this.connectionPromise = null;
        }
      }
    });

    return this.connectionPromise;
  }

  async subscribe(symbols) {
    await this.connect();
    if (!Array.isArray(symbols)) {
      symbols = [symbols];
    }
    symbols.forEach(symbol => this.subscribedSymbols.add(symbol));
    this.worker.postMessage({ command: 'subscribe', symbols });
  }

  async unsubscribe(symbols) {
    await this.connect();
    if (!Array.isArray(symbols)) {
      symbols = [symbols];
    }
    symbols.forEach(symbol => this.subscribedSymbols.delete(symbol));
    this.worker.postMessage({ command: 'unsubscribe', symbols });
  }
  
  onPriceUpdate(callback) {
    this.addCallback('price_update', callback);
  }

  onSubscriptionConfirmed(callback) {
    this.addCallback('subscription_confirmed', callback);
  }

  onPositionsUpdate(callback) {
    this.addCallback('positions_update', callback);
  }
  
  addCallback(type, callback) {
    if (!this.callbacks.has(type)) {
      this.callbacks.set(type, new Set());
    }
    this.callbacks.get(type).add(callback);
  }

  handleMessage(message) {
    const callbacks = this.callbacks.get(message.type);
    if (callbacks) {
      callbacks.forEach(callback => {
        try {
          // Pass the whole message for price_update and subscription_confirmed
          // and just the data for others, which matches the legacy client's behavior.
          const dataToSend = (message.type === 'price_update' || message.type === 'subscription_confirmed')
            ? message
            : message.data;
          callback(dataToSend);
        } catch (error) {
          console.error(`Error in callback for message type ${message.type}:`, error);
        }
      });
    }
  }

  // Methods that send requests to the backend
  requestPositions() {
    this.sendMessage({ type: "get_positions" });
  }

  requestOpenOrders() {
    this.sendMessage({ type: "get_open_orders" });
  }
  
  async sendMessage(message) {
    await this.connect();
    // In a worker setup, we'd post a message to the worker.
    // This method provides a consistent API for sending various message types.
    this.worker.postMessage({ command: 'message', message });
  }

  getConnectionStatus() {
    return {
      isConnected: this.isConnected,
      subscribedSymbols: Array.from(this.subscribedSymbols),
    };
  }

  async ensurePersistentSubscriptions(dataTypes = ["orders", "positions"]) {
    await this.connect();
    this.worker.postMessage({
      command: "subscribe_persistent",
      type: "subscribe_persistent",
      data_types: dataTypes,
    });
  }

  async replaceAllSubscriptions(symbols = []) {
    await this.connect();
    
    const stock_symbols = symbols.filter(s => !this.isOptionSymbol(s));
    const option_symbols = symbols.filter(s => this.isOptionSymbol(s));

    this.worker.postMessage({
      command: "subscribe_replace_all",
      action: "subscribe_replace_all",
      stock_symbols,
      option_symbols,
    });
  }

  isOptionSymbol(symbol) {
    return symbol && symbol.length > 10 && /\d{6}[CP]\d{8}/.test(symbol);
  }

  disconnect() {
    console.log("🔌 Disconnecting WebSocket client...");
    
    // Clean up connection promise
    if (this.rejectConnection) {
      this.rejectConnection(new Error("Connection manually disconnected"));
      this.rejectConnection = null;
      this.resolveConnection = null;
    }
    
    if (this.worker) {
      this.worker.terminate();
      this.worker = null;
    }
    
    this.isConnected.value = false;
    this.connectionPromise = null;
    console.log("✅ WebSocket client disconnected and worker terminated.");
  }
}

export default new WebSocketStreamingClient();
