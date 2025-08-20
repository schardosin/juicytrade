/**
 * Centralized Options Calculator Service
 * Handles all profit/loss calculations for options strategies
 */

/**
 * Calculate comprehensive multi-leg options P&L analysis
 * @param {Array} selectedOptions - Array of selected option legs
 * @param {Array} optionsData - Array of live options data with prices
 * @param {Number} underlyingPrice - Current underlying asset price
 * @returns {Object} Complete P&L analysis
 */
export function calculateMultiLegProfitLoss(
  selectedOptions,
  optionsData,
  underlyingPrice,
  limitPrice
) {
  if (!selectedOptions || selectedOptions.length === 0) {
    return {
      maxProfit: 0,
      maxLoss: 0,
      netPremium: 0,
      breakEvenPoints: [],
      currentPL: 0,
      positions: [],
    };
  }

  // Convert selected options to positions format
  const positions = convertToPositions(selectedOptions, optionsData);

  // Calculate net premium (what we pay/receive upfront)
  const netPremium = calculateNetPremium(positions);

  // Generate payoff analysis
  const payoffAnalysis = generatePayoffAnalysis(
    positions,
    underlyingPrice,
    limitPrice
  );

  return {
    maxProfit: payoffAnalysis.maxProfit,
    maxLoss: payoffAnalysis.maxLoss,
    netPremium: netPremium,
    breakEvenPoints: payoffAnalysis.breakEvenPoints,
    currentPL: payoffAnalysis.currentPL,
    positions: positions,
  };
}

/**
 * Convert selected options to standardized positions format
 * @param {Array} selectedOptions - Selected option legs
 * @param {Array} optionsData - Live options data
 * @returns {Array} Standardized positions
 */
function convertToPositions(selectedOptions, optionsData) {
  return selectedOptions.map((selection) => {
    // Find live option data
    const liveOption = optionsData.find(
      (opt) => opt.symbol === selection.symbol
    );

    // Calculate current price (mid price)
    const currentPrice = liveOption ? (liveOption.bid + liveOption.ask) / 2 : 0;

    // Calculate entry price (natural price based on side)
    const entryPrice = liveOption
      ? selection.side === "buy"
        ? liveOption.ask
        : liveOption.bid
      : 0;

    // Calculate cost basis (negative for long positions, positive for short)
    const costBasis =
      selection.side === "buy"
        ? -entryPrice * selection.quantity * 100
        : entryPrice * selection.quantity * 100;

    return {
      symbol: selection.symbol,
      asset_class: "us_option",
      side: selection.side === "buy" ? "long" : "short",
      qty: selection.side === "buy" ? selection.quantity : -selection.quantity,
      strike_price: selection.strike_price || selection.strike,
      option_type: selection.type?.toLowerCase() || "call",
      expiry_date: selection.expiry,
      current_price: currentPrice,
      avg_entry_price: entryPrice,
      cost_basis: costBasis,
      market_value:
        selection.side === "buy"
          ? currentPrice * selection.quantity * 100
          : -currentPrice * selection.quantity * 100,
      unrealized_pl:
        selection.side === "buy"
          ? (currentPrice - entryPrice) * selection.quantity * 100
          : (entryPrice - currentPrice) * selection.quantity * 100,
      quantity: selection.quantity,
    };
  });
}

/**
 * Calculate net premium (upfront cost/credit)
 * @param {Array} positions - Position array
 * @returns {Number} Net premium (negative = we pay, positive = we receive)
 */
function calculateNetPremium(positions) {
  return positions.reduce((total, position) => {
    // For long positions: we pay premium (negative cost basis)
    // For short positions: we receive premium (positive cost basis)
    return total + position.cost_basis / 100; // Convert back to per-share basis
  }, 0);
}

/**
 * Generate comprehensive payoff analysis
 * @param {Array} positions - Position array
 * @param {Number} underlyingPrice - Current underlying price
 * @returns {Object} Payoff analysis
 */
