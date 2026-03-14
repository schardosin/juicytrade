import { describe, it, expect } from "vitest";
import {
  cumulativeNormalDistribution,
  blackScholesPrice,
  calculateT0Payoffs,
} from "../src/utils/blackScholes.js";

// ---------------------------------------------------------------------------
// 1. cumulativeNormalDistribution
// ---------------------------------------------------------------------------
describe("cumulativeNormalDistribution", () => {
  it("CND(0) equals 0.5 exactly", () => {
    expect(cumulativeNormalDistribution(0)).toBeCloseTo(0.5, 7);
  });

  it("CND(large positive) approaches 1.0", () => {
    expect(cumulativeNormalDistribution(10)).toBeCloseTo(1.0, 7);
  });

  it("CND(large negative) approaches 0.0", () => {
    expect(cumulativeNormalDistribution(-10)).toBeCloseTo(0.0, 7);
  });

  it("satisfies symmetry: CND(x) + CND(-x) = 1.0", () => {
    const testValues = [0.5, 1.0, 1.5, 2.0, 2.5, 3.0];
    for (const x of testValues) {
      const sum = cumulativeNormalDistribution(x) + cumulativeNormalDistribution(-x);
      expect(sum).toBeCloseTo(1.0, 7);
    }
  });

  it("CND(1.0) ≈ 0.8413", () => {
    expect(cumulativeNormalDistribution(1.0)).toBeCloseTo(0.8413, 4);
  });

  it("CND(-1.0) ≈ 0.1587", () => {
    expect(cumulativeNormalDistribution(-1.0)).toBeCloseTo(0.1587, 4);
  });

  it("CND(1.96) ≈ 0.975", () => {
    expect(cumulativeNormalDistribution(1.96)).toBeCloseTo(0.975, 3);
  });

  it("CND(-1.96) ≈ 0.025", () => {
    expect(cumulativeNormalDistribution(-1.96)).toBeCloseTo(0.025, 3);
  });

  it("CND(2.326) ≈ 0.99 (99th percentile)", () => {
    expect(cumulativeNormalDistribution(2.326)).toBeCloseTo(0.99, 2);
  });

  it("returns values strictly between 0 and 1 for finite inputs", () => {
    const testValues = [-5, -3, -1, 0, 1, 3, 5];
    for (const x of testValues) {
      const val = cumulativeNormalDistribution(x);
      expect(val).toBeGreaterThan(0);
      expect(val).toBeLessThan(1);
    }
  });

  it("is monotonically increasing", () => {
    let prev = cumulativeNormalDistribution(-5);
    for (let x = -4; x <= 5; x += 0.5) {
      const curr = cumulativeNormalDistribution(x);
      expect(curr).toBeGreaterThan(prev);
      prev = curr;
    }
  });
});

