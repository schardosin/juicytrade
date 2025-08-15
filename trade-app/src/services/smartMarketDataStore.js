import {
  reactive,
  computed,
  watch,
  onScopeDispose,
  getCurrentScope,
} from "vue";
import webSocketClient from "./webSocketClient.js";
import api from "./api.js";
import MarketHoursUtil from "../utils/marketHours.js";

/**
 * Data Health Monitor - Monitors the health of all data sources
 */
class DataHealthMonitor {
  constructor(store) {
    this.store = store;
    this.healthChecks = new Map();
    this.healthCheckInterval = 60000; // Check every minute
    this.healthTimer = null;
  }

  async start() {
    // Wait for the initial WebSocket connection before starting health checks
    try {
      await webSocketClient.connect();
    } catch (error) {
      console.warn("⚠️ Initial WebSocket connection failed, will retry via health monitoring");
    }
    
    // Log initial market status
    const marketStatus = MarketHoursUtil.getMarketStatus();
    if (!marketStatus.isOpen) {
      console.log('📊 Data freshness checks will be skipped during off-market hours');
    }
    
    // Start periodic health checks
    this.healthTimer = setInterval(() => {
      this.performHealthChecks();
    }, this.healthCheckInterval);
    // Wait a bit before first health check to allow initial connection to stabilize
    setTimeout(() => {
      this.performHealthChecks();
    }, 5000); // 5 second delay for initial health check
  }

  stop() {
    if (this.healthTimer) {
      clearInterval(this.healthTimer);
      this.healthTimer = null;
    }
  }

  async performHealthChecks() {
    // Don't try to connect during health checks - just check current status
    // Connection attempts should be handled by recovery, not health checks

    const marketStatus = MarketHoursUtil.getMarketStatus();
    const checkPromises = {
      websocket: this.checkWebSocketHealth(),
      greeks: marketStatus.isOpen ? this.checkGreeksHealth() : Promise.resolve(),
      dataFreshness: marketStatus.isOpen ? this.checkDataFreshness() : Promise.resolve(),
    };

    const results = await Promise.allSettled(Object.values(checkPromises));
    const checkNames = Object.keys(checkPromises);
    const failedChecks = [];

    results.forEach((result, index) => {
      if (result.status === 'rejected') {
        failedChecks.push({
          name: checkNames[index],
          reason: result.reason.message,
        });
      }
    });

    if (failedChecks.length > 0) {
      console.warn(`⚠️ Health check failures: ${failedChecks.length}/${results.length}`);
      failedChecks.forEach(failure => {
        console.warn(`   - ${failure.name}: ${failure.reason}`);
      });

      this.store.systemState.isHealthy = false;
      this.store.systemState.failedComponents.clear();
      failedChecks.forEach(failure => {
        this.store.systemState.failedComponents.add(failure.name);
      });

      // If WebSocket is unhealthy, trigger recovery
      if (failedChecks.some(f => f.name === 'websocket')) {
        console.log('🚑 WebSocket health check failed, triggering recovery');
        setTimeout(() => {
          this.store.recoveryManager.executeRecovery('websocket_disconnected');
        }, 1000);
      }
    } else {
      this.store.systemState.isHealthy = true;
      this.store.systemState.failedComponents.clear();
    }

    this.store.systemState.lastHealthCheck = Date.now();
  }

  async checkWebSocketHealth() {
    const wsStatus = webSocketClient.getConnectionStatus();
    if (!wsStatus.isConnected.value) {
      throw new Error("WebSocket not connected");
    }
    // Don't try to connect during health checks - just check status
    // Connection attempts should be handled by recovery, not health checks
  }

  async checkGreeksHealth() {
    if (this.store.activeGreeksSubscriptions.size > 0) {
      const timestamps = [];
      for (const symbol of this.store.activeGreeksSubscriptions) {
        const greekData = this.store.optionGreeks.get(symbol);
        if (greekData) {
          timestamps.push(greekData.timestamp || 0);
        }
      }

      // If we have subscriptions but no data yet for any of them, don't fail.
      if (timestamps.length === 0) {
        return;
      }

      const oldestGreeks = Math.min(...timestamps);

      // Greeks should be updated within 5 minutes during market hours
      const staleThreshold = Date.now() - 300000; // 5 minutes
      if (oldestGreeks < staleThreshold) {
        const ageMinutes = Math.round((Date.now() - oldestGreeks) / 60000);
        throw new Error(`Greeks data is stale (${ageMinutes} minutes old)`);
      }
    }
  }

