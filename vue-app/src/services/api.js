import axios from "axios";

const API_BASE_URL = "/api";

export const api = {
  // Get next market date
  async getNextMarketDate() {
    try {
      const response = await axios.get(`${API_BASE_URL}/next_market_date`);
      return response.data.next_market_date;
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
      const data = response.data;
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
      return response.data;
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
      return response.data;
    } catch (error) {
      console.error("Error fetching option prices:", error);
      throw error;
    }
  },

  // Place butterfly order
  async placeButterflyOrder(orderPayload) {
    try {
      const response = await axios.post(
        `${API_BASE_URL}/place_butterfly_order`,
        orderPayload
      );
      return response.data;
    } catch (error) {
      console.error("Error placing butterfly order:", error);
      throw error;
    }
  },

  // Get current open positions
  async getPositions() {
    try {
      const response = await axios.get(`${API_BASE_URL}/positions`);
      return response.data;
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
      return response.data;
    } catch (error) {
      console.error("Error calculating adjustments:", error);
      throw error;
    }
  },
};

export default api;
