import { reactive, computed, onScopeDispose, getCurrentScope } from "vue";
import webSocketClient from "./webSocketClient.js";

/**
 * Smart Market Data Store
 *
 * Automatically manages WebSocket subscriptions based on component usage.
 * Components simply call getStockPrice() or getOptionPrice() and the system
 * handles all subscription management automatically.
 *
 * Key Features:
 * - Zero component logic required
 * - Automatic subscription/unsubscription
 * - Broker limit compliance
 * - Weekend persistence for displayed symbols
 * - Memory leak prevention
 */
class SmartMarketDataStore {
  constructor() {
    // Reactive data stores
    this.stockPrices = reactive(new Map());
    this.optionPrices = reactive(new Map());

    // Access tracking
    this.lastAccess = new Map(); // symbol -> timestamp
    this.activeSubscriptions = new Set(); // currently subscribed symbols

    // Configuration
    this.accessTimeout = 45000; // 45 seconds grace period
    this.cleanupInterval = 30000; // Check every 30 seconds
    this.heartbeatInterval = 20000; // Component heartbeat every 20 seconds
    this.debounceDelay = 100; // 100ms debounce for backend updates

    // Internal state
    this.updateTimer = null;
    this.isInitialized = false;

    // Start automatic cleanup
    this.startCleanupTimer();

    console.log("🚀 SmartMarketDataStore initialized");
  }

  /**
   * Initialize the store and connect to WebSocket data flow
   */
  initialize() {
    if (this.isInitialized) return;

    // Set up WebSocket data routing
    this.setupWebSocketIntegration();
    this.isInitialized = true;

    console.log("📡 SmartMarketDataStore connected to WebSocket");
  }

  /**
   * Get reactive stock price for a symbol
   * Automatically handles subscription management
   */
  getStockPrice(symbol) {
    if (!symbol) return computed(() => null);

    // Ensure subscription
    this.ensureSubscription(symbol);

    // Create reactive price reference that tracks access
    const reactivePrice = computed(() => {
      // Update access timestamp every time this computed is evaluated
      this.lastAccess.set(symbol, Date.now());
      return this.stockPrices.get(symbol) || null;
    });

    return reactivePrice;
  }

  /**
   * Get reactive option price for a symbol
   * Automatically handles subscription management
   */
  getOptionPrice(symbol) {
    if (!symbol) return computed(() => null);

    // Ensure subscription
    this.ensureSubscription(symbol);

    // Create reactive price reference that tracks access
    const reactivePrice = computed(() => {
      // Update access timestamp every time this computed is evaluated
      this.lastAccess.set(symbol, Date.now());
      return this.optionPrices.get(symbol) || null;
    });

    return reactivePrice;
  }

  /**
   * Ensure subscription for a symbol
   */
  ensureSubscription(symbol) {
    if (!symbol) return;

    // Initialize access timestamp
    this.lastAccess.set(symbol, Date.now());

    // Ensure we're subscribed to this symbol
    if (!this.activeSubscriptions.has(symbol)) {
      this.subscribeToSymbol(symbol);
    }
  }

  /**
   * Subscribe to a symbol
   */
  async subscribeToSymbol(symbol) {
    this.activeSubscriptions.add(symbol);
    console.log(`📈 Auto-subscribed to: ${symbol}`);

    // Schedule backend update
    this.scheduleBackendUpdate();
  }

  /**
   * Unsubscribe from a symbol
   */
  async unsubscribeFromSymbol(symbol) {
    this.activeSubscriptions.delete(symbol);

    // Clean up data
    if (this.isOptionSymbol(symbol)) {
      this.optionPrices.delete(symbol);
    } else {
      this.stockPrices.delete(symbol);
    }

    console.log(`📉 Auto-unsubscribed from: ${symbol}`);

    // Schedule backend update
    this.scheduleBackendUpdate();
  }

  /**
   * Schedule debounced backend subscription update
   */
  scheduleBackendUpdate() {
    if (this.updateTimer) {
      clearTimeout(this.updateTimer);
    }

    this.updateTimer = setTimeout(() => {
      this.updateBackendSubscriptions();
    }, this.debounceDelay);
  }

