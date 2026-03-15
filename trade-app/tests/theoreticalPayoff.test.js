import { describe, it, expect } from "vitest";
import {
  calculateTimesToExpiry,
  calculateWeightedAverageIV,
  calculateTheoreticalPayoffs,
} from "../src/utils/theoreticalPayoff.js";

const MS_PER_YEAR = 365.25 * 24 * 60 * 60 * 1000;

function makePosition(overrides = {}) {
  return {
    symbol: "SPY_CALL_500",
    asset_class: "us_option",
    qty: 1,
    strike_price: 500,
    option_type: "call",
    expiry_date: "2026-06-19",
    avg_entry_price: 10,
    ...overrides,
  };
}

function mockGetIV(ivMap) {
  return (symbol) => ivMap[symbol] ?? null;
}

describe("calculateTimesToExpiry", () => {
  it("returns empty array for null positions", () => {
    const result = calculateTimesToExpiry(new Date(), null);
    expect(result).toEqual([]);
  });

  it("returns empty array for empty positions", () => {
    const result = calculateTimesToExpiry(new Date(), []);
    expect(result).toEqual([]);
  });

  it("returns positive T for future expiry", () => {
    const now = new Date("2026-01-01T12:00:00");
    const positions = [makePosition({ expiry_date: "2026-06-19" })];

    const result = calculateTimesToExpiry(now, positions);

    expect(result).toHaveLength(1);
    expect(result[0]).toBeGreaterThan(0);
  });

  it("returns 0 for past expiry", () => {
    const now = new Date("2027-01-01T12:00:00");
    const positions = [makePosition({ expiry_date: "2026-06-19" })];

    const result = calculateTimesToExpiry(now, positions);

    expect(result).toHaveLength(1);
    expect(result[0]).toBe(0);
  });

  it("returns T=0 when selected date matches expiry calendar day (midnight)", () => {
    const selectedDate = new Date("2026-06-19T00:00:00");
    const positions = [makePosition({ expiry_date: "2026-06-19" })];

    const result = calculateTimesToExpiry(selectedDate, positions);

    expect(result).toHaveLength(1);
    expect(result[0]).toBe(0);
  });

  it("returns T=0 when selected date is later in the expiry day (9 AM)", () => {
    const selectedDate = new Date("2026-06-19T09:00:00");
    const positions = [makePosition({ expiry_date: "2026-06-19" })];

    const result = calculateTimesToExpiry(selectedDate, positions);

    expect(result).toHaveLength(1);
    expect(result[0]).toBe(0);
  });

  it("uses 4 PM market close time for expiry (non-expiry day)", () => {
    const now = new Date("2026-06-18T09:00:00");
    const positions = [makePosition({ expiry_date: "2026-06-19" })];

    const result = calculateTimesToExpiry(now, positions);

    expect(result[0]).toBeGreaterThan(0);
    // From June 18 09:00 to June 19 16:00 = 31 hours
    const expectedHours = 31;
    const expectedT = expectedHours / 24 / 365.25;
    expect(result[0]).toBeCloseTo(expectedT, 4);
  });

  it("returns 0 for position missing expiry_date", () => {
    const now = new Date("2026-01-01T12:00:00");
    const positions = [makePosition({ expiry_date: undefined })];

    const result = calculateTimesToExpiry(now, positions);

    expect(result).toHaveLength(1);
    expect(result[0]).toBe(0);
  });

  it("calculates correct T for multiple legs with different expiries", () => {
    const now = new Date("2026-01-15T12:00:00");
    const positions = [
      makePosition({ expiry_date: "2026-02-15" }),
      makePosition({ expiry_date: "2026-03-15" }),
      makePosition({ expiry_date: "2026-04-15" }),
    ];

    const result = calculateTimesToExpiry(now, positions);

    expect(result).toHaveLength(3);
    expect(result[0]).toBeGreaterThan(0);
    expect(result[1]).toBeGreaterThan(result[0]);
    expect(result[2]).toBeGreaterThan(result[1]);
  });

  it("handles mixed expiry statuses (some past, some future)", () => {
    const now = new Date("2026-07-01T12:00:00");
    const positions = [
      makePosition({ expiry_date: "2026-06-01" }),
      makePosition({ expiry_date: "2026-08-01" }),
      makePosition({ expiry_date: "2026-12-01" }),
    ];

    const result = calculateTimesToExpiry(now, positions);

    expect(result).toHaveLength(3);
    expect(result[0]).toBe(0);
    expect(result[1]).toBeGreaterThan(0);
    expect(result[2]).toBeGreaterThan(result[1]);
  });

  it("uses 365.25 days per year for accurate time calculation", () => {
    const now = new Date("2026-01-01T12:00:00");
    const expiry = new Date("2027-01-01T16:00:00");
    const positions = [makePosition({ expiry_date: "2027-01-01" })];

    const result = calculateTimesToExpiry(now, positions);

    const msToExpiry = expiry.getTime() - now.getTime();
    const expectedT = msToExpiry / MS_PER_YEAR;
    expect(result[0]).toBeCloseTo(expectedT, 10);
  });
});

