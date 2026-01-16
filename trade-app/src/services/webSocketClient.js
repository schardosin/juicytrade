// trade-app/src/services/webSocketClient.js
import { ref } from 'vue';

class WebSocketStreamingClient {
  constructor(baseUrl = null) {
    // Use environment variable or auto-detect WebSocket URL
    if (!baseUrl) {
      // Check if JUICYTRADE_WEBSOCKET_URL is defined and not empty
      const envWebSocketUrl = import.meta.env.JUICYTRADE_WEBSOCKET_URL;
      
      if (envWebSocketUrl && envWebSocketUrl.trim() !== '') {
        // Use the configured WebSocket URL from environment
        this.baseUrl = envWebSocketUrl;
      } else {
        // Fallback: check if we're in development mode (localhost:3001) or production
        if (window.location.hostname === 'localhost' && window.location.port === '3001') {
          // Development mode - use default backend port
          this.baseUrl = 'ws://localhost:8008';
        } else {
          // Production/containerized mode - auto-detect based on current location
          const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
          const host = window.location.host;
          this.baseUrl = `${protocol}//${host}`;
        }
      }
    } else {
      this.baseUrl = baseUrl;
    }
    this.worker = null;
    this.callbacks = new Map();
    this.isConnected = ref(false); // Make this reactive
    this.subscribedSymbols = new Set();
    this.connectionPromise = null;
    this.isDisconnecting = false; // Flag to prevent connections during disconnect

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
      const { type, message, payload, detail } = event.data;
      if (type === 'status') {
        this.handleStatusUpdate(message);
      } else if (type === 'data') {
        this.handleMessage(payload);
      } else if (type === 'recovery') {
        this.handleRecoveryEvent(message, detail);
      } else if (type === 'status_info') {
        this.handleStatusInfo(message);
      }
    };

