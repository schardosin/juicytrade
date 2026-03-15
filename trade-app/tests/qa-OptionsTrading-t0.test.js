/**
 * QA tests for OptionsTrading.vue computeT0ForPositions logic.
 *
 * Since computeT0ForPositions is a local helper inside setup() and is not
 * exported, we reimplement the identical algorithm here and test it directly.
 * This verifies the core logic without mounting the full Vue component.
 *
 * The reference implementation (OptionsTrading.vue:763-800):
 *
 *   1. Filter positions for asset_class === "us_option"
 *   2. For each, call getOptionGreeks(pos.symbol).value
 *   3. If ANY leg has !greeks || !greeks.implied_volatility → return null (FR-6)
 *   4. Compute T = max( msToExpiry / MS_PER_YEAR, 0 )
 *   5. Build legs array with strike_price, option_type, qty, avg_entry_price, iv, T
 *   6. Call calculateT0Payoffs({ prices, legs, riskFreeRate: 0.05, creditAdjustment })
 *
 * Covers:
 *  1. Returns null when any leg is missing IV
 *  2. Returns null when no option positions
 *  3. Returns null when Greeks exist but implied_volatility is falsy (0 or null)
 *  4. Correctly builds legs array
 *  5. Time-to-expiry (T) is positive for future dates
 *  6. Time-to-expiry (T) is clamped to 0 for past dates
 *  7. Uses _creditAdjustment from payoffData
 *  8. Risk-free rate is hardcoded at 0.05
 */

import { describe, it, expect, vi, beforeEach } from "vitest";
import { calculateT0Payoffs } from "../src/utils/blackScholes.js";

// ---------------------------------------------------------------------------
// Faithful reimplementation of computeT0ForPositions
// ---------------------------------------------------------------------------

const MS_PER_YEAR = 365.25 * 24 * 60 * 60 * 1000;

/**
 * Exact replica of OptionsTrading.vue:763-800.
 *
 * @param {Array}    positions        - Positions array (may include non-option assets)
 * @param {Object}   payoffData       - { prices, _creditAdjustment }
 * @param {Function} getOptionGreeks  - (symbol) => { value: { implied_volatility } }
 * @param {Date}     [now]            - Override "now" for deterministic tests
 * @returns {number[]|null}
 */
