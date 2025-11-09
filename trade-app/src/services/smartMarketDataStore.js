import {
  reactive,
  computed,
  watch,
  onScopeDispose,
  getCurrentScope,
  nextTick,
} from "vue";
import webSocketClient from "./webSocketClient.js";
import api, { setServicesStoppingState } from "./api.js";
import MarketHoursUtil from "../utils/marketHours.js";
import authService from "./authService.js";

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
    // Don't try to connect here - let the authentication-aware system handle connections
    // Health monitoring should only check existing connections, not create them
    
    // Skip data freshness checks during off-market hours
    const marketStatus = MarketHoursUtil.getMarketStatus();
    
    // Start periodic health checks
    this.healthTimer = setInterval(() => {
      // Only perform health checks if user is authenticated
      if (!authService.isAuthEnabled() || authService.isAuthenticated()) {
        this.performHealthChecks();
      }
    }, this.healthCheckInterval);
    // Wait a bit before first health check to allow initial connection to stabilize
    setTimeout(() => {
      // Only perform initial health check if user is authenticated
      if (!authService.isAuthEnabled() || authService.isAuthenticated()) {
        this.performHealthChecks();
      }
    }, 5000); // 5 second delay for initial health check
  }

  stop() {
    if (this.healthTimer) {
      clearInterval(this.healthTimer);
      this.healthTimer = null;
    }
  }

  async performHealthChecks() {
    // Don't perform health checks when services are stopping
    if (this.isServicesStopping) {
      return;
    }
    
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

      // If WebSocket is unhealthy, trigger recovery (only if authenticated)
      if (failedChecks.some(f => f.name === 'websocket') && 
          (!authService.isAuthEnabled() || authService.isAuthenticated())) {
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
    this.recoveryHistory = new Map(); // Track recovery success rates
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

  /**
   * Calculate recovery confidence based on historical success rates and current conditions
   */
  calculateRecoveryConfidence(scenario, failedComponents = []) {
    let confidence = 0.7; // Base confidence
    
    // Get historical success rate for this scenario
    const history = this.recoveryHistory.get(scenario);
    if (history && history.attempts > 0) {
      const successRate = history.successes / history.attempts;
      confidence = (confidence + successRate) / 2; // Average with historical data
    }
    
    // Adjust based on failure type
    if (scenario === 'websocket_disconnected') {
      confidence += 0.2; // WebSocket recovery is usually reliable
    }
    
    if (failedComponents.length === 1) {
      confidence += 0.1; // Single component failures are easier to recover
    } else if (failedComponents.length > 2) {
      confidence -= 0.2; // Multiple failures are harder to recover
    }
    
    // Adjust based on recent recovery attempts
    const recentFailures = this.getRecentFailureCount(scenario);
    if (recentFailures > 2) {
      confidence -= 0.3; // Recent failures reduce confidence
    }
    
    return Math.max(0.1, Math.min(1.0, confidence));
  }

  /**
   * Get recent failure count for a scenario (last 10 minutes)
   */
  getRecentFailureCount(scenario) {
    const history = this.recoveryHistory.get(scenario);
    if (!history || !history.recentAttempts) return 0;
    
    const tenMinutesAgo = Date.now() - 600000;
    return history.recentAttempts.filter(attempt => 
      attempt.timestamp > tenMinutesAgo && !attempt.success
    ).length;
  }

  /**
   * Record recovery attempt result
   */
  recordRecoveryAttempt(scenario, success) {
    let history = this.recoveryHistory.get(scenario);
    if (!history) {
      history = {
        attempts: 0,
        successes: 0,
        recentAttempts: []
      };
      this.recoveryHistory.set(scenario, history);
    }
    
    history.attempts++;
    if (success) {
      history.successes++;
    }
    
    // Track recent attempts (keep last 20)
    history.recentAttempts.push({
      timestamp: Date.now(),
      success
    });
    
    if (history.recentAttempts.length > 20) {
      history.recentAttempts = history.recentAttempts.slice(-20);
    }
  }

  /**
   * Get recovery statistics for debugging
   */
  getRecoveryStats() {
    const stats = {};
    for (const [scenario, history] of this.recoveryHistory.entries()) {
      const successRate = history.attempts > 0 ? history.successes / history.attempts : 0;
      const recentFailures = this.getRecentFailureCount(scenario);
      
      stats[scenario] = {
        totalAttempts: history.attempts,
        totalSuccesses: history.successes,
        successRate: Math.round(successRate * 100) + '%',
        recentFailures,
        confidence: this.calculateRecoveryConfidence(scenario, [])
      };
    }
    return stats;
  }

  async executeRecovery(scenario, context = {}) {
    // Don't attempt recovery when services are stopping or user not authenticated
    if (this.isServicesStopping || (authService.isAuthEnabled() && !authService.isAuthenticated())) {
      console.log(`🚫 Skipping recovery for ${scenario} - services stopping or user not authenticated`);
      return false;
    }
    
    if (this.store.systemState.recoveryInProgress) {
      console.log("🔄 Recovery already in progress, skipping");
      return false;
    }

    this.store.systemState.recoveryInProgress = true;
    this.store.systemState.lastRecoveryAttempt = Date.now();

    let success = false;
    try {
      console.log(`🚑 Starting recovery for scenario: ${scenario}`);
      
      const strategy = this.recoveryStrategies.get(scenario);
      if (strategy) {
        await strategy(context);
        success = true;
        console.log(`✅ Recovery completed for scenario: ${scenario}`);
      } else {
        console.warn(`⚠️ No recovery strategy for scenario: ${scenario}`);
        success = false;
      }
    } catch (error) {
      console.error(`❌ Recovery failed for scenario ${scenario}:`, error);
      success = false;
    } finally {
      // Record the recovery attempt
      this.recordRecoveryAttempt(scenario, success);
      this.store.systemState.recoveryInProgress = false;
    }

    return success;
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
    
    // Clear stale Greeks data first
    const cutoffTime = Date.now() - 60000; // 1 minute ago
    for (const [symbol, greeksData] of this.store.optionGreeks.entries()) {
      if (greeksData.timestamp < cutoffTime) {
        console.log(`📊 Clearing stale Greeks for ${symbol}`);
        this.store.optionGreeks.delete(symbol);
      }
    }
    
    // Force immediate Greeks update for all active subscriptions
    if (this.store.activeGreeksSubscriptions.size > 0) {
      console.log(`📊 Forcing Greeks update for ${this.store.activeGreeksSubscriptions.size} symbols`);
      await this.store.updateGreeksData();
    }
    
    // Also try to refresh WebSocket connection if Greeks are streaming
    try {
      const config = await this.store.getData("providers.config");
      if (config && config.streaming_greeks) {
        console.log("📊 Greeks are streaming, refreshing WebSocket connection");
        // Trigger a WebSocket refresh to get fresh streaming Greeks
        const allSymbols = Array.from(this.store.activeSubscriptions);
        if (allSymbols.length > 0) {
          await webSocketClient.replaceAllSubscriptions(allSymbols);
        }
      }
    } catch (error) {
      console.warn("⚠️ Failed to check Greeks streaming config:", error);
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
    
    // Only notify UI if recovery fails (system wakeup is usually a critical recovery)
    if (!this.store.systemState.isHealthy) {
      this.notifyUIRecovery('system_wakeup', true);
    }
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
    
    // Don't notify UI for successful automatic recovery - let silent recovery handle UI
    // UI will only be notified if silent recovery fails
    console.log("✅ WebSocket recovery completed silently");
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
   * Only notifies UI when recovery fails after silent period, not on successful recovery
   */
  notifyUIRecovery(recoveryType, shouldShowUI = false) {
    // Only dispatch UI notification events if explicitly requested
    // This prevents automatic UI notifications during successful silent recovery
    if (shouldShowUI) {
      window.dispatchEvent(new CustomEvent('websocket-recovered', {
        detail: {
          timestamp: Date.now(),
          recoveryType: recoveryType,
          subscriptions: Array.from(this.store.activeSubscriptions)
        }
      }));
    }
    
    // Always dispatch internal recovery event for system components to handle
    window.dispatchEvent(new CustomEvent('system-recovery-internal', {
      detail: {
        timestamp: Date.now(),
        recoveryType: recoveryType,
        subscriptions: Array.from(this.store.activeSubscriptions),
        showUI: shouldShowUI
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

    // Current symbol's daily 6M data (simplified single-symbol approach)
    this.currentSymbolDaily6M = {
      symbol: null,
      bars: [],
      lastUpdated: null,
      providerIdentity: null,
      isLoading: false
    };

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

    // Authentication state tracking
    this.isAuthenticated = false;
    this.authStateListeners = [];
    
    // Service state tracking
    this.isServicesStopping = false;

    // Set up authentication state monitoring
    this.setupAuthenticationIntegration();

    // Only start services if authentication is enabled and user is authenticated
    // This prevents connections when not logged in
    this.initializeConditionally();

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
      // Only perform health check if user is authenticated
      if (!authService.isAuthEnabled() || authService.isAuthenticated()) {
        this.performHealthCheck();
      }
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
            // Only perform health check if user is authenticated
            if (!authService.isAuthEnabled() || authService.isAuthenticated()) {
              this.performHealthCheck();
            }
          }, 2000); // Longer delay for system to stabilize after wake
        } else {
          // Short absence, just check connection health
          setTimeout(() => {
            // Only perform health check if user is authenticated
            if (!authService.isAuthEnabled() || authService.isAuthenticated()) {
              this.performHealthCheck();
            }
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
        // Only perform health check if user is authenticated
        if (!authService.isAuthEnabled() || authService.isAuthenticated()) {
          this.performHealthCheck();
        }
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
    // Only perform health check if user is authenticated
    if (!authService.isAuthEnabled() || authService.isAuthenticated()) {
      return await this.healthMonitor.performHealthChecks();
    }
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
      // Return early if services are stopping
      if (this.isServicesStopping) {
        return null;
      }
      
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
      // Return early if services are stopping
      if (this.isServicesStopping) {
        return null;
      }
      
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
      // Return early if services are stopping
      if (this.isServicesStopping) {
        return null;
      }
      
      // Update access timestamp every time this computed is evaluated
      this.lastGreeksAccess.set(symbol, Date.now());
      return this.optionGreeks.get(symbol) || null;
    });

    return reactiveGreeks;
  }

  /**
   * Get reactive IVx data for a symbol - NEW API-based approach
   * Uses TTL caching strategy for optimal performance and reliability
   */
  getIvxData(symbol) {
    return computed(() => {
      // Return early if services are stopping
      if (this.isServicesStopping) {
        return { isLoading: false, expirations: [], error: 'Services stopping' };
      }
      
      // Handle computed refs - extract actual string value
      let actualSymbol = symbol;
      if (typeof symbol === 'function') {
        actualSymbol = symbol();
      } else if (symbol && typeof symbol === 'object' && 'value' in symbol) {
        actualSymbol = symbol.value;
      }
      
      
      if (!actualSymbol) {
        return { isLoading: false, expirations: [] };
      }

      const key = `ivxData.${actualSymbol}`;
      
      // Check loading state
      const isLoading = this.loading.has(key);
      
      // Get cached data
      const cachedData = this.data.get(key);
      
      // Check for errors
      const error = this.errors.get(key);
      
      if (error) {
        return { 
          isLoading: false, 
          expirations: [], 
          error: error.message,
          status: 'error' 
        };
      }
      
      if (isLoading && !cachedData) {
        return { 
          isLoading: true, 
          expirations: [],
          status: 'loading' 
        };
      }
      
      if (cachedData && cachedData.expirations) {
        return {
          isLoading: false,
          expirations: cachedData.expirations,
          cached: cachedData.cached,
          calculationTime: cachedData.calculation_time,
          status: 'completed'
        };
      }
      
      // No data yet - trigger fetch but don't wait
      const strategy = this.strategies.get('ivxData.*');
      this.fetchOnDemandData(key, strategy).catch(err => {
        console.error(`IVx fetch failed for ${actualSymbol}:`, err.message);
        // Set error state so UI can show error instead of infinite loading
        this.setError(key, err);
      });
      
      return { 
        isLoading: true, 
        expirations: [],
        status: 'loading' 
      };
    });
  }

  /**
   * Get reactive previous close price for a symbol
   * Automatically handles fetching and caching
   */
  getPreviousClose(symbol) {
    if (!symbol) return computed(() => null);

    // Don't ensure data when services are stopping
    if (!this.isServicesStopping) {
      // Ensure we have the previous close data
      this.ensurePreviousClose(symbol);
    }

    // Create reactive price reference
    const reactivePreviousClose = computed(() => {
      // Return early if services are stopping
      if (this.isServicesStopping) {
        return null;
      }
      
      return this.previousClosePrices.get(symbol) || null;
    });

    return reactivePreviousClose;
  }

  /**
   * Ensure subscription for a symbol
   */
  ensureSubscription(symbol) {
    // Don't create subscriptions when services are stopping
    if (this.isServicesStopping) {
      return;
    }
    
    // Handle computed refs and extract actual string value
    let actualSymbol = symbol;
    if (typeof symbol === 'function') {
      actualSymbol = symbol();
    } else if (symbol && typeof symbol === 'object' && 'value' in symbol) {
      actualSymbol = symbol.value;
    }

    if (!actualSymbol || typeof actualSymbol !== 'string') return;

    // Initialize access timestamp
    this.lastAccess.set(actualSymbol, Date.now());

    // Ensure we're subscribed to this symbol
    if (!this.activeSubscriptions.has(actualSymbol)) {
      this.subscribeToSymbol(actualSymbol);
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
    
    // Don't fetch data when services are stopping
    if (this.isServicesStopping) {
      return;
    }

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
    // Don't schedule updates when services are stopping
    if (this.isServicesStopping) {
      return;
    }
    
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
    // Don't attempt WebSocket operations if services are stopping or not authenticated
    if (this.isServicesStopping || !this.isAuthenticated) {
      return;
    }

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
    // Don't update Greeks data when services are stopping
    if (this.isServicesStopping) {
      return;
    }
    
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
      // Don't update Greeks when services are stopping
      if (!this.isServicesStopping) {
        this.updateGreeksData();
      }
    }, 60000);
  }

  /**
   * Start keep-alive timer system
   * Sends periodic keep-alive messages to maintain symbol subscriptions
   */
  startKeepaliveTimer() {
    setInterval(() => {
      // Don't send keepalive when services are stopping
      if (!this.isServicesStopping) {
        this.sendKeepalive();
      }
    }, this.keepaliveInterval);
  }

  /**
   * Send keep-alive message for all actively registered symbols
   */
  async sendKeepalive() {
    // Don't send keepalive when services are stopping
    if (this.isServicesStopping) {
      return;
    }
    
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





    // Only connect to WebSocket if authenticated
    // The webSocketClient.connect() method already has authentication checks,
    // but we should avoid calling it at all when not authenticated
    if (this.isAuthenticated) {
      try {
        await webSocketClient.connect();
      } catch (error) {
        console.error("❌ Failed to connect to WebSocket via worker:", error);
      }
    } else {
      console.log("🔒 Skipping WebSocket connection setup - user not authenticated");
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
      
      // Update today's candle in daily 6M data if this is a stock price
      this.updateTodaysCandleWithPrice(symbol, finalPrice);
    }

    // Update access timestamp only if actively subscribed (avoid unnecessary Map operations)
    if (this.activeSubscriptions.has(symbol)) {
      this.lastAccess.set(symbol, Date.now());
    }
  }

  /**
   * Handle incoming Greeks updates from WebSocket - optimized for high frequency
   */
  async handleGreeksUpdate(data) {
    const { symbol } = data;
    if (!symbol) return;

    // CRITICAL FIX: Only process streaming Greeks if streaming provider is configured
    // Check provider configuration first
    try {
      const config = await this.getData("providers.config");
      
      // If no streaming Greeks provider is configured, ignore streaming Greeks
      if (!config || !config.streaming_greeks) {
        return;
      }
    } catch (error) {
      console.warn("⚠️ Could not check provider config, ignoring streaming Greeks:", error);
      return;
    }

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

    console.log(`📊 Processing streaming Greeks for ${symbol}:`, { delta, gamma, theta, vega });

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
      // Return early if services are stopping to prevent any data access
      if (this.isServicesStopping) {
        return null;
      }
      
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
      // Return early if services are stopping to prevent any data access
      if (this.isServicesStopping) {
        return null;
      }
      
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
    // CRITICAL FIX: Don't fetch data when services are stopping (during logout)
    if (this.isServicesStopping) {
      console.log(`🚫 Blocking getData for ${key} - services are stopping`);
      throw new Error(`Data fetch blocked - services are stopping`);
    }
    
    // Don't fetch data if not authenticated (additional safety check)
    if (authService.isAuthEnabled() && !authService.isAuthenticated()) {
      console.log(`🔒 Blocking getData for ${key} - user not authenticated`);
      throw new Error(`Data fetch blocked - user not authenticated`);
    }

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
    const fetchData = async (options = {}) => {
      try {
        this.setLoading(key, true);
        
        // Check if the method exists
        if (!api[config.method]) {
          throw new Error(`API method '${config.method}' not found`);
        }

        let params = config.params || [];
        
        // Handle IVx data source specially
        if (key === 'ivx') {
          // For IVx, we need to fetch data for all tracked symbols
          const trackedSymbols = this.getTrackedIvxSymbols();
          
          if (trackedSymbols.length === 0) {
            return;
          }
          
          // Fetch IVx data for all tracked symbols
          const allIvxData = await this.fetchIvxForAllSymbols(trackedSymbols);
          
          if (config.updateMethod) {
            config.updateMethod(key, allIvxData);
          } else {
            this.updateData(key, allIvxData);
          }
          
          return;
        }
        
        // Handle other data sources with dynamic parameters
        if (key.includes('.')) {
          const keyParts = key.split('.');
          params = keyParts.slice(1);
        }
        
        const response = await api[config.method](...params);
        
        if (config.updateMethod) {
          config.updateMethod(key, response);
        } else {
          this.updateData(key, response);
        }

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
    const timer = setInterval(() => {
      // Don't fetch data when services are stopping
      if (!this.isServicesStopping) {
        fetchData();
      }
    }, config.interval);
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
    // CRITICAL FIX: Don't fetch data when services are stopping (during logout)
    if (this.isServicesStopping) {
      console.log(`🚫 Blocking data fetch for ${key} - services are stopping`);
      throw new Error(`Data fetch blocked - services are stopping`);
    }
    
    // Don't fetch data if not authenticated (additional safety check)
    if (authService.isAuthEnabled() && !authService.isAuthenticated()) {
      console.log(`🔒 Blocking data fetch for ${key} - user not authenticated`);
      throw new Error(`Data fetch blocked - user not authenticated`);
    }

    const cached = this.cache.get(key);

    // Check cache validity with dynamic TTL for IVx 0DTE
    const effectiveTtl = this.getEffectiveTtl(key, config.ttl, cached);
    if (!forceRefresh && cached && !this.isCacheExpired(cached, effectiveTtl)) {
      return cached.data;
    }
    
    if (config.strategy === "periodic") {
      if (!this.timers.has(key)) {
        this.setupPeriodicUpdate(key, config);
      }
      return this.data.get(key);
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
      // For IVx API, we need response.data (the actual IVx data)
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
   * Get effective TTL based on data type and content
   */
  getEffectiveTtl(key, defaultTtl, cachedData) {
    // For IVx data, use shorter cache for 0DTE expirations
    if (key.startsWith('ivxData.') && cachedData && cachedData.data) {
      const today = new Date().toISOString().split('T')[0]; // YYYY-MM-DD format
      const hasZeroDte = cachedData.data.expirations && 
        cachedData.data.expirations.some(exp => exp.expiration_date === today);
      
      if (hasZeroDte) {
        return 60000; // 1 minute for 0DTE
      }
    }
    return defaultTtl;
  }

  /**
   * Check if cached data is expired
   */
  isCacheExpired(cached, ttl) {
    return Date.now() - cached.timestamp > ttl;
  }

  /**
   * Clear cached data for a specific key
   */
  clearCacheData(key) {
    if (this.cache.has(key)) {
      this.cache.delete(key);
      console.log(`🗑️ Cleared cache for: ${key}`);
      return true;
    }
    return false;
  }

  /**
   * Track access for on-demand strategies
   */
  trackDataAccess(key) {
    // Don't track or fetch data when services are stopping
    if (this.isServicesStopping) {
      return;
    }
    
    let config = this.strategies.get(key);

    // If exact key not found, try wildcard patterns
    if (!config) {
      config = this.findWildcardConfig(key);
    }

    if (config && (config.strategy === "on-demand" || config.strategy === "periodic")) {
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
   * Subscribe to a symbol
   */
  async subscribeToSymbol(symbol) {
    // Don't create subscriptions when services are stopping
    if (this.isServicesStopping) {
      return;
    }
    
    this.activeSubscriptions.add(symbol);

    // For stock symbols, proactively load daily 6M data in background
    // Only if services are running (which means auth is properly initialized and user is authenticated)
    if (symbol.length <= 10 && this.isServicesRunning()) { // Stock symbol
      this.ensureCurrentSymbolDaily6M(symbol).catch(err => {
        console.warn(`⚠️ Background daily 6M load failed for ${symbol}:`, err.message);
      });
      
      // NEW: Preload Overview static data asynchronously
      this.preloadOverviewData(symbol).catch(err => {
        console.warn(`⚠️ Background Overview data load failed for ${symbol}:`, err.message);
      });
    }

    // Schedule backend update
    this.scheduleBackendUpdate();
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

  // ===== Current Symbol Daily 6M Data Management =====

  /**
   * Get 6 months of daily candles for current symbol with live-updated today's candle
   * Returns an object shaped like the API: { bars: [...] }
   */
  async getDaily6MHistory(symbol) {
    if (!symbol) {
      return { bars: [] };
    }

    // Ensure we have data for this symbol
    await this.ensureCurrentSymbolDaily6M(symbol);

    // Wait for loading to complete if still in progress
    if (this.currentSymbolDaily6M.isLoading) {
      // Poll until loading is complete or timeout
      const maxWaitTime = 10000; // 10 seconds max wait
      const pollInterval = 100; // Check every 100ms
      let waitTime = 0;
      
      while (this.currentSymbolDaily6M.isLoading && waitTime < maxWaitTime) {
        await new Promise(resolve => setTimeout(resolve, pollInterval));
        waitTime += pollInterval;
      }
    }

    // Return current data (always up-to-date with live prices)
    return { bars: this.currentSymbolDaily6M.bars };
  }

  /**
   * Ensure current symbol's daily 6M data is loaded and up-to-date
   */
  async ensureCurrentSymbolDaily6M(symbol) {
    // If symbol changed, clear and reload
    if (this.currentSymbolDaily6M.symbol !== symbol) {
      await this.loadCurrentSymbolDaily6M(symbol);
    }
  }

  /**
   * Load 6M daily data for a symbol (background fetch)
   */
  async loadCurrentSymbolDaily6M(symbol) {
    if (!symbol) return;

    // Don't load data if not authenticated
    if (authService.isAuthEnabled() && !authService.isAuthenticated()) {
      console.log("🔒 Skipping daily 6M data load - user not authenticated");
      return;
    }

    // Clear previous data
    this.currentSymbolDaily6M = {
      symbol: symbol,
      bars: [],
      lastUpdated: null,
      providerIdentity: null,
      isLoading: true
    };

    try {
      // Get provider identity for consistency
      const providerIdentity = await this.computeProviderIdentity();
      
      // Fetch 6M baseline data
      const sixMonthsAgo = this.getSixMonthsAgoISO();
      const baseline = await api.getHistoricalData(symbol, "D", {
        start_date: sixMonthsAgo
      });
      
      const baselineBars = baseline && baseline.bars ? baseline.bars : [];
      
      // Update current symbol data
      this.currentSymbolDaily6M = {
        symbol: symbol,
        bars: baselineBars,
        lastUpdated: Date.now(),
        providerIdentity: providerIdentity,
        isLoading: false
      };
      
    } catch (err) {
      console.error(`❌ Failed to load 6M daily data for ${symbol}:`, err);
      this.currentSymbolDaily6M.isLoading = false;
    }
  }

  /**
   * Update today's candle with live price data
   */
  updateTodaysCandleWithPrice(symbol, price) {
    // Only update if this is the current symbol
    if (this.currentSymbolDaily6M.symbol !== symbol || !this.currentSymbolDaily6M.bars.length) {
      return;
    }

    const todayDate = this.formatISODate(new Date());
    const bars = this.currentSymbolDaily6M.bars;
    const lastBarIndex = bars.length - 1;
    
    if (lastBarIndex < 0) return;

    const lastBar = bars[lastBarIndex];
    const lastBarDate = this.extractDateString(lastBar.time);

    if (lastBarDate === todayDate) {
      // Update existing today's candle
      bars[lastBarIndex] = {
        ...lastBar,
        high: Math.max(lastBar.high, price),
        low: Math.min(lastBar.low, price),
        close: price
        // Keep original open and time
      };
    } else {
      // Create new candle for today
      bars.push({
        time: todayDate,
        open: price,
        high: price,
        low: price,
        close: price,
        volume: 0 // Will be updated if volume data available
      });
    }

    this.currentSymbolDaily6M.lastUpdated = Date.now();
  }

  /**
   * Compute provider identity string for consistency checking
   */
  async computeProviderIdentity() {
    try {
      const config = await this.getData("providers.config");
      const routed = config?.service_routing?.historical_data ?? null;
      if (routed) {
        return `historical:${routed}`;
      }
      const compact = {
        service_routing: config?.service_routing || {},
        provider_instances_count: Array.isArray(config?.provider_instances)
          ? config.provider_instances.length
          : 0
      };
      return `config:${JSON.stringify(compact)}`;
    } catch {
      return "config:unknown";
    }
  }

  /**
   * Clear current symbol data
   */
  clearCurrentSymbolDaily6M() {
    this.currentSymbolDaily6M = {
      symbol: null,
      bars: [],
      lastUpdated: null,
      providerIdentity: null,
      isLoading: false
    };
  }

  /**
   * Extract YYYY-MM-DD string from various time formats
   */
  extractDateString(timeVal) {
    if (!timeVal && timeVal !== 0) return null;

    if (typeof timeVal === "string") {
      if (!timeVal.includes(" ")) {
        return timeVal;
      }
      const dt = new Date(timeVal);
      if (!isNaN(dt.getTime())) {
        return this.formatISODate(dt);
      }
      return null;
    }

    if (typeof timeVal === "number") {
      const ms = timeVal < 2e10 ? timeVal * 1000 : timeVal;
      const d = new Date(ms);
      return this.formatISODate(d);
    }

    if (typeof timeVal === "object" && timeVal.year && timeVal.month && timeVal.day) {
      const y = timeVal.year;
      const m = String(timeVal.month).padStart(2, "0");
      const dd = String(timeVal.day).padStart(2, "0");
      return `${y}-${m}-${dd}`;
    }

    return null;
  }

  /**
   * Format date to YYYY-MM-DD
   */
  formatISODate(d) {
    const year = d.getFullYear();
    const month = String(d.getMonth() + 1).padStart(2, "0");
    const day = String(d.getDate()).padStart(2, "0");
    return `${year}-${month}-${day}`;
  }

  /**
   * Six months ago date in YYYY-MM-DD
   */
  getSixMonthsAgoISO() {
    const now = new Date();
    const sixMonthsAgo = new Date(now.getFullYear(), now.getMonth() - 6, now.getDate());
    return this.formatISODate(sixMonthsAgo);
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

      // Provider change invalidates current symbol data
      this.clearCurrentSymbolDaily6M();

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

  // ===== OVERVIEW DATA PRELOADING SYSTEM =====

  /**
   * Preload Overview tab ALL data for a symbol (background fetch)
   * Loads ALL data needed for Overview tab through unified SMART DATA system
   */
  async preloadOverviewData(symbol) {
    if (!symbol) return;

    // Don't load data if not authenticated
    if (authService.isAuthEnabled() && !authService.isAuthenticated()) {
      console.log("🔒 Skipping Overview data preload - user not authenticated");
      return;
    }

    const key = `overviewData.${symbol}`;
    
    // CRITICAL: Clear any existing Overview data for other symbols immediately
    // This prevents showing stale data from previous symbol while new data loads
    this.clearOtherOverviewData(symbol);
    
    try {
      this.setLoading(key, true);      
      // Get a date range to ensure we get the most recent trading day's data
      // instead of just today (which might not have data yet)
      const today = new Date();
      const threeDaysAgo = new Date(today.getTime() - (3 * 24 * 60 * 60 * 1000));
      const threeDaysAgoISO = this.formatISODate(threeDaysAgo);
      
      const [
        previousClose,
        weekRange52,
        averageVolume,
        todaysData,
        expirations
      ] = await Promise.allSettled([
        api.getPreviousClose(symbol),
        api.get52WeekRange(symbol),
        api.getAverageVolume(symbol, 20),
        api.getHistoricalData(symbol, 'D', {
          start_date: threeDaysAgoISO,
          limit: 3  // Get last 3 days to ensure we have recent data
        }),
        api.getAvailableExpirations(symbol)
      ]);
      
      // Extract today's OHLCV data with debugging - get the most recent bar
      let todayBar = null;
      if (todaysData.status === 'fulfilled' && todaysData.value?.bars?.length > 0) {
        // Get the most recent bar (last in the array)
        const bars = todaysData.value.bars;
        todayBar = bars[bars.length - 1];
      } else {
        console.warn(`⚠️ No recent bar data found for ${symbol}:`, todaysData);
      }
      
      // DEBUG: Log the todaysData result to see what's happening
      if (todaysData.status === 'rejected') {
        console.error(`❌ Failed to load recent OHLC data for ${symbol}:`, todaysData.reason);
      }
      
      // Build complete overview data object with ALL data
      const overviewData = {
        symbol,
        // Static data (doesn't change during day)
        previousClose: previousClose.status === 'fulfilled' ? previousClose.value : null,
        weekRange52: weekRange52.status === 'fulfilled' ? weekRange52.value : null,
        averageVolume: averageVolume.status === 'fulfilled' ? averageVolume.value?.average_volume : null,
        expirations: expirations.status === 'fulfilled' ? expirations.value : [],
        
        // Today's data (changes during day, but good for initial load)
        todayData: todayBar ? {
          open: todayBar.open,
          high: todayBar.high,
          low: todayBar.low,
          close: todayBar.close,
          volume: todayBar.volume
        } : null,
        
        // Current price snapshot (will be updated by WebSocket streaming)
        currentPriceSnapshot: null, // Will be updated by WebSocket streaming
        
        // Metadata
        hasOptions: false,
        lastUpdated: Date.now(),
        isLoading: false
      };

      // Determine if symbol has options
      overviewData.hasOptions = overviewData.expirations && overviewData.expirations.length > 0;

      // Load options chain if symbol has options
      if (overviewData.hasOptions && overviewData.expirations.length > 0) {
        try {
          const firstExpiration = overviewData.expirations[0];
          const expirationDate = typeof firstExpiration === 'string' 
            ? firstExpiration 
            : firstExpiration.date || firstExpiration;
          
          const chainData = await api.getOptionsChainBasic(
            symbol, 
            expirationDate, 
            overviewData.currentPriceSnapshot?.price || 0, 
            5
          );
          
          overviewData.optionsChain = chainData;
        } catch (optionsError) {
          console.warn(`⚠️ Could not load options chain for ${symbol}:`, optionsError.message);
          overviewData.optionsChain = null;
        }
      }

      // Store the complete preloaded data
      this.updateData(key, overviewData);
      
    } catch (error) {
      console.error(`❌ Failed to preload Overview data for ${symbol}:`, error);
      this.setError(key, error);
    } finally {
      this.setLoading(key, false);
    }
  }

  /**
   * Clear Overview data AND WebSocket price data for all symbols except the specified one
   * Prevents showing stale data from previous symbol (including bid/ask off-hours)
   */
  clearOtherOverviewData(currentSymbol) {
    const keysToDelete = [];
    const symbolsToDeleteFromWebSocket = [];
    
    // Find all Overview data keys for other symbols
    for (const [key, data] of this.data.entries()) {
      if (key.startsWith('overviewData.') && key !== `overviewData.${currentSymbol}`) {
        keysToDelete.push(key);
      }
    }
    
    // Find all WebSocket price data for other symbols
    for (const symbol of this.stockPrices.keys()) {
      if (symbol !== currentSymbol) {
        symbolsToDeleteFromWebSocket.push(symbol);
      }
    }
    
    for (const symbol of this.optionPrices.keys()) {
      if (symbol !== currentSymbol) {
        symbolsToDeleteFromWebSocket.push(symbol);
      }
    }
    
    // Clear Overview data immediately
    keysToDelete.forEach(key => {
      this.data.delete(key);
      this.cache.delete(key);
      this.errors.delete(key);
    });
    
    // Clear WebSocket price data for other symbols immediately
    symbolsToDeleteFromWebSocket.forEach(symbol => {
      this.stockPrices.delete(symbol);
      this.optionPrices.delete(symbol);
      // Also clear access tracking for these symbols
      this.lastAccess.delete(symbol);
    });
    
    if (keysToDelete.length > 0 || symbolsToDeleteFromWebSocket.length > 0) {
      console.log(`🧹 Cleared Overview data for ${keysToDelete.length} previous symbols and WebSocket data for ${symbolsToDeleteFromWebSocket.length} symbols`);
    }
  }

  /**
   * Get today's opening price for a symbol
   * Fetches today's OHLCV data and extracts the open price
   */
  async getTodaysOpen(symbol) {
    try {
      const today = new Date().toISOString().split('T')[0]; // YYYY-MM-DD format
      const todayData = await api.getHistoricalData(symbol, 'D', {
        start_date: today,
        limit: 1
      });
      
      return todayData?.bars?.[0]?.open || null;
    } catch (error) {
      console.warn(`⚠️ Could not fetch today's open for ${symbol}:`, error.message);
      return null;
    }
  }

  /**
   * Get reactive Overview data for a symbol
   * Returns preloaded static data if available (non-blocking)
   */
  getOverviewData(symbol) {
    if (!symbol) return computed(() => null);

    // Trigger loading immediately and asynchronously (non-blocking)
    this.ensureOverviewDataLoaded(symbol);

    return computed(() => {
      // Return early if services are stopping
      if (this.isServicesStopping) {
        return null;
      }
      
      const key = `overviewData.${symbol}`;
      return this.data.get(key) || null;
    });
  }

  /**
   * Ensure overview data is loaded for a symbol (non-blocking async trigger)
   */
  ensureOverviewDataLoaded(symbol) {
    if (!symbol || this.isServicesStopping || !this.isServicesRunning()) {
      return;
    }

    const key = `overviewData.${symbol}`;
    const existingData = this.data.get(key);
    
    // If no data exists, trigger background loading (fire and forget)
    if (!existingData) {
      // Use setImmediate equivalent to ensure this doesn't block
      setTimeout(() => {
        this.preloadOverviewData(symbol).catch(err => {
          console.warn(`⚠️ Background Overview data load failed for ${symbol}:`, err.message);
        });
      }, 0); // Next tick - completely non-blocking
    }
  }

  /**
   * Refresh Overview data for a symbol (force reload)
   */
  async refreshOverviewData(symbol) {
    if (!symbol) return;
    
    const key = `overviewData.${symbol}`;
    
    // Clear existing data
    this.data.delete(key);
    this.cache.delete(key);
    this.errors.delete(key);
    
    // Reload data
    return await this.preloadOverviewData(symbol);
  }

  /**
   * Set up periodic refresh for Overview data (5 minutes, static data only)
   * Only refreshes data for symbols that have been preloaded
   */
  setupOverviewDataPeriodicRefresh() {
    // Set up periodic refresh every 5 minutes
    const overviewRefreshTimer = setInterval(() => {
      // Don't refresh when services are stopping
      if (this.isServicesStopping) {
        return;
      }
      
      this.refreshActiveOverviewData();
    }, 300000); // 5 minutes

    // Store timer for cleanup
    this.timers.set('overviewDataRefresh', overviewRefreshTimer);
    
  }

  /**
   * Refresh Overview data for all symbols that have preloaded data
   * Only updates static data - never interferes with live streams
   */
  async refreshActiveOverviewData() {
    const activeOverviewSymbols = [];
    
    // Find all symbols with preloaded Overview data
    for (const [key, data] of this.data.entries()) {
      if (key.startsWith('overviewData.') && data && data.symbol) {
        activeOverviewSymbols.push(data.symbol);
      }
    }
    
    if (activeOverviewSymbols.length === 0) {
      return; // No symbols to refresh
    }
    
    console.log(`📊 Refreshing Overview data for ${activeOverviewSymbols.length} symbols...`);
    
    // Refresh each symbol's static data
    for (const symbol of activeOverviewSymbols) {
      try {
        const key = `overviewData.${symbol}`;
        const existingData = this.data.get(key);
        
        if (!existingData) continue;
        
        // Load updated static data in parallel (same as preload, but update existing)
        const [
          weekRange52,
          averageVolume
        ] = await Promise.allSettled([
          api.get52WeekRange(symbol),
          api.getAverageVolume(symbol, 20)
        ]);

        // Update only the data that can change (52-week range, average volume)
        // Previous close and today's open don't change during trading day
        const updatedData = {
          ...existingData,
          weekRange52: weekRange52.status === 'fulfilled' ? weekRange52.value : existingData.weekRange52,
          averageVolume: averageVolume.status === 'fulfilled' ? averageVolume.value?.average_volume : existingData.averageVolume,
          lastUpdated: Date.now()
        };

        // Store the updated data
        this.updateData(key, updatedData);
        
      } catch (error) {
        console.warn(`⚠️ Failed to refresh Overview data for ${symbol}:`, error.message);
      }
    }
    
    console.log(`✅ Overview data refresh completed for ${activeOverviewSymbols.length} symbols`);
  }

  /**
   * Set up authentication integration
   * Monitors auth state and controls service initialization
   */
  setupAuthenticationIntegration() {
    // Listen for authentication state changes
    const authStateListener = async (authState) => {
      const wasAuthenticated = this.isAuthenticated;
      this.isAuthenticated = authState.authenticated;

      if (this.isAuthenticated && !wasAuthenticated) {
        // User just logged in - start services
        this.startServices();
      } else if (!this.isAuthenticated && wasAuthenticated) {
        // User just logged out - stop services
        await this.stopServices();
      }
    };

    // Add listener to auth service
    authService.addListener(authStateListener);
    this.authStateListeners.push(authStateListener);
  }

  /**
   * Initialize services conditionally based on authentication
   */
  async initializeConditionally() {
    try {
      // Initialize auth service first
      const authInitSuccess = await authService.init();
      
      // Only proceed if auth service initialized successfully
      if (!authInitSuccess) {
        console.log("🔐 Authentication service failed to initialize, services will not start");
        return;
      }
      
      // Check if auth is enabled and user is authenticated
      if (authService.isAuthEnabled()) {
        this.isAuthenticated = authService.isAuthenticated();
        
        if (this.isAuthenticated) {
          this.startServices();
        } else {
          console.log("🔐 User not authenticated, services will start after login");
        }
      } else {
        // Auth is disabled, start services immediately
        console.log("🔐 Authentication disabled, starting services");
        this.isAuthenticated = true; // Treat as authenticated when auth is disabled
        this.startServices();
      }
    } catch (error) {
      console.error("❌ Failed to initialize authentication integration:", error);
      // Do not start services if authentication initialization fails
      console.log("🔐 Authentication initialization failed, services will not start");
    }
  }

  /**
   * Start all services (called when authenticated or auth disabled)
   */
  startServices() {
    if (this.isInitialized) {
      return;
    }

    // Set service state flags
    this.isServicesStopping = false;
    
    // Reset the global API blocking flag synchronously
    setServicesStoppingState(false);

    // Initialize WebSocket integration
    this.initialize();

    // Start keep-alive system
    this.startKeepaliveTimer();

    // Start Greeks periodic updates
    this.startGreeksPeriodicUpdates();

    // Register provider configuration data sources
    this.setupProviderDataSources();

    // Register positions data source
    this.setupPositionsDataSource();

    // Configure additional data sources (moved from main.js)
    this.configureAdditionalDataSources();

    // Start health monitoring
    this.startHealthMonitoring();

    // Listen for system recovery events
    this.setupRecoveryListeners();

    // CRITICAL: Update backend subscriptions for any symbols that were added before authentication
    if (this.activeSubscriptions.size > 0) {
      this.scheduleBackendUpdate();
    }
    
  }

  /**
   * Stop all services (called when user logs out)
   */
  async stopServices() {
    console.log("🛑 Stopping SmartMarketDataStore services");

    // CRITICAL FIX: Set stopping flag FIRST to prevent new data fetching
    this.isServicesStopping = true;
    
    // Also set the global API blocking flag synchronously
    setServicesStoppingState(true);

    // Disconnect WebSocket
    webSocketClient.disconnect();

    // Clear all timers FIRST to prevent any new data requests
    if (this.updateTimer) {
      clearTimeout(this.updateTimer);
      this.updateTimer = null;
    }
    if (this.greeksUpdateTimer) {
      clearInterval(this.greeksUpdateTimer);
      this.greeksUpdateTimer = null;
    }
    if (this.greeksImmediateUpdateTimer) {
      clearTimeout(this.greeksImmediateUpdateTimer);
      this.greeksImmediateUpdateTimer = null;
    }

    // Clear periodic timers
    this.timers.forEach((timer) => clearInterval(timer));
    this.timers.clear();

    // Stop health monitoring
    if (this.healthMonitor) {
      this.healthMonitor.stop();
    }

    // Clear all subscriptions and data
    this.activeSubscriptions.clear();
    this.activeGreeksSubscriptions.clear();
    this.lastAccess.clear();
    this.lastGreeksAccess.clear();
    
    // Wait for next tick to ensure blocking flags are in effect before clearing reactive data
    await nextTick();
    
    // Clear all reactive data (this might trigger Vue reactivity, but we've blocked fetching)
    this.stockPrices.clear();
    this.optionPrices.clear();
    this.optionGreeks.clear();
    this.previousClosePrices.clear();
    this.data.clear();
    this.cache.clear();
    this.loading.clear();
    this.errors.clear();

    // Clear current symbol data
    this.clearCurrentSymbolDaily6M();

    // Clear component registrations
    this.symbolUsageCount.clear();
    this.componentRegistrations.clear();

    // Reset initialization flag
    this.isInitialized = false;

    console.log("✅ SmartMarketDataStore services stopped");
    
    // Reset stopping flag after cleanup is complete
    setTimeout(() => {
      this.isServicesStopping = false;
      
      // Also reset the global API blocking flag synchronously  
      setServicesStoppingState(false);
    }, 1000); // Allow 1 second for any pending operations to complete
  }

  /**
   * Configure additional data sources (moved from main.js)
   * Called when services start to ensure data sources are configured only when authenticated
   */
  configureAdditionalDataSources() {
    // Auto-updating data (Periodic strategy)
    this.registerDataSource("balance", {
      strategy: "periodic",
      method: "getAccount",
      interval: 60000, // 1 minute
    });

    // Static data (One-time strategy)
    this.registerDataSource("accountInfo", {
      strategy: "once",
      method: "getAccount",
    });

    // On-demand data (Cached strategy)
    this.registerDataSource("optionsChain.*", {
      strategy: "on-demand",
      method: "getOptionsChain",
      ttl: 300000, // 5 minutes
    });

    this.registerDataSource("historicalData.*", {
      strategy: "on-demand",
      method: "getHistoricalData",
      ttl: 300000, // 5 minutes
    });

    this.registerDataSource("symbolLookup.*", {
      strategy: "on-demand",
      method: "lookupSymbols",
      ttl: 600000, // 10 minutes
    });

    this.registerDataSource("expirationDates.*", {
      strategy: "on-demand",
      method: "getAvailableExpirations",
      ttl: 3600000, // 1 hour
    });

    this.registerDataSource("ivxData.*", {
      strategy: "on-demand",
      method: "getIvxData",
      ttl: 300000, // 5 minutes (same as backend cache)
    });

    // Overview data periodic refresh (5 minutes, static data only)
    this.setupOverviewDataPeriodicRefresh();
  }

  /**
   * Check if services are running (for debugging)
   */
  isServicesRunning() {
    return this.isInitialized && this.isAuthenticated && !this.isServicesStopping;
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

    // Clear current symbol data
    this.clearCurrentSymbolDaily6M();

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