function generatePayoffAnalysis(positions, underlyingPrice, limitPrice) {
  if (!positions || positions.length === 0) {
    return { maxProfit: 0, maxLoss: 0, breakEvenPoints: [], currentPL: 0 };
  }

  // Find price range based on strikes
  const strikes = positions.map((pos) => pos.strike_price);
  const minStrike = Math.min(...strikes);
  const maxStrike = Math.max(...strikes);
  const strikeRange = maxStrike - minStrike;

  // Create price range for analysis (extend beyond strikes, lower to 0 for accuracy with puts)
  const lowerBound = 0;
  const upperBound = Math.ceil(Math.max(underlyingPrice * 2, maxStrike + strikeRange * 2));
  const step = strikeRange > 50 ? (strikeRange > 100 ? 5 : 2) : 1;

  const prices = [];
  const payoffs = [];

  // Calculate payoff at each price point
  for (let price = lowerBound; price <= upperBound; price += step) {
    prices.push(price);

    let totalPayoff = 0;

    // Calculate payoff for each position at this price
    for (const position of positions) {
      const { strike_price, option_type, qty, avg_entry_price } = position;

      // Calculate intrinsic value
      let intrinsicValue = 0;
      if (option_type === "call") {
        intrinsicValue = Math.max(price - strike_price, 0);
      } else if (option_type === "put") {
        intrinsicValue = Math.max(strike_price - price, 0);
      }

      // Calculate position P&L
      const premiumPaid = avg_entry_price || 0;
      let positionPayoff;

      if (qty > 0) {
        // Long position: (intrinsic_value - premium_paid) * qty * 100
        positionPayoff = (intrinsicValue - premiumPaid) * qty * 100;
      } else {
        // Short position: (premium_received - intrinsic_value) * |qty| * 100
        positionPayoff = (premiumPaid - intrinsicValue) * Math.abs(qty) * 100;
      }

      totalPayoff += positionPayoff;
    }

    payoffs.push(totalPayoff);
  }

  // Calculate natural net premium
  const naturalNet = calculateNetPremium(positions);

  // Calculate max profit and max loss from the natural payoffs FIRST
  // This ensures they scale properly with quantity regardless of limit price
  let maxProfit = Math.max(...payoffs);
  let maxLoss = Math.min(...payoffs);

    // Calculate net calls and puts for unlimited overrides
  const netCalls = positions.filter(p => p.option_type === 'call').reduce((sum, p) => sum + p.qty, 0);
  const netPuts = positions.filter(p => p.option_type === 'put').reduce((sum, p) => sum + p.qty, 0);

  // Override for unlimited potential - FIXED LOGIC
  // Only set unlimited profit if there's a NET long position in calls
  if (netCalls > 0) {
    maxProfit = Infinity;
  }
  
  // Only set unlimited loss if there's a NET short position in calls
  if (netCalls < 0) {
    maxLoss = -Infinity;
  }

  // NOW apply limit price adjustment for break-even calculation and display
  // This affects the entire payoff curve but preserves the max profit/loss relationships
  let adjustment = 0;
  if (limitPrice != null && positions.length > 0) {
    // Determine if strategy is credit or debit based on natural
    const isCredit = naturalNet > 0;
    
    // Calculate effective net based on limit price
    const effectiveNet = isCredit ? limitPrice : -limitPrice;
    
    // Determine if strategy is symmetrical or asymmetrical
    const quantities = positions.map(pos => Math.abs(pos.qty));
    const minQuantity = Math.min(...quantities);
    const maxQuantity = Math.max(...quantities);
    
    // Check if all legs scale proportionally (symmetrical)
    // A strategy is symmetrical if all quantities are multiples of the minimum quantity
    const isSymmetrical = quantities.every(qty => qty % minQuantity === 0);
    
    let totalEffectiveNet;
    
    if (isSymmetrical) {
      // SYMMETRICAL TRADE: Limit price is per-unit, scale by quantity
      // Example: Spread with qty=2, limit=$0.50 → total=$1.00
      totalEffectiveNet = effectiveNet * minQuantity;
    } else {
      // ASYMMETRICAL TRADE: Limit price is total for the strategy
      // Example: Ratio spread, limit price represents total strategy price
      totalEffectiveNet = effectiveNet;
    }
    
    // Calculate adjustment based on total values
    const adjustmentPerShare = totalEffectiveNet - naturalNet;
    adjustment = adjustmentPerShare * 100;
  }

  // Apply adjustment to payoffs for break-even calculations (but don't modify original payoffs array)
  const adjustedPayoffs = payoffs.map(payoff => payoff + adjustment);

  // If max profit/loss are not infinite, apply the same adjustment
  if (maxProfit !== Infinity) {
    maxProfit += adjustment;
  }
  if (maxLoss !== -Infinity) {
    maxLoss += adjustment;
  }

  // Find break-even points using adjusted payoffs
  const breakEvenPoints = findBreakEvenPoints(prices, adjustedPayoffs);

  // Calculate current unrealized P&L
  const currentPL = positions.reduce(
    (sum, pos) => sum + (pos.unrealized_pl || 0),
    0
  ) + adjustment;

  return {
    maxProfit: formatNumber(maxProfit),
    maxLoss: formatNumber(maxLoss),
    breakEvenPoints: breakEvenPoints.map((point) => formatNumber(point, 2)),
    currentPL: formatNumber(currentPL),
    prices,
    payoffs: adjustedPayoffs,
  };
}