    this.worker.onerror = (error) => {
      console.error("Error in WebSocket worker:", error);
    };
  }

  handleStatusUpdate(status) {
    const wasConnected = this.isConnected.value;
    this.isConnected.value = (status === 'connected');
    
    // Handle authentication failures specially
    if (status === 'auth_failed') {
      console.log("🔒 WebSocket authentication failed - forcing disconnect");
      this.isConnected.value = false;
      
      // Set disconnecting flag to prevent reconnection attempts
      this.isDisconnecting = true;
      
      // Force disconnect to clean up everything
      this.disconnect();
      
      // Emit auth failure event for UI handling
      window.dispatchEvent(new CustomEvent('websocket-auth-failed', {
        detail: {
          status: status,
          timestamp: Date.now(),
          message: 'WebSocket authentication failed'
        }
      }));
      
      return;
    }
    
    // Emit detailed status event for UI components
    window.dispatchEvent(new CustomEvent('websocket-status-change', {
      detail: {
        status: status,
        timestamp: Date.now(),
        wasConnected: wasConnected,
        isConnected: this.isConnected.value
      }
    }));
    
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

  async connect() {
    // Check if we're in the process of disconnecting
    if (this.isDisconnecting) {
      console.log("🚫 Connection attempt blocked - currently disconnecting");
      return Promise.reject(new Error("Connection blocked - currently disconnecting"));
    }

    if (this.connectionPromise) {
      return this.connectionPromise;
    }

    // If already connected, resolve immediately
    if (this.isConnected.value) {
      return Promise.resolve();
    }

    // CRITICAL: Check authentication BEFORE attempting any connection
    try {
      // Import authService dynamically to avoid circular dependency
      const { default: authService } = await import('./authService.js');
      
      // Check if authentication is enabled
      if (authService.isAuthEnabled()) {
        // Check if user is authenticated
        if (!authService.isAuthenticated()) {
          console.log("🔒 User not authenticated, WebSocket connection will be rejected");
          const error = new Error("User not authenticated - WebSocket connection not allowed");
          throw error;
        }
      }
    } catch (error) {
      console.error("❌ Authentication check failed:", error);
      // Don't even attempt connection if authentication fails
      return Promise.reject(error);
    }

    // Ensure worker exists before attempting connection
    if (!this.worker) {
      console.log("🔄 Worker not found, reinitializing...");
      this.initializeWorker();
    }

    this.connectionPromise = new Promise(async (resolve, reject) => {
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
        // Get authentication credentials (this will also check auth again)
        const authCredentials = await this.getAuthenticationCredentials();
        
        // Build WebSocket URL with authentication
        let wsUrl = `${this.baseUrl}/ws`;
        if (authCredentials.token) {
          wsUrl += `?token=${encodeURIComponent(authCredentials.token)}`;
        }

        this.worker.postMessage({ 
          command: 'connect', 
          url: wsUrl,
          headers: authCredentials.headers
        });
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

  /**
   * Get authentication credentials for WebSocket connection
   */
  async getAuthenticationCredentials() {
    try {
      // Import authService dynamically to avoid circular dependency
      const { default: authService } = await import('./authService.js');
      
      // Check if authentication is enabled
      if (!authService.isAuthEnabled()) {
        return { token: null, headers: {} };
      }

      // Check if user is authenticated
      if (!authService.isAuthenticated()) {
        console.log("🔒 User not authenticated, WebSocket connection will be rejected");
        throw new Error("User not authenticated - WebSocket connection not allowed");
      }

      // Get session cookie name from auth config
      let sessionCookieName = 'juicytrade_session'; // Default fallback
      try {
        const authConfig = await authService.getAuthConfig();
        if (authConfig && authConfig.session_cookie_name) {
          sessionCookieName = authConfig.session_cookie_name;
        }
      } catch (error) {
        console.warn('⚠️ Could not get auth config, using default session cookie name');
      }

      // Try to get session token from cookies
      const cookies = document.cookie.split(';');
      let sessionToken = null;
      
      for (const cookie of cookies) {
        const [name, value] = cookie.trim().split('=');
        if (name === sessionCookieName) {
          sessionToken = value;
          break;
        }
      }

      if (sessionToken) {
        return { 
          token: sessionToken, 
          headers: {} 
        };
      }

      // Debug: log all cookies to see what's available
      console.log("🔍 All cookies:", document.cookie);
      console.log("🔍 Looking for cookie:", sessionCookieName);

      // Fallback: try to get user info and create basic auth
      const user = authService.getUser();
      if (user && user.username) {
        console.log("🔐 Using basic auth for WebSocket authentication");
        // For simple auth, we could use basic auth, but session cookie is preferred
        return { token: null, headers: {} };
      }

      console.warn("⚠️ No authentication credentials available for WebSocket");
      return { token: null, headers: {} };

    } catch (error) {
      console.error("❌ Error getting authentication credentials:", error);
      return { token: null, headers: {} };
    }
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

  onGreeksUpdate(callback) {
    this.addCallback('greeks_update', callback);
  }

  onSubscriptionConfirmed(callback) {
    this.addCallback('subscription_confirmed', callback);
  }

  onPositionsUpdate(callback) {
    this.addCallback('positions_update', callback);
  }

  onIvxUpdate(callback) {
    // Register callback for all IVx message types
    this.addCallback('ivx_status', callback);
    this.addCallback('ivx_update', callback);
    this.addCallback('ivx_complete', callback);
    this.addCallback('error', callback);
  }

  onOrderEvent(callback) {
    this.addCallback('order_event', callback);
    return () => {
      const callbacks = this.callbacks.get('order_event');
      if (callbacks) {
        callbacks.delete(callback);
      }
    };
  }

  onIvxStatus(callback) {
    this.addCallback('ivx_status', callback);
  }

  async subscribeToIvx(symbols) {
    await this.connect();
    if (!Array.isArray(symbols)) {
      symbols = [symbols];
    }
    this.worker.postMessage({ 
      command: 'message', 
      message: { 
        type: 'subscribe_ivx', 
        symbols 
      } 
    });
  }

  async unsubscribeFromIvx(symbols) {
    await this.connect();
    if (!Array.isArray(symbols)) {
      symbols = [symbols];
    }
    this.worker.postMessage({ 
      command: 'message', 
      message: { 
        type: 'unsubscribe_ivx', 
        symbols 
      } 
    });
  }
  
  addCallback(type, callback) {
    if (!this.callbacks.has(type)) {
      this.callbacks.set(type, new Set());
    }
    this.callbacks.get(type).add(callback);
  }

  handleMessage(message) {
    // Add data freshness validation
    if (message.timestamp_ms) {
      const currentTime = Date.now();
      const messageAge = (currentTime - message.timestamp_ms) / 1000; // Age in seconds
      
      // Ignore data older than 30 seconds for price updates
      if ((message.type === 'price_update' || message.type === 'greeks_update') && messageAge > 30) {
        console.debug(`🕒 Ignoring stale ${message.type} for ${message.symbol}: ${messageAge.toFixed(1)}s old`);
        return;
      }
      
      // Log if data is getting old (but still process it for non-price data)
      if (messageAge > 10) {
        console.debug(`⚠️ Processing aged ${message.type} for ${message.symbol}: ${messageAge.toFixed(1)}s old`);
      }
    }
    
    const callbacks = this.callbacks.get(message.type);
    
    if (callbacks) {
      callbacks.forEach((callback) => {
        try {
          const dataToSend = (
            message.type === 'price_update' || 
            message.type === 'greeks_update' || 
            message.type === 'subscription_confirmed' ||
            message.type === 'order_event' ||
            message.type === 'ivx_status' ||
            message.type === 'ivx_update' ||
            message.type === 'ivx_complete' ||
            message.type === 'error'
          ) ? message : message.data;
          callback(dataToSend);
        } catch (error) {
          console.error(`[WEBSOCKET] Error in callback for message type ${message.type}:`, error);
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

  async sendKeepalive(symbols = []) {
    await this.connect();
    
    this.worker.postMessage({
      command: "keepalive",
      action: "keepalive",
      symbols: symbols,
    });
  }

  isOptionSymbol(symbol) {
    return symbol && symbol.length > 10 && /\d{6}[CP]\d{8}/.test(symbol);
  }

  // Enhanced recovery and connection management methods
  handleRecoveryEvent(recoveryType, detail) {
    console.log(`🚑 Recovery event: ${recoveryType}`, detail);
    
    // Notify the smart market data store about recovery
    import('./smartMarketDataStore.js').then(({ smartMarketDataStore }) => {
      // Clear stale data when recovery happens
      this.clearStaleData();
      
      // Trigger recovery in the store - this will handle silent recovery internally
      smartMarketDataStore.triggerRecovery('websocket_recovery', {
        recoveryType,
        detail,
        timestamp: Date.now()
      });
      
      // Don't dispatch UI recovery events here - let the SmartMarketDataStore 
      // decide whether to show UI notifications based on silent recovery logic
      console.log('✅ Recovery event handled, silent recovery logic will determine UI notifications');
    }).catch(error => {
      console.error('❌ Failed to handle recovery event:', error);
    });
  }

  handleStatusInfo(statusInfo) {
    console.log('📊 Connection status info:', statusInfo);
    // Store status info for debugging
    this.lastStatusInfo = statusInfo;
  }

  clearStaleData() {
    console.log('🧹 Clearing stale data after recovery');
    
    // Import and clear stale data from the store
    import('./smartMarketDataStore.js').then(({ smartMarketDataStore }) => {
      // Clear price data that might be stale
      const cutoffTime = Date.now() - 60000; // 1 minute ago
      
      // Clear stale stock prices
      for (const [symbol, priceData] of smartMarketDataStore.stockPrices.entries()) {
        if (priceData.timestamp < cutoffTime) {
          console.log(`🧹 Clearing stale stock price for ${symbol}`);
          smartMarketDataStore.stockPrices.delete(symbol);
        }
      }
      
      // Clear stale option prices
      for (const [symbol, priceData] of smartMarketDataStore.optionPrices.entries()) {
        if (priceData.timestamp < cutoffTime) {
          console.log(`🧹 Clearing stale option price for ${symbol}`);
          smartMarketDataStore.optionPrices.delete(symbol);
        }
      }
      
      // Clear stale Greeks data
      for (const [symbol, greeksData] of smartMarketDataStore.optionGreeks.entries()) {
        if (greeksData.timestamp < cutoffTime) {
          console.log(`🧹 Clearing stale Greeks for ${symbol}`);
          smartMarketDataStore.optionGreeks.delete(symbol);
        }
      }
      
      console.log('✅ Stale data cleared successfully');
    }).catch(error => {
      console.error('❌ Failed to clear stale data:', error);
    });
  }

  // Force recovery method for manual triggers
  async forceRecovery() {
    console.log('🚑 Forcing connection recovery');
    if (this.worker) {
      this.worker.postMessage({ command: 'force_recovery' });
    }
  }

  // Get detailed connection status
  async getDetailedStatus() {
    return new Promise((resolve) => {
      if (!this.worker) {
        resolve({
          connected: false,
          error: 'Worker not initialized'
        });
        return;
      }
      
      // Set up one-time listener for status response
      const handleStatusResponse = (event) => {
        if (event.data.type === 'status_info') {
          this.worker.removeEventListener('message', handleStatusResponse);
          resolve({
            connected: this.isConnected.value,
            ...event.data.message
          });
        }
      };
      
      this.worker.addEventListener('message', handleStatusResponse);
      this.worker.postMessage({ command: 'get_status' });
      
      // Timeout after 5 seconds
      setTimeout(() => {
        this.worker.removeEventListener('message', handleStatusResponse);
        resolve({
          connected: this.isConnected.value,
          error: 'Status request timeout'
        });
      }, 5000);
    });
  }

  // Enhanced disconnect with immediate worker cleanup
  disconnect() {
    console.log("🔌 Disconnecting WebSocket client...");
    
    // Set a flag to prevent any new connection attempts
    this.isDisconnecting = true;
    
    // Clean up connection promise
    if (this.rejectConnection) {
      this.rejectConnection(new Error("Connection manually disconnected"));
      this.rejectConnection = null;
      this.resolveConnection = null;
    }
    
    // Clear any connection timeout
    if (this.connectionTimeout) {
      clearTimeout(this.connectionTimeout);
      this.connectionTimeout = null;
    }
    
    if (this.worker) {
      // Send disconnect command to worker first
      try {
        this.worker.postMessage({ command: 'disconnect' });
        console.log("📤 Disconnect command sent to worker");
      } catch (error) {
        console.warn('⚠️ Failed to send disconnect command to worker:', error);
      }
      
      // CRITICAL FIX: Terminate worker IMMEDIATELY - no delay!
      try {
        this.worker.terminate();
        this.worker = null;
        console.log("💀 Worker terminated immediately");
      } catch (error) {
        console.error('❌ Failed to terminate worker:', error);
        // Force null the worker reference even if termination failed
        this.worker = null;
      }
    }
    
    // Clear all state
    this.isConnected.value = false;
    this.connectionPromise = null;
    this.subscribedSymbols.clear();
    
    // Reset disconnecting flag after a short delay to allow for new connections later
    setTimeout(() => {
      this.isDisconnecting = false;
    }, 1000);
  }
}

export default new WebSocketStreamingClient();
