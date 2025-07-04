import { ref, computed } from "vue";
import orderService from "../services/orderService";

/**
 * Order Management Composable
 * Provides centralized order state and functionality
 */
export function useOrderManagement() {
  // Reactive state
  const showOrderConfirmation = ref(false);
  const showOrderResult = ref(false);
  const orderData = ref(null);
  const orderResult = ref(null);
  const isPlacingOrder = ref(false);

  // Computed properties
  const canPlaceOrder = computed(() => {
    return orderData.value && !isPlacingOrder.value;
  });

  /**
   * Initialize order confirmation dialog
   * @param {Object} data - Order data configuration
   */
  const initializeOrder = (data) => {
    orderData.value = data;
    showOrderConfirmation.value = true;
  };

  /**
   * Handle order confirmation
   * @param {Object} confirmedOrderData - Final order data from dialog
   */
  const handleOrderConfirmation = async (confirmedOrderData) => {
    if (!confirmedOrderData) return;

    isPlacingOrder.value = true;
    showOrderConfirmation.value = false;

    try {
      // Validate order before submission
      const validation = orderService.validateOrder(confirmedOrderData);
      if (!validation.isValid) {
        orderResult.value = {
          success: false,
          error: "Validation failed",
          message: `Order validation failed: ${validation.errors.join(", ")}`,
          details: { validationErrors: validation.errors },
        };
        showOrderResult.value = true;
        return;
      }

      // Place the order
      const result = await orderService.placeOrder(confirmedOrderData);
      orderResult.value = result;
      showOrderResult.value = true;
    } catch (error) {
      console.error("Error in order confirmation:", error);
      orderResult.value = {
        success: false,
        error: error.message,
        message: `Order submission failed: ${error.message}`,
      };
      showOrderResult.value = true;
    } finally {
      isPlacingOrder.value = false;
    }
  };

  /**
   * Handle order cancellation
   */
  const handleOrderCancellation = () => {
    showOrderConfirmation.value = false;
    orderData.value = null;
  };

  /**
   * Handle order result dialog close
   */
  const handleOrderResultClose = () => {
    showOrderResult.value = false;
    orderResult.value = null;
  };

  /**
   * Handle view positions action
   */
  const handleViewPositions = () => {
    showOrderResult.value = false;
    // Emit event or navigate to positions page
    // This can be customized based on routing needs
  };

  /**
   * Build butterfly order data from component state
   * @param {Object} params - Order parameters
   * @returns {Object} Formatted order data
   */
  const buildButterflyOrderData = (params) => {
    const {
      symbol,
      expiry,
      strategyType,
      butterflyInfo,
      orderPrice,
      orderOffset = 0,
      underlyingPrice,
      currentOptionsPrice,
      calculatedOrderPrice,
      optionsChain, // Need the actual options chain to get real symbols
    } = params;

    // Helper function to find option by strike and type
    const findOption = (strike, type) => {
      return optionsChain?.find(
        (opt) =>
          Math.abs(parseFloat(opt.strike_price) - strike) < 0.01 &&
          opt.type === type
      );
    };

    // Build legs based on strategy type using REAL option symbols
    let legs = [];
    if (strategyType === "IRON Butterfly") {
      const putLower = findOption(butterflyInfo.lower_strike, "put");
      const putAtm = findOption(butterflyInfo.atm_strike, "put");
      const callAtm = findOption(butterflyInfo.atm_strike, "call");
      const callUpper = findOption(butterflyInfo.upper_strike, "call");

      legs = [
        {
          symbol:
            putLower?.symbol ||
            `${symbol}_${formatExpiry(expiry)}_${butterflyInfo.lower_strike}_P`,
          side: "buy",
          ratio_qty: "1",
          type: "Put",
          strike: `$${butterflyInfo.lower_strike.toFixed(2)}`,
          price: butterflyInfo.lower_price,
          displaySymbol: symbol,
          date: formatExpiry(expiry),
        },
        {
          symbol:
            putAtm?.symbol ||
            `${symbol}_${formatExpiry(expiry)}_${butterflyInfo.atm_strike}_P`,
          side: "sell",
          ratio_qty: "1",
          type: "Put",
          strike: `$${butterflyInfo.atm_strike.toFixed(2)}`,
          price: butterflyInfo.atm_put_price,
          displaySymbol: symbol,
          date: formatExpiry(expiry),
        },
        {
          symbol:
            callAtm?.symbol ||
            `${symbol}_${formatExpiry(expiry)}_${butterflyInfo.atm_strike}_C`,
          side: "sell",
          ratio_qty: "1",
          type: "Call",
          strike: `$${butterflyInfo.atm_strike.toFixed(2)}`,
          price: butterflyInfo.atm_call_price,
          displaySymbol: symbol,
          date: formatExpiry(expiry),
        },
        {
          symbol:
            callUpper?.symbol ||
            `${symbol}_${formatExpiry(expiry)}_${butterflyInfo.upper_strike}_C`,
          side: "buy",
          ratio_qty: "1",
          type: "Call",
          strike: `$${butterflyInfo.upper_strike.toFixed(2)}`,
          price: butterflyInfo.upper_price,
          displaySymbol: symbol,
          date: formatExpiry(expiry),
        },
      ];
    } else {
      // CALL or PUT Butterfly
      const optionType = strategyType === "CALL Butterfly" ? "Call" : "Put";
      const optionTypeKey = strategyType === "CALL Butterfly" ? "call" : "put";
      const optionSymbol = strategyType === "CALL Butterfly" ? "C" : "P";

      const lowerOption = findOption(butterflyInfo.lower_strike, optionTypeKey);
      const atmOption = findOption(butterflyInfo.atm_strike, optionTypeKey);
      const upperOption = findOption(butterflyInfo.upper_strike, optionTypeKey);

      legs = [
        {
          symbol:
            lowerOption?.symbol ||
            `${symbol}_${formatExpiry(expiry)}_${
              butterflyInfo.lower_strike
            }_${optionSymbol}`,
          side: "buy",
          ratio_qty: "1",
          type: optionType,
          strike: `$${butterflyInfo.lower_strike.toFixed(2)}`,
          price: butterflyInfo.lower_price,
          displaySymbol: symbol,
          date: formatExpiry(expiry),
        },
        {
          symbol:
            atmOption?.symbol ||
            `${symbol}_${formatExpiry(expiry)}_${
              butterflyInfo.atm_strike
            }_${optionSymbol}`,
          side: "sell",
          ratio_qty: "2",
          type: optionType,
          strike: `$${butterflyInfo.atm_strike.toFixed(2)}`,
          price: butterflyInfo.atm_price,
          displaySymbol: symbol,
          date: formatExpiry(expiry),
        },
        {
          symbol:
            upperOption?.symbol ||
            `${symbol}_${formatExpiry(expiry)}_${
              butterflyInfo.upper_strike
            }_${optionSymbol}`,
          side: "buy",
          ratio_qty: "1",
          type: optionType,
          strike: `$${butterflyInfo.upper_strike.toFixed(2)}`,
          price: butterflyInfo.upper_price,
          displaySymbol: symbol,
          date: formatExpiry(expiry),
        },
      ];
    }

    return {
      symbol,
      expiry,
      strategyType,
      legs,
      orderPrice,
      orderOffset,
      underlyingPrice,
      currentOptionsPrice,
      calculatedOrderPrice,
    };
  };

  /**
   * Build multi-leg adjustment order data
   * @param {Object} params - Order parameters
   * @returns {Object} Formatted order data
   */
  const buildMultiLegOrderData = (params) => {
    const {
      symbol,
      expiry,
      selectedOptions,
      selectedOptionsMap,
      orderQuantities,
      orderPrices,
      optionsChain,
      combinedOrderPrice,
      orderType = "limit",
      timeInForce = "day",
      underlyingPrice,
      positions = [],
    } = params;

    // Calculate current market values for selected options
    let currentOptionsPrice = 0;
    let totalContracts = 0;

    // Build legs from selected options
    const legs = selectedOptions.map((optionSymbol) => {
      const option = optionsChain.find((opt) => opt.symbol === optionSymbol);
      const selectionType = selectedOptionsMap[optionSymbol];
      const quantity = orderQuantities[optionSymbol] || 1;
      const price = orderPrices[optionSymbol] || 0;

      // Calculate contribution to current options price
      // For buy: we pay money (negative contribution to net credit)
      // For sell: we receive money (positive contribution to net credit)
      const marketValue = selectionType === "buy" ? -price : price;
      currentOptionsPrice += marketValue * quantity;
      totalContracts += quantity;

      return {
        symbol: optionSymbol,
        side: selectionType,
        ratio_qty: quantity.toString(),
        type: option?.type === "call" ? "Call" : "Put",
        strike: option?.strike_price
          ? `$${option.strike_price.toFixed(2)}`
          : "-",
        price,
        displaySymbol: symbol,
        date: formatExpiry(expiry),
        quantity,
      };
    });

    // Calculate current position P&L for context
    const currentPositionPL = positions.reduce(
      (sum, pos) => sum + (pos.unrealized_pl || 0),
      0
    );

    // Calculate estimated P&L impact of this adjustment
    const adjustmentCost = combinedOrderPrice * 100; // Convert to dollars
    const estimatedNewPL = currentPositionPL + adjustmentCost;

    return {
      symbol,
      expiry,
      strategyType: "Multi-Leg Adjustment",
      legs,
      orderPrice: combinedOrderPrice,
      orderOffset: 0,
      orderType,
      timeInForce,
      // Rich data for dialog display
      underlyingPrice,
      currentOptionsPrice: Math.abs(currentOptionsPrice),
      calculatedOrderPrice: combinedOrderPrice,
      currentPositionPL,
      estimatedNewPL,
      adjustmentCost,
      totalContracts,
      // Additional context
      adjustmentType: currentOptionsPrice >= 0 ? "Credit" : "Debit",
      positionCount: positions.length,
    };
  };

  /**
   * Build position close order data
   * @param {Object} params - Order parameters
   * @returns {Object} Formatted order data
   */
  const buildCloseOrderData = (params) => {
    const {
      symbol,
      expiry,
      selectedPositions,
      positions,
      closeOrderPrice,
      orderType = "limit",
      timeInForce = "day",
      underlyingPrice,
    } = params;

    // Calculate current market values for positions being closed
    let currentOptionsPrice = 0;
    let totalContracts = 0;
    let currentPositionPL = 0;

    // Build legs for closing positions
    const legs = selectedPositions.map((positionSymbol) => {
      const position = positions.find((pos) => pos.symbol === positionSymbol);

      // For closing: do the opposite operation
      const closingSide = position.qty < 0 ? "buy" : "sell";
      const closingQuantity = Math.abs(position.qty);

      // Calculate current market value and P&L for this position
      const marketValue = position.market_value || 0;
      const unrealizedPL = position.unrealized_pl || 0;
      const currentPrice = position.current_price || 0;

      // Calculate net credit/debit for closing this position
      // For closing: if we're buying to close (was short), we pay money (negative)
      // If we're selling to close (was long), we receive money (positive)
      const netValue = closingSide === "buy" ? -currentPrice : currentPrice;
      currentOptionsPrice += netValue * closingQuantity;

      currentPositionPL += unrealizedPL;
      totalContracts += closingQuantity;

      return {
        symbol: positionSymbol,
        side: closingSide,
        ratio_qty: closingQuantity.toString(),
        type: position.asset_class === "us_option" ? "Option" : "Stock",
        strike: position.strike_price
          ? `$${position.strike_price.toFixed(2)}`
          : "-",
        price: currentPrice,
        displaySymbol: symbol,
        date: formatExpiry(expiry),
        quantity: closingQuantity,
      };
    });

    // Calculate estimated proceeds from closing
    const estimatedProceeds = Math.abs(closeOrderPrice) * 100; // Convert to dollars
    const closeType = closeOrderPrice >= 0 ? "Credit" : "Debit";

    // Calculate total P&L impact (current P&L + proceeds from closing)
    const totalPLImpact =
      currentPositionPL +
      (closeOrderPrice >= 0 ? estimatedProceeds : -estimatedProceeds);

    return {
      symbol,
      expiry,
      strategyType: "Position Close",
      legs,
      orderPrice: Math.abs(closeOrderPrice),
      orderOffset: 0,
      orderType,
      timeInForce,
      // Rich data for dialog display
      underlyingPrice,
      currentOptionsPrice: Math.abs(currentOptionsPrice),
      calculatedOrderPrice: Math.abs(closeOrderPrice),
      currentPositionPL,
      estimatedProceeds,
      totalPLImpact,
      totalContracts,
      // Additional context
      closeType,
      positionCount: selectedPositions.length,
      isClosingOrder: true,
    };
  };

  /**
   * Format expiry date for display
   * @param {Date|string} expiry - Expiry date
   * @returns {string} Formatted expiry string
   */
  const formatExpiry = (expiry) => {
    if (!expiry) return "";
    if (typeof expiry === "string") return expiry;
    if (expiry instanceof Date) {
      return expiry.toISOString().split("T")[0];
    }
    return expiry.toString();
  };

  /**
   * Reset all order state
   */
  const resetOrderState = () => {
    showOrderConfirmation.value = false;
    showOrderResult.value = false;
    orderData.value = null;
    orderResult.value = null;
    isPlacingOrder.value = false;
  };

  return {
    // State
    showOrderConfirmation,
    showOrderResult,
    orderData,
    orderResult,
    isPlacingOrder,

    // Computed
    canPlaceOrder,

    // Methods
    initializeOrder,
    handleOrderConfirmation,
    handleOrderCancellation,
    handleOrderResultClose,
    handleViewPositions,
    buildButterflyOrderData,
    buildMultiLegOrderData,
    buildCloseOrderData,
    resetOrderState,
  };
}
