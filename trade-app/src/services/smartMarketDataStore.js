import {
  reactive,
  computed,
  watch,
  onScopeDispose,
  getCurrentScope,
} from "vue";
import webSocketClient from "./webSocketClient.js";
import api from "./api.js";

/**
 * Smart Market Data Store - Unified Data Management System
 *
 * Manages all application data through different strategies:
 * - WebSocket Strategy: Real-time prices (existing functionality)
 * - Periodic Strategy: Auto-refreshing data (account balance, positions)
 * - One-time Strategy: Static cached data (account info)
 * - On-demand Strategy: Manual refresh with TTL (historical data, options chains)
 *
 * Key Features:
 * - Zero component logic required
 * - Automatic subscription/unsubscription for WebSocket data
 * - Automatic periodic updates for account data
 * - Smart caching for on-demand data
 * - Unified interface for all data types
 * - Memory leak prevention
 */
class SmartMarketDataStore {
  constructor() {
    // Reactive data stores - WebSocket data
    this.stockPrices = reactive(new Map());
    this.optionPrices = reactive(new Map());
    this.previousClosePrices = reactive(new Map()); // Previous close prices cache

    // Reactive data stores - Greeks data (periodic updates)
    this.optionGreeks = reactive(new Map());

    // Reactive data stores - REST API data
    this.data = reactive(new Map()); // General data store for all REST API data

    // Access tracking for WebSocket subscriptions
    this.lastAccess = new Map(); // symbol -> timestamp
    this.activeSubscriptions = new Set(); // currently subscribed symbols

    // Access tracking for Greeks subscriptions (same pattern)
    this.lastGreeksAccess = new Map(); // symbol -> timestamp
    this.activeGreeksSubscriptions = new Set(); // currently subscribed Greeks symbols

    // REST API data management
    this.strategies = new Map(); // data source configurations
    this.loading = reactive(new Map()); // loading states
    this.errors = reactive(new Map()); // error states
    this.timers = new Map(); // periodic update timers
    this.cache = new Map(); // TTL cache for on-demand data

    // Configuration
    this.accessTimeout = 45000; // 45 seconds grace period
    this.cleanupInterval = 30000; // Check every 30 seconds
    this.heartbeatInterval = 20000; // Component heartbeat every 20 seconds
    this.debounceDelay = 100; // 100ms debounce for backend updates

    // Internal state
    this.updateTimer = null;
    this.greeksUpdateTimer = null; // Timer for Greeks periodic updates
    this.greeksImmediateUpdateTimer = null; // Timer for immediate Greeks updates
    this.isInitialized = false;

    // Start automatic cleanup
    this.startCleanupTimer();

    // Start Greeks periodic updates
    this.startGreeksPeriodicUpdates();

    // Register provider configuration data sources
    this.setupProviderDataSources();
  }

  /**
   * Set up provider configuration data sources
   */
  setupProviderDataSources() {
    // Available providers - cached for 5 minutes (providers don't change often)
    this.registerDataSource("providers.available", {
      strategy: "on-demand",
      method: "getAvailableProviders",
      ttl: 5 * 60 * 1000, // 5 minutes
    });

    // Current provider configuration - on-demand with 5 minute cache
    this.registerDataSource("providers.config", {
      strategy: "on-demand",
      method: "getProviderConfig",
      ttl: 5 * 60 * 1000, // 5 minutes cache
    });
  }

