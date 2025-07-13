import axios from "axios";

const API_BASE_URL = "/api";

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
    try {
      const response = await axios.get(`${API_BASE_URL}/expiration_dates`, {
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
    } catch (error) {
      console.error("Error fetching available expirations:", error);
      return null;
    }
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

  // Get options chain
  async getOptionsChain(symbol, expiry, strategyType) {
    try {
      const response = await axios.get(`${API_BASE_URL}/options_chain`, {
        params: {
          symbol,
          expiry: expiry,
          strategy_type: strategyType,
        },
      });
      return response.data.data;
    } catch (error) {
      console.error("Error fetching options chain:", error);
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

  // Place multi-leg order (updated to use correct backend endpoint)
  async placeButterflyOrder(orderPayload) {
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
};

export default api;
