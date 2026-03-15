/**
 * Black-Scholes Option Pricing Utility
 *
 * Pure utility module with zero external dependencies (no Vue, no Chart.js).
 * Implements the Black-Scholes model for European option pricing and provides
 * a T+0 payoff calculation for multi-leg positions.
 *
 * @module blackScholes
 */

// Abramowitz & Stegun constants (formula 26.2.17)
const A1 = 0.31938153;
const A2 = -0.356563782;
const A3 = 1.781477937;
const A4 = -1.821255978;
const A5 = 1.330274429;
const GAMMA = 0.2316419;
const INV_SQRT_2PI = 1 / Math.sqrt(2 * Math.PI);

/**
 * Cumulative standard normal distribution function.
 * Uses the rational approximation by Abramowitz and Stegun (Handbook of
 * Mathematical Functions, formula 26.2.17). Accuracy: |error| < 7.5e-8.
 *
 * @param {number} x - The z-score
 * @returns {number} - Probability P(Z <= x), in range [0, 1]
 */
export function cumulativeNormalDistribution(x) {
  if (x >= 0) {
    const k = 1 / (1 + GAMMA * x);
    const kSq = k * k;
    const kCu = kSq * k;
    const k4 = kCu * k;
    const k5 = k4 * k;
    const poly = A1 * k + A2 * kSq + A3 * kCu + A4 * k4 + A5 * k5;
    const pdf = INV_SQRT_2PI * Math.exp(-0.5 * x * x);
    return 1 - pdf * poly;
  }
  // For x < 0, use symmetry: N(x) = 1 - N(-x)
  return 1 - cumulativeNormalDistribution(-x);
}

/**
 * Calculate the Black-Scholes theoretical price for a European option.
 *
 * @param {Object} params
 * @param {string} params.optionType - "call" or "put"
 * @param {number} params.S - Current underlying price (the hypothetical price point)
 * @param {number} params.K - Strike price
 * @param {number} params.T - Time to expiration in years (e.g., 30/365 = 0.0822)
 * @param {number} params.r - Risk-free interest rate (e.g., 0.05 for 5%)
 * @param {number} params.sigma - Implied volatility (e.g., 0.25 for 25%)
 * @returns {number} - Theoretical option price (per share)
 */
export function blackScholesPrice({ optionType, S, K, T, r, sigma }) {
  const isCall =
    optionType === "call" || optionType === "CALL" || optionType === "C";

  // Edge case: S <= 0 — Math.log(S/K) is undefined
  if (S <= 0) {
    if (isCall) {
      return 0;
    }
    // Put value when S=0 is the discounted strike
    return K * Math.exp(-r * Math.max(T, 0));
  }

  // Edge case: T <= 0 (expired option) — return intrinsic value
  if (T <= 0) {
    if (isCall) {
      return Math.max(S - K, 0);
    }
    return Math.max(K - S, 0);
  }

  // Edge case: sigma <= 0 — no volatility, return intrinsic value
  // This prevents NaN from division by zero in d1/d2
  if (sigma <= 0) {
    if (isCall) {
      return Math.max(S - K, 0);
    }
    return Math.max(K - S, 0);
  }

  // Pre-compute shared values for performance
  const sqrtT = Math.sqrt(T);
  const sigmaSqrtT = sigma * sqrtT;
  const d1Constant = (r + (sigma * sigma) / 2) * T;
  const discountFactor = Math.exp(-r * T);

  const d1 = (Math.log(S / K) + d1Constant) / sigmaSqrtT;
  const d2 = d1 - sigmaSqrtT;

  if (isCall) {
    return (
      S * cumulativeNormalDistribution(d1) -
      K * discountFactor * cumulativeNormalDistribution(d2)
    );
  }
  // Put
  return (
    K * discountFactor * cumulativeNormalDistribution(-d2) -
    S * cumulativeNormalDistribution(-d1)
  );
}