function computeT0ForPositions(positions, payoffData, getOptionGreeks, now = new Date()) {
  const optionPositions = positions.filter(
    (pos) => pos.asset_class === "us_option"
  );
  if (optionPositions.length === 0) return null;

  const legs = [];

  for (const pos of optionPositions) {
    const greeksRef = getOptionGreeks(pos.symbol);
    const greeks = greeksRef.value;

    if (!greeks || !greeks.implied_volatility) {
      return null; // Per FR-6: if ANY leg lacks IV, skip T+0 entirely
    }

    const expiryDate = new Date(pos.expiry_date + "T16:00:00");
    const msToExpiry = expiryDate.getTime() - now.getTime();
    const T = Math.max(msToExpiry / MS_PER_YEAR, 0);

    legs.push({
      strike_price: pos.strike_price,
      option_type: pos.option_type,
      qty: pos.qty,
      avg_entry_price: pos.avg_entry_price || 0,
      iv: greeks.implied_volatility,
      T: T,
    });
  }

  return calculateT0Payoffs({
    prices: payoffData.prices,
    legs: legs,
    riskFreeRate: 0.05,
    creditAdjustment: payoffData._creditAdjustment || 0,
  });
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeOptionPosition(overrides = {}) {
  return {
    symbol: "SPY_CALL_500_2026-06-19",
    asset_class: "us_option",
    side: "long",
    qty: 1,
    strike_price: 500,
    option_type: "call",
    expiry_date: "2026-06-19",
    avg_entry_price: 10,
    underlying_symbol: "SPY",
    ...overrides,
  };
}

function makeStockPosition(overrides = {}) {
  return {
    symbol: "SPY",
    asset_class: "us_equity",
    side: "long",
    qty: 100,
    avg_entry_price: 500,
    ...overrides,
  };
}

/**
 * Create a mock getOptionGreeks function from a map of symbol → greeks.
 * Returns a ref-like object with a .value property (mirroring Vue ref behavior).
 */
function mockGetOptionGreeks(greeksMap) {
  return (symbol) => ({
    value: greeksMap[symbol] ?? null,
  });
}

const defaultPrices = [480, 490, 500, 510, 520];
const defaultPayoffData = {
  prices: defaultPrices,
  _creditAdjustment: 0,
};

// ---------------------------------------------------------------------------
// 1. Returns null when any leg is missing IV
// ---------------------------------------------------------------------------
describe("QA: computeT0 returns null when any leg is missing IV (FR-6)", () => {
  it("returns null when one of two legs has no greeks", () => {
    const positions = [
      makeOptionPosition({ symbol: "SPY_CALL_500", strike_price: 500 }),
      makeOptionPosition({ symbol: "SPY_CALL_510", strike_price: 510, qty: -1 }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: 0.25 },
      SPY_CALL_510: null, // no greeks for this leg
    });

    const result = computeT0ForPositions(positions, defaultPayoffData, getGreeks);
    expect(result).toBeNull();
  });

  it("returns null when first leg has greeks but second has no IV field", () => {
    const positions = [
      makeOptionPosition({ symbol: "SPY_CALL_500", strike_price: 500 }),
      makeOptionPosition({ symbol: "SPY_CALL_510", strike_price: 510, qty: -1 }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: 0.25 },
      SPY_CALL_510: { delta: 0.5, gamma: 0.01 }, // greeks exist, but no IV
    });

    const result = computeT0ForPositions(positions, defaultPayoffData, getGreeks);
    expect(result).toBeNull();
  });

  it("returns non-null when ALL legs have valid IV", () => {
    const positions = [
      makeOptionPosition({ symbol: "SPY_CALL_500", strike_price: 500 }),
      makeOptionPosition({ symbol: "SPY_CALL_510", strike_price: 510, qty: -1 }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: 0.25 },
      SPY_CALL_510: { implied_volatility: 0.28 },
    });

    const result = computeT0ForPositions(positions, defaultPayoffData, getGreeks);
    expect(result).not.toBeNull();
    expect(Array.isArray(result)).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// 2. Returns null when no option positions
// ---------------------------------------------------------------------------
describe("QA: computeT0 returns null when no option positions", () => {
  it("returns null when all positions are equities", () => {
    const positions = [
      makeStockPosition({ symbol: "SPY" }),
      makeStockPosition({ symbol: "AAPL" }),
    ];

    const getGreeks = mockGetOptionGreeks({});

    const result = computeT0ForPositions(positions, defaultPayoffData, getGreeks);
    expect(result).toBeNull();
  });

  it("returns null when positions array is empty", () => {
    const getGreeks = mockGetOptionGreeks({});
    const result = computeT0ForPositions([], defaultPayoffData, getGreeks);
    expect(result).toBeNull();
  });

  it("filters out non-option positions and processes only options", () => {
    const positions = [
      makeStockPosition({ symbol: "SPY" }),
      makeOptionPosition({ symbol: "SPY_CALL_500", strike_price: 500 }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: 0.25 },
    });

    const result = computeT0ForPositions(positions, defaultPayoffData, getGreeks);
    expect(result).not.toBeNull();
    expect(result).toHaveLength(defaultPrices.length);
  });
});

// ---------------------------------------------------------------------------
// 3. Returns null when Greeks object exists but implied_volatility is falsy
// ---------------------------------------------------------------------------
describe("QA: computeT0 returns null when IV is falsy", () => {
  it("returns null when implied_volatility is 0", () => {
    const positions = [
      makeOptionPosition({ symbol: "SPY_CALL_500" }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: 0 },
    });

    const result = computeT0ForPositions(positions, defaultPayoffData, getGreeks);
    expect(result).toBeNull();
  });

  it("returns null when implied_volatility is null", () => {
    const positions = [
      makeOptionPosition({ symbol: "SPY_CALL_500" }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: null },
    });

    const result = computeT0ForPositions(positions, defaultPayoffData, getGreeks);
    expect(result).toBeNull();
  });

  it("returns null when implied_volatility is undefined", () => {
    const positions = [
      makeOptionPosition({ symbol: "SPY_CALL_500" }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: undefined },
    });

    const result = computeT0ForPositions(positions, defaultPayoffData, getGreeks);
    expect(result).toBeNull();
  });

  it("returns null when implied_volatility is empty string", () => {
    const positions = [
      makeOptionPosition({ symbol: "SPY_CALL_500" }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: "" },
    });

    const result = computeT0ForPositions(positions, defaultPayoffData, getGreeks);
    expect(result).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// 4. Correctly builds legs array
// ---------------------------------------------------------------------------
describe("QA: computeT0 correctly builds legs array", () => {
  it("leg has all required fields: strike_price, option_type, qty, avg_entry_price, iv, T", () => {
    // We spy on calculateT0Payoffs to capture the legs argument
    const capturedArgs = [];
    const originalCalcT0 = calculateT0Payoffs;

    // Instead of spying on the module, we verify via the output's consistency
    // with known inputs. We construct a single-leg scenario and verify the
    // result matches what calculateT0Payoffs would produce with correct legs.

    const futureDate = "2027-01-15"; // well in the future
    const positions = [
      makeOptionPosition({
        symbol: "SPY_PUT_490",
        strike_price: 490,
        option_type: "put",
        qty: -2,
        avg_entry_price: 7.5,
        expiry_date: futureDate,
      }),
    ];

    const iv = 0.30;
    const getGreeks = mockGetOptionGreeks({
      SPY_PUT_490: { implied_volatility: iv },
    });

    const now = new Date("2026-03-14T12:00:00Z");
    const result = computeT0ForPositions(positions, defaultPayoffData, getGreeks, now);

    // Manually compute what the leg should look like
    const expiryDate = new Date(futureDate + "T16:00:00");
    const expectedT = Math.max(
      (expiryDate.getTime() - now.getTime()) / MS_PER_YEAR,
      0
    );

    // Now call calculateT0Payoffs directly with the expected leg and compare
    const expectedResult = calculateT0Payoffs({
      prices: defaultPrices,
      legs: [
        {
          strike_price: 490,
          option_type: "put",
          qty: -2,
          avg_entry_price: 7.5,
          iv: iv,
          T: expectedT,
        },
      ],
      riskFreeRate: 0.05,
      creditAdjustment: 0,
    });

    expect(result).toEqual(expectedResult);
  });

  it("avg_entry_price defaults to 0 when missing", () => {
    const positions = [
      makeOptionPosition({
        symbol: "SPY_CALL_500",
        avg_entry_price: undefined, // missing
        expiry_date: "2027-01-15",
      }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: 0.25 },
    });

    const now = new Date("2026-03-14T12:00:00Z");
    const result = computeT0ForPositions(positions, defaultPayoffData, getGreeks, now);

    // Calculate expected with avg_entry_price = 0
    const expiryDate = new Date("2027-01-15T16:00:00");
    const T = Math.max((expiryDate.getTime() - now.getTime()) / MS_PER_YEAR, 0);
    const expectedResult = calculateT0Payoffs({
      prices: defaultPrices,
      legs: [
        {
          strike_price: 500,
          option_type: "call",
          qty: 1,
          avg_entry_price: 0,
          iv: 0.25,
          T,
        },
      ],
      riskFreeRate: 0.05,
      creditAdjustment: 0,
    });

    expect(result).toEqual(expectedResult);
  });
});