/**
 * Find break-even points where payoff crosses zero
 * @param {Array} prices - Price array
 * @param {Array} payoffs - Payoff array
 * @returns {Array} Break-even points
 */
function findBreakEvenPoints(prices, payoffs) {
  const breakEvenPoints = [];

  for (let i = 1; i < payoffs.length; i++) {
    if (
      (payoffs[i - 1] <= 0 && payoffs[i] >= 0) ||
      (payoffs[i - 1] >= 0 && payoffs[i] <= 0)
    ) {
      // Linear interpolation for more precise break-even
      const x1 = prices[i - 1];
      const x2 = prices[i];
      const y1 = payoffs[i - 1];
      const y2 = payoffs[i];

      if (y2 !== y1) {
        const breakEven = x1 - (y1 * (x2 - x1)) / (y2 - y1);
        breakEvenPoints.push(breakEven);
      }
    }
  }

  return breakEvenPoints;
}

/**
 * Calculate buying power effect (max risk for most strategies)
 * @param {Object} analysis - P&L analysis object
 * @returns {Number} Buying power effect
 */
export function calculateBuyingPowerEffect(analysis) {
  // For most options strategies, BP effect is the max loss
  return Math.abs(analysis.maxLoss);
}

/**
 * Format number with proper precision and remove floating point errors
 * @param {Number} value - Number to format
 * @param {Number} decimals - Number of decimal places (default: 2)
 * @returns {Number} Formatted number
 */
export function formatNumber(value, decimals = 2) {
  if (typeof value !== "number" || isNaN(value)) return 0;
  if (value === Infinity) return Infinity;
  if (value === -Infinity) return -Infinity;
  return Math.round(value * Math.pow(10, decimals)) / Math.pow(10, decimals);
}

/**
 * Format currency value for display
 * @param {Number} value - Value to format
 * @param {Number} decimals - Decimal places (default: 2)
 * @returns {String} Formatted currency string
 */
export function formatCurrency(value, decimals = 2) {
  const formatted = formatNumber(value, decimals);
  return formatted.toFixed(decimals);
}

/**
 * Determine if a strategy results in net credit or debit
 * @param {Number} netPremium - Net premium value
 * @returns {Object} Credit/debit information
 */
export function getCreditDebitInfo(netPremium) {
  return {
    isCredit: netPremium > 0,
    isDebit: netPremium < 0,
    amount: Math.abs(netPremium),
    label: netPremium > 0 ? "cr" : "db",
  };
}

/**
 * Calculate Greeks approximation (simplified)
 * @param {Array} positions - Position array
 * @returns {Object} Greeks approximation
 */
export function calculateGreeks(positions) {
  // This is a simplified approximation
  // In a real application, you'd use proper options pricing models

  let netDelta = 0;
  let netTheta = 0;

  positions.forEach((position) => {
    // Simplified delta calculation
    const delta = position.option_type === "call" ? 0.5 : -0.5;
    netDelta += delta * position.qty;

    // Simplified theta calculation (time decay)
    const theta = -0.05; // Approximate daily theta
    netTheta += theta * position.qty;
  });

  return {
    delta: formatNumber(netDelta, 2),
    theta: formatNumber(netTheta, 2),
  };
}

export default {
  calculateMultiLegProfitLoss,
  calculateBuyingPowerEffect,
  formatNumber,
  formatCurrency,
  getCreditDebitInfo,
  calculateGreeks,
};