/**
 * Calculate T+0 payoff array for a multi-leg options position.
 *
 * For each price point in the given prices array, computes the theoretical
 * P&L of the entire position using Black-Scholes pricing for each leg.
 *
 * @param {Object} params
 * @param {number[]} params.prices - Array of underlying price points (x-axis)
 * @param {Array<Object>} params.legs - Array of leg objects, each containing:
 *   @param {number} legs[].strike_price - Strike price
 *   @param {string} legs[].option_type - "call" or "put"
 *   @param {number} legs[].qty - Signed quantity (+long, -short)
 *   @param {number} legs[].avg_entry_price - Entry price per share
 *   @param {number} legs[].iv - Implied volatility for this leg
 *   @param {number} legs[].T - Time to expiration in years for this leg
 * @param {number} [params.riskFreeRate=0.05] - Risk-free rate
 * @param {number} [params.creditAdjustment=0] - Credit/debit adjustment in dollars
 * @returns {number[]} - Array of T+0 P&L values (same length as prices array)
 */
export function calculateT0Payoffs({
  prices,
  legs,
  riskFreeRate = 0.05,
  creditAdjustment = 0,
}) {
  if (!legs || legs.length === 0) {
    // No legs — return creditAdjustment for each price point (or zeros if none)
    return prices.map(() => creditAdjustment);
  }

  // Pre-compute per-leg constants outside the price loop for performance.
  // For a 4-leg strategy with 500 price points this avoids 2000 redundant
  // sqrt / exp calls.
  const legConstants = legs.map((leg) => {
    const { strike_price, option_type, qty, avg_entry_price, iv, T } = leg;
    const sigma = iv;
    const r = riskFreeRate;
    const isExpiredOrNoVol = T <= 0 || sigma <= 0;

    let sqrtT, sigmaSqrtT, d1Constant, discountFactor;
    if (!isExpiredOrNoVol) {
      sqrtT = Math.sqrt(T);
      sigmaSqrtT = sigma * sqrtT;
      d1Constant = (r + (sigma * sigma) / 2) * T;
      discountFactor = Math.exp(-r * T);
    } else {
      sqrtT = 0;
      sigmaSqrtT = 0;
      d1Constant = 0;
      discountFactor = Math.exp(-r * Math.max(T, 0));
    }

    return {
      K: strike_price,
      optionType: option_type,
      qty,
      entryPrice: avg_entry_price || 0,
      isExpiredOrNoVol,
      isCall:
        option_type === "call" || option_type === "CALL" || option_type === "C",
      sqrtT,
      sigmaSqrtT,
      d1Constant,
      discountFactor,
    };
  });

  const result = new Array(prices.length);

  for (let p = 0; p < prices.length; p++) {
    const S = prices[p];
    let totalPayoff = 0;

    for (let l = 0; l < legConstants.length; l++) {
      const lc = legConstants[l];
      let theoreticalPrice;

      if (S <= 0) {
        // S <= 0 edge case
        theoreticalPrice = lc.isCall ? 0 : lc.K * lc.discountFactor;
      } else if (lc.isExpiredOrNoVol) {
        // Expired or zero vol — intrinsic value
        theoreticalPrice = lc.isCall
          ? Math.max(S - lc.K, 0)
          : Math.max(lc.K - S, 0);
      } else {
        // Standard Black-Scholes (inlined for performance)
        const d1 =
          (Math.log(S / lc.K) + lc.d1Constant) / lc.sigmaSqrtT;
        const d2 = d1 - lc.sigmaSqrtT;

        if (lc.isCall) {
          theoreticalPrice =
            S * cumulativeNormalDistribution(d1) -
            lc.K * lc.discountFactor * cumulativeNormalDistribution(d2);
        } else {
          theoreticalPrice =
            lc.K * lc.discountFactor * cumulativeNormalDistribution(-d2) -
            S * cumulativeNormalDistribution(-d1);
        }
      }

      // P&L for this leg: (theoreticalPrice - entryPrice) * qty * 100
      // qty is signed: positive for long, negative for short
      totalPayoff += (theoreticalPrice - lc.entryPrice) * lc.qty * 100;
    }

    // Apply credit/debit adjustment
    totalPayoff += creditAdjustment;

    result[p] = Math.round((totalPayoff + Number.EPSILON) * 100) / 100;
  }

  return result;
}
