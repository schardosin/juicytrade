/**
 * QA-specific tests for blackScholes.js
 *
 * These tests cover gaps not already in the dev's blackScholes.test.js:
 *  1. Zero-dependency verification (code quality)
 *  2. BS with very large T (T=10 years)
 *  3. BS with very large sigma (sigma=5.0)
 *  4. BS call price >= intrinsic value when T>0
 *  5. BS put price >= intrinsic value when T>0
 *  6. calculateT0Payoffs: creditAdjustment with empty legs exact value check
 *  7. calculateT0Payoffs: T=0 vertical spread matches manual expiration payoffs (AC-4)
 *  8. calculateT0Payoffs: very small T convergence to intrinsic
 *  9. Exports verification (exactly 3 named exports)
 */

import { describe, it, expect } from "vitest";
import { readFileSync } from "fs";
import { resolve } from "path";
import {
  cumulativeNormalDistribution,
  blackScholesPrice,
  calculateT0Payoffs,
} from "../src/utils/blackScholes.js";

// ---------------------------------------------------------------------------
// 1. Zero-dependency verification
// ---------------------------------------------------------------------------
describe("QA: Zero-dependency verification", () => {
  it("blackScholes.js contains no import or require statements", () => {
    const filePath = resolve(__dirname, "../src/utils/blackScholes.js");
    const source = readFileSync(filePath, "utf-8");

    // Check for ES module imports (import ... from ...)
    const importRegex = /^\s*import\s+.+\s+from\s+/m;
    expect(importRegex.test(source)).toBe(false);

    // Check for dynamic import()
    const dynamicImportRegex = /\bimport\s*\(/;
    expect(dynamicImportRegex.test(source)).toBe(false);

    // Check for CommonJS require()
    const requireRegex = /\brequire\s*\(/;
    expect(requireRegex.test(source)).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// 2. BS with very large T (T=10 years) — no NaN/Infinity
// ---------------------------------------------------------------------------
describe("QA: blackScholesPrice with very large T", () => {
  const largeT = 10; // 10 years

  it("call with T=10 years returns finite positive value", () => {
    const price = blackScholesPrice({
      optionType: "call",
      S: 100,
      K: 100,
      T: largeT,
      r: 0.05,
      sigma: 0.2,
    });
    expect(Number.isFinite(price)).toBe(true);
    expect(Number.isNaN(price)).toBe(false);
    expect(price).toBeGreaterThan(0);
  });

  it("put with T=10 years returns finite positive value", () => {
    const price = blackScholesPrice({
      optionType: "put",
      S: 100,
      K: 100,
      T: largeT,
      r: 0.05,
      sigma: 0.2,
    });
    expect(Number.isFinite(price)).toBe(true);
    expect(Number.isNaN(price)).toBe(false);
    expect(price).toBeGreaterThan(0);
  });

  it("put-call parity still holds at T=10", () => {
    const params = { S: 100, K: 100, T: largeT, r: 0.05, sigma: 0.2 };
    const C = blackScholesPrice({ optionType: "call", ...params });
    const P = blackScholesPrice({ optionType: "put", ...params });
    const parity = params.S - params.K * Math.exp(-params.r * params.T);
    expect(C - P).toBeCloseTo(parity, 4);
  });

  it("deep OTM call with T=10 returns finite non-negative value", () => {
    const price = blackScholesPrice({
      optionType: "call",
      S: 50,
      K: 200,
      T: largeT,
      r: 0.05,
      sigma: 0.2,
    });
    expect(Number.isFinite(price)).toBe(true);
    expect(price).toBeGreaterThanOrEqual(0);
  });

  it("deep ITM put with T=10 returns finite positive value", () => {
    const price = blackScholesPrice({
      optionType: "put",
      S: 50,
      K: 200,
      T: largeT,
      r: 0.05,
      sigma: 0.2,
    });
    expect(Number.isFinite(price)).toBe(true);
    expect(price).toBeGreaterThan(0);
  });
});

// ---------------------------------------------------------------------------
// 3. BS with very large sigma (sigma=5.0) — no NaN/Infinity
// ---------------------------------------------------------------------------
describe("QA: blackScholesPrice with very large sigma", () => {
  const largeSigma = 5.0; // 500% IV

  it("call with sigma=5.0 returns finite positive value", () => {
    const price = blackScholesPrice({
      optionType: "call",
      S: 100,
      K: 100,
      T: 1,
      r: 0.05,
      sigma: largeSigma,
    });
    expect(Number.isFinite(price)).toBe(true);
    expect(Number.isNaN(price)).toBe(false);
    expect(price).toBeGreaterThan(0);
  });

  it("put with sigma=5.0 returns finite positive value", () => {
    const price = blackScholesPrice({
      optionType: "put",
      S: 100,
      K: 100,
      T: 1,
      r: 0.05,
      sigma: largeSigma,
    });
    expect(Number.isFinite(price)).toBe(true);
    expect(Number.isNaN(price)).toBe(false);
    expect(price).toBeGreaterThan(0);
  });

  it("put-call parity holds at sigma=5.0", () => {
    const params = { S: 100, K: 100, T: 1, r: 0.05, sigma: largeSigma };
    const C = blackScholesPrice({ optionType: "call", ...params });
    const P = blackScholesPrice({ optionType: "put", ...params });
    const parity = params.S - params.K * Math.exp(-params.r * params.T);
    expect(C - P).toBeCloseTo(parity, 4);
  });

  it("call price with sigma=5.0 should not exceed S", () => {
    // A European call can never be worth more than S
    const price = blackScholesPrice({
      optionType: "call",
      S: 100,
      K: 100,
      T: 1,
      r: 0.05,
      sigma: largeSigma,
    });
    expect(price).toBeLessThanOrEqual(100);
  });

  it("put price with sigma=5.0 should not exceed K*e^(-rT)", () => {
    // A European put can never be worth more than K*e^(-rT)
    const price = blackScholesPrice({
      optionType: "put",
      S: 100,
      K: 100,
      T: 1,
      r: 0.05,
      sigma: largeSigma,
    });
    expect(price).toBeLessThanOrEqual(100 * Math.exp(-0.05));
  });

  it("combined extreme: T=10, sigma=5.0 produces finite values", () => {
    const callPrice = blackScholesPrice({
      optionType: "call",
      S: 100,
      K: 100,
      T: 10,
      r: 0.05,
      sigma: largeSigma,
    });
    const putPrice = blackScholesPrice({
      optionType: "put",
      S: 100,
      K: 100,
      T: 10,
      r: 0.05,
      sigma: largeSigma,
    });
    expect(Number.isFinite(callPrice)).toBe(true);
    expect(Number.isFinite(putPrice)).toBe(true);
    expect(callPrice).toBeGreaterThan(0);
    expect(putPrice).toBeGreaterThan(0);
  });
});

// ---------------------------------------------------------------------------
// 4. BS call price is always >= intrinsic value (for T > 0)
// ---------------------------------------------------------------------------
describe("QA: call price >= intrinsic value for T > 0", () => {
  const testCases = [
    // [S, K, description]
    [120, 100, "deep ITM"],
    [105, 100, "slightly ITM"],
    [100, 100, "ATM"],
    [95, 100, "slightly OTM"],
    [80, 100, "deep OTM"],
    [50, 150, "very deep OTM"],
    [200, 100, "very deep ITM"],
    [100, 50, "deep ITM, low strike"],
  ];

  for (const [S, K, desc] of testCases) {
    it(`call price >= max(S-K, 0) when ${desc} (S=${S}, K=${K})`, () => {
      const price = blackScholesPrice({
        optionType: "call",
        S,
        K,
        T: 0.25,
        r: 0.05,
        sigma: 0.3,
      });
      const intrinsic = Math.max(S - K, 0);
      expect(price).toBeGreaterThanOrEqual(intrinsic - 0.001); // tiny tolerance for rounding
    });
  }

  it("call price >= intrinsic across a sweep of S values", () => {
    for (let S = 50; S <= 150; S += 5) {
      const K = 100;
      const price = blackScholesPrice({
        optionType: "call",
        S,
        K,
        T: 0.5,
        r: 0.05,
        sigma: 0.25,
      });
      const intrinsic = Math.max(S - K, 0);
      expect(price).toBeGreaterThanOrEqual(intrinsic - 0.001);
    }
  });
});

// ---------------------------------------------------------------------------
// 5. BS put price is always >= intrinsic value (for T > 0)
// ---------------------------------------------------------------------------
describe("QA: put price >= intrinsic value for T > 0", () => {
  const testCases = [
    [80, 100, "deep ITM put"],
    [95, 100, "slightly ITM put"],
    [100, 100, "ATM"],
    [105, 100, "slightly OTM put"],
    [120, 100, "deep OTM put"],
    [150, 50, "very deep OTM put"],
    [50, 200, "very deep ITM put"],
  ];

  for (const [S, K, desc] of testCases) {
    it(`put price >= max(K-S, 0) when ${desc} (S=${S}, K=${K})`, () => {
      const price = blackScholesPrice({
        optionType: "put",
        S,
        K,
        T: 0.25,
        r: 0.05,
        sigma: 0.3,
      });
      const intrinsic = Math.max(K - S, 0);
      // For European puts, price can technically be slightly below intrinsic
      // due to discounting, but should be very close. Use tolerance.
      // Actually: European put can be < intrinsic for deep ITM with high r.
      // The correct bound is: P >= max(K*e^(-rT) - S, 0)
      const lowerBound = Math.max(K * Math.exp(-0.05 * 0.25) - S, 0);
      expect(price).toBeGreaterThanOrEqual(lowerBound - 0.001);
    });
  }

  it("put price >= discounted intrinsic across a sweep of S values", () => {
    const T = 0.5;
    const r = 0.05;
    for (let S = 50; S <= 150; S += 5) {
      const K = 100;
      const price = blackScholesPrice({
        optionType: "put",
        S,
        K,
        T,
        r,
        sigma: 0.25,
      });
      const lowerBound = Math.max(K * Math.exp(-r * T) - S, 0);
      expect(price).toBeGreaterThanOrEqual(lowerBound - 0.001);
    }
  });
});

// ---------------------------------------------------------------------------
// 6. calculateT0Payoffs: creditAdjustment with empty legs — exact value
// ---------------------------------------------------------------------------
describe("QA: calculateT0Payoffs creditAdjustment with empty legs", () => {
  it("empty legs + creditAdjustment=42.50 returns array of exactly 42.50", () => {
    const prices = [80, 90, 100, 110, 120];
    const result = calculateT0Payoffs({
      prices,
      legs: [],
      creditAdjustment: 42.5,
    });

    expect(result).toHaveLength(prices.length);
    for (const val of result) {
      expect(val).toBe(42.5);
    }
  });

  it("empty legs + creditAdjustment=-123.45 returns array of exactly -123.45", () => {
    const prices = [50, 100, 150];
    const result = calculateT0Payoffs({
      prices,
      legs: [],
      creditAdjustment: -123.45,
    });

    expect(result).toHaveLength(prices.length);
    for (const val of result) {
      expect(val).toBe(-123.45);
    }
  });

  it("empty legs + creditAdjustment=0 returns array of exactly 0", () => {
    const prices = [100, 200];
    const result = calculateT0Payoffs({
      prices,
      legs: [],
      creditAdjustment: 0,
    });

    expect(result).toHaveLength(prices.length);
    for (const val of result) {
      expect(val).toBe(0);
    }
  });

  it("null legs + creditAdjustment returns array of creditAdjustment", () => {
    const prices = [90, 100, 110];
    const result = calculateT0Payoffs({
      prices,
      legs: null,
      creditAdjustment: 500,
    });

    expect(result).toHaveLength(prices.length);
    for (const val of result) {
      expect(val).toBe(500);
    }
  });
});

// ---------------------------------------------------------------------------
// 7. AC-4: T=0 vertical spread payoffs match manual expiration payoffs
// ---------------------------------------------------------------------------
describe("QA: AC-4 — T=0 bull call spread matches expiration payoffs", () => {
  // Bull call spread: long call at K1=100, short call at K2=110
  // Net debit = 7 - 3 = $4 per share
  const K1 = 100; // long call strike
  const K2 = 110; // short call strike
  const longEntry = 7;
  const shortEntry = 3;
  const netDebit = longEntry - shortEntry; // $4 per share

  const legs = [
    {
      strike_price: K1,
      option_type: "call",
      qty: 1,
      avg_entry_price: longEntry,
      iv: 0.25,
      T: 0,
    },
    {
      strike_price: K2,
      option_type: "call",
      qty: -1,
      avg_entry_price: shortEntry,
      iv: 0.25,
      T: 0,
    },
  ];

  /**
   * Manual expiration payoff for a bull call spread at price S:
   *   Long call payoff = (max(S - K1, 0) - longEntry) * 100
   *   Short call payoff = (max(S - K2, 0) - shortEntry) * (-1) * 100
   *   Total = long + short
   */
  function manualExpirationPayoff(S) {
    const longPL = (Math.max(S - K1, 0) - longEntry) * 1 * 100;
    const shortPL = (Math.max(S - K2, 0) - shortEntry) * -1 * 100;
    return longPL + shortPL;
  }

  it("T=0 payoffs match manual expiration payoffs across price range", () => {
    const prices = [80, 85, 90, 95, 100, 102, 105, 108, 110, 115, 120, 130];
    const result = calculateT0Payoffs({ prices, legs });

    for (let i = 0; i < prices.length; i++) {
      const expected = manualExpirationPayoff(prices[i]);
      expect(result[i]).toBeCloseTo(expected, 1);
    }
  });

  it("below lower strike: max loss = -netDebit * 100 = -$400", () => {
    const prices = [70, 80, 90, 95, 100];
    const result = calculateT0Payoffs({ prices, legs });

    // Below K1=100, both calls expire worthless
    // P&L = (0 - 7)*100 + (0 - 3)*(-1)*100 = -700 + 300 = -400
    for (let i = 0; i < prices.length - 1; i++) {
      // All prices below K1 should have the same max loss
      expect(result[i]).toBeCloseTo(-netDebit * 100, 1); // -$400
    }
  });

  it("above upper strike: max profit = (K2 - K1 - netDebit) * 100 = $600", () => {
    const prices = [110, 115, 120, 130, 150];
    const result = calculateT0Payoffs({ prices, legs });

    const maxProfit = (K2 - K1 - netDebit) * 100; // (110-100-4)*100 = $600
    for (const val of result) {
      expect(val).toBeCloseTo(maxProfit, 1);
    }
  });

  it("between strikes: P&L is linearly increasing", () => {
    const prices = [101, 103, 105, 107, 109];
    const result = calculateT0Payoffs({ prices, legs });

    // Between K1 and K2, long call has value, short call is worthless
    // P&L increases with S
    for (let i = 1; i < result.length; i++) {
      expect(result[i]).toBeGreaterThan(result[i - 1]);
    }
  });

  it("breakeven at S = K1 + netDebit = 104", () => {
    const prices = [104];
    const result = calculateT0Payoffs({ prices, legs });

    // At S=104: long call = 4 (ITM by 4), short call = 0 (OTM)
    // P&L = (4-7)*100 + (0-3)*(-1)*100 = -300 + 300 = 0
    expect(result[0]).toBeCloseTo(0, 1);
  });
});

// ---------------------------------------------------------------------------
// 8. Very small T convergence: T=0.001 should approximate intrinsic
// ---------------------------------------------------------------------------
describe("QA: calculateT0Payoffs very small T convergence", () => {
  it("single long call at T=0.001 produces values close to intrinsic P&L", () => {
    const K = 100;
    const entryPrice = 5;
    const tinyT = 0.001; // ~8.76 hours
    const prices = [80, 90, 95, 100, 105, 110, 120];

    const result = calculateT0Payoffs({
      prices,
      legs: [
        {
          strike_price: K,
          option_type: "call",
          qty: 1,
          avg_entry_price: entryPrice,
          iv: 0.25,
          T: tinyT,
        },
      ],
    });

    for (let i = 0; i < prices.length; i++) {
      const intrinsic = Math.max(prices[i] - K, 0);
      const intrinsicPL = (intrinsic - entryPrice) * 100;
      // With T=0.001, time value is very small, so result should be close
      // to intrinsic P&L. Allow tolerance of $50 (~$0.50/share time value)
      expect(Math.abs(result[i] - intrinsicPL)).toBeLessThan(50);
    }
  });

  it("single long put at T=0.001 produces values close to intrinsic P&L", () => {
    const K = 100;
    const entryPrice = 5;
    const tinyT = 0.001;
    const prices = [80, 90, 95, 100, 105, 110, 120];

    const result = calculateT0Payoffs({
      prices,
      legs: [
        {
          strike_price: K,
          option_type: "put",
          qty: 1,
          avg_entry_price: entryPrice,
          iv: 0.25,
          T: tinyT,
        },
      ],
    });

    for (let i = 0; i < prices.length; i++) {
      const intrinsic = Math.max(K - prices[i], 0);
      const intrinsicPL = (intrinsic - entryPrice) * 100;
      expect(Math.abs(result[i] - intrinsicPL)).toBeLessThan(50);
    }
  });

  it("T=0.001 values are closer to intrinsic than T=0.1 values", () => {
    const K = 100;
    const entryPrice = 5;
    const prices = [90, 100, 110];

    const tinyResult = calculateT0Payoffs({
      prices,
      legs: [
        {
          strike_price: K,
          option_type: "call",
          qty: 1,
          avg_entry_price: entryPrice,
          iv: 0.25,
          T: 0.001,
        },
      ],
    });

    const largerResult = calculateT0Payoffs({
      prices,
      legs: [
        {
          strike_price: K,
          option_type: "call",
          qty: 1,
          avg_entry_price: entryPrice,
          iv: 0.25,
          T: 0.1,
        },
      ],
    });

    // For each ATM/near-money price, the tiny-T result should be closer
    // to intrinsic than the larger-T result
    for (let i = 0; i < prices.length; i++) {
      const intrinsic = Math.max(prices[i] - K, 0);
      const intrinsicPL = (intrinsic - entryPrice) * 100;
      const tinyDiff = Math.abs(tinyResult[i] - intrinsicPL);
      const largerDiff = Math.abs(largerResult[i] - intrinsicPL);
      expect(tinyDiff).toBeLessThanOrEqual(largerDiff + 0.01); // small tolerance
    }
  });
});

// ---------------------------------------------------------------------------
// 9. Exports verification: exactly 3 named exports
// ---------------------------------------------------------------------------
describe("QA: exports verification", () => {
  it("blackScholes.js exports exactly 3 functions", async () => {
    const mod = await import("../src/utils/blackScholes.js");
    const exportNames = Object.keys(mod).sort();
    expect(exportNames).toEqual(
      [
        "blackScholesPrice",
        "calculateT0Payoffs",
        "cumulativeNormalDistribution",
      ].sort()
    );
  });

  it("cumulativeNormalDistribution is a function", () => {
    expect(typeof cumulativeNormalDistribution).toBe("function");
  });

  it("blackScholesPrice is a function", () => {
    expect(typeof blackScholesPrice).toBe("function");
  });

  it("calculateT0Payoffs is a function", () => {
    expect(typeof calculateT0Payoffs).toBe("function");
  });

  it("there is no default export", async () => {
    const mod = await import("../src/utils/blackScholes.js");
    expect(mod.default).toBeUndefined();
  });
});
