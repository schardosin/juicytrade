import axios from "axios";

class StreamingClient {
  constructor(baseUrl = "/api") {
    this.baseUrl = baseUrl;
    this.optionCallbacks = new Map();
    this.stockCallbacks = new Map();
    this.pollingInterval = null;
    this.isPolling = false;
    this.pollIntervalMs = 2000; // Reduced to 2 seconds to prevent browser crashes
  }

  async checkServiceStatus() {
    try {
      const response = await axios.get(`${this.baseUrl}/`);
      return response.data;
    } catch (error) {
      console.error("Error checking service status:", error);
      return null;
    }
  }

  async restartStreaming() {
    try {
      const response = await axios.post(`${this.baseUrl}/restart`);
      return response.status === 200;
    } catch (error) {
      console.error("Error restarting streaming service:", error);
      return false;
    }
  }

  subscribeOption(symbol, callback) {
    if (!this.optionCallbacks.has(symbol)) {
      this.optionCallbacks.set(symbol, []);
    }
    this.optionCallbacks.get(symbol).push(callback);

    // Start polling if not already running
    if (!this.isPolling) {
      this.startPolling();
    }

    return true;
  }

  subscribeStock(symbol, callback) {
    if (!this.stockCallbacks.has(symbol)) {
      this.stockCallbacks.set(symbol, []);
    }
    this.stockCallbacks.get(symbol).push(callback);

    // Start polling if not already running
    if (!this.isPolling) {
      this.startPolling();
    }

    return true;
  }

  startPolling() {
    if (this.isPolling) return;

    this.isPolling = true;
    this.pollingInterval = setInterval(() => {
      this.pollPrices();
    }, this.pollIntervalMs);

    console.log("Started polling for price updates");
  }

  stopPolling() {
    if (this.pollingInterval) {
      clearInterval(this.pollingInterval);
      this.pollingInterval = null;
    }
    this.isPolling = false;
    console.log("Stopped polling for price updates");
  }

  async pollPrices() {
    try {
      // Poll option prices
      const optionSymbols = Array.from(this.optionCallbacks.keys());
      if (optionSymbols.length > 0) {
        const optionResponse = await axios.get(
          `${this.baseUrl}/prices/options`,
          {
            params: { symbols: optionSymbols.join(",") },
          }
        );

        for (const [symbol, priceData] of Object.entries(optionResponse.data)) {
          if (this.optionCallbacks.has(symbol) && priceData.ask !== null) {
            const callbacks = this.optionCallbacks.get(symbol);
            callbacks.forEach((callback) => {
              try {
                callback(symbol, priceData);
              } catch (error) {
                console.error(`Error in option callback for ${symbol}:`, error);
              }
            });
          }
        }
      }

      // Poll stock prices
      const stockSymbols = Array.from(this.stockCallbacks.keys());
      if (stockSymbols.length > 0) {
        const stockResponse = await axios.get(`${this.baseUrl}/prices/stocks`, {
          params: { symbols: stockSymbols.join(",") },
        });

        for (const [symbol, priceData] of Object.entries(stockResponse.data)) {
          if (this.stockCallbacks.has(symbol) && priceData.ask !== null) {
            const callbacks = this.stockCallbacks.get(symbol);
            callbacks.forEach((callback) => {
              try {
                callback(symbol, priceData);
              } catch (error) {
                console.error(`Error in stock callback for ${symbol}:`, error);
              }
            });
          }
        }
      }
    } catch (error) {
      console.error("Error polling prices:", error);
    }
  }

  unsubscribeOption(symbol, callback = null) {
    if (this.optionCallbacks.has(symbol)) {
      if (callback) {
        const callbacks = this.optionCallbacks.get(symbol);
        const index = callbacks.indexOf(callback);
        if (index > -1) {
          callbacks.splice(index, 1);
        }
        if (callbacks.length === 0) {
          this.optionCallbacks.delete(symbol);
        }
      } else {
        this.optionCallbacks.delete(symbol);
      }
    }
  }

  unsubscribeStock(symbol, callback = null) {
    if (this.stockCallbacks.has(symbol)) {
      if (callback) {
        const callbacks = this.stockCallbacks.get(symbol);
        const index = callbacks.indexOf(callback);
        if (index > -1) {
          callbacks.splice(index, 1);
        }
        if (callbacks.length === 0) {
          this.stockCallbacks.delete(symbol);
        }
      } else {
        this.stockCallbacks.delete(symbol);
      }
    }
  }
}

export default new StreamingClient();