  /**
   * Update backend WebSocket subscriptions
   */
  async updateBackendSubscriptions() {
    const allSymbols = Array.from(this.activeSubscriptions);

    console.log(`🔄 Updating backend subscriptions:`, {
      total: allSymbols.length,
      symbols: allSymbols.slice(0, 10), // Show first 10 for logging
      hasMore: allSymbols.length > 10,
    });

    try {
      // Update WebSocket subscriptions
      await webSocketClient.replaceAllSubscriptions(allSymbols);
    } catch (error) {
      console.error("❌ Failed to update backend subscriptions:", error);
    }
  }

  /**
   * Start automatic cleanup timer
   */
  startCleanupTimer() {
    setInterval(() => {
      this.cleanupUnusedSymbols();
    }, this.cleanupInterval);

    console.log(
      `🧹 Cleanup timer started (${this.cleanupInterval}ms interval)`
    );
  }

  /**
   * Clean up symbols that haven't been accessed recently
   */
  cleanupUnusedSymbols() {
    const cutoff = Date.now() - this.accessTimeout;
    const toUnsubscribe = [];

    this.lastAccess.forEach((timestamp, symbol) => {
      if (timestamp < cutoff) {
        toUnsubscribe.push(symbol);
      }
    });

    if (toUnsubscribe.length > 0) {
      console.log(`🧹 Cleaning up unused symbols:`, toUnsubscribe);

      toUnsubscribe.forEach((symbol) => {
        this.lastAccess.delete(symbol);
        this.unsubscribeFromSymbol(symbol);
      });
    }
  }

  /**
   * Set up WebSocket integration to receive price updates
   */
  async setupWebSocketIntegration() {
    // Connect to WebSocket
    try {
      await webSocketClient.connect();
      console.log("✅ WebSocket connected successfully");
    } catch (error) {
      console.error("❌ Failed to connect to WebSocket:", error);
    }

    // Route price updates to our store
    webSocketClient.onPriceUpdate((data) => {
      this.handlePriceUpdate(data);
    });
  }

  /**
   * Handle incoming price updates from WebSocket
   */
  handlePriceUpdate(data) {
    const { symbol } = data;
    if (!symbol) return;

    // Extract price data
    const priceData = {
      price: data.price || data.data?.last || data.data?.mid,
      bid: data.data?.bid,
      ask: data.data?.ask,
      timestamp: Date.now(),
    };

    // Calculate mid price if needed
    if (!priceData.price && priceData.bid && priceData.ask) {
      priceData.price = (priceData.bid + priceData.ask) / 2;
    }

    // Update appropriate store
    if (this.isOptionSymbol(symbol)) {
      this.optionPrices.set(symbol, priceData);
    } else {
      this.stockPrices.set(symbol, priceData);
    }

    // Update access timestamp to keep subscription alive
    if (this.activeSubscriptions.has(symbol)) {
      this.lastAccess.set(symbol, Date.now());
    }
  }

  /**
   * Check if a symbol is an option symbol
   */
  isOptionSymbol(symbol) {
    return symbol && symbol.length > 10 && /\d{6}[CP]\d{8}/.test(symbol);
  }

  /**
   * Get debug information about current state
   */
  getDebugInfo() {
    return {
      activeSubscriptions: Array.from(this.activeSubscriptions),
      totalSymbols: this.activeSubscriptions.size,
      stockPrices: this.stockPrices.size,
      optionPrices: this.optionPrices.size,
      lastAccess: Object.fromEntries(
        Array.from(this.lastAccess.entries()).map(([symbol, timestamp]) => [
          symbol,
          new Date(timestamp).toLocaleTimeString(),
        ])
      ),
    };
  }

  /**
   * Force cleanup of all subscriptions (for testing)
   */
  forceCleanup() {
    console.log("🧹 Force cleanup initiated");

    // Clear all subscriptions
    this.activeSubscriptions.clear();
    this.lastAccess.clear();
    this.stockPrices.clear();
    this.optionPrices.clear();

    // Update backend
    this.updateBackendSubscriptions();
  }
}

// Create and export singleton instance
export const smartMarketDataStore = new SmartMarketDataStore();
export default smartMarketDataStore;
