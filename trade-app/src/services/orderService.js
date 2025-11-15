import api from "./api";

/**
 * Centralized Order Management Service
 * Handles all order-related operations across the application
 */
class OrderService {
  /**
   * Preview an order to get cost estimates and validation
   * @param {Object} orderData - Order configuration
   * @returns {Promise<Object>} Preview result
   */
  async previewOrder(orderData) {
    try {
      const orderPayload = this.buildOrderPayload(orderData);
      console.log("OrderService: Previewing order with payload:", orderPayload);

      const result = await api.previewOrder(orderPayload);

      // Handle the direct response format from backend
      if (result.success === false || result.status === 'error') {
        // Handle validation errors from API response
        const errorMessage = result.validation_errors?.[0] || result.message || "Unknown error";
        console.error("OrderService: Order preview failed with validation errors:", result.validation_errors);
        return {
          success: false,
          error: errorMessage,
          message: errorMessage, // Use just the error message, not prefixed
          preview: result,
        };
      } else if (result.status === 'ok' || result.success !== false) {
        console.log("OrderService: Order preview successful:", result);
        return {
          success: true,
          preview: result,
          message: "Order preview generated successfully",
        };
      } else {
        const errorMessage = result.validation_errors?.[0] || result.message || "Unknown error";
        console.error("OrderService: Order preview failed:", errorMessage);
        return {
          success: false,
          error: errorMessage,
          message: errorMessage,
          preview: result,
        };
      }
    } catch (error) {
      console.error("OrderService: Order preview error:", error);
      return {
        success: false,
        error: error.message,
        message: `Order preview failed: ${error.message}`,
      };
    }
  }

  /**
   * Place an order with smart routing (single-leg vs multi-leg)
   * @param {Object} orderData - Order configuration
   * @returns {Promise<Object>} Order result
   */
  async placeOrder(orderData) {
    try {
      const orderPayload = this.buildOrderPayload(orderData);
      console.log("OrderService: Placing order with payload:", orderPayload);

      // Determine order type and route accordingly
      const orderType = this.determineOrderType(orderPayload.legs);
      let result;

      if (orderType === 'single-leg') {
        result = await api.placeSingleLegOrder(orderPayload);
      } else {
        result = await api.placeMultiLegOrder(orderPayload);
      }

      if (result.success) {
        console.log("OrderService: Order placed successfully:", result);
        return {
          success: true,
          order: result.data,
          message: `${orderType} order submitted successfully! Order ID: ${
            result.data?.id || "N/A"
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
   * Place a single-leg order
   * @param {Object} orderData - Order configuration
   * @returns {Promise<Object>} Order result
   */
  async placeSingleLegOrder(orderData) {
    try {
      const orderPayload = this.buildOrderPayload(orderData);
      console.log("OrderService: Placing single-leg order with payload:", orderPayload);

      const result = await api.placeSingleLegOrder(orderPayload);

      if (result.success) {
        console.log("OrderService: Single-leg order placed successfully:", result);
        return {
          success: true,
          order: result.data,
          message: `Single-leg order submitted successfully! Order ID: ${
            result.data?.id || "N/A"
          }`,
        };
      } else {
        console.error("OrderService: Single-leg order failed:", result.error);
        return {
          success: false,
          error: result.error || "Unknown error",
          message: `Single-leg order failed: ${result.error || "Unknown error"}`,
        };
      }
    } catch (error) {
      console.error("OrderService: Single-leg order submission error:", error);
      return {
        success: false,
        error: error.message,
        message: `Single-leg order submission failed: ${error.message}`,
      };
    }
  }

  /**
   * Place a multi-leg order
   * @param {Object} orderData - Order configuration
   * @returns {Promise<Object>} Order result
   */
  async placeMultiLegOrder(orderData) {
    try {
      const orderPayload = this.buildOrderPayload(orderData);
      console.log("OrderService: Placing multi-leg order with payload:", orderPayload);

      const result = await api.placeMultiLegOrder(orderPayload);

      if (result.success) {
        console.log("OrderService: Multi-leg order placed successfully:", result);
        return {
          success: true,
          order: result.data,
          message: `Multi-leg order submitted successfully! Order ID: ${
            result.data?.id || "N/A"
          }`,
        };
      } else {
        console.error("OrderService: Multi-leg order failed:", result.error);
        return {
          success: false,
          error: result.error || "Unknown error",
          message: `Multi-leg order failed: ${result.error || "Unknown error"}`,
        };
      }
    } catch (error) {
      console.error("OrderService: Multi-leg order submission error:", error);
      return {
        success: false,
        error: error.message,
        message: `Multi-leg order submission failed: ${error.message}`,
      };
    }
  }

  /**
   * Determine if order is single-leg or multi-leg
   * @param {Array} legs - Order legs (may be undefined for equity orders)
   * @returns {string} 'single-leg' or 'multi-leg'
   */
  determineOrderType(legs) {
    // For equity orders (no legs), treat as single-leg
    if (!legs || legs.length === 0) {
      return 'single-leg';
    }
    
    if (legs.length === 1) {
      return 'single-leg';
    } else {
      return 'multi-leg';
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
      side,
      stopPrice,
    } = orderData;

    // Check if this is an equity order (no legs, has symbol and side)
    if (!legs && symbol && side) {
      // Equity order
      const payload = {
        symbol: symbol,
        side: side,
        qty: quantity,
        order_type: orderType.toLowerCase(),
        time_in_force: timeInForce.toLowerCase(),
      };

      // Add short sell flag if present
      if (orderData.is_short_sell) {
        payload.is_short_sell = true;
      }

      // Add price parameters based on order type
      if (limitPrice !== undefined && (orderType === "limit" || orderType === "stop_limit")) {
        payload.limit_price = limitPrice;
      }
      if (stopPrice !== undefined && (orderType === "stop" || orderType === "stop_limit" || orderType === "stop_market")) {
        payload.stop_price = stopPrice;
      }

      return payload;
    } else {
      // Options order (legacy format)
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
      side: leg.action,
      qty: parseInt(leg.ratio_qty || leg.quantity || 1),
    }));
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

    // Check if this is an equity order (no legs)
    if (!orderData.legs && orderData.side) {
      // Equity order validation
      if (!orderData.side) {
        errors.push("Order side is required");
      }
      if (!orderData.quantity || orderData.quantity <= 0) {
        errors.push("Quantity must be greater than 0");
      }
      if (orderData.orderType === "limit" && (!orderData.limitPrice || orderData.limitPrice <= 0)) {
        errors.push("Limit price is required for limit orders");
      }
      if ((orderData.orderType === "stop" || orderData.orderType === "stop_limit") && (!orderData.stopPrice || orderData.stopPrice <= 0)) {
        errors.push("Stop price is required for stop orders");
      }
    } else {
      // Options order validation (legacy)
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
