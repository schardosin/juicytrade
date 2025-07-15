/**
 * Options Strategy Detection and Labeling Utility
 * Centralized logic for identifying and labeling options strategies
 */

/**
 * Detect the strategy type based on order legs
 * @param {Array} legs - Array of order legs with symbol, side, qty
 * @returns {string} - Strategy name
 */
export function detectStrategy(legs) {
  if (!legs || legs.length === 0) {
    return "Single Leg";
  }

  if (legs.length === 1) {
    return "Single Leg";
  }

  if (legs.length === 2) {
    return detectTwoLegStrategy(legs);
  }

  if (legs.length === 4) {
    return detectFourLegStrategy(legs);
  }

  return `${legs.length}-Leg Strategy`;
}

/**
 * Detect two-leg strategy types
 * @param {Array} legs - Array of 2 order legs
 * @returns {string} - Strategy name
 */
function detectTwoLegStrategy(legs) {
  const leg1 = parseOptionLeg(legs[0]);
  const leg2 = parseOptionLeg(legs[1]);

  if (!leg1 || !leg2) {
    return "Two-Leg Strategy";
  }

  // Same expiration and same option type (both calls or both puts)
  if (leg1.expiry === leg2.expiry && leg1.type === leg2.type) {
    const isBuyingSpreads =
      legs.some((leg) => leg.side.includes("buy")) &&
      legs.some((leg) => leg.side.includes("sell"));

    if (isBuyingSpreads) {
      if (leg1.type === "call" || leg1.type === "c") {
        // Determine if it's debit or credit spread
        const longLeg = legs.find((leg) => leg.side.includes("buy"));
        const shortLeg = legs.find((leg) => leg.side.includes("sell"));

        if (longLeg && shortLeg) {
          const longStrike = parseOptionLeg(longLeg)?.strike;
          const shortStrike = parseOptionLeg(shortLeg)?.strike;

          if (longStrike && shortStrike) {
            if (longStrike < shortStrike) {
              return "Call Debit Spread";
            } else {
              return "Call Credit Spread";
            }
          }
        }
        return "Call Spread";
      } else {
        // Put spread
        const longLeg = legs.find((leg) => leg.side.includes("buy"));
        const shortLeg = legs.find((leg) => leg.side.includes("sell"));

        if (longLeg && shortLeg) {
          const longStrike = parseOptionLeg(longLeg)?.strike;
          const shortStrike = parseOptionLeg(shortLeg)?.strike;

          if (longStrike && shortStrike) {
            if (longStrike > shortStrike) {
              return "Put Debit Spread";
            } else {
              return "Put Credit Spread";
            }
          }
        }
        return "Put Spread";
      }
    }
  }

  // Different option types (call + put)
  if (leg1.type !== leg2.type) {
    const bothBuy = legs.every((leg) => leg.side.includes("buy"));
    const bothSell = legs.every((leg) => leg.side.includes("sell"));

    if (bothBuy) {
      return "Long Straddle";
    } else if (bothSell) {
      return "Short Straddle";
    } else {
      return "Synthetic Position";
    }
  }

  return "Two-Leg Strategy";
}

/**
 * Detect four-leg strategy types
 * @param {Array} legs - Array of 4 order legs
 * @returns {string} - Strategy name
 */
function detectFourLegStrategy(legs) {
  const parsedLegs = legs.map(parseOptionLeg).filter(Boolean);

  if (parsedLegs.length !== 4) {
    return "Four-Leg Strategy";
  }

  // Check if it's an Iron Condor (2 calls + 2 puts, same expiry)
  const calls = parsedLegs.filter(
    (leg) => leg.type === "call" || leg.type === "c"
  );
  const puts = parsedLegs.filter(
    (leg) => leg.type === "put" || leg.type === "p"
  );

  if (calls.length === 2 && puts.length === 2) {
    const sameExpiry = parsedLegs.every(
      (leg) => leg.expiry === parsedLegs[0].expiry
    );

    if (sameExpiry) {
      return "Iron Condor";
    }
  }

  // Check if it's an Iron Butterfly
  const strikes = [...new Set(parsedLegs.map((leg) => leg.strike))];
  if (strikes.length === 3) {
    return "Iron Butterfly";
  }

  return "Four-Leg Strategy";
}

