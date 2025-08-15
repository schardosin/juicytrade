import axios from "axios";

const API_BASE_URL = "/api";

// Circuit Breaker implementation for API resilience
class APICircuitBreaker {
  constructor(failureThreshold = 5, recoveryTimeout = 30000) {
    this.failureCount = 0;
    this.failureThreshold = failureThreshold;
    this.recoveryTimeout = recoveryTimeout;
    this.state = 'CLOSED'; // CLOSED, OPEN, HALF_OPEN
    this.nextAttempt = Date.now();
    this.name = 'API_CIRCUIT_BREAKER';
  }

  async execute(apiCall) {
    if (this.state === 'OPEN') {
      if (Date.now() < this.nextAttempt) {
        throw new Error(`Circuit breaker is OPEN. Next attempt in ${Math.ceil((this.nextAttempt - Date.now()) / 1000)}s`);
      }
      this.state = 'HALF_OPEN';
    }

    try {
      const result = await apiCall();
      this.onSuccess();
      return result;
    } catch (error) {
      this.onFailure();
      throw error;
    }
  }

  onSuccess() {
    this.failureCount = 0;
    this.state = 'CLOSED';
  }

  onFailure() {
    this.failureCount++;
    if (this.failureCount >= this.failureThreshold) {
      this.state = 'OPEN';
      this.nextAttempt = Date.now() + this.recoveryTimeout;
      console.warn(`🔴 Circuit breaker opened after ${this.failureCount} failures. Recovery in ${this.recoveryTimeout/1000}s`);
    }
  }

  getStatus() {
    return {
      state: this.state,
      failureCount: this.failureCount,
      nextAttempt: this.nextAttempt
    };
  }
}

// Retry wrapper with exponential backoff
async function withRetry(apiCall, options = {}) {
  const {
    maxRetries = 3,
    baseDelay = 1000,
    maxDelay = 10000,
    backoffFactor = 2,
    retryCondition = (error) => error.code !== 'ECONNABORTED' && (!error.response || error.response.status >= 500)
  } = options;

  let lastError;
  
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      return await apiCall();
    } catch (error) {
      lastError = error;
      
      // Don't retry if condition is not met
      if (!retryCondition(error)) {
        throw error;
      }
      
      // Don't retry on last attempt
      if (attempt === maxRetries) {
        break;
      }
      
      const delay = Math.min(baseDelay * Math.pow(backoffFactor, attempt - 1), maxDelay);
      console.warn(`🔄 API call failed (attempt ${attempt}/${maxRetries}), retrying in ${delay}ms:`, error.message);
      
      await new Promise(resolve => setTimeout(resolve, delay));
    }
  }
  
  throw lastError;
}

// Global circuit breaker instance
const circuitBreaker = new APICircuitBreaker();

// Enhanced axios instance with timeout and retry configuration
const apiClient = axios.create({
  timeout: 15000, // 15 second timeout
  headers: {
    'Content-Type': 'application/json',
  }
});

