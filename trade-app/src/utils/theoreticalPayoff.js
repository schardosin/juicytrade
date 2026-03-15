/**
 * Theoretical Payoff Calculation Utility
 *
 * Provides functions for calculating theoretical P/L at a future date (T+X)
 * using the Black-Scholes model with adjustable time-to-expiry and IV.
 *
 * @module theoreticalPayoff
 */

import { calculateT0Payoffs } from './blackScholes.js';

const MS_PER_YEAR = 365.25 * 24 * 60 * 60 * 1000;
const MARKET_CLOSE_HOUR = 16;

/**
 * Calculate time-to-expiry in years for each leg from a selected date.
 *
 * For each position, computes the fractional years from the selected date
 * to the leg's expiration date. Uses market close time (4:00 PM ET) for
 * expiration. Returns 0 for legs that have already expired at the selected date.
 *
 * @param {Date} selectedDate - The date to calculate time-to-expiry from
 * @param {Array<Object>} positions - Array of option position objects
 * @param {string} positions[].expiry_date - Expiration date string (YYYY-MM-DD format)
 * @returns {number[]} Array of time-to-expiry values in years for each leg
 */
export function calculateTimesToExpiry(selectedDate, positions) {
  if (!positions || positions.length === 0) {
    return [];
  }

  return positions.map((pos) => {
    if (!pos.expiry_date) {
      return 0;
    }

    // On expiration day, force T=0 so Black-Scholes returns pure intrinsic value,
    // ensuring the theoretical line perfectly overlays the expiration line.
    const [expiryYear, expiryMonth, expiryDay] = pos.expiry_date.split('-').map(Number);
    const selYear = selectedDate.getFullYear();
    const selMonth = selectedDate.getMonth() + 1;
    const selDay = selectedDate.getDate();

    if (expiryYear === selYear && expiryMonth === selMonth && expiryDay === selDay) {
      return 0;
    }

    const expiryDate = new Date(`${pos.expiry_date}T${MARKET_CLOSE_HOUR}:00:00`);
    const msToExpiry = expiryDate.getTime() - selectedDate.getTime();
    const T = msToExpiry / MS_PER_YEAR;

    return Math.max(T, 0);
  });
}

/**
 * Calculate weighted average implied volatility from all legs.
 *
 * Weights each leg's IV by its absolute notional value (|qty| * strike).
 * Returns null if any leg is missing IV data.
 *
 * @param {Array<Object>} positions - Array of option position objects
 * @param {number} positions[].qty - Position quantity (signed: positive for long, negative for short)
 * @param {number} positions[].strike_price - Strike price
 * @param {Function} getIV - Function that takes symbol and returns IV (decimal) or null
 * @returns {number|null} Weighted average IV as decimal (e.g., 0.25 for 25%), or null if unavailable
 */
export function calculateWeightedAverageIV(positions, getIV) {
  if (!positions || positions.length === 0) {
    return null;
  }

  let totalNotional = 0;
  let weightedIVSum = 0;

  for (const pos of positions) {
    const iv = getIV(pos.symbol);

    if (iv === null || iv === undefined || iv <= 0 || !Number.isFinite(iv)) {
      return null;
    }

    const notional = Math.abs(pos.qty) * pos.strike_price;
    totalNotional += notional;
    weightedIVSum += iv * notional;
  }

  if (totalNotional === 0) {
    return null;
  }

  return weightedIVSum / totalNotional;
}

/**
 * Calculate theoretical P/L values for a given date and IV.
 *
 * Builds leg data with adjusted time-to-expiry and IV, then uses
 * the existing calculateT0Payoffs function from blackScholes.js.
 *
 * @param {Object} params - Parameters object
 * @param {number[]} params.prices - Array of underlying price points
 * @param {Array<Object>} params.positions - Array of option position objects
 * @param {string} params.positions[].symbol - Option symbol
 * @param {number} params.positions[].strike_price - Strike price
 * @param {string} params.positions[].option_type - "call" or "put"
 * @param {number} params.positions[].qty - Signed quantity (+long, -short)
 * @param {number} params.positions[].avg_entry_price - Entry price per share
 * @param {string} params.positions[].expiry_date - Expiration date (YYYY-MM-DD)
 * @param {Date} params.selectedDate - Target date for T+X calculation
 * @param {number|null} params.selectedIV - IV override (decimal), or null to use per-leg IVs
 * @param {Function} params.getIV - Function to get IV for a symbol: (symbol) => number|null
 * @param {number} [params.riskFreeRate=0.05] - Risk-free rate (default 5%)
 * @param {number} [params.creditAdjustment=0] - Credit/debit adjustment in dollars
 * @returns {number[]|null} Array of theoretical P/L values, or null if IV unavailable
 */
export function calculateTheoreticalPayoffs({
  prices,
  positions,
  selectedDate,
  selectedIV,
  getIV,
  riskFreeRate = 0.05,
  creditAdjustment = 0,
}) {
  if (!prices || prices.length === 0) {
    return [];
  }

  if (!positions || positions.length === 0) {
    return prices.map(() => creditAdjustment);
  }

  const timesToExpiry = calculateTimesToExpiry(selectedDate, positions);

  const legs = [];

  for (let i = 0; i < positions.length; i++) {
    const pos = positions[i];
    const T = timesToExpiry[i];

    const iv = selectedIV !== null && selectedIV !== undefined
      ? selectedIV
      : getIV(pos.symbol);

    if (iv === null || iv === undefined || iv <= 0 || !Number.isFinite(iv)) {
      return null;
    }

    legs.push({
      strike_price: pos.strike_price,
      option_type: pos.option_type,
      qty: pos.qty,
      avg_entry_price: pos.avg_entry_price || 0,
      iv: iv,
      T: T,
    });
  }

  return calculateT0Payoffs({
    prices,
    legs,
    riskFreeRate,
    creditAdjustment,
  });
}