  async checkDataFreshness() {
    // Check if price data is fresh (within 5 minutes for active subscriptions during market hours)
    const cutoff = Date.now() - 300000; // 5 minutes
    let staleCount = 0;
    const totalSubscriptions = this.store.activeSubscriptions.size;

    if (totalSubscriptions === 0) {
      return; // No subscriptions to check
    }

    this.store.activeSubscriptions.forEach(symbol => {
      const priceData = this.store.stockPrices.get(symbol) || this.store.optionPrices.get(symbol);
      if (priceData && priceData.timestamp < cutoff) {
        staleCount++;
      }
    });

    if (staleCount > totalSubscriptions * 0.5) {
      const ageMinutes = Math.round((Date.now() - cutoff) / 60000);
      throw new Error(`Too many stale prices: ${staleCount}/${totalSubscriptions} (older than ${ageMinutes} minutes)`);
    }
  }
}

/**
 * Data Recovery Manager - Handles recovery from various failure scenarios
 */
class DataRecoveryManager {
  constructor(store) {
    this.store = store;
    this.recoveryStrategies = new Map();
    this.setupRecoveryStrategies();
  }

  setupRecoveryStrategies() {
    this.recoveryStrategies.set('websocket_disconnected', this.recoverWebSocket.bind(this));
    this.recoveryStrategies.set('websocket_recovery', this.recoverFromWebSocketRecovery.bind(this));
    this.recoveryStrategies.set('api_failure', this.recoverAPI.bind(this));
    this.recoveryStrategies.set('greeks_stale', this.recoverGreeks.bind(this));
    this.recoveryStrategies.set('data_stale', this.recoverStaleData.bind(this));
    this.recoveryStrategies.set('system_wakeup', this.recoverFromWakeup.bind(this));
    this.recoveryStrategies.set('stale_connection', this.recoverFromStaleConnection.bind(this));
  }

  async executeRecovery(scenario, context = {}) {
    if (this.store.systemState.recoveryInProgress) {
      console.log("🔄 Recovery already in progress, skipping");
      return;
    }

    this.store.systemState.recoveryInProgress = true;
    this.store.systemState.lastRecoveryAttempt = Date.now();

    try {
      console.log(`🚑 Starting recovery for scenario: ${scenario}`);
      
      const strategy = this.recoveryStrategies.get(scenario);
      if (strategy) {
        await strategy(context);
        console.log(`✅ Recovery completed for scenario: ${scenario}`);
      } else {
        console.warn(`⚠️ No recovery strategy for scenario: ${scenario}`);
      }
    } catch (error) {
      console.error(`❌ Recovery failed for scenario ${scenario}:`, error);
    } finally {
      this.store.systemState.recoveryInProgress = false;
    }
  }

  async recoverWebSocket(context) {
    console.log("🔌 Recovering WebSocket connection");
    
    try {
      // Force disconnect and reconnect
      webSocketClient.disconnect();
      await new Promise(resolve => setTimeout(resolve, 2000)); // Increased delay for stability
      
      console.log("🔄 Attempting to reconnect WebSocket...");
      await webSocketClient.connect();
      
      // Restore subscriptions
      const allSymbols = Array.from(this.store.activeSubscriptions);
      if (allSymbols.length > 0) {
        console.log(`🔄 Restoring ${allSymbols.length} subscriptions after recovery`);
        await webSocketClient.replaceAllSubscriptions(allSymbols);
      }
      
      console.log("✅ WebSocket recovery completed successfully");
    } catch (error) {
      console.error("❌ WebSocket recovery failed:", error);
      throw error;
    }
  }

  async recoverAPI(context) {
    console.log("🌐 Recovering API connection");
    
    // Clear any cached data that might be stale
    this.store.cache.clear();
    
    // Retry critical data sources
    const criticalSources = ['providers.config', 'providers.available'];
    for (const source of criticalSources) {
      try {
        await this.store.getData(source, { forceRefresh: true });
      } catch (error) {
        console.warn(`⚠️ Failed to recover ${source}:`, error.message);
      }
    }
  }