// Request interceptor for logging and monitoring
apiClient.interceptors.request.use(
  (config) => {
    config.metadata = { startTime: Date.now() };
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor for error handling and monitoring
apiClient.interceptors.response.use(
  (response) => {
    const duration = Date.now() - response.config.metadata.startTime;
    if (duration > 5000) {
      console.warn(`⚠️ Slow API response: ${response.config.url} took ${duration}ms`);
    }
    return response;
  },
  (error) => {
    const duration = error.config?.metadata ? Date.now() - error.config.metadata.startTime : 0;
    console.error(`❌ API Error: ${error.config?.url} (${duration}ms):`, error.message);
    return Promise.reject(error);
  }
);

export const api = {
  // Get next market date
  async getNextMarketDate() {
    try {
      const response = await axios.get(`${API_BASE_URL}/next_market_date`);
      return response.data.data.next_market_date;
    } catch (error) {
      console.error("Error fetching next market date:", error);
      throw error;
    }
  },

  // Get underlying stock price
  async getUnderlyingPrice(symbol) {
    try {
      const response = await axios.get(`${API_BASE_URL}/prices/stocks`, {
        params: { symbols: symbol },
      });
      const data = response.data.data;
      if (symbol in data) {
        const priceData = data[symbol];
        if (typeof priceData === "object" && priceData !== null) {
          const { ask, bid } = priceData;
          if (ask !== null && bid !== null) {
            return (ask + bid) / 2;
          } else if (ask !== null) {
            return ask;
          } else if (bid !== null) {
            return bid;
          }
        } else if (typeof priceData === "number") {
          return priceData;
        }
      }
      return null;
    } catch (error) {
      console.error("Error fetching underlying price:", error);
      throw error;
    }
  },

  // Get available expiration dates for a symbol from options contracts
  async getAvailableExpirations(symbol) {
    return await circuitBreaker.execute(async () => {
      return await withRetry(async () => {
        const response = await apiClient.get(`${API_BASE_URL}/expiration_dates`, {
          params: { symbol },
        });

        if (
          response.data &&
          response.data.success &&
          response.data.data &&
          response.data.data.expiration_dates
        ) {
          return response.data.data.expiration_dates;
        }

        return null;
      }, {
        maxRetries: 3,
        baseDelay: 1000,
        retryCondition: (error) => {
          // Retry on network errors and 5xx server errors
          return !error.response || error.response.status >= 500 || error.code === 'ECONNRESET';
        }
      });
    }).catch(error => {
      console.error("Error fetching available expirations:", error);
      return null; // Return null to trigger fallback behavior
    });
  },

  // Get expiration dates for a symbol (legacy method)
  async getExpirationDates(symbol) {
    try {
      const response = await axios.get(`${API_BASE_URL}/expiration_dates`, {
        params: { symbol },
      });
      return response.data.data;
    } catch (error) {
      console.error("Error fetching expiration dates:", error);
      // Return null to trigger fallback in the component
      return null;
    }
  },

  // Get basic options chain (fast loading, no Greeks)
  async getOptionsChainBasic(
    symbol,
    expiry,
    underlyingPrice = null,
    strikeCount = 20,
    type = null,
    underlyingSymbol = null
  ) {
    return await circuitBreaker.execute(async () => {
      return await withRetry(async () => {
        const params = {
          symbol,
          expiry,
          strike_count: strikeCount,
        };

        if (underlyingPrice !== null) {
          params.underlying_price = underlyingPrice;
        }

        if (type !== null) {
          params.type = type;
        }

        if (underlyingSymbol !== null) {
          params.underlying_symbol = underlyingSymbol;
        }

        const response = await apiClient.get(`${API_BASE_URL}/options_chain_basic`, {
          params,
        });
        return response.data.data;
      }, {
        maxRetries: 2,
        baseDelay: 1500,
        retryCondition: (error) => {
          return !error.response || error.response.status >= 500 || error.code === 'ECONNRESET';
        }
      });
    });
  },

  // Get Greeks for multiple option symbols
  async getOptionsGreeks(symbolsArray) {
    try {
      const symbols = Array.isArray(symbolsArray)
        ? symbolsArray.join(",")
        : symbolsArray;
      const response = await axios.get(`${API_BASE_URL}/options_greeks`, {
        params: { symbols },
      });
      return response.data.data;
    } catch (error) {
      console.error("Error fetching options Greeks:", error);
      throw error;
    }
  },

  // Get smart options chain with configurable loading
  async getOptionsChainSmart(symbol, expiry, options = {}) {
    try {
      const params = {
        symbol,
        expiry,
        underlying_price: options.underlyingPrice,
        atm_range: options.atmRange || 20,
        include_greeks: options.includeGreeks || false,
        strikes_only: options.strikesOnly || false,
      };

      const response = await axios.get(`${API_BASE_URL}/options_chain_smart`, {
        params,
      });
      return response.data.data;
    } catch (error) {
      console.error("Error fetching smart options chain:", error);
      throw error;
    }
  },

  // Fetch option prices
  async fetchOptionPrices(symbols) {
    if (!symbols || symbols.length === 0) {
      return {};
    }
    try {
      const response = await axios.get(`${API_BASE_URL}/prices/options`, {
        params: { symbols: symbols.join(",") },
      });
      return response.data.data;
    } catch (error) {
      console.error("Error fetching option prices:", error);
      throw error;
    }
  },

  // Preview order to get cost estimates and validation
  async previewOrder(orderPayload) {
    try {
      const response = await axios.post(
        `${API_BASE_URL}/orders/preview`,
        orderPayload
      );
      return response.data;
    } catch (error) {
      console.error("Error previewing order:", error);
      throw error;
    }
  },

  // Place single-leg option order
  async placeSingleLegOrder(orderPayload) {
    try {
      const response = await axios.post(
        `${API_BASE_URL}/orders/single-leg`,
        orderPayload
      );
      return response.data;
    } catch (error) {
      console.error("Error placing single-leg order:", error);
      throw error;
    }
  },

  // Place multi-leg order (renamed from placeButterflyOrder for clarity)
  async placeMultiLegOrder(orderPayload) {
    try {
      const response = await axios.post(
        `${API_BASE_URL}/orders/multi-leg`,
        orderPayload
      );
      return response.data;
    } catch (error) {
      console.error("Error placing multi-leg order:", error);
      throw error;
    }
  },

  // Deprecated: Use placeMultiLegOrder instead
  async placeButterflyOrder(orderPayload) {
    console.warn('placeButterflyOrder is deprecated, use placeMultiLegOrder instead');
    return this.placeMultiLegOrder(orderPayload);
  },

  // Get current open positions
  async getPositions() {
    try {
      const response = await axios.get(`${API_BASE_URL}/positions`);
      return response.data.data;
    } catch (error) {
      console.error("Error fetching positions:", error);
      throw error;
    }
  },

  // Calculate adjustment suggestions
  async calculateAdjustments(requestData) {
    try {
      const response = await axios.post(
        `${API_BASE_URL}/calculate_adjustments`,
        requestData
      );
      return response.data.data;
    } catch (error) {
      console.error("Error calculating adjustments:", error);
      throw error;
    }
  },

  // Symbol lookup
  async lookupSymbols(query) {
    try {
      const response = await axios.get(`${API_BASE_URL}/symbols/lookup`, {
        params: { q: query },
      });
      return response.data.data.symbols;
    } catch (error) {
      console.error("Error looking up symbols:", error);
      throw error;
    }
  },

  // Get current subscription status
  async getSubscriptionStatus() {
    try {
      const response = await axios.get(`${API_BASE_URL}/subscriptions/status`);
      return response.data.data;
    } catch (error) {
      console.error("Error getting subscription status:", error);
      throw error;
    }
  },

  // Get account information including balance and buying power
  async getAccount() {
    try {
      const response = await axios.get(`${API_BASE_URL}/account`);
      return response.data.data;
    } catch (error) {
      console.error("Error fetching account information:", error);
      throw error;
    }
  },

  // Get orders with optional status filter
  async getOrders(status = "all") {
    try {
      const response = await axios.get(`${API_BASE_URL}/orders`, {
        params: { status },
      });
      return response.data.data;
    } catch (error) {
      console.error("Error fetching orders:", error);
      throw error;
    }
  },

  // Cancel an order
  async cancelOrder(orderId) {
    try {
      const response = await axios.delete(`${API_BASE_URL}/orders/${orderId}`);
      return response.data;
    } catch (error) {
      console.error("Error cancelling order:", error);
      throw error;
    }
  },

  // Get historical data (for LightweightChart and other components)
  async getHistoricalData(symbol, timeframe, options = {}) {
    try {
      const params = {
        timeframe,
        limit: options.limit,
        start_date: options.start_date,
        end_date: options.end_date,
      };

      const response = await axios.get(
        `${API_BASE_URL}/chart/historical/${symbol}`,
        {
          params,
        }
      );
      return response.data.data;
    } catch (error) {
      console.error("Error fetching historical data:", error);
      throw error;
    }
  },

  // Get previous day's close price
  async getPreviousClose(symbol) {
    try {
      const today = new Date();
      const previousDay = new Date(today);
      previousDay.setDate(today.getDate() - 1);
      const endDate = previousDay.toISOString().split('T')[0];
      
      const response = await axios.get(`${API_BASE_URL}/chart/historical/${symbol}`, {
        params: {
          timeframe: 'D',
          limit: 1,
          end_date: endDate
        }
      });
      
      return response.data.data.bars[0].close;
    } catch (error) {
      console.error("Error fetching previous close:", error);
      throw error;
    }
  },

  // Get available providers and their capabilities
  async getAvailableProviders() {
    try {
      const response = await axios.get(`${API_BASE_URL}/providers/available`);
      return response.data.data;
    } catch (error) {
      console.error("Error fetching available providers:", error);
      throw error;
    }
  },

  // Get current provider configuration
  async getProviderConfig() {
    try {
      const response = await axios.get(`${API_BASE_URL}/providers/config`);
      return response.data.data;
    } catch (error) {
      console.error("Error fetching provider configuration:", error);
      throw error;
    }
  },

  // Update provider configuration
  async updateProviderConfig(config) {
    try {
      const response = await axios.put(`${API_BASE_URL}/providers/config`, config);
      return response.data;
    } catch (error) {
      console.error("Error updating provider configuration:", error);
      throw error;
    }
  },

  // === Provider Instance Management APIs ===

  // Get provider types and their field definitions
  async getProviderTypes() {
    try {
      const response = await axios.get(`${API_BASE_URL}/providers/types`);
      return response.data.data;
    } catch (error) {
      console.error("Error fetching provider types:", error);
      throw error;
    }
  },

  // Get all provider instances
  async getProviderInstances() {
    try {
      const response = await axios.get(`${API_BASE_URL}/providers/instances`);
      return response.data.data;
    } catch (error) {
      console.error("Error fetching provider instances:", error);
      throw error;
    }
  },

  // Create a new provider instance
  async createProviderInstance(instanceData) {
    try {
      const response = await axios.post(`${API_BASE_URL}/providers/instances`, instanceData);
      return response.data;
    } catch (error) {
      console.error("Error creating provider instance:", error);
      throw error;
    }
  },

  // Update an existing provider instance
  async updateProviderInstance(instanceId, updateData) {
    try {
      const response = await axios.put(`${API_BASE_URL}/providers/instances/${instanceId}`, updateData);
      return response.data;
    } catch (error) {
      console.error("Error updating provider instance:", error);
      throw error;
    }
  },

  // Toggle provider instance active/inactive
  async toggleProviderInstance(instanceId) {
    try {
      const response = await axios.put(`${API_BASE_URL}/providers/instances/${instanceId}/toggle`);
      return response.data;
    } catch (error) {
      console.error("Error toggling provider instance:", error);
      throw error;
    }
  },

  // Delete a provider instance
  async deleteProviderInstance(instanceId) {
    try {
      const response = await axios.delete(`${API_BASE_URL}/providers/instances/${instanceId}`);
      return response.data;
    } catch (error) {
      console.error("Error deleting provider instance:", error);
      throw error;
    }
  },

  // Test provider connection without saving
  async testProviderConnection(connectionData) {
    try {
      const response = await axios.post(`${API_BASE_URL}/providers/instances/test`, connectionData);
      return response.data;
    } catch (error) {
      console.error("Error testing provider connection:", error);
      throw error;
    }
  },

  // === Watchlist Management APIs ===

  // Get all watchlists
  async getWatchlists() {
    try {
      const response = await axios.get(`${API_BASE_URL}/watchlists`);
      return response.data.data;
    } catch (error) {
      console.error("Error fetching watchlists:", error);
      throw error;
    }
  },

  // Get a specific watchlist by ID
  async getWatchlist(watchlistId) {
    try {
      const response = await axios.get(`${API_BASE_URL}/watchlists/${watchlistId}`);
      return response.data.data;
    } catch (error) {
      console.error("Error fetching watchlist:", error);
      throw error;
    }
  },

  // Create a new watchlist
  async createWatchlist(watchlistData) {
    try {
      const response = await axios.post(`${API_BASE_URL}/watchlists`, watchlistData);
      return response.data.data;
    } catch (error) {
      console.error("Error creating watchlist:", error);
      throw error;
    }
  },

  // Update an existing watchlist
  async updateWatchlist(watchlistId, updateData) {
    try {
      const response = await axios.put(`${API_BASE_URL}/watchlists/${watchlistId}`, updateData);
      return response.data.data;
    } catch (error) {
      console.error("Error updating watchlist:", error);
      throw error;
    }
  },

  // Delete a watchlist
  async deleteWatchlist(watchlistId) {
    try {
      const response = await axios.delete(`${API_BASE_URL}/watchlists/${watchlistId}`);
      return response.data;
    } catch (error) {
      console.error("Error deleting watchlist:", error);
      throw error;
    }
  },

  // Add a symbol to a watchlist
  async addSymbolToWatchlist(watchlistId, symbol) {
    try {
      const response = await axios.post(`${API_BASE_URL}/watchlists/${watchlistId}/symbols`, {
        symbol: symbol
      });
      return response.data.data;
    } catch (error) {
      console.error("Error adding symbol to watchlist:", error);
      throw error;
    }
  },

  // Remove a symbol from a watchlist
  async removeSymbolFromWatchlist(watchlistId, symbol) {
    try {
      const response = await axios.delete(`${API_BASE_URL}/watchlists/${watchlistId}/symbols/${symbol}`);
      return response.data.data;
    } catch (error) {
      console.error("Error removing symbol from watchlist:", error);
      throw error;
    }
  },

  // Get the active watchlist
  async getActiveWatchlist() {
    try {
      const response = await axios.get(`${API_BASE_URL}/watchlists/active`);
      return response.data.data;
    } catch (error) {
      console.error("Error fetching active watchlist:", error);
      throw error;
    }
  },

  // Set the active watchlist
  async setActiveWatchlist(watchlistId) {
    try {
      const response = await axios.put(`${API_BASE_URL}/watchlists/active`, {
        watchlist_id: watchlistId
      });
      return response.data.data;
    } catch (error) {
      console.error("Error setting active watchlist:", error);
      throw error;
    }
  },

  // Search watchlists by name or symbols
  async searchWatchlists(query) {
    try {
      const response = await axios.post(`${API_BASE_URL}/watchlists/search`, {
        query: query
      });
      return response.data.data;
    } catch (error) {
      console.error("Error searching watchlists:", error);
      throw error;
    }
  },

  // Get all unique symbols across all watchlists
  async getAllWatchlistSymbols() {
    try {
      const response = await axios.get(`${API_BASE_URL}/watchlists/symbols/all`);
      return response.data.data;
    } catch (error) {
      console.error("Error fetching all watchlist symbols:", error);
      throw error;
    }
  },
};

export default api;