/**
 * Parse option symbol to extract details
 * @param {Object} leg - Order leg with symbol
 * @returns {Object|null} - Parsed option details
 */
function parseOptionLeg(leg) {
  if (!leg || !leg.symbol) {
    return null;
  }

  const symbol = leg.symbol;

  // Check if it's an option symbol
  if (!isOptionSymbol(symbol)) {
    return null;
  }

  try {
    const match = symbol.match(/^([A-Z]+)(\d{6})([CP])(\d{8})$/);
    if (!match) return null;

    const [, underlying, dateStr, type, strikeStr] = match;

    // Parse date: YYMMDD -> YYYY-MM-DD
    const year = 2000 + parseInt(dateStr.substring(0, 2));
    const month = dateStr.substring(2, 4);
    const day = dateStr.substring(4, 6);
    const expiry = `${year}-${month}-${day}`;

    // Parse strike: 8 digits with 3 decimal places
    const strike = parseInt(strikeStr) / 1000;

    return {
      underlying,
      expiry,
      type: type.toLowerCase(),
      strike,
      side: leg.side,
    };
  } catch (error) {
    console.error("Error parsing option leg:", symbol, error);
    return null;
  }
}

/**
 * Check if symbol is an option symbol
 * @param {string} symbol - Symbol to check
 * @returns {boolean} - True if option symbol
 */
function isOptionSymbol(symbol) {
  return symbol && symbol.length > 10 && /[CP]\d{8}$/.test(symbol);
}

/**
 * Get strategy description for display
 * @param {string} strategy - Strategy name
 * @returns {string} - Display description
 */
export function getStrategyDescription(strategy) {
  const descriptions = {
    "Single Leg": "Single option position",
    "Call Debit Spread":
      "Bullish spread - buy lower strike, sell higher strike",
    "Call Credit Spread":
      "Bearish spread - sell lower strike, buy higher strike",
    "Put Debit Spread": "Bearish spread - buy higher strike, sell lower strike",
    "Put Credit Spread":
      "Bullish spread - sell higher strike, buy lower strike",
    "Call Spread": "Vertical call spread",
    "Put Spread": "Vertical put spread",
    "Long Straddle": "Buy call and put at same strike",
    "Short Straddle": "Sell call and put at same strike",
    "Iron Condor": "Sell call spread and put spread",
    "Iron Butterfly": "Sell straddle and buy protective wings",
    "Synthetic Position": "Combination replicating stock position",
  };

  return descriptions[strategy] || strategy;
}

/**
 * Get strategy risk profile
 * @param {string} strategy - Strategy name
 * @returns {Object} - Risk profile with max profit/loss info
 */
export function getStrategyRiskProfile(strategy) {
  const profiles = {
    "Call Debit Spread": {
      maxLoss: "Limited",
      maxProfit: "Limited",
      bias: "Bullish",
    },
    "Call Credit Spread": {
      maxLoss: "Limited",
      maxProfit: "Limited",
      bias: "Bearish",
    },
    "Put Debit Spread": {
      maxLoss: "Limited",
      maxProfit: "Limited",
      bias: "Bearish",
    },
    "Put Credit Spread": {
      maxLoss: "Limited",
      maxProfit: "Limited",
      bias: "Bullish",
    },
    "Iron Condor": {
      maxLoss: "Limited",
      maxProfit: "Limited",
      bias: "Neutral",
    },
    "Iron Butterfly": {
      maxLoss: "Limited",
      maxProfit: "Limited",
      bias: "Neutral",
    },
    "Long Straddle": {
      maxLoss: "Limited",
      maxProfit: "Unlimited",
      bias: "Volatile",
    },
    "Short Straddle": {
      maxLoss: "Unlimited",
      maxProfit: "Limited",
      bias: "Stable",
    },
  };

  return (
    profiles[strategy] || {
      maxLoss: "Variable",
      maxProfit: "Variable",
      bias: "Unknown",
    }
  );
}
