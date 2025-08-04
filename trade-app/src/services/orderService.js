import api from "./api";

/**
 * Centralized Order Management Service
 * Handles all order-related operations across the application
 */
class OrderService {
  /**
   * Place an order with standardized payload structure
   * @param {Object} orderData - Order configuration
   * @returns {Promise<Object>} Order result
   */
  async placeOrder(orderData) {
    try {
      const orderPayload = this.buildOrderPayload(orderData);
      console.log("OrderService: Placing order with payload:", orderPayload);

      const result = await api.placeButterflyOrder(orderPayload);

      if (result.success) {
        console.log("OrderService: Order placed successfully:", result);
        return {
          success: true,
          order: result.order,
          message: `Order submitted successfully! Order ID: ${
            result.order?.id || "N/A"
          }`,
        };
      } else {
        console.error("OrderService: Order failed:", result.error);
        return {
          success: false,
          error: result.error || "Unknown error",
          message: `Order failed: ${result.error || "Unknown error"}`,
        };
      }
    } catch (error) {
      console.error("OrderService: Order submission error:", error);
      return {
        success: false,
        error: error.message,
        message: `Order submission failed: ${error.message}`,
      };
    }
  }

  /**
   * Build standardized order payload
   * @param {Object} orderData - Order configuration
   * @returns {Object} Formatted order payload
   */
  buildOrderPayload(orderData) {
    const {
      symbol,
      expiry,
      strategyType,
      legs,
      limitPrice,
      orderPrice,
      orderOffset = 0,
      quantity = 1,
      timeInForce = "day",
      orderType = "limit",
    } = orderData;

    // Use limitPrice if available, otherwise fall back to orderPrice
    // FIXED: Trust the UI to send correctly signed values - no more sign correction
    const price = limitPrice !== undefined ? limitPrice : orderPrice;

    return {
      legs: this.formatLegs(legs),
      order_type: orderType.toLowerCase(),
      time_in_force: timeInForce.toLowerCase(),
      limit_price: price, // Backend expects limit_price, not order_price
    };
  }

  /**
   * Determine if an order is a credit order (we receive money)
   * @param {Object} orderData - Order configuration
   * @returns {boolean} True if credit order, false if debit order
   */
  isCreditOrder(orderData) {
    const { netPremium, limitPrice } = orderData;

    // Primary: Use netPremium if available (consistent with UI logic)
    // Note: netPremium <= 0 means credit (we receive money)
    if (netPremium !== undefined) {
      return netPremium <= 0; // FIXED: Consistent with UI logic
    }

    // Secondary: Check limitPrice (negative means we receive money)
    if (limitPrice !== undefined) {
      return limitPrice < 0;
    }

    // Default to debit if we can't determine
    return false;
  }
  /**
   * Format legs for API payload
   * @param {Array} legs - Order legs
   * @returns {Array} Formatted legs
   */
  formatLegs(legs) {
    return legs.map((leg) => ({
      symbol: leg.symbol,
      side: this.formatSide(leg.side),
      qty: parseInt(leg.ratio_qty || leg.quantity || 1),
    }));
  }

  /**
   * Format side for API (convert to backend format)
   * @param {string} side - Order side (buy/sell)
   * @returns {string} Formatted side
   */
  formatSide(side) {
    const sideMap = {
      buy: "buy_to_open",
      sell: "sell_to_open",
    };
    return sideMap[side.toLowerCase()] || side;
  }

  /**
   * Format expiry date for API
   * @param {Date|string} expiry - Expiry date
   * @returns {string} Formatted expiry string
   */
  formatExpiry(expiry) {
    if (typeof expiry === "string") {
      return expiry;
    }
    if (expiry instanceof Date) {
      return expiry.toISOString().split("T")[0];
    }
    return expiry;
  }

  /**
   * Validate order data before submission
   * @param {Object} orderData - Order data to validate
   * @returns {Object} Validation result
   */
  validateOrder(orderData) {
    const errors = [];

    if (!orderData.symbol) {
      errors.push("Symbol is required");
    }

    if (!orderData.expiry) {
      errors.push("Expiry date is required");
    }

    if (!orderData.legs || orderData.legs.length === 0) {
      errors.push("At least one leg is required");
    }

    if (orderData.orderPrice === null || orderData.orderPrice === undefined) {
      errors.push("Order price is required");
    }

    // Validate legs
    if (orderData.legs) {
      orderData.legs.forEach((leg, index) => {
        if (!leg.symbol) {
          errors.push(`Leg ${index + 1}: Symbol is required`);
        }
        if (!leg.side) {
          errors.push(`Leg ${index + 1}: Side is required`);
        }
        if (!leg.ratio_qty) {
          errors.push(`Leg ${index + 1}: Quantity is required`);
        }
      });
    }

    return {
      isValid: errors.length === 0,
      errors,
    };
  }

  /**
   * Calculate order summary information
   * @param {Object} orderData - Order data
   * @returns {Object} Order summary
   */
  calculateOrderSummary(orderData) {
    const { legs, orderPrice, strategyType } = orderData;

    // Calculate total premium
    let totalPremium = 0;
    if (legs) {
      totalPremium = legs.reduce((sum, leg) => {
        const legValue =
          (leg.side === "buy" ? -1 : 1) *
          (leg.price || 0) *
          (leg.ratio_qty || 1);
        return sum + legValue;
      }, 0);
    }

    // Calculate max profit/loss (simplified)
    let maxProfit = 0;
    let maxLoss = 0;

    if (strategyType === "IRON Butterfly") {
      maxProfit = Math.abs(orderPrice) * 100;
      maxLoss = (2 - Math.abs(orderPrice)) * 100; // Assuming 2-point spread
    } else {
      maxProfit = (2 - Math.abs(orderPrice)) * 100;
      maxLoss = Math.abs(orderPrice) * 100;
    }

    // Use unified credit/debit logic: Negative = Credit, Positive = Debit
    return {
      totalPremium,
      maxProfit,
      maxLoss,
      netCredit: orderPrice < 0, // Negative price = Credit (we receive money)
      netDebit: orderPrice >= 0, // Positive price = Debit (we pay money)
    };
  }
}

// Export singleton instance
export default new OrderService();