// ---------------------------------------------------------------------------
// 5. Time-to-expiry (T) is positive for future dates
// ---------------------------------------------------------------------------
describe("QA: T is positive for future expiry dates", () => {
  it("T > 0 when expiry_date is in the future", () => {
    const futureExpiry = "2027-06-18";
    const now = new Date("2026-03-14T12:00:00Z");

    const expiryDate = new Date(futureExpiry + "T16:00:00");
    const msToExpiry = expiryDate.getTime() - now.getTime();
    const T = Math.max(msToExpiry / MS_PER_YEAR, 0);

    expect(T).toBeGreaterThan(0);
    // Should be roughly 1.26 years
    expect(T).toBeGreaterThan(1.0);
    expect(T).toBeLessThan(2.0);
  });

  it("function produces finite positive results for future-dated leg", () => {
    const positions = [
      makeOptionPosition({
        symbol: "SPY_CALL_500",
        expiry_date: "2027-06-18",
      }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: 0.25 },
    });

    const now = new Date("2026-03-14T12:00:00Z");
    const result = computeT0ForPositions(positions, defaultPayoffData, getGreeks, now);

    expect(result).not.toBeNull();
    for (const val of result) {
      expect(Number.isFinite(val)).toBe(true);
    }
  });
});

// ---------------------------------------------------------------------------
// 6. Time-to-expiry (T) is clamped to 0 for past dates
// ---------------------------------------------------------------------------
describe("QA: T is clamped to 0 for past expiry dates", () => {
  it("T = 0 when expiry_date is in the past", () => {
    const pastExpiry = "2025-01-15";
    const now = new Date("2026-03-14T12:00:00Z");

    const expiryDate = new Date(pastExpiry + "T16:00:00");
    const msToExpiry = expiryDate.getTime() - now.getTime();
    const T = Math.max(msToExpiry / MS_PER_YEAR, 0);

    expect(T).toBe(0);
  });

  it("past-dated position produces intrinsic-value payoffs (T=0 behavior)", () => {
    const pastExpiry = "2025-01-15";
    const positions = [
      makeOptionPosition({
        symbol: "SPY_CALL_500",
        strike_price: 500,
        option_type: "call",
        qty: 1,
        avg_entry_price: 10,
        expiry_date: pastExpiry,
      }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: 0.25 },
    });

    const now = new Date("2026-03-14T12:00:00Z");
    const prices = [490, 500, 510, 520];
    const payoffData = { prices, _creditAdjustment: 0 };

    const result = computeT0ForPositions(positions, payoffData, getGreeks, now);

    // At T=0, call value = max(S-K, 0), so P&L = (max(S-500, 0) - 10) * 100
    const expected = prices.map(
      (S) => Math.round(((Math.max(S - 500, 0) - 10) * 100 + Number.EPSILON) * 100) / 100
    );

    expect(result).toEqual(expected);
  });

  it("T = 0 when expiry is earlier today (already past 4pm close)", () => {
    // Expiry today at 4pm, now is 5pm
    const todayExpiry = "2026-03-14";
    const now = new Date("2026-03-14T21:00:00Z"); // 5pm ET = 21:00 UTC

    const expiryDate = new Date(todayExpiry + "T16:00:00");
    const msToExpiry = expiryDate.getTime() - now.getTime();
    const T = Math.max(msToExpiry / MS_PER_YEAR, 0);

    // 16:00 local < 21:00 UTC so msToExpiry < 0, T clamped to 0
    // Note: this depends on system timezone. The key test is the Math.max(..., 0) clamp.
    if (msToExpiry < 0) {
      expect(T).toBe(0);
    } else {
      // If running in a timezone where T16:00:00 is interpreted as UTC,
      // expiry is still future relative to 21:00 UTC — T would be 0 regardless
      // because 16:00 UTC < 21:00 UTC
      expect(T).toBe(0);
    }
  });
});

