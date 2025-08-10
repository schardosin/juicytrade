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

  // Create price range for analysis (extend beyond strikes)
  const lowerBound = Math.floor(minStrike - strikeRange * 0.3);
  const upperBound = Math.ceil(maxStrike + strikeRange * 0.3);
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

  // Find max profit and max loss
  let maxProfit = Math.max(...payoffs);
  let maxLoss = Math.min(...payoffs);

  // For defined risk strategies, calculate max profit/loss based on limit price
  if (isDefinedRisk(positions)) {
    const netCalls = positions.filter(p => p.option_type === 'call').reduce((sum, p) => sum + p.qty, 0);
    if (netCalls > 0) {
        maxProfit = Infinity;
    } else if (limitPrice) {
      const { callSpreadWidth, putSpreadWidth } = getSpreadWidths(positions);
      const naturalPremium = calculateNetPremium(positions);
      const isCredit = naturalPremium > 0;

      if (isCredit) {
        maxProfit = limitPrice * 100;
        // For credit spreads, max loss is the width of the spread minus the credit received
        const spreadWidth = callSpreadWidth || putSpreadWidth;
        maxLoss = (spreadWidth - limitPrice) * 100;
      } else {
        // Debit spread
        maxLoss = Math.abs(limitPrice) * 100;
        const spreadWidth = callSpreadWidth || putSpreadWidth;
        maxProfit = (spreadWidth * 100) - maxLoss;
      }
    }
  } else {
    // Undefined risk
    const netCalls = positions.filter(p => p.option_type === 'call').reduce((sum, p) => sum + p.qty, 0);
    const netPuts = positions.filter(p => p.option_type === 'put').reduce((sum, p) => sum + p.qty, 0);

    if (netCalls < 0) maxLoss = -Infinity;
    if (netPuts < 0) maxLoss = -Infinity;

    // Cap profit on credit trades
    if (limitPrice > 0) {
      maxProfit = limitPrice * 100;
    }
  }

  // Find break-even points
  const breakEvenPoints = findBreakEvenPoints(prices, payoffs);

  // Calculate current unrealized P&L
  const currentPL = positions.reduce(
    (sum, pos) => sum + (pos.unrealized_pl || 0),
    0
  );

  return {
    maxProfit: formatNumber(maxProfit),
    maxLoss: formatNumber(maxLoss),
    breakEvenPoints: breakEvenPoints.map((point) => formatNumber(point, 2)),
    currentPL: formatNumber(currentPL),
    prices,
    payoffs,
  };
}

/**
 * Find break-even points where payoff crosses zero
 * @param {Array} prices - Price array
 * @param {Array} payoffs - Payoff array
 * @returns {Array} Break-even points
 */
function isDefinedRisk(positions) {
  const longCalls = positions.filter(p => p.qty > 0 && p.option_type === 'call').reduce((sum, p) => sum + p.qty, 0);
  const shortCalls = positions.filter(p => p.qty < 0 && p.option_type === 'call').reduce((sum, p) => sum + Math.abs(p.qty), 0);
  const longPuts = positions.filter(p => p.qty > 0 && p.option_type === 'put').reduce((sum, p) => sum + p.qty, 0);
  const shortPuts = positions.filter(p => p.qty < 0 && p.option_type === 'put').reduce((sum, p) => sum + Math.abs(p.qty), 0);

  return longCalls >= shortCalls && longPuts >= shortPuts;
}

function getSpreadWidths(positions) {
  const calls = positions.filter(p => p.option_type === 'call');
  const puts = positions.filter(p => p.option_type === 'put');

  let callSpreadWidth = 0;
  if (calls.length === 2) {
    callSpreadWidth = Math.abs(calls[0].strike_price - calls[1].strike_price);
  }

  let putSpreadWidth = 0;
  if (puts.length === 2) {
    putSpreadWidth = Math.abs(puts[0].strike_price - puts[1].strike_price);
  }

  return { callSpreadWidth, putSpreadWidth };
}

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