  async recoverGreeks(context) {
    console.log("📊 Recovering Greeks data");
    
    // Force immediate Greeks update
    if (this.store.activeGreeksSubscriptions.size > 0) {
      await this.store.updateGreeksData();
    }
  }

  async recoverStaleData(context) {
    console.log("🔄 Recovering stale data");
    
    // Trigger WebSocket health check
    webSocketClient.checkConnectionHealth();
    
    // Force refresh of periodic data
    this.store.timers.forEach((timer, key) => {
      // Trigger immediate update for periodic data sources
      const config = this.store.strategies.get(key);
      if (config && config.strategy === 'periodic') {
        this.store.setupPeriodicUpdate(key, config);
      }
    });
  }

  async recoverFromWakeup(context) {
    console.log("😴 Recovering from system wakeup");
    
    // Comprehensive recovery after sleep/hibernate
    await Promise.all([
      this.recoverWebSocket(context),
      this.recoverAPI(context),
      this.recoverGreeks(context)
    ]);
    
    // Refresh all cached data
    this.store.cache.clear();
    
    // Trigger immediate health check
    await this.store.healthMonitor.performHealthChecks();
    
    // Notify UI components about recovery completion
    this.notifyUIRecovery('system_wakeup');
  }

  async recoverFromWebSocketRecovery(context) {
    console.log("🔄 Handling WebSocket recovery event", context);
    
    // The WebSocket client has already cleared stale data
    // We just need to ensure our subscriptions are restored
    const allSymbols = Array.from(this.store.activeSubscriptions);
    if (allSymbols.length > 0) {
      console.log(`🔄 Ensuring ${allSymbols.length} subscriptions are active after recovery`);
      try {
        await webSocketClient.replaceAllSubscriptions(allSymbols);
      } catch (error) {
        console.error("❌ Failed to restore subscriptions after recovery:", error);
      }
    }
    
    // Update system health
    this.store.systemState.isHealthy = true;
    this.store.systemState.failedComponents.clear();
    
    // Notify UI components
    this.notifyUIRecovery('websocket_recovery');
  }

  async recoverFromStaleConnection(context) {
    console.log("🔄 Recovering from stale connection", context);
    
    // Clear potentially stale data
    const cutoffTime = Date.now() - 120000; // 2 minutes ago
    
    // Clear stale price data
    for (const [symbol, priceData] of this.store.stockPrices.entries()) {
      if (priceData.timestamp < cutoffTime) {
        this.store.stockPrices.delete(symbol);
      }
    }
    
    for (const [symbol, priceData] of this.store.optionPrices.entries()) {
      if (priceData.timestamp < cutoffTime) {
        this.store.optionPrices.delete(symbol);
      }
    }
    
    // Clear stale Greeks data
    for (const [symbol, greeksData] of this.store.optionGreeks.entries()) {
      if (greeksData.timestamp < cutoffTime) {
        this.store.optionGreeks.delete(symbol);
      }
    }
    
    // Force WebSocket recovery
    await this.recoverWebSocket(context);
  }