// ---------------------------------------------------------------------------
// 7. Uses _creditAdjustment from payoffData
// ---------------------------------------------------------------------------
describe("QA: computeT0 uses _creditAdjustment from payoffData", () => {
  it("non-zero _creditAdjustment shifts all T+0 values", () => {
    const positions = [
      makeOptionPosition({
        symbol: "SPY_CALL_500",
        expiry_date: "2027-01-15",
      }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: 0.25 },
    });

    const now = new Date("2026-03-14T12:00:00Z");

    const resultNoAdj = computeT0ForPositions(
      positions,
      { prices: defaultPrices, _creditAdjustment: 0 },
      getGreeks,
      now
    );

    const creditAdj = 350; // $350 adjustment
    const resultWithAdj = computeT0ForPositions(
      positions,
      { prices: defaultPrices, _creditAdjustment: creditAdj },
      getGreeks,
      now
    );

    expect(resultNoAdj).not.toBeNull();
    expect(resultWithAdj).not.toBeNull();

    // Every value should be shifted by exactly creditAdjustment
    for (let i = 0; i < defaultPrices.length; i++) {
      expect(resultWithAdj[i]).toBeCloseTo(resultNoAdj[i] + creditAdj, 1);
    }
  });

  it("_creditAdjustment defaults to 0 when falsy", () => {
    const positions = [
      makeOptionPosition({
        symbol: "SPY_CALL_500",
        expiry_date: "2027-01-15",
      }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: 0.25 },
    });

    const now = new Date("2026-03-14T12:00:00Z");

    const resultUndefined = computeT0ForPositions(
      positions,
      { prices: defaultPrices, _creditAdjustment: undefined },
      getGreeks,
      now
    );

    const resultZero = computeT0ForPositions(
      positions,
      { prices: defaultPrices, _creditAdjustment: 0 },
      getGreeks,
      now
    );

    expect(resultUndefined).toEqual(resultZero);
  });

  it("negative _creditAdjustment shifts values down", () => {
    const positions = [
      makeOptionPosition({
        symbol: "SPY_CALL_500",
        expiry_date: "2027-01-15",
      }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: 0.25 },
    });

    const now = new Date("2026-03-14T12:00:00Z");

    const resultNoAdj = computeT0ForPositions(
      positions,
      { prices: defaultPrices, _creditAdjustment: 0 },
      getGreeks,
      now
    );

    const resultNegAdj = computeT0ForPositions(
      positions,
      { prices: defaultPrices, _creditAdjustment: -200 },
      getGreeks,
      now
    );

    for (let i = 0; i < defaultPrices.length; i++) {
      expect(resultNegAdj[i]).toBeCloseTo(resultNoAdj[i] - 200, 1);
    }
  });
});