// ---------------------------------------------------------------------------
// 2. blackScholesPrice
// ---------------------------------------------------------------------------
describe("blackScholesPrice", () => {
  // Textbook example: S=100, K=100, T=1, r=0.05, sigma=0.20
  // Expected call price ≈ 10.4506 (from standard BS tables)
  const baseParams = { S: 100, K: 100, T: 1, r: 0.05, sigma: 0.2 };

  describe("standard pricing", () => {
    it("ATM call with known parameters matches textbook value", () => {
      const price = blackScholesPrice({ optionType: "call", ...baseParams });
      expect(price).toBeCloseTo(10.4506, 2);
    });

    it("ATM put with known parameters matches textbook value", () => {
      const price = blackScholesPrice({ optionType: "put", ...baseParams });
      // Put from put-call parity: P = C - S + K*e^(-rT) = 10.4506 - 100 + 100*e^(-0.05) ≈ 5.5735
      expect(price).toBeCloseTo(5.5735, 2);
    });

    it("satisfies put-call parity: C - P = S - K*e^(-rT)", () => {
      const C = blackScholesPrice({ optionType: "call", ...baseParams });
      const P = blackScholesPrice({ optionType: "put", ...baseParams });
      const parity = baseParams.S - baseParams.K * Math.exp(-baseParams.r * baseParams.T);
      expect(C - P).toBeCloseTo(parity, 6);
    });

    it("put-call parity holds for various parameters", () => {
      const testCases = [
        { S: 50, K: 55, T: 0.5, r: 0.03, sigma: 0.3 },
        { S: 200, K: 180, T: 0.25, r: 0.05, sigma: 0.15 },
        { S: 500, K: 500, T: 2, r: 0.04, sigma: 0.25 },
        { S: 100, K: 120, T: 0.1, r: 0.05, sigma: 0.4 },
      ];
      for (const params of testCases) {
        const C = blackScholesPrice({ optionType: "call", ...params });
        const P = blackScholesPrice({ optionType: "put", ...params });
        const parity = params.S - params.K * Math.exp(-params.r * params.T);
        expect(C - P).toBeCloseTo(parity, 5);
      }
    });

    it("deep ITM call price approaches S - K*e^(-rT)", () => {
      const price = blackScholesPrice({
        optionType: "call",
        S: 200,
        K: 100,
        T: 1,
        r: 0.05,
        sigma: 0.2,
      });
      const intrinsicWithDiscount = 200 - 100 * Math.exp(-0.05);
      // Deep ITM: price should be very close to discounted intrinsic
      expect(price).toBeCloseTo(intrinsicWithDiscount, 1);
    });

    it("deep OTM call price approaches 0", () => {
      const price = blackScholesPrice({
        optionType: "call",
        S: 50,
        K: 200,
        T: 0.1,
        r: 0.05,
        sigma: 0.2,
      });
      expect(price).toBeCloseTo(0, 4);
    });

    it("deep ITM put price approaches K*e^(-rT) - S", () => {
      const price = blackScholesPrice({
        optionType: "put",
        S: 50,
        K: 200,
        T: 1,
        r: 0.05,
        sigma: 0.2,
      });
      const intrinsicWithDiscount = 200 * Math.exp(-0.05) - 50;
      expect(price).toBeCloseTo(intrinsicWithDiscount, 1);
    });

    it("deep OTM put price approaches 0", () => {
      const price = blackScholesPrice({
        optionType: "put",
        S: 200,
        K: 50,
        T: 0.1,
        r: 0.05,
        sigma: 0.2,
      });
      expect(price).toBeCloseTo(0, 4);
    });

    it("option price increases with higher volatility", () => {
      const lowVol = blackScholesPrice({
        optionType: "call",
        S: 100,
        K: 100,
        T: 1,
        r: 0.05,
        sigma: 0.1,
      });
      const highVol = blackScholesPrice({
        optionType: "call",
        S: 100,
        K: 100,
        T: 1,
        r: 0.05,
        sigma: 0.4,
      });
      expect(highVol).toBeGreaterThan(lowVol);
    });

    it("option price increases with more time to expiration", () => {
      const shortTerm = blackScholesPrice({
        optionType: "call",
        S: 100,
        K: 100,
        T: 0.1,
        r: 0.05,
        sigma: 0.2,
      });
      const longTerm = blackScholesPrice({
        optionType: "call",
        S: 100,
        K: 100,
        T: 1,
        r: 0.05,
        sigma: 0.2,
      });
      expect(longTerm).toBeGreaterThan(shortTerm);
    });

    it("returns non-negative prices for all standard inputs", () => {
      const types = ["call", "put"];
      const strikes = [80, 100, 120];
      for (const ot of types) {
        for (const K of strikes) {
          const price = blackScholesPrice({
            optionType: ot,
            S: 100,
            K,
            T: 0.5,
            r: 0.05,
            sigma: 0.25,
          });
          expect(price).toBeGreaterThanOrEqual(0);
        }
      }
    });
  });

  describe("edge cases", () => {
    it("T = 0 returns intrinsic value for ITM call", () => {
      const price = blackScholesPrice({
        optionType: "call",
        S: 110,
        K: 100,
        T: 0,
        r: 0.05,
        sigma: 0.2,
      });
      expect(price).toBe(10); // max(110 - 100, 0)
    });

    it("T = 0 returns 0 for OTM call", () => {
      const price = blackScholesPrice({
        optionType: "call",
        S: 90,
        K: 100,
        T: 0,
        r: 0.05,
        sigma: 0.2,
      });
      expect(price).toBe(0);
    });

    it("T = 0 returns intrinsic value for ITM put", () => {
      const price = blackScholesPrice({
        optionType: "put",
        S: 90,
        K: 100,
        T: 0,
        r: 0.05,
        sigma: 0.2,
      });
      expect(price).toBe(10); // max(100 - 90, 0)
    });

    it("T = 0 returns 0 for OTM put", () => {
      const price = blackScholesPrice({
        optionType: "put",
        S: 110,
        K: 100,
        T: 0,
        r: 0.05,
        sigma: 0.2,
      });
      expect(price).toBe(0);
    });

    it("negative T treated as expired (intrinsic value)", () => {
      const price = blackScholesPrice({
        optionType: "call",
        S: 110,
        K: 100,
        T: -0.5,
        r: 0.05,
        sigma: 0.2,
      });
      expect(price).toBe(10);
    });

    it("sigma = 0 returns intrinsic value for ITM call", () => {
      const price = blackScholesPrice({
        optionType: "call",
        S: 110,
        K: 100,
        T: 1,
        r: 0.05,
        sigma: 0,
      });
      expect(price).toBe(10);
    });

    it("sigma = 0 returns 0 for OTM call", () => {
      const price = blackScholesPrice({
        optionType: "call",
        S: 90,
        K: 100,
        T: 1,
        r: 0.05,
        sigma: 0,
      });
      expect(price).toBe(0);
    });

    it("negative sigma treated as zero vol (intrinsic value)", () => {
      const price = blackScholesPrice({
        optionType: "call",
        S: 110,
        K: 100,
        T: 1,
        r: 0.05,
        sigma: -0.1,
      });
      expect(price).toBe(10);
    });

    it("S = 0 returns 0 for call", () => {
      const price = blackScholesPrice({
        optionType: "call",
        S: 0,
        K: 100,
        T: 1,
        r: 0.05,
        sigma: 0.2,
      });
      expect(price).toBe(0);
    });

    it("S = 0 returns K*e^(-rT) for put", () => {
      const price = blackScholesPrice({
        optionType: "put",
        S: 0,
        K: 100,
        T: 1,
        r: 0.05,
        sigma: 0.2,
      });
      expect(price).toBeCloseTo(100 * Math.exp(-0.05), 6);
    });

    it("S < 0 returns 0 for call", () => {
      const price = blackScholesPrice({
        optionType: "call",
        S: -50,
        K: 100,
        T: 1,
        r: 0.05,
        sigma: 0.2,
      });
      expect(price).toBe(0);
    });

    it("S < 0 returns K*e^(-rT) for put", () => {
      const price = blackScholesPrice({
        optionType: "put",
        S: -50,
        K: 100,
        T: 1,
        r: 0.05,
        sigma: 0.2,
      });
      expect(price).toBeCloseTo(100 * Math.exp(-0.05), 6);
    });

    it("does not return NaN for any edge case combination", () => {
      const edgeCases = [
        { optionType: "call", S: 0, K: 100, T: 0, r: 0, sigma: 0 },
        { optionType: "put", S: 0, K: 100, T: 0, r: 0, sigma: 0 },
        { optionType: "call", S: -10, K: 100, T: -1, r: 0.05, sigma: -0.2 },
        { optionType: "call", S: 100, K: 100, T: 0.0001, r: 0.05, sigma: 0.0001 },
        { optionType: "put", S: 1e-10, K: 100, T: 1, r: 0.05, sigma: 0.5 },
      ];
      for (const params of edgeCases) {
        const price = blackScholesPrice(params);
        expect(Number.isNaN(price)).toBe(false);
        expect(Number.isFinite(price)).toBe(true);
      }
    });

    it("handles alternative option type strings", () => {
      const callUppercase = blackScholesPrice({
        optionType: "CALL",
        ...baseParams,
      });
      const callSingle = blackScholesPrice({
        optionType: "C",
        ...baseParams,
      });
      const callLower = blackScholesPrice({
        optionType: "call",
        ...baseParams,
      });

      expect(callUppercase).toBeCloseTo(callLower, 10);
      expect(callSingle).toBeCloseTo(callLower, 10);
    });
  });
});