  /**
   * Initialize the store and connect to WebSocket data flow
   */
  initialize() {
    if (this.isInitialized) return;

    // Set up WebSocket data routing
    this.setupWebSocketIntegration();
    this.isInitialized = true;

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
   * Get reactive option Greeks for a symbol
   * Automatically handles subscription management (same pattern as prices)
   */
  getOptionGreeks(symbol) {
    if (!symbol) return computed(() => null);

    // Ensure Greeks subscription (same pattern as price subscriptions)
    this.ensureGreeksSubscription(symbol);

    // Create reactive Greeks reference that tracks access
    const reactiveGreeks = computed(() => {
      // Update access timestamp every time this computed is evaluated
      this.lastGreeksAccess.set(symbol, Date.now());
      return this.optionGreeks.get(symbol) || null;
    });

    return reactiveGreeks;
  }

  /**
   * Get reactive previous close price for a symbol
   * Automatically handles fetching and caching
   */
  getPreviousClose(symbol) {
    if (!symbol) return computed(() => null);

    // Ensure we have the previous close data
    this.ensurePreviousClose(symbol);

    // Create reactive price reference
    const reactivePreviousClose = computed(() => {
      return this.previousClosePrices.get(symbol) || null;
    });

    return reactivePreviousClose;
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
   * Ensure Greeks subscription for a symbol (same pattern as price subscriptions)
   */
  ensureGreeksSubscription(symbol) {
    if (!symbol) return;

    // Initialize access timestamp
    this.lastGreeksAccess.set(symbol, Date.now());

    // Ensure we're subscribed to Greeks for this symbol
    if (!this.activeGreeksSubscriptions.has(symbol)) {
      this.subscribeToGreeks(symbol);
    }
  }

  /**
   * Ensure previous close data for a symbol
   */
  async ensurePreviousClose(symbol) {
    if (!symbol) return;

    // If not cached, fetch it
    if (!this.previousClosePrices.has(symbol)) {
      try {
        const price = await api.getPreviousClose(symbol);
        this.previousClosePrices.set(symbol, price);
      } catch (error) {
        console.error(`❌ Error fetching previous close for ${symbol}:`, error);
      }
    }
  }

  /**
   * Subscribe to a symbol
   */
  async subscribeToSymbol(symbol) {
    this.activeSubscriptions.add(symbol);

    // Schedule backend update
    this.scheduleBackendUpdate();
  }

  /**
   * Unsubscribe from a symbol
   */
  async unsubscribeFromSymbol(symbol) {
    this.activeSubscriptions.delete(symbol);

    // DON'T delete price data - keep last known prices for UI continuity
    // The price data will remain available for components to use
    // Only remove from active subscriptions to stop receiving new updates

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

    try {
      // Update WebSocket subscriptions
      await webSocketClient.replaceAllSubscriptions(allSymbols);
    } catch (error) {
      console.error("❌ Failed to update backend subscriptions:", error);
    }
  }

  /**
   * Subscribe to Greeks for a symbol (same pattern as price subscriptions)
   */
  subscribeToGreeks(symbol) {
    const wasEmpty = this.activeGreeksSubscriptions.size === 0;
    this.activeGreeksSubscriptions.add(symbol);
    console.log(`📊 Greeks subscription added for: ${symbol}`);
    
    // Check if this symbol already has Greeks data
    const hasExistingData = this.optionGreeks.has(symbol);
    
    // Trigger immediate update if:
    // 1. This is the first subscription (wasEmpty), OR
    // 2. This symbol doesn't have existing Greeks data
    if (wasEmpty || !hasExistingData) {
      console.log(`📊 Scheduling immediate Greeks update for new symbols`);
      // Clear any existing timeout to avoid multiple rapid updates
      if (this.greeksImmediateUpdateTimer) {
        clearTimeout(this.greeksImmediateUpdateTimer);
      }
      // Use a small delay to allow all subscriptions to be added before updating
      this.greeksImmediateUpdateTimer = setTimeout(() => {
        this.updateGreeksData();
        this.greeksImmediateUpdateTimer = null;
      }, 100); // 100ms delay to batch subscriptions
    }
  }

  /**
   * Unsubscribe from Greeks for a symbol
   */
  unsubscribeFromGreeks(symbol) {
    this.activeGreeksSubscriptions.delete(symbol);
    // Keep Greeks data for UI continuity (same as prices)
    console.log(`📊 Greeks subscription removed for: ${symbol}`);
  }

  /**
   * Update Greeks data for all active subscriptions
   */
  async updateGreeksData() {
    const symbols = Array.from(this.activeGreeksSubscriptions);
    
    if (symbols.length === 0) {
      return; // No symbols to update
    }

    try {
      console.log(`📊 Updating Greeks for ${symbols.length} symbols`);
      const greeksData = await api.getOptionsGreeks(symbols);
      
      // Update Greeks data for each symbol
      if (greeksData && typeof greeksData === 'object') {
        Object.entries(greeksData).forEach(([symbol, greeks]) => {
          this.optionGreeks.set(symbol, {
            ...greeks,
            timestamp: Date.now(),
          });
        });
        console.log(`📊 Greeks updated successfully for ${Object.keys(greeksData).length} symbols`);
      }
    } catch (error) {
      console.error("❌ Error updating Greeks:", error);
    }
  }

  /**
   * Start Greeks periodic updates (every 60 seconds)
   */
  startGreeksPeriodicUpdates() {
    // Set up periodic updates every 60 seconds
    this.greeksUpdateTimer = setInterval(() => {
      this.updateGreeksData();
    }, 60000);
    console.log("📊 Greeks periodic updates started (60s interval)");
  }

  /**
   * Start automatic cleanup timer
   */
  startCleanupTimer() {
    setInterval(() => {
      this.cleanupUnusedSymbols();
      this.cleanupUnusedGreeks(); // Also cleanup Greeks
    }, this.cleanupInterval);
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
      toUnsubscribe.forEach((symbol) => {
        this.lastAccess.delete(symbol);
        this.unsubscribeFromSymbol(symbol);
      });
    }
  }

  /**
   * Clean up Greeks that haven't been accessed recently (same pattern as prices)
   */
  cleanupUnusedGreeks() {
    const cutoff = Date.now() - this.accessTimeout;
    const toUnsubscribe = [];

    this.lastGreeksAccess.forEach((timestamp, symbol) => {
      if (timestamp < cutoff) {
        toUnsubscribe.push(symbol);
      }
    });

    if (toUnsubscribe.length > 0) {
      console.log(`📊 Cleaning up unused Greeks for ${toUnsubscribe.length} symbols`);
      toUnsubscribe.forEach((symbol) => {
        this.lastGreeksAccess.delete(symbol);
        this.unsubscribeFromGreeks(symbol);
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

  // ===== REST API DATA MANAGEMENT METHODS =====

  /**
   * Register a data source with its update strategy
   */
  registerDataSource(key, config) {
    this.strategies.set(key, config);

    // Auto-setup based on strategy
    if (config.strategy === "periodic") {
      this.setupPeriodicUpdate(key, config);
    }

  }

  /**
   * Get reactive data for any key (unified interface)
   */
  getReactiveData(key) {
    return computed(() => {
      // Track access for on-demand strategies
      this.trackDataAccess(key);
      return this.data.get(key);
    });
  }

  /**
   * Get data with options (for on-demand and one-time strategies)
   */
  async getData(key, options = {}) {
    let config = this.strategies.get(key);

    // If exact key not found, try wildcard patterns
    if (!config) {
      config = this.findWildcardConfig(key);
    }

    if (!config) {
      throw new Error(`Data source '${key}' not configured`);
    }

    switch (config.strategy) {
      case "once":
        return await this.fetchOnceData(key, config, options.forceRefresh);
      case "on-demand":
        return await this.fetchOnDemandData(key, config, options.forceRefresh);
      case "periodic":
        // For periodic data, just return current value
        return this.data.get(key);
      default:
        throw new Error(`Unknown strategy: ${config.strategy}`);
    }
  }

  /**
   * Find configuration for wildcard patterns
   */
  findWildcardConfig(key) {
    for (const [pattern, config] of this.strategies.entries()) {
      if (pattern.includes("*")) {
        // Convert wildcard pattern to regex
        const regexPattern = pattern.replace(/\*/g, ".*");
        const regex = new RegExp(`^${regexPattern}$`);
        if (regex.test(key)) {
          return config;
        }
      }
    }
    return null;
  }

  /**
   * Set up periodic updates for a data source
   */
  setupPeriodicUpdate(key, config) {
    const fetchData = async () => {
      try {
        this.setLoading(key, true);
        const response = await api[config.method](...(config.params || []));
        
        // Extract actual data from API response wrapper
        const data = response.data !== undefined ? response.data : response;
        
        this.updateData(key, data);
      } catch (error) {
        this.setError(key, error);
      } finally {
        this.setLoading(key, false);
      }
    };

    // Initial fetch
    fetchData();

    // Set up periodic updates
    const timer = setInterval(fetchData, config.interval);
    this.timers.set(key, timer);

  }

  /**
   * Fetch one-time data (cached permanently)
   */
  async fetchOnceData(key, config, forceRefresh = false) {
    // Check if already fetched and not forcing refresh
    if (!forceRefresh && this.data.has(key)) {
      return this.data.get(key);
    }

    try {
      this.setLoading(key, true);
      const response = await api[config.method](...(config.params || []));
      
      // Extract actual data from API response wrapper
      const data = response.data !== undefined ? response.data : response;
      
      this.updateData(key, data);
      return data;
    } catch (error) {
      this.setError(key, error);
      throw error;
    } finally {
      this.setLoading(key, false);
    }
  }

  /**
   * Fetch on-demand data with TTL caching
   */
  async fetchOnDemandData(key, config, forceRefresh = false) {
    const cached = this.cache.get(key);

    // Check cache validity
    if (!forceRefresh && cached && !this.isCacheExpired(cached, config.ttl)) {
      return cached.data;
    }

    try {
      this.setLoading(key, true);

      // Handle dynamic parameters for on-demand strategies
      let params = config.params || [];

      // Extract parameters from key for dynamic methods
      if (key.includes(".")) {
        const keyParts = key.split(".");
        if (keyParts.length > 1) {
          // For keys like "optionsChain.AAPL.2025-01-17", extract symbol and expiry
          params = keyParts.slice(1);
        }
      }

      const response = await api[config.method](...params);
      
      // Extract actual data from API response wrapper
      const data = response.data !== undefined ? response.data : response;

      // Cache the data
      this.cache.set(key, {
        data,
        timestamp: Date.now(),
      });

      this.updateData(key, data);
      return data;
    } catch (error) {
      this.setError(key, error);
      throw error;
    } finally {
      this.setLoading(key, false);
    }
  }

  /**
   * Check if cached data is expired
   */
  isCacheExpired(cached, ttl) {
    return Date.now() - cached.timestamp > ttl;
  }

  /**
   * Track access for on-demand strategies
   */
  trackDataAccess(key) {
    let config = this.strategies.get(key);

    // If exact key not found, try wildcard patterns
    if (!config) {
      config = this.findWildcardConfig(key);
    }

    if (config && config.strategy === "on-demand") {
      // Trigger fetch if not cached or expired
      this.getData(key).catch(console.error);
    }
  }

  /**
   * Update data (called by strategies)
   */
  updateData(key, newData) {
    this.data.set(key, newData);
    this.errors.delete(key);
  }

  /**
   * Set error state
   */
  setError(key, error) {
    this.errors.set(key, error);
    console.error(`❌ Data error for ${key}:`, error);
  }

  /**
   * Set loading state
   */
  setLoading(key, isLoading) {
    if (isLoading) {
      this.loading.set(key, true);
    } else {
      this.loading.delete(key);
    }
  }

  /**
   * Get loading state
   */
  isLoading(key) {
    return computed(() => this.loading.has(key));
  }

  /**
   * Get error state
   */
  getError(key) {
    return computed(() => this.errors.get(key));
  }

  /**
   * Get orders by status (On-Demand Fresh strategy)
   * Always fetches fresh data from API - no caching for orders
   */
  async getOrdersByStatus(status = "all") {
    const key = `orders.${status}`;

    try {
      this.setLoading(key, true);

      // Always fetch fresh data for orders
      const data = await api.getOrders(status);

      // Update the data store
      this.updateData(key, data);

      return data;
    } catch (error) {
      this.setError(key, error);
      console.error(`❌ Error fetching orders with status ${status}:`, error);
      throw error;
    } finally {
      this.setLoading(key, false);
    }
  }

  /**
   * Update provider configuration
   * Updates the provider config and refreshes the data
   */
  async updateProviderConfig(newConfig) {
    const key = "providers.config";

    try {
      this.setLoading(key, true);

      // Send update to backend
      const response = await api.updateProviderConfig(newConfig);
      
      // The backend returns success message but not the updated config
      // So we use the config we sent, since the update was successful
      if (response.success) {
        // Use the config we sent since the update was successful
        this.updateData(key, newConfig);
      } else {
        // If not successful, try to extract from response
        const updatedConfig = response.data || response;
        this.updateData(key, updatedConfig);
      }

      // Also refresh available providers in case capabilities changed
      await this.getData("providers.available", { forceRefresh: true });

      return newConfig;
    } catch (error) {
      this.setError(key, error);
      console.error(`❌ Error updating provider config:`, error);
      throw error;
    } finally {
      this.setLoading(key, false);
    }
  }

  /**
   * Force cleanup of all subscriptions and data (for testing)
   */
  forceCleanup() {
    console.log("🧹 Force cleanup initiated");

    // Clear WebSocket subscriptions
    this.activeSubscriptions.clear();
    this.lastAccess.clear();
    this.stockPrices.clear();
    this.optionPrices.clear();

    // Clear REST API data
    this.data.clear();
    this.cache.clear();
    this.loading.clear();
    this.errors.clear();

    // Clear timers
    this.timers.forEach((timer) => clearInterval(timer));
    this.timers.clear();

    // Update backend
    this.updateBackendSubscriptions();
  }
}

// Create and export singleton instance
export const smartMarketDataStore = new SmartMarketDataStore();
export default smartMarketDataStore;