describe("calculateWeightedAverageIV", () => {
  it("returns null for null positions", () => {
    const getIV = mockGetIV({ SPY_CALL_500: 0.25 });
    const result = calculateWeightedAverageIV(null, getIV);
    expect(result).toBeNull();
  });

  it("returns null for empty positions", () => {
    const getIV = mockGetIV({ SPY_CALL_500: 0.25 });
    const result = calculateWeightedAverageIV([], getIV);
    expect(result).toBeNull();
  });

  it("returns null when any leg has missing IV", () => {
    const positions = [
      makePosition({ symbol: "SPY_CALL_500", qty: 1, strike_price: 500 }),
      makePosition({ symbol: "SPY_CALL_510", qty: -1, strike_price: 510 }),
    ];
    const getIV = mockGetIV({
      SPY_CALL_500: 0.25,
      SPY_CALL_510: null,
    });

    const result = calculateWeightedAverageIV(positions, getIV);

    expect(result).toBeNull();
  });

  it("returns single leg IV when only one position", () => {
    const positions = [makePosition({ qty: 1, strike_price: 500 })];
    const getIV = mockGetIV({ SPY_CALL_500: 0.25 });

    const result = calculateWeightedAverageIV(positions, getIV);

    expect(result).toBeCloseTo(0.25, 6);
  });

  it("weights by absolute notional (|qty| * strike)", () => {
    const positions = [
      makePosition({ symbol: "LEG1", qty: 1, strike_price: 100 }),
      makePosition({ symbol: "LEG2", qty: 2, strike_price: 200 }),
    ];
    const getIV = mockGetIV({ LEG1: 0.20, LEG2: 0.30 });

    const result = calculateWeightedAverageIV(positions, getIV);

    const notional1 = 1 * 100;
    const notional2 = 2 * 200;
    const totalNotional = notional1 + notional2;
    const expected = (0.20 * notional1 + 0.30 * notional2) / totalNotional;
    expect(result).toBeCloseTo(expected, 6);
  });

  it("treats short positions (negative qty) as positive weight", () => {
    const positions = [
      makePosition({ symbol: "LONG", qty: 1, strike_price: 100 }),
      makePosition({ symbol: "SHORT", qty: -2, strike_price: 100 }),
    ];
    const getIV = mockGetIV({ LONG: 0.20, SHORT: 0.30 });

    const result = calculateWeightedAverageIV(positions, getIV);

    const notional1 = 1 * 100;
    const notional2 = 2 * 100;
    const totalNotional = notional1 + notional2;
    const expected = (0.20 * notional1 + 0.30 * notional2) / totalNotional;
    expect(result).toBeCloseTo(expected, 6);
  });

  it("returns null for zero or negative IV", () => {
    const positions = [makePosition({ symbol: "SPY_CALL_500" })];
    const getIV = mockGetIV({ SPY_CALL_500: 0 });
    const result = calculateWeightedAverageIV(positions, getIV);
    expect(result).toBeNull();
  });

  it("returns null for NaN IV", () => {
    const positions = [makePosition({ symbol: "SPY_CALL_500" })];
    const getIV = mockGetIV({ SPY_CALL_500: NaN });
    const result = calculateWeightedAverageIV(positions, getIV);
    expect(result).toBeNull();
  });

  it("returns null when all positions have zero notional", () => {
    const positions = [makePosition({ qty: 0, strike_price: 0 })];
    const getIV = mockGetIV({ SPY_CALL_500: 0.25 });
    const result = calculateWeightedAverageIV(positions, getIV);
    expect(result).toBeNull();
  });
});