// ---------------------------------------------------------------------------
// 3. calculateT0Payoffs
// ---------------------------------------------------------------------------
describe("calculateT0Payoffs", () => {
  describe("single leg strategies", () => {
    it("single long call — T+0 P&L matches manual calculation", () => {
      const K = 100;
      const entryPrice = 5;
      const iv = 0.25;
      const T = 30 / 365; // 30 days
      const prices = [80, 90, 100, 110, 120];

      const result = calculateT0Payoffs({
        prices,
        legs: [
          {
            strike_price: K,
            option_type: "call",
            qty: 1,
            avg_entry_price: entryPrice,
            iv,
            T,
          },
        ],
      });

      expect(result).toHaveLength(prices.length);

      // Verify each point manually
      for (let i = 0; i < prices.length; i++) {
        const bsPrice = blackScholesPrice({
          optionType: "call",
          S: prices[i],
          K,
          T,
          r: 0.05,
          sigma: iv,
        });
        const expectedPL = (bsPrice - entryPrice) * 1 * 100;
        expect(result[i]).toBeCloseTo(expectedPL, 1);
      }
    });

    it("single short put — T+0 P&L matches manual calculation", () => {
      const K = 100;
      const entryPrice = 4;
      const iv = 0.3;
      const T = 60 / 365;
      const prices = [80, 90, 100, 110, 120];

      const result = calculateT0Payoffs({
        prices,
        legs: [
          {
            strike_price: K,
            option_type: "put",
            qty: -1, // Short
            avg_entry_price: entryPrice,
            iv,
            T,
          },
        ],
      });

      expect(result).toHaveLength(prices.length);

      for (let i = 0; i < prices.length; i++) {
        const bsPrice = blackScholesPrice({
          optionType: "put",
          S: prices[i],
          K,
          T,
          r: 0.05,
          sigma: iv,
        });
        // P&L = (theoreticalPrice - entryPrice) * qty * 100
        // qty is -1, so: (bsPrice - 4) * (-1) * 100 = (4 - bsPrice) * 100
        const expectedPL = (bsPrice - entryPrice) * (-1) * 100;
        expect(result[i]).toBeCloseTo(expectedPL, 1);
      }
    });
  });

  describe("multi-leg strategies", () => {
    it("iron condor (4 legs) — T+0 curve has expected shape", () => {
      const T = 30 / 365;
      const iv = 0.2;
      const prices = [];
      for (let p = 470; p <= 530; p += 5) prices.push(p);

      const legs = [
        { strike_price: 480, option_type: "put", qty: 1, avg_entry_price: 2, iv, T },
        { strike_price: 490, option_type: "put", qty: -1, avg_entry_price: 4, iv, T },
        { strike_price: 510, option_type: "call", qty: -1, avg_entry_price: 4, iv, T },
        { strike_price: 520, option_type: "call", qty: 1, avg_entry_price: 2, iv, T },
      ];

      const result = calculateT0Payoffs({ prices, legs });

      expect(result).toHaveLength(prices.length);

      // The center of the iron condor (around 500) should have the highest P&L
      const centerIndex = prices.indexOf(500);
      expect(centerIndex).toBeGreaterThan(-1);

      // Center should have the BEST P&L (highest value in the array)
      // With 30 DTE, the short options still have significant time value that
      // hasn't decayed, so the center P&L may be negative. The key property
      // is that the center is the BEST point on the curve.
      const maxPL = Math.max(...result);
      expect(result[centerIndex]).toBeCloseTo(maxPL, 0);

      // Edges should have WORSE P&L than center
      expect(result[0]).toBeLessThan(result[centerIndex]); // far left: worse
      expect(result[result.length - 1]).toBeLessThan(result[centerIndex]); // far right: worse

      // T+0 curve should be smoother (less extreme) than expiration payoff.
      // At the center, T+0 profit should be LESS than max profit at expiration
      // because time value hasn't fully decayed yet.
      const expirationMaxProfit = (4 + 4 - 2 - 2) * 100; // $400 net credit
      expect(result[centerIndex]).toBeLessThan(expirationMaxProfit);

      // The T+0 max loss at the edges should be less severe than expiration
      // max loss because the options still have time value
      const expirationMaxLoss = -600; // (width - netCredit) * 100
      expect(result[0]).toBeGreaterThan(expirationMaxLoss);
      expect(result[result.length - 1]).toBeGreaterThan(expirationMaxLoss);
    });

    it("bull call spread (2 legs) — T+0 P&L values are between expiration extremes", () => {
      const T = 45 / 365;
      const iv = 0.25;
      const prices = [90, 95, 100, 105, 110, 115, 120];

      const legs = [
        { strike_price: 100, option_type: "call", qty: 1, avg_entry_price: 8, iv, T },
        { strike_price: 110, option_type: "call", qty: -1, avg_entry_price: 3, iv, T },
      ];

      const result = calculateT0Payoffs({ prices, legs });

      expect(result).toHaveLength(prices.length);

      // All values should be finite numbers
      for (const val of result) {
        expect(Number.isFinite(val)).toBe(true);
      }

      // Net debit = 8 - 3 = $5 per share → $500
      // Max loss at expiration = -$500 (below lower strike)
      // Max profit at expiration = (110-100-5) * 100 = $500 (above upper strike)
      // T+0 values should be within these bounds (roughly)
      const maxAtExpiry = 500;
      const minAtExpiry = -500;

      // The T+0 curve should not exceed expiration max profit (time value remaining)
      // At S=120 (deep ITM), T+0 should be close to but slightly less than $500
      const atDeepItm = result[prices.indexOf(120)];
      expect(atDeepItm).toBeLessThanOrEqual(maxAtExpiry + 10); // small tolerance
      expect(atDeepItm).toBeGreaterThan(0); // but should be positive

      // At S=90 (deep OTM), T+0 loss should be slightly less severe than $500
      // (because there's still some time value)
      const atDeepOtm = result[prices.indexOf(90)];
      expect(atDeepOtm).toBeGreaterThan(minAtExpiry - 10); // should be better than max loss
      expect(atDeepOtm).toBeLessThan(0); // but still negative
    });
  });

  describe("0 DTE (T = 0) — T+0 matches expiration payoff", () => {
    it("long call at T=0 matches intrinsic value payoff", () => {
      const K = 100;
      const entryPrice = 5;
      const prices = [85, 90, 95, 100, 105, 110, 115];

      const result = calculateT0Payoffs({
        prices,
        legs: [
          {
            strike_price: K,
            option_type: "call",
            qty: 1,
            avg_entry_price: entryPrice,
            iv: 0.25, // IV doesn't matter when T=0
            T: 0,
          },
        ],
      });

      // At T=0, theoretical price = intrinsic value
      // P&L = (max(S-K, 0) - entryPrice) * 100
      for (let i = 0; i < prices.length; i++) {
        const intrinsic = Math.max(prices[i] - K, 0);
        const expectedPL = (intrinsic - entryPrice) * 100;
        expect(result[i]).toBeCloseTo(expectedPL, 1);
      }
    });

    it("iron condor at T=0 matches expiration payoff exactly", () => {
      const prices = [];
      for (let p = 470; p <= 530; p += 5) prices.push(p);

      const legs = [
        { strike_price: 480, option_type: "put", qty: 1, avg_entry_price: 2, iv: 0.2, T: 0 },
        { strike_price: 490, option_type: "put", qty: -1, avg_entry_price: 4, iv: 0.2, T: 0 },
        { strike_price: 510, option_type: "call", qty: -1, avg_entry_price: 4, iv: 0.2, T: 0 },
        { strike_price: 520, option_type: "call", qty: 1, avg_entry_price: 2, iv: 0.2, T: 0 },
      ];

      const result = calculateT0Payoffs({ prices, legs });

      // At T=0, each leg's theoretical price equals intrinsic value
      // So T+0 payoff should match expiration payoff exactly
      for (let i = 0; i < prices.length; i++) {
        const S = prices[i];
        let expPayoff = 0;
        for (const leg of legs) {
          const intrinsic =
            leg.option_type === "call"
              ? Math.max(S - leg.strike_price, 0)
              : Math.max(leg.strike_price - S, 0);
          expPayoff += (intrinsic - leg.avg_entry_price) * leg.qty * 100;
        }
        expect(result[i]).toBeCloseTo(expPayoff, 1);
      }
    });
  });

  describe("mixed expirations", () => {
    it("each leg uses its own T value", () => {
      const prices = [90, 100, 110];
      const entryPrice = 5;

      // Two calls at same strike but different expirations
      const legs = [
        { strike_price: 100, option_type: "call", qty: 1, avg_entry_price: entryPrice, iv: 0.25, T: 30 / 365 },
        { strike_price: 100, option_type: "call", qty: -1, avg_entry_price: entryPrice, iv: 0.25, T: 90 / 365 },
      ];

      const result = calculateT0Payoffs({ prices, legs });

      expect(result).toHaveLength(prices.length);

      // Verify against individual BS calculations
      for (let i = 0; i < prices.length; i++) {
        const S = prices[i];
        const bs1 = blackScholesPrice({
          optionType: "call",
          S,
          K: 100,
          T: 30 / 365,
          r: 0.05,
          sigma: 0.25,
        });
        const bs2 = blackScholesPrice({
          optionType: "call",
          S,
          K: 100,
          T: 90 / 365,
          r: 0.05,
          sigma: 0.25,
        });

        // Long the 30-day, short the 90-day
        const expected = (bs1 - entryPrice) * 1 * 100 + (bs2 - entryPrice) * (-1) * 100;
        expect(result[i]).toBeCloseTo(expected, 1);
      }

      // The longer-dated option should be worth more, so at ATM the
      // calendar spread should have a negative T+0 P&L (we're short the
      // more expensive option)
      const atmIndex = prices.indexOf(100);
      expect(result[atmIndex]).toBeLessThan(0);
    });
  });

  describe("creditAdjustment", () => {
    it("applies positive credit adjustment to all values", () => {
      const prices = [90, 100, 110];
      const legs = [
        { strike_price: 100, option_type: "call", qty: 1, avg_entry_price: 5, iv: 0.25, T: 30 / 365 },
      ];

      const baseResult = calculateT0Payoffs({ prices, legs, creditAdjustment: 0 });
      const adjustedResult = calculateT0Payoffs({ prices, legs, creditAdjustment: 200 });

      for (let i = 0; i < prices.length; i++) {
        expect(adjustedResult[i]).toBeCloseTo(baseResult[i] + 200, 1);
      }
    });

    it("applies negative credit adjustment to all values", () => {
      const prices = [90, 100, 110];
      const legs = [
        { strike_price: 100, option_type: "call", qty: 1, avg_entry_price: 5, iv: 0.25, T: 30 / 365 },
      ];

      const baseResult = calculateT0Payoffs({ prices, legs, creditAdjustment: 0 });
      const adjustedResult = calculateT0Payoffs({ prices, legs, creditAdjustment: -150 });

      for (let i = 0; i < prices.length; i++) {
        expect(adjustedResult[i]).toBeCloseTo(baseResult[i] - 150, 1);
      }
    });
  });

  describe("edge cases", () => {
    it("empty legs array returns array of zeros", () => {
      const prices = [90, 100, 110];
      const result = calculateT0Payoffs({ prices, legs: [] });

      expect(result).toHaveLength(prices.length);
      for (const val of result) {
        expect(val).toBe(0);
      }
    });

    it("empty legs with creditAdjustment returns array of that adjustment", () => {
      const prices = [90, 100, 110];
      const result = calculateT0Payoffs({
        prices,
        legs: [],
        creditAdjustment: 300,
      });

      expect(result).toHaveLength(prices.length);
      for (const val of result) {
        expect(val).toBeCloseTo(300, 1);
      }
    });

    it("null legs returns array of zeros", () => {
      const prices = [90, 100, 110];
      const result = calculateT0Payoffs({ prices, legs: null });

      expect(result).toHaveLength(prices.length);
      for (const val of result) {
        expect(val).toBe(0);
      }
    });

    it("output length matches input prices length", () => {
      const lengths = [0, 1, 5, 100];
      for (const len of lengths) {
        const prices = Array.from({ length: len }, (_, i) => 90 + i);
        const result = calculateT0Payoffs({
          prices,
          legs: [
            { strike_price: 100, option_type: "call", qty: 1, avg_entry_price: 5, iv: 0.25, T: 30 / 365 },
          ],
        });
        expect(result).toHaveLength(len);
      }
    });

    it("handles negative price points (from wide range generation) without NaN", () => {
      const prices = [-50, -10, 0, 10, 50, 100, 150];
      const legs = [
        { strike_price: 100, option_type: "call", qty: 1, avg_entry_price: 5, iv: 0.25, T: 30 / 365 },
        { strike_price: 100, option_type: "put", qty: 1, avg_entry_price: 3, iv: 0.25, T: 30 / 365 },
      ];

      const result = calculateT0Payoffs({ prices, legs });

      expect(result).toHaveLength(prices.length);
      for (const val of result) {
        expect(Number.isNaN(val)).toBe(false);
        expect(Number.isFinite(val)).toBe(true);
      }
    });

    it("default riskFreeRate is 0.05", () => {
      const prices = [100];
      const legs = [
        { strike_price: 100, option_type: "call", qty: 1, avg_entry_price: 5, iv: 0.25, T: 30 / 365 },
      ];

      const defaultResult = calculateT0Payoffs({ prices, legs });
      const explicitResult = calculateT0Payoffs({
        prices,
        legs,
        riskFreeRate: 0.05,
      });

      expect(defaultResult[0]).toBeCloseTo(explicitResult[0], 10);
    });

    it("default creditAdjustment is 0", () => {
      const prices = [100];
      const legs = [
        { strike_price: 100, option_type: "call", qty: 1, avg_entry_price: 5, iv: 0.25, T: 30 / 365 },
      ];

      const defaultResult = calculateT0Payoffs({ prices, legs });
      const explicitResult = calculateT0Payoffs({
        prices,
        legs,
        creditAdjustment: 0,
      });

      expect(defaultResult[0]).toBeCloseTo(explicitResult[0], 10);
    });

    it("handles legs with avg_entry_price = 0", () => {
      const prices = [90, 100, 110];
      const result = calculateT0Payoffs({
        prices,
        legs: [
          { strike_price: 100, option_type: "call", qty: 1, avg_entry_price: 0, iv: 0.25, T: 30 / 365 },
        ],
      });

      expect(result).toHaveLength(prices.length);
      // With zero entry price, P&L = theoreticalPrice * 100
      // All values should be non-negative for a long call
      for (const val of result) {
        expect(val).toBeGreaterThanOrEqual(0);
      }
    });
  });

  describe("performance sanity check", () => {
    it("computes 4-leg × 500 price points in under 50ms", () => {
      const prices = Array.from({ length: 500 }, (_, i) => 400 + i);
      const legs = [
        { strike_price: 480, option_type: "put", qty: 1, avg_entry_price: 2, iv: 0.2, T: 30 / 365 },
        { strike_price: 490, option_type: "put", qty: -1, avg_entry_price: 4, iv: 0.2, T: 30 / 365 },
        { strike_price: 510, option_type: "call", qty: -1, avg_entry_price: 4, iv: 0.2, T: 30 / 365 },
        { strike_price: 520, option_type: "call", qty: 1, avg_entry_price: 2, iv: 0.2, T: 30 / 365 },
      ];

      const start = performance.now();
      const result = calculateT0Payoffs({ prices, legs });
      const elapsed = performance.now() - start;

      expect(result).toHaveLength(500);
      expect(elapsed).toBeLessThan(50);
    });
  });
});