// ---------------------------------------------------------------------------
// 8. Risk-free rate is hardcoded at 0.05
// ---------------------------------------------------------------------------
describe("QA: risk-free rate is hardcoded at 0.05", () => {
  it("output matches calculateT0Payoffs called with riskFreeRate=0.05", () => {
    const futureExpiry = "2027-01-15";
    const iv = 0.30;
    const positions = [
      makeOptionPosition({
        symbol: "SPY_CALL_500",
        strike_price: 500,
        option_type: "call",
        qty: 1,
        avg_entry_price: 15,
        expiry_date: futureExpiry,
      }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: iv },
    });

    const now = new Date("2026-03-14T12:00:00Z");
    const expiryDate = new Date(futureExpiry + "T16:00:00");
    const T = Math.max(
      (expiryDate.getTime() - now.getTime()) / MS_PER_YEAR,
      0
    );

    const result = computeT0ForPositions(positions, defaultPayoffData, getGreeks, now);

    // Verify it matches r=0.05 exactly
    const expectedWith005 = calculateT0Payoffs({
      prices: defaultPrices,
      legs: [
        { strike_price: 500, option_type: "call", qty: 1, avg_entry_price: 15, iv, T },
      ],
      riskFreeRate: 0.05,
      creditAdjustment: 0,
    });

    expect(result).toEqual(expectedWith005);
  });

  it("output differs from calculateT0Payoffs called with riskFreeRate=0.10", () => {
    const futureExpiry = "2027-01-15";
    const iv = 0.30;
    const positions = [
      makeOptionPosition({
        symbol: "SPY_CALL_500",
        strike_price: 500,
        option_type: "call",
        qty: 1,
        avg_entry_price: 15,
        expiry_date: futureExpiry,
      }),
    ];

    const getGreeks = mockGetOptionGreeks({
      SPY_CALL_500: { implied_volatility: iv },
    });

    const now = new Date("2026-03-14T12:00:00Z");
    const expiryDate = new Date(futureExpiry + "T16:00:00");
    const T = Math.max(
      (expiryDate.getTime() - now.getTime()) / MS_PER_YEAR,
      0
    );

    const result = computeT0ForPositions(positions, defaultPayoffData, getGreeks, now);

    // Compute with a different rate to prove r=0.05 matters
    const expectedWith010 = calculateT0Payoffs({
      prices: defaultPrices,
      legs: [
        { strike_price: 500, option_type: "call", qty: 1, avg_entry_price: 15, iv, T },
      ],
      riskFreeRate: 0.10,
      creditAdjustment: 0,
    });

    // At least some price points should differ
    let anyDifference = false;
    for (let i = 0; i < defaultPrices.length; i++) {
      if (Math.abs(result[i] - expectedWith010[i]) > 0.01) {
        anyDifference = true;
        break;
      }
    }
    expect(anyDifference).toBe(true);
  });
});