describe("calculateTheoreticalPayoffs", () => {
  const defaultPrices = [480, 490, 500, 510, 520];
  const defaultSelectedDate = new Date("2026-01-15T12:00:00");
  const defaultExpiryDate = "2026-06-19";

  it("returns empty array for null prices", () => {
    const positions = [makePosition()];
    const getIV = mockGetIV({ SPY_CALL_500: 0.25 });
    const result = calculateTheoreticalPayoffs({
      prices: null,
      positions,
      selectedDate: defaultSelectedDate,
      selectedIV: null,
      getIV,
    });
    expect(result).toEqual([]);
  });

  it("returns empty array for empty prices", () => {
    const positions = [makePosition()];
    const getIV = mockGetIV({ SPY_CALL_500: 0.25 });
    const result = calculateTheoreticalPayoffs({
      prices: [],
      positions,
      selectedDate: defaultSelectedDate,
      selectedIV: null,
      getIV,
    });
    expect(result).toEqual([]);
  });

  it("returns creditAdjustment array when positions empty", () => {
    const result = calculateTheoreticalPayoffs({
      prices: defaultPrices,
      positions: [],
      selectedDate: defaultSelectedDate,
      selectedIV: null,
      getIV: () => null,
      creditAdjustment: 100,
    });

    expect(result).toHaveLength(defaultPrices.length);
    for (const val of result) {
      expect(val).toBeCloseTo(100, 1);
    }
  });

  it("returns creditAdjustment array when positions null", () => {
    const result = calculateTheoreticalPayoffs({
      prices: defaultPrices,
      positions: null,
      selectedDate: defaultSelectedDate,
      selectedIV: null,
      getIV: () => null,
      creditAdjustment: 50,
    });

    expect(result).toHaveLength(defaultPrices.length);
    for (const val of result) {
      expect(val).toBeCloseTo(50, 1);
    }
  });

  it("returns null when IV is unavailable", () => {
    const positions = [makePosition()];
    const getIV = mockGetIV({ SPY_CALL_500: null });

    const result = calculateTheoreticalPayoffs({
      prices: defaultPrices,
      positions,
      selectedDate: defaultSelectedDate,
      selectedIV: null,
      getIV,
    });

    expect(result).toBeNull();
  });

  it("uses selectedIV override when provided", () => {
    const positions = [makePosition({ expiry_date: defaultExpiryDate })];
    const getIV = mockGetIV({ SPY_CALL_500: 0.50 });

    const resultWithOverride = calculateTheoreticalPayoffs({
      prices: defaultPrices,
      positions,
      selectedDate: defaultSelectedDate,
      selectedIV: 0.25,
      getIV,
    });

    expect(resultWithOverride).not.toBeNull();
    expect(resultWithOverride).toHaveLength(defaultPrices.length);
  });

  it("uses getIV when selectedIV is null", () => {
    const positions = [makePosition({ expiry_date: defaultExpiryDate })];
    const getIV = mockGetIV({ SPY_CALL_500: 0.25 });

    const result = calculateTheoreticalPayoffs({
      prices: defaultPrices,
      positions,
      selectedDate: defaultSelectedDate,
      selectedIV: null,
      getIV,
    });

    expect(result).not.toBeNull();
    expect(result).toHaveLength(defaultPrices.length);
  });

  it("returns null when any leg has zero IV and no override", () => {
    const positions = [
      makePosition({ symbol: "LEG1", expiry_date: defaultExpiryDate }),
      makePosition({ symbol: "LEG2", expiry_date: defaultExpiryDate }),
    ];
    const getIV = mockGetIV({ LEG1: 0.25, LEG2: 0 });

    const result = calculateTheoreticalPayoffs({
      prices: defaultPrices,
      positions,
      selectedDate: defaultSelectedDate,
      selectedIV: null,
      getIV,
    });

    expect(result).toBeNull();
  });

  it("calculates correct T for future expiry", () => {
    const now = new Date("2026-01-15T12:00:00");
    const positions = [makePosition({
      expiry_date: "2026-03-15",
      strike_price: 500,
      option_type: "call",
      qty: 1,
      avg_entry_price: 10,
    })];
    const getIV = mockGetIV({ SPY_CALL_500: 0.25 });

    const result = calculateTheoreticalPayoffs({
      prices: defaultPrices,
      positions,
      selectedDate: now,
      selectedIV: 0.25,
      getIV,
    });

    expect(result).not.toBeNull();
    expect(result).toHaveLength(defaultPrices.length);
    for (const val of result) {
      expect(Number.isFinite(val)).toBe(true);
    }
  });

  it("handles expired legs (T=0) correctly", () => {
    const now = new Date("2027-07-01T12:00:00");
    const positions = [makePosition({
      expiry_date: "2027-06-01",
      strike_price: 500,
      option_type: "call",
      qty: 1,
      avg_entry_price: 10,
    })];
    const getIV = mockGetIV({ SPY_CALL_500: 0.25 });

    const result = calculateTheoreticalPayoffs({
      prices: defaultPrices,
      positions,
      selectedDate: now,
      selectedIV: 0.25,
      getIV,
    });

    expect(result).not.toBeNull();
    expect(result).toHaveLength(defaultPrices.length);
  });

  it("handles multi-leg positions correctly", () => {
    const positions = [
      makePosition({ symbol: "PUT_480", option_type: "put", qty: 1, strike_price: 480 }),
      makePosition({ symbol: "PUT_490", option_type: "put", qty: -1, strike_price: 490 }),
      makePosition({ symbol: "CALL_510", option_type: "call", qty: -1, strike_price: 510 }),
      makePosition({ symbol: "CALL_520", option_type: "call", qty: 1, strike_price: 520 }),
    ];
    const getIV = mockGetIV({
      PUT_480: 0.20,
      PUT_490: 0.20,
      CALL_510: 0.20,
      CALL_520: 0.20,
    });

    const result = calculateTheoreticalPayoffs({
      prices: defaultPrices,
      positions,
      selectedDate: defaultSelectedDate,
      selectedIV: 0.20,
      getIV,
    });

    expect(result).not.toBeNull();
    expect(result).toHaveLength(defaultPrices.length);
    for (const val of result) {
      expect(Number.isFinite(val)).toBe(true);
    }
  });

  it("applies creditAdjustment correctly", () => {
    const positions = [makePosition()];
    const getIV = mockGetIV({ SPY_CALL_500: 0.25 });

    const resultNoAdjust = calculateTheoreticalPayoffs({
      prices: defaultPrices,
      positions,
      selectedDate: defaultSelectedDate,
      selectedIV: 0.25,
      getIV,
      creditAdjustment: 0,
    });

    const resultWithAdjust = calculateTheoreticalPayoffs({
      prices: defaultPrices,
      positions,
      selectedDate: defaultSelectedDate,
      selectedIV: 0.25,
      getIV,
      creditAdjustment: 200,
    });

    expect(resultWithAdjust).not.toBeNull();
    expect(resultNoAdjust).not.toBeNull();

    for (let i = 0; i < defaultPrices.length; i++) {
      expect(resultWithAdjust[i]).toBeCloseTo(resultNoAdjust[i] + 200, 1);
    }
  });

  it("uses default riskFreeRate of 0.05", () => {
    const positions = [makePosition()];
    const getIV = mockGetIV({ SPY_CALL_500: 0.25 });

    const resultDefault = calculateTheoreticalPayoffs({
      prices: defaultPrices,
      positions,
      selectedDate: defaultSelectedDate,
      selectedIV: 0.25,
      getIV,
    });

    const resultExplicit = calculateTheoreticalPayoffs({
      prices: defaultPrices,
      positions,
      selectedDate: defaultSelectedDate,
      selectedIV: 0.25,
      getIV,
      riskFreeRate: 0.05,
    });

    expect(resultDefault).not.toBeNull();
    expect(resultExplicit).not.toBeNull();

    for (let i = 0; i < defaultPrices.length; i++) {
      expect(resultDefault[i]).toBeCloseTo(resultExplicit[i], 6);
    }
  });

  it("handles avg_entry_price defaulting to 0", () => {
    const positions = [makePosition({ avg_entry_price: undefined })];
    const getIV = mockGetIV({ SPY_CALL_500: 0.25 });

    const result = calculateTheoreticalPayoffs({
      prices: defaultPrices,
      positions,
      selectedDate: defaultSelectedDate,
      selectedIV: 0.25,
      getIV,
    });

    expect(result).not.toBeNull();
    expect(result).toHaveLength(defaultPrices.length);
    for (const val of result) {
      expect(Number.isFinite(val)).toBe(true);
    }
  });

  it("returns different values for different T values", () => {
    const positions = [makePosition({
      strike_price: 500,
      option_type: "call",
      qty: 1,
      avg_entry_price: 10,
    })];
    const getIV = mockGetIV({ SPY_CALL_500: 0.25 });

    const now = new Date("2026-01-15T12:00:00");
    const closer = new Date("2026-05-01T12:00:00");

    const resultNow = calculateTheoreticalPayoffs({
      prices: [500],
      positions,
      selectedDate: now,
      selectedIV: 0.25,
      getIV,
    });

    const resultCloser = calculateTheoreticalPayoffs({
      prices: [500],
      positions,
      selectedDate: closer,
      selectedIV: 0.25,
      getIV,
    });

    expect(resultNow).not.toBeNull();
    expect(resultCloser).not.toBeNull();
    expect(resultNow[0]).not.toBeCloseTo(resultCloser[0], 1);
  });

  it("theoretical P/L equals intrinsic value when selected date is expiration day", () => {
    // Long call: strike=500, qty=1, avg_entry_price=10, expiry=2026-06-19
    // At price=520: intrinsic = (520-500) * 1 * 100 = 2000
    // Cost = 10 * 1 * 100 = 1000
    // Expected P/L = 2000 - 1000 = 1000
    const positions = [makePosition({
      strike_price: 500,
      option_type: "call",
      qty: 1,
      avg_entry_price: 10,
      expiry_date: "2026-06-19",
    })];
    const getIV = mockGetIV({ SPY_CALL_500: 0.25 });

    const result = calculateTheoreticalPayoffs({
      prices: [520],
      positions,
      selectedDate: new Date("2026-06-19T00:00:00"),
      selectedIV: 0.25,
      getIV,
    });

    expect(result).not.toBeNull();
    expect(result).toHaveLength(1);
    expect(result[0]).toBeCloseTo(1000, 0);
  });
});