  /**
   * Notify UI components about recovery events
   */
  notifyUIRecovery(recoveryType) {
    window.dispatchEvent(new CustomEvent('websocket-recovered', {
      detail: {
        timestamp: Date.now(),
        recoveryType: recoveryType,
        subscriptions: Array.from(this.store.activeSubscriptions)
      }
    }));
  }
}

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

    // Reactive data stores - Greeks data (streaming updates)
    this.optionGreeks = reactive(new Map());

    // Reactive data stores - REST API data
    this.data = reactive(new Map()); // General data store for all REST API data

    // Access tracking for WebSocket subscriptions
    this.lastAccess = new Map(); // symbol -> timestamp
    this.activeSubscriptions = new Set(); // currently subscribed symbols

    // Access tracking for Greeks subscriptions (same pattern)
    this.lastGreeksAccess = new Map(); // symbol -> timestamp
    this.activeGreeksSubscriptions = new Set(); // currently subscribed Greeks symbols

    // Component registration system for precise subscription management
    this.symbolUsageCount = new Map(); // symbol -> usage count
    this.componentRegistrations = new Map(); // componentId -> Set<symbols>

    // REST API data management
    this.strategies = new Map(); // data source configurations
    this.loading = reactive(new Map()); // loading states
    this.errors = reactive(new Map()); // error states
    this.timers = new Map(); // periodic update timers
    this.cache = new Map(); // TTL cache for on-demand data

    // Configuration
    this.accessTimeout = 300000; // 5 minutes grace period for keep-alive determination
    this.keepaliveInterval = 15000; // Send keep-alive every 15 seconds
    this.debounceDelay = 100; // 100ms debounce for backend updates

    // Internal state
    this.updateTimer = null;
    this.greeksUpdateTimer = null; // Timer for Greeks periodic updates
    this.greeksImmediateUpdateTimer = null; // Timer for immediate Greeks updates
    this.isInitialized = false;

    // Recovery and health monitoring
    this.healthMonitor = new DataHealthMonitor(this);
    this.recoveryManager = new DataRecoveryManager(this);
    this.systemState = reactive({
      isHealthy: true,
      lastHealthCheck: Date.now(),
      failedComponents: new Set(),
      recoveryInProgress: false,
      lastRecoveryAttempt: null
    });

    // Start keep-alive system
    this.startKeepaliveTimer();

    // Start Greeks periodic updates
    this.startGreeksPeriodicUpdates();

    // Register provider configuration data sources
    this.setupProviderDataSources();

    // Register positions data source
    this.setupPositionsDataSource();

    // Start health monitoring
    this.startHealthMonitoring();

    // Listen for system recovery events
    this.setupRecoveryListeners();

    // Expose for debugging
    window.smartMarketDataStore = this;
  }

  /**
   * Start health monitoring system
   */
  startHealthMonitoring() {
    this.healthMonitor.start();
  }

  /**
   * Set up recovery event listeners
   */
  setupRecoveryListeners() {
    // Track when page becomes hidden to detect sleep
    let pageHiddenTime = null;
    
    // Listen for WebSocket recovery events
    window.addEventListener('websocket-recovered', (event) => {
      console.log('🎉 WebSocket recovery detected, performing data refresh');
      this.performHealthCheck();
    });

    // Listen for page visibility changes (sleep/wake detection)
    document.addEventListener('visibilitychange', () => {
      if (document.hidden) {
        // Page became hidden - record time for sleep detection
        pageHiddenTime = Date.now();
        console.log('👁️ Page became hidden');
      } else {
        // Page became visible - check if we were asleep
        const hiddenDuration = pageHiddenTime ? Date.now() - pageHiddenTime : 0;
        console.log(`👁️ Page became visible after ${Math.round(hiddenDuration / 1000)}s`);
        
        // If hidden for more than 30 seconds, likely system sleep
        if (hiddenDuration > 30000) {
          console.log('😴 Detected potential system sleep/wake cycle, performing health check');
          setTimeout(() => {
            this.performHealthCheck();
          }, 2000); // Longer delay for system to stabilize after wake
        } else {
          // Short absence, just check connection health
          setTimeout(() => {
            this.performHealthCheck();
          }, 1000);
        }
        pageHiddenTime = null;
      }
    });

    // Listen for network status changes
    if ('onLine' in navigator) {
      window.addEventListener('online', () => {
        console.log('🌐 Network came online, performing recovery');
        this.recoveryManager.executeRecovery('api_failure');
      });
      
      window.addEventListener('offline', () => {
        console.log('📴 Network went offline');
      });
    }

    // Listen for focus/blur events as additional sleep detection
    window.addEventListener('focus', () => {
      setTimeout(() => {
        this.performHealthCheck();
      }, 500);
    });
  }

  /**
   * Get system health status (for UI components)
   */
  getSystemHealth() {
    return computed(() => ({
      isHealthy: this.systemState.isHealthy,
      lastHealthCheck: this.systemState.lastHealthCheck,
      failedComponents: Array.from(this.systemState.failedComponents),
      recoveryInProgress: this.systemState.recoveryInProgress,
      lastRecoveryAttempt: this.systemState.lastRecoveryAttempt
    }));
  }

  /**
   * Manually trigger recovery for a specific scenario
   */
  async triggerRecovery(scenario, context = {}) {
    return await this.recoveryManager.executeRecovery(scenario, context);
  }

  /**
   * Manually trigger health check
   */
  async performHealthCheck() {
    return await this.healthMonitor.performHealthChecks();
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
   * Set up positions data source
   */
  setupPositionsDataSource() {
    // Positions - periodic updates every 30 seconds
    this.registerDataSource("positions", {
      strategy: "periodic",
      method: "getPositions",
      interval: 30 * 1000, // 30 seconds
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
      // Use the correct method to replace all subscriptions
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
    
    // Check if this symbol already has Greeks data
    const hasExistingData = this.optionGreeks.has(symbol);
    
    // Trigger immediate update if:
    // 1. This is the first subscription (wasEmpty), OR
    // 2. This symbol doesn't have existing Greeks data
    if (wasEmpty || !hasExistingData) {
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
  }

  /**
   * Update Greeks data for all active subscriptions
   * Only makes API calls if streaming Greeks are not available
   */
  async updateGreeksData() {
    const symbols = Array.from(this.activeGreeksSubscriptions);
    
    if (symbols.length === 0) {
      return; // No symbols to update
    }

    // Check if streaming Greeks are available
    const shouldUseAPI = await this.shouldUseGreeksAPI();
    
    if (!shouldUseAPI) {
      return; // Streaming Greeks are available, don't make API calls
    }

    console.log("📊 Using API Greeks for", symbols.length, "symbols");

    try {
      const greeksData = await api.getOptionsGreeks(symbols);
      
      // Update Greeks data for each symbol
      if (greeksData && typeof greeksData === 'object') {
        // Handle the new Greeks API response structure
        const greeksMap = greeksData.greeks || greeksData;
        
        if (greeksMap && typeof greeksMap === 'object') {
          Object.entries(greeksMap).forEach(([symbol, greeks]) => {
            this.optionGreeks.set(symbol, {
              ...greeks,
              timestamp: Date.now(),
            });
          });
        }
      }
    } catch (error) {
      console.error("❌ Error updating Greeks:", error);
    }
  }

  /**
   * Check if we should use API for Greeks or rely on streaming
   */
  async shouldUseGreeksAPI() {
    try {
      // Get current provider configuration
      const config = await this.getData("providers.config");
      
      if (!config) {
        return true; // No config available, use API as fallback
      }
      
      // If streaming_greeks is configured, don't use API
      if (config.streaming_greeks) {
        return false; // Streaming Greeks available
      }
      
      // If only API Greeks are configured, use API
      if (config.greeks) {
        return true; // Use API Greeks
      }
      
      // No Greeks provider configured, don't make API calls
      return false;
      
    } catch (error) {
      console.error("❌ Error checking Greeks configuration:", error);
      return true; // Fallback to API on error
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
  }

  /**
   * Start keep-alive timer system
   * Sends periodic keep-alive messages to maintain symbol subscriptions
   */
  startKeepaliveTimer() {
    setInterval(() => {
      this.sendKeepalive();
    }, this.keepaliveInterval);
  }

  /**
   * Send keep-alive message for all actively registered symbols
   */
  async sendKeepalive() {
    // Use component registration system for precise keep-alive
    const activeSymbols = Array.from(this.symbolUsageCount.keys());
    
    // Send keep-alive to backend if we have active symbols
    if (activeSymbols.length > 0) {
      try {
        await webSocketClient.sendKeepalive(activeSymbols);
     } catch (error) {
        console.error("❌ Failed to send keep-alive:", error);
      }
    }
  }

  // ===== COMPONENT REGISTRATION SYSTEM =====

  /**
   * Register a component's usage of a symbol
   * This creates a precise subscription that's immediately removed when component unmounts
   */
  registerSymbolUsage(symbol, componentId) {
    if (!symbol || !componentId) return;

    // Increment usage count
    const currentCount = this.symbolUsageCount.get(symbol) || 0;
    this.symbolUsageCount.set(symbol, currentCount + 1);

    // Track which symbols this component is using
    if (!this.componentRegistrations.has(componentId)) {
      this.componentRegistrations.set(componentId, new Set());
    }
    this.componentRegistrations.get(componentId).add(symbol);

    // Ensure subscription if this is the first registration
    if (currentCount === 0) {
      this.ensureSubscription(symbol);
    }
  }

  /**
   * Unregister a component's usage of a symbol
   * Decrements usage count and removes subscription if no components are using it
   */
  unregisterSymbolUsage(symbol, componentId) {
    if (!symbol || !componentId) return;

    // Decrement usage count
    const currentCount = this.symbolUsageCount.get(symbol) || 0;
    if (currentCount <= 1) {
      this.symbolUsageCount.delete(symbol);
      // Remove from active subscriptions when no components are using it
      this.unsubscribeFromSymbol(symbol);
    } else {
      this.symbolUsageCount.set(symbol, currentCount - 1);
    }

    // Remove from component's registration list
    const componentSymbols = this.componentRegistrations.get(componentId);
    if (componentSymbols) {
      componentSymbols.delete(symbol);
      if (componentSymbols.size === 0) {
        this.componentRegistrations.delete(componentId);
      }
    }
  }

  /**
   * Unregister all symbols for a component (called on component unmount)
   */
  unregisterComponent(componentId) {
    const componentSymbols = this.componentRegistrations.get(componentId);
    if (!componentSymbols) return;

    // Unregister each symbol this component was using
    for (const symbol of componentSymbols) {
      this.unregisterSymbolUsage(symbol, componentId);
    }
  }

  /**
   * Get debug information about component registrations
   */
  getRegistrationDebugInfo() {
    return {
      symbolUsageCount: Object.fromEntries(this.symbolUsageCount),
      componentRegistrations: Object.fromEntries(
        Array.from(this.componentRegistrations.entries()).map(([componentId, symbols]) => [
          componentId,
          Array.from(symbols)
        ])
      ),
      totalRegisteredSymbols: this.symbolUsageCount.size,
      totalComponents: this.componentRegistrations.size
    };
  }

  /**
   * Set up WebSocket integration to receive price updates
   */
  async setupWebSocketIntegration() {
    // The new webSocketClient connects automatically via the worker
    // We just need to set up the listeners
    
    webSocketClient.onPriceUpdate((data) => {
      this.handlePriceUpdate(data);
    });

    webSocketClient.onGreeksUpdate((data) => {
      this.handleGreeksUpdate(data);
    });

    webSocketClient.onSubscriptionConfirmed((data) => {

    });

    webSocketClient.onPositionsUpdate((data) => {
      this.updateData('positions', data);
    });

    // Connect to WebSocket
    try {
      await webSocketClient.connect();
    } catch (error) {
      console.error("❌ Failed to connect to WebSocket via worker:", error);
    }
  }

  /**
   * Handle incoming price updates from WebSocket - optimized for high frequency
   */
  handlePriceUpdate(data) {
    const { symbol } = data;
    if (!symbol) return;

    // Extract bid/ask first
    const bid = data.data?.bid;
    const ask = data.data?.ask;
    const last = data.data?.last;

    // Prioritize mid price calculation from bid/ask when both are available
    let finalPrice = null;
    if (bid && ask) {
      finalPrice = (bid + ask) * 0.5;
    } else {
      // Fall back to other price fields if bid/ask not available
      finalPrice = data.price || last || data.data?.mid;
    }

    if (!finalPrice) return; // Skip if no valid price

    // Minimal price data object
    const priceData = {
      price: finalPrice,
      bid,
      ask,
      last,
      timestamp: Date.now(),
    };

    // Fast symbol type check and update
    if (symbol.length > 10) {
      this.optionPrices.set(symbol, priceData);
    } else {
      this.stockPrices.set(symbol, priceData);
    }

    // Update access timestamp only if actively subscribed (avoid unnecessary Map operations)
    if (this.activeSubscriptions.has(symbol)) {
      this.lastAccess.set(symbol, Date.now());
    }
  }

  /**
   * Handle incoming Greeks updates from WebSocket - optimized for high frequency
   */
  handleGreeksUpdate(data) {
    const { symbol } = data;
    if (!symbol) return;

    // Fast path - direct Greeks extraction
    const dataObj = data.data || data;
    const delta = dataObj.delta;
    const gamma = dataObj.gamma;
    const theta = dataObj.theta;
    const vega = dataObj.vega;
    const iv = dataObj.implied_volatility;

    // Skip if no valid Greeks data
    if (delta === undefined && gamma === undefined && theta === undefined && vega === undefined && iv === undefined) {
      return;
    }

    // Minimal Greeks data object
    const greeksData = {
      delta,
      gamma,
      theta,
      vega,
      implied_volatility: iv,
      timestamp: Date.now(),
    };

    // Update Greeks store
    this.optionGreeks.set(symbol, greeksData);

    // Update access timestamp only if actively subscribed
    if (this.activeGreeksSubscriptions.has(symbol)) {
      this.lastGreeksAccess.set(symbol, Date.now());
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
   * Get filtered positions for a specific symbol
   * This automatically updates when positions data changes or symbol changes
   */
  getFilteredPositions(symbol) {
    return computed(() => {
      // Track access for positions data
      this.trackDataAccess('positions');
      
      const allPositions = this.data.get('positions');
      
      if (!allPositions || !symbol) {
        return null;
      }

      // Helper function to get symbol group (handles SPX/SPXW grouping)
      const getSymbolGroup = (sym) => {
        if (sym === "SPX" || sym === "SPXW") {
          return ["SPX", "SPXW"];
        }
        return [sym];
      };

      // Helper function to extract underlying symbol from option symbol
      const extractUnderlyingFromOptionSymbol = (optionSymbol) => {
        const match = optionSymbol.match(/^([A-Z]+)/);
        return match ? match[1] : null;
      };

      const symbolGroup = getSymbolGroup(symbol);
      let filteredPositions = [];

      // Handle different response structures
      if (allPositions.enhanced && allPositions.symbol_groups) {
        // New hierarchical structure
        allPositions.symbol_groups.forEach((symbolGroupData) => {
          if (
            symbolGroup.includes(symbolGroupData.symbol) &&
            (symbolGroupData.asset_class === "options" || symbolGroupData.asset_class === "us_option")
          ) {
            symbolGroupData.strategies.forEach((strategy) => {
              strategy.legs.forEach((leg) => {
                filteredPositions.push({
                  ...leg,
                  underlying_symbol: symbolGroupData.symbol,
                  asset_class: "us_option"
                });
              });
            });
          }
        });
      } else if (allPositions.enhanced && allPositions.position_groups) {
        // Old enhanced structure
        allPositions.position_groups.forEach((group) => {
          if (
            symbolGroup.includes(group.symbol) &&
            (group.asset_class === "options" || group.asset_class === "us_option")
          ) {
            group.legs.forEach((leg) => {
              filteredPositions.push({
                ...leg,
                underlying_symbol: group.symbol,
                asset_class: "us_option"
              });
            });
          }
        });
      } else if (allPositions.positions && Array.isArray(allPositions.positions)) {
        // Fallback to old structure
        filteredPositions = allPositions.positions.filter((pos) => {
          const isOption = pos.asset_class === "us_option" || pos.asset_class === "option";
          const underlyingFromSymbol = extractUnderlyingFromOptionSymbol(pos.symbol);
          const isCurrentSymbolGroup = symbolGroup.includes(underlyingFromSymbol);

          return isOption && isCurrentSymbolGroup;
        });
      }

      return {
        ...allPositions,
        positions: filteredPositions,
        filtered_for_symbol: symbol,
        filtered_count: filteredPositions.length
      };
    });
  }

  /**
   * Force refresh positions data
   * Useful when switching symbols to ensure we have the latest data
   */
  async refreshPositions() {
    const config = this.strategies.get("positions");
    if (config && config.strategy === "periodic") {
      // Clear any existing timer and restart
      const existingTimer = this.timers.get("positions");
      if (existingTimer) {
        clearInterval(existingTimer);
      }
      this.setupPeriodicUpdate("positions", config);
    }
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
        
        // Check if the method exists
        if (!api[config.method]) {
          throw new Error(`API method '${config.method}' not found`);
        }
        
        const response = await api[config.method](...(config.params || []));
        // The API methods already return the extracted data, not the full response
        // So we use the response directly
        this.updateData(key, response);
      } catch (error) {
        console.error(`❌ Error fetching periodic data for ${key}:`, error);
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
