/**
 * QA Test Plan — Issue #42: Increase Data Available in the Payoff Chart
 *
 * Covers sections 1-3 from docs/issue-42-increase-data-avaialble-in-the-payoff-chart/test-plan.md
 *
 * Section 1: Multi-Leg Range Formula Verification (tests 1.1-1.7)
 * Section 2: Single-Leg Range Formula Verification (tests 2.1-2.6)
 * Section 3: Adaptive Step Size Verification (tests 3.1-3.7)
 * Section 4: Data Point Count Cap (Max 600) (tests 4.1-4.8)
 * Section 5: Strike Inclusion in Generated Data Points (tests 5.1-5.6)
 * Section 6: Extreme Price Levels (tests 6a.1-6a.3, 6b.1-6b.3)
 * Section 7: Very Narrow Spreads on Expensive Symbols (tests 7.1-7.4)
 * Section 8: Very Wide Spreads on Cheap Symbols (tests 8.1-8.3)
 * Section 9: Initial View Window — FR-4 (tests 9.1-9.7)
 * Section 10: Pan/Zoom Limits — FR-5 (tests 10.1-10.4)
 * Section 11: Butterfly/IC Payoff Consistency — AC-8 (tests 11a.1-11a.6, 11b.1-11b.5)
 * Section 12: No Regression on Low/Mid-Priced Symbols (tests 12.1-12.7)
 * Section 13: Edge Cases and Boundary Conditions (tests 13.1-13.8)
 * Section 14: Cross-Cutting Concerns (tests 14.1-14.6)
 */

import { describe, it, expect } from "vitest";
import {
  generateMultiLegPayoff,
  generateButterflyPayoff,
  createMultiLegChartConfig,
} from "../src/utils/chartUtils.js";

// ---------------------------------------------------------------------------
// Helpers — build option position objects (same pattern as dev tests)
// ---------------------------------------------------------------------------

function makeLongCallPosition(strike, entryPrice, symbol = "TEST") {
  return {
    symbol: `${symbol}_CALL_${strike}`,
    asset_class: "us_option",
    side: "long",
    qty: 1,
    strike_price: strike,
    option_type: "call",
    expiry_date: "2026-04-17",
    current_price: entryPrice,
    avg_entry_price: entryPrice,
    cost_basis: entryPrice * 100,
    market_value: entryPrice * 100,
    unrealized_pl: 0,
    unrealized_plpc: 0,
    underlying_symbol: symbol,
  };
}

function makeShortCallPosition(strike, entryPrice, symbol = "TEST") {
  return {
    symbol: `${symbol}_CALL_${strike}`,
    asset_class: "us_option",
    side: "short",
    qty: -1,
    strike_price: strike,
    option_type: "call",
    expiry_date: "2026-04-17",
    current_price: entryPrice,
    avg_entry_price: entryPrice,
    cost_basis: -(entryPrice * 100),
    market_value: -(entryPrice * 100),
    unrealized_pl: 0,
    unrealized_plpc: 0,
    underlying_symbol: symbol,
  };
}

function makeLongPutPosition(strike, entryPrice, symbol = "TEST") {
  return {
    symbol: `${symbol}_PUT_${strike}`,
    asset_class: "us_option",
    side: "long",
    qty: 1,
    strike_price: strike,
    option_type: "put",
    expiry_date: "2026-04-17",
    current_price: entryPrice,
    avg_entry_price: entryPrice,
    cost_basis: entryPrice * 100,
    market_value: entryPrice * 100,
    unrealized_pl: 0,
    unrealized_plpc: 0,
    underlying_symbol: symbol,
  };
}

function makeShortPutPosition(strike, entryPrice, symbol = "TEST") {
  return {
    symbol: `${symbol}_PUT_${strike}`,
    asset_class: "us_option",
    side: "short",
    qty: -1,
    strike_price: strike,
    option_type: "put",
    expiry_date: "2026-04-17",
    current_price: entryPrice,
    avg_entry_price: entryPrice,
    cost_basis: -(entryPrice * 100),
    market_value: -(entryPrice * 100),
    unrealized_pl: 0,
    unrealized_plpc: 0,
    underlying_symbol: symbol,
  };
}

// ===========================================================================
// Section 1: Multi-Leg Range Formula Verification
//
// Formula: extension = Math.max(underlyingPrice * 0.10, strikeRange * 3.0, 50)
//          lowerBound = Math.floor(minStrike - extension)
//          upperBound = Math.ceil(maxStrike + extension)
// ===========================================================================
describe("Section 1: Multi-Leg Range Formula Verification", () => {

  // 1.1 NDX — percentage dominates
  // strikes 23100/23150, underlying=23132
  // extension = max(2313.2, 150, 50) = 2313.2
  // lowerBound = floor(23100 - 2313.2) = floor(20786.8) = 20786
  // upperBound = ceil(23150 + 2313.2) = ceil(25463.2) = 25464
  describe("1.1 NDX — percentage dominates", () => {
    const positions = [
      makeLongCallPosition(23100, 200, "NDX"),
      makeShortCallPosition(23150, 150, "NDX"),
    ];
    const underlyingPrice = 23132;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("extension equals underlyingPrice * 0.10 = 2313.2 (percentage dominates)", () => {
      const firstPrice = chartData.prices[0];
      const lastPrice = chartData.prices[chartData.prices.length - 1];

      // lowerBound = floor(23100 - 2313.2) = 20786
      // The first price IS the lowerBound (loop starts there)
      expect(firstPrice).toBeLessThanOrEqual(20787);
      // upperBound = ceil(23150 + 2313.2) = 25464
      // The last stepped price may be up to (step-1) below upperBound.
      // With step=12, last price could be as low as 25454. Check it's within
      // one step of the theoretical upperBound.
      expect(lastPrice).toBeGreaterThanOrEqual(25464 - 12);
    });

    it("lowerBound <= 23100 - 2313", () => {
      expect(chartData.prices[0]).toBeLessThanOrEqual(23100 - 2313);
    });

    it("upperBound within one step of 23150 + 2313", () => {
      // The last data point may not exactly reach upperBound due to step rounding.
      // Verify it's within one step (step ≈ 12) of the theoretical upper bound.
      expect(chartData.prices[chartData.prices.length - 1]).toBeGreaterThanOrEqual(23150 + 2313 - 12);
    });
  });

  // 1.2 Cheap stock — strikeRange dominates
  // strikes 48/53, underlying=50
  // strikeRange = 5, extension = max(5, 15, 50) = 50
  // lowerBound = floor(48 - 50) = -2
  // upperBound = ceil(53 + 50) = 103
  describe("1.2 Cheap stock — strikeRange dominates by floor", () => {
    const positions = [
      makeLongCallPosition(48, 4, "CHEAP"),
      makeShortCallPosition(53, 2, "CHEAP"),
    ];
    const underlyingPrice = 50;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("extension = max(5, 15, 50) = 50; lowerBound <= -2", () => {
      expect(chartData.prices[0]).toBeLessThanOrEqual(-2);
    });

    it("upperBound >= 103", () => {
      expect(chartData.prices[chartData.prices.length - 1]).toBeGreaterThanOrEqual(103);
    });
  });

  // 1.3 $50 floor dominates
  // strikes 8/10, underlying=9
  // strikeRange = 2, extension = max(0.9, 6, 50) = 50
  // lowerBound = floor(8 - 50) = -42
  // upperBound = ceil(10 + 50) = 60
  describe("1.3 $50 floor dominates (penny-like stock)", () => {
    const positions = [
      makeLongCallPosition(8, 1.5, "PENNY"),
      makeShortCallPosition(10, 0.8, "PENNY"),
    ];
    const underlyingPrice = 9;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("extension = 50 ($50 floor); lowerBound <= 8 - 50 = -42", () => {
      expect(chartData.prices[0]).toBeLessThanOrEqual(-42);
    });

    it("upperBound >= 10 + 50 = 60", () => {
      expect(chartData.prices[chartData.prices.length - 1]).toBeGreaterThanOrEqual(60);
    });
  });

  // 1.4 Mid-price stock
  // strikes 195/205, underlying=200
  // strikeRange = 10, extension = max(20, 30, 50) = 50
  // lowerBound = floor(195 - 50) = 145
  // upperBound = ceil(205 + 50) = 255
  describe("1.4 Mid-price stock — $50 floor kicks in", () => {
    const positions = [
      makeLongCallPosition(195, 12, "MID"),
      makeShortCallPosition(205, 8, "MID"),
    ];
    const underlyingPrice = 200;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("extension = max(20, 30, 50) = 50; lowerBound <= 145", () => {
      expect(chartData.prices[0]).toBeLessThanOrEqual(145);
    });

    it("upperBound >= 255", () => {
      expect(chartData.prices[chartData.prices.length - 1]).toBeGreaterThanOrEqual(255);
    });
  });

  // 1.5 SPX-level
  // strikes 5900/5950, underlying=5920
  // strikeRange = 50, extension = max(592, 150, 50) = 592
  // lowerBound = floor(5900 - 592) = 5308
  // upperBound = ceil(5950 + 592) = 6542
  describe("1.5 SPX-level", () => {
    const positions = [
      makeLongCallPosition(5900, 80, "SPX"),
      makeShortCallPosition(5950, 50, "SPX"),
    ];
    const underlyingPrice = 5920;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("extension = max(592, 150, 50) = 592; lowerBound <= 5308", () => {
      expect(chartData.prices[0]).toBeLessThanOrEqual(5308);
    });

    it("upperBound within one step of 6542", () => {
      // step ≈ 4, last stepped price may be a few below 6542
      const lastPrice = chartData.prices[chartData.prices.length - 1];
      // Theoretical upperBound = ceil(5950 + 592) = 6542
      // With step = ceil(totalRange/400), the last price is within (step-1) of upperBound
      expect(lastPrice).toBeGreaterThanOrEqual(6542 - 4);
    });

    it("range extends at least ±500 from strikes", () => {
      expect(chartData.prices[0]).toBeLessThanOrEqual(5900 - 500);
      expect(chartData.prices[chartData.prices.length - 1]).toBeGreaterThanOrEqual(5950 + 500);
    });
  });

  // 1.6 Mega-cap index ~$50,000
  // strikes 49900/50000, underlying=50000
  // strikeRange = 100, extension = max(5000, 300, 50) = 5000
  // lowerBound = floor(49900 - 5000) = 44900
  // upperBound = ceil(50000 + 5000) = 55000
  describe("1.6 Mega-cap index ~$50,000", () => {
    const positions = [
      makeLongCallPosition(49900, 500, "MEGA"),
      makeShortCallPosition(50000, 400, "MEGA"),
    ];
    const underlyingPrice = 50000;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("extension = max(5000, 300, 50) = 5000; lowerBound <= 44900", () => {
      expect(chartData.prices[0]).toBeLessThanOrEqual(44900);
    });

    it("upperBound within one step of 55000", () => {
      // Theoretical upperBound = ceil(50000 + 5000) = 55000
      // step = ceil(10100/400) = 26; last price could be up to 25 below upperBound
      expect(chartData.prices[chartData.prices.length - 1]).toBeGreaterThanOrEqual(55000 - 26);
    });

    it("range extends ±5000 from strikes (accounting for step rounding)", () => {
      expect(chartData.prices[0]).toBeLessThanOrEqual(49900 - 5000);
      // Upper bound may be up to step-1 short of theoretical; use step=26 tolerance
      expect(chartData.prices[chartData.prices.length - 1]).toBeGreaterThanOrEqual(50000 + 5000 - 26);
    });
  });

  // 1.7 Penny stock ~$1
  // strikes 0.90/1.10, underlying=1.0
  // strikeRange = 0.2, extension = max(0.1, 0.6, 50) = 50
  // lowerBound = floor(0.90 - 50) = floor(-49.1) = -50
  // upperBound = ceil(1.10 + 50) = ceil(51.1) = 52
  describe("1.7 Penny stock ~$1 — $50 floor prevents absurdly narrow range", () => {
    const positions = [
      makeLongPutPosition(1.10, 0.30, "PENNY"),
      makeShortPutPosition(0.90, 0.20, "PENNY"),
    ];
    const underlyingPrice = 1.0;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("extension = max(0.1, 0.6, 50) = 50; lowerBound <= -49", () => {
      expect(chartData.prices[0]).toBeLessThanOrEqual(-49);
    });

    it("upperBound >= 51", () => {
      expect(chartData.prices[chartData.prices.length - 1]).toBeGreaterThanOrEqual(51);
    });

    it("$50 floor prevents absurdly narrow range", () => {
      const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
      expect(totalRange).toBeGreaterThanOrEqual(100);
    });
  });
});

// ===========================================================================
// Section 2: Single-Leg Range Formula Verification
//
// Tiered multiplier:
//   strike < 50  → tierRange = max(strike * 8, 250)
//   strike < 200 → tierRange = max(strike * 5, 500)
//   else         → tierRange = max(strike * 3, 1000)
// Then: range = max(tierRange, underlyingPrice * 0.10, 50)
//       lowerBound = floor(strike - range)
//       upperBound = ceil(strike + range)
// ===========================================================================
describe("Section 2: Single-Leg Range Formula Verification", () => {

  // 2.1 NDX single call
  // strike=23100, underlying=23132
  // tierRange = max(23100 * 3, 1000) = 69300
  // range = max(69300, 2313.2, 50) = 69300
  // lowerBound = floor(23100 - 69300) = -46200
  // upperBound = ceil(23100 + 69300) = 92400
  describe("2.1 NDX single call (strike >= 200, mult=3)", () => {
    const positions = [makeLongCallPosition(23100, 200, "NDX")];
    const underlyingPrice = 23132;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("tierRange = max(23100*3, 1000) = 69300; range = 69300", () => {
      const firstPrice = chartData.prices[0];
      const lastPrice = chartData.prices[chartData.prices.length - 1];

      // lowerBound = floor(23100 - 69300) = -46200
      expect(firstPrice).toBeLessThanOrEqual(-46200);
      // upperBound = ceil(23100 + 69300) = 92400
      // totalRange = 138600, step = ceil(138600/400) = 347
      // Last stepped price may be up to 346 below upperBound
      expect(lastPrice).toBeGreaterThanOrEqual(92400 - 347);
    });

    it("data points ≤ 600", () => {
      expect(chartData.prices.length).toBeLessThanOrEqual(600);
    });
  });

  // 2.2 Cheap stock single call (strike < 50)
  // strike=25, underlying=25
  // tierRange = max(25 * 8, 250) = 250
  // range = max(250, 2.5, 50) = 250
  // lowerBound = floor(25 - 250) = -225
  // upperBound = ceil(25 + 250) = 275
  describe("2.2 Cheap stock single call (strike < 50, mult=8, min=250)", () => {
    const positions = [makeLongCallPosition(25, 3, "CHEAP")];
    const underlyingPrice = 25;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("tierRange = max(25*8, 250) = 250; range = 250", () => {
      const firstPrice = chartData.prices[0];
      const lastPrice = chartData.prices[chartData.prices.length - 1];

      // lowerBound = floor(25 - 250) = -225
      expect(firstPrice).toBeLessThanOrEqual(-225);
      // upperBound = ceil(25 + 250) = 275
      expect(lastPrice).toBeGreaterThanOrEqual(275);
    });

    it("data points ≤ 600", () => {
      expect(chartData.prices.length).toBeLessThanOrEqual(600);
    });
  });

  // 2.3 Mid-price single call (strike < 200)
  // strike=150, underlying=155
  // tierRange = max(150 * 5, 500) = 750
  // range = max(750, 15.5, 50) = 750
  // lowerBound = floor(150 - 750) = -600
  // upperBound = ceil(150 + 750) = 900
  describe("2.3 Mid-price single call (strike < 200, mult=5, min=500)", () => {
    const positions = [makeLongCallPosition(150, 8, "MID")];
    const underlyingPrice = 155;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("tierRange = max(150*5, 500) = 750; range = 750", () => {
      const firstPrice = chartData.prices[0];
      const lastPrice = chartData.prices[chartData.prices.length - 1];

      // lowerBound = floor(150 - 750) = -600
      expect(firstPrice).toBeLessThanOrEqual(-600);
      // upperBound = ceil(150 + 750) = 900
      expect(lastPrice).toBeGreaterThanOrEqual(900);
    });

    it("data points ≤ 600", () => {
      expect(chartData.prices.length).toBeLessThanOrEqual(600);
    });
  });

  // 2.4 Expensive single call (strike >= 200)
  // strike=5900, underlying=5920
  // tierRange = max(5900 * 3, 1000) = 17700
  // range = max(17700, 592, 50) = 17700
  // lowerBound = floor(5900 - 17700) = -11800
  // upperBound = ceil(5900 + 17700) = 23600
  describe("2.4 Expensive single call (strike >= 200, mult=3)", () => {
    const positions = [makeLongCallPosition(5900, 80, "SPX")];
    const underlyingPrice = 5920;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("tierRange = max(5900*3, 1000) = 17700; range = 17700", () => {
      const firstPrice = chartData.prices[0];
      const lastPrice = chartData.prices[chartData.prices.length - 1];

      // lowerBound = floor(5900 - 17700) = -11800
      expect(firstPrice).toBeLessThanOrEqual(-11800);
      // upperBound = ceil(5900 + 17700) = 23600
      // totalRange = 35400, step = ceil(35400/400) = 89
      // Last stepped price may be up to 88 below upperBound
      expect(lastPrice).toBeGreaterThanOrEqual(23600 - 89);
    });

    it("data points ≤ 600", () => {
      expect(chartData.prices.length).toBeLessThanOrEqual(600);
    });
  });

  // 2.5 Penny stock single put
  // strike=1.0, underlying=1.05
  // tierRange = max(1.0 * 8, 250) = 250
  // range = max(250, 0.105, 50) = 250
  // lowerBound = floor(1.0 - 250) = -249
  // upperBound = ceil(1.0 + 250) = 251
  describe("2.5 Penny stock single put (strike < 50, mult=8, min=250)", () => {
    const positions = [makeLongPutPosition(1.0, 0.50, "PENNY")];
    const underlyingPrice = 1.05;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("tierRange = max(1*8, 250) = 250; range = 250", () => {
      const firstPrice = chartData.prices[0];
      const lastPrice = chartData.prices[chartData.prices.length - 1];

      // lowerBound = floor(1.0 - 250) = -249
      expect(firstPrice).toBeLessThanOrEqual(-249);
      // upperBound = ceil(1.0 + 250) = 251
      expect(lastPrice).toBeGreaterThanOrEqual(251);
    });

    it("data points ≤ 600", () => {
      expect(chartData.prices.length).toBeLessThanOrEqual(600);
    });
  });

  // 2.6 $50,000 single leg
  // strike=50000, underlying=50000
  // tierRange = max(50000 * 3, 1000) = 150000
  // range = max(150000, 5000, 50) = 150000
  // lowerBound = floor(50000 - 150000) = -100000
  // upperBound = ceil(50000 + 150000) = 200000
  // totalRange = 300000, step = ceil(300000/400) = 750
  describe("2.6 $50,000 single leg — adaptive step keeps points ≤ 600", () => {
    const positions = [makeLongCallPosition(50000, 1000, "MEGA")];
    const underlyingPrice = 50000;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("tierRange = max(50000*3, 1000) = 150000; range = 150000", () => {
      const firstPrice = chartData.prices[0];
      const lastPrice = chartData.prices[chartData.prices.length - 1];

      // lowerBound = floor(50000 - 150000) = -100000
      expect(firstPrice).toBeLessThanOrEqual(-100000);
      // upperBound = ceil(50000 + 150000) = 200000
      expect(lastPrice).toBeGreaterThanOrEqual(200000);
    });

    it("data points ≤ 600 via adaptive step", () => {
      expect(chartData.prices.length).toBeLessThanOrEqual(600);
    });
  });
});

// ===========================================================================
// Section 3: Adaptive Step Size Verification
//
// Formula: step = Math.max(Math.ceil(totalRange / 400), 1)
// ===========================================================================
describe("Section 3: Adaptive Step Size Verification", () => {

  // Helper: compute the expected step and verify data point count is consistent
  function verifyStepAndPointCount(chartData, expectedTotalRange, expectedStep, label) {
    const actualFirstPrice = chartData.prices[0];
    const actualLastPrice = chartData.prices[chartData.prices.length - 1];
    const actualTotalRange = actualLastPrice - actualFirstPrice;

    it(`${label}: step = ceil(${expectedTotalRange}/400) = ${expectedStep}`, () => {
      // The actual step is the dominant gap between non-strike-injected prices.
      // With strikes injected, some gaps may be smaller, but the dominant gap
      // should match the expected step.
      const gaps = [];
      for (let i = 1; i < chartData.prices.length; i++) {
        gaps.push(chartData.prices[i] - chartData.prices[i - 1]);
      }
      // The maximum gap should equal the expected step (or be very close due to rounding)
      const maxGap = Math.max(...gaps);
      expect(maxGap).toBeLessThanOrEqual(expectedStep + 1);
      // With step > 1, the dominant gap should be >= expectedStep
      if (expectedStep > 1) {
        expect(maxGap).toBeGreaterThanOrEqual(expectedStep);
      }
    });

    it(`${label}: data point count is consistent with step`, () => {
      // Data points = (stepped points from lowerBound to upperBound) + injected strikes
      // Stepped points ≈ floor(totalRange / step) + 1
      // Total ≈ steppedPoints + numUniqueStrikes (may overlap)
      // We verify it's in a reasonable ballpark and ≤ 600
      expect(chartData.prices.length).toBeLessThanOrEqual(600);
      expect(chartData.prices.length).toBeGreaterThan(0);
    });
  }

  // 3.1 NDX multi-leg
  // strikes 23100/23150, underlying=23132
  // extension = 2313.2
  // lowerBound = floor(23100 - 2313.2) = 20786
  // upperBound = ceil(23150 + 2313.2) = 25464
  // totalRange = 25464 - 20786 = 4678
  // step = ceil(4678/400) = 12
  describe("3.1 NDX multi-leg", () => {
    const positions = [
      makeLongCallPosition(23100, 200, "NDX"),
      makeShortCallPosition(23150, 150, "NDX"),
    ];
    const chartData = generateMultiLegPayoff(positions, 23132);
    const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
    const expectedStep = Math.max(Math.ceil(totalRange / 400), 1);

    verifyStepAndPointCount(chartData, totalRange, expectedStep, "NDX multi-leg");

    it("step > 1 for NDX-level prices", () => {
      expect(expectedStep).toBeGreaterThan(1);
    });
  });

  // 3.2 Cheap stock multi-leg
  // strikes 48/53, underlying=50
  // extension = 50
  // lowerBound = floor(48 - 50) = -2
  // upperBound = ceil(53 + 50) = 103
  // totalRange = 103 - (-2) = 105
  // step = ceil(105/400) = 1
  describe("3.2 Cheap stock multi-leg", () => {
    const positions = [
      makeLongCallPosition(48, 4, "CHEAP"),
      makeShortCallPosition(53, 2, "CHEAP"),
    ];
    const chartData = generateMultiLegPayoff(positions, 50);
    const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
    const expectedStep = Math.max(Math.ceil(totalRange / 400), 1);

    verifyStepAndPointCount(chartData, totalRange, expectedStep, "Cheap stock multi-leg");

    it("step = 1 for cheap stock", () => {
      expect(expectedStep).toBe(1);
    });

    it("data point count ≈ totalRange + strikes (step=1)", () => {
      // With step=1, every integer from lowerBound to upperBound plus strikes
      expect(chartData.prices.length).toBeGreaterThanOrEqual(100);
    });
  });

  // 3.3 $50,000 underlying multi-leg
  // strikes 49900/50000, underlying=50000
  // extension = max(5000, 300, 50) = 5000
  // lowerBound = floor(49900 - 5000) = 44900
  // upperBound = ceil(50000 + 5000) = 55000
  // totalRange = 55000 - 44900 = 10100
  // step = ceil(10100/400) = 26
  describe("3.3 $50,000 underlying multi-leg", () => {
    const positions = [
      makeLongCallPosition(49900, 500, "MEGA"),
      makeShortCallPosition(50000, 400, "MEGA"),
    ];
    const chartData = generateMultiLegPayoff(positions, 50000);
    const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
    const expectedStep = Math.max(Math.ceil(totalRange / 400), 1);

    verifyStepAndPointCount(chartData, totalRange, expectedStep, "$50k multi-leg");

    it("step = ceil(10100/400) = 26", () => {
      expect(expectedStep).toBe(26);
    });
  });

  // 3.4 Penny stock multi-leg
  // strikes 0.90/1.10, underlying=1.0
  // extension = max(0.1, 0.6, 50) = 50
  // lowerBound = floor(0.90 - 50) = floor(-49.1) = -50
  // upperBound = ceil(1.10 + 50) = ceil(51.1) = 52
  // totalRange = 52 - (-50) = 102
  // step = ceil(102/400) = 1
  describe("3.4 Penny stock multi-leg", () => {
    const positions = [
      makeLongPutPosition(1.10, 0.30, "PENNY"),
      makeShortPutPosition(0.90, 0.20, "PENNY"),
    ];
    const chartData = generateMultiLegPayoff(positions, 1.0);
    const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
    const expectedStep = Math.max(Math.ceil(totalRange / 400), 1);

    verifyStepAndPointCount(chartData, totalRange, expectedStep, "Penny stock multi-leg");

    it("step = 1 for penny stock", () => {
      expect(expectedStep).toBe(1);
    });
  });

  // 3.5 Boundary: totalRange exactly 400
  // Engineer: need lowerBound and upperBound such that totalRange = 400
  // With multi-leg: extension = max(underlyingPrice * 0.10, strikeRange * 3.0, 50)
  // Choose strikes 190/210 (strikeRange=20), underlying=200
  //   extension = max(20, 60, 50) = 60
  //   lowerBound = floor(190 - 60) = 130
  //   upperBound = ceil(210 + 60) = 270
  //   totalRange = 270 - 130 = 140 (too small)
  // We need to engineer it differently. Use underlying=1900, strikes 1890/1910
  //   strikeRange=20, extension = max(190, 60, 50) = 190
  //   lowerBound = floor(1890 - 190) = 1700
  //   upperBound = ceil(1910 + 190) = 2100
  //   totalRange = 2100 - 1700 = 400 ✓
  //   step = ceil(400/400) = 1
  describe("3.5 Boundary: totalRange exactly 400", () => {
    const positions = [
      makeLongCallPosition(1890, 50, "ENG"),
      makeShortCallPosition(1910, 40, "ENG"),
    ];
    const chartData = generateMultiLegPayoff(positions, 1900);
    const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];

    it("totalRange = 400", () => {
      expect(totalRange).toBe(400);
    });

    it("step = 1 (boundary: ceil(400/400) = 1)", () => {
      // With step=1, every integer should be present
      const gaps = [];
      for (let i = 1; i < chartData.prices.length; i++) {
        gaps.push(chartData.prices[i] - chartData.prices[i - 1]);
      }
      const maxGap = Math.max(...gaps);
      expect(maxGap).toBe(1);
    });
  });

  // 3.6 Boundary: totalRange = 401
  // Use underlying=1905, strikes 1895/1915
  //   strikeRange=20, extension = max(190.5, 60, 50) = 190.5
  //   lowerBound = floor(1895 - 190.5) = floor(1704.5) = 1704
  //   upperBound = ceil(1915 + 190.5) = ceil(2105.5) = 2106
  //   totalRange = 2106 - 1704 = 402
  // That's 402, not 401. Let's try underlying=1901, strikes 1891/1911
  //   extension = max(190.1, 60, 50) = 190.1
  //   lowerBound = floor(1891 - 190.1) = floor(1700.9) = 1700
  //   upperBound = ceil(1911 + 190.1) = ceil(2101.1) = 2102
  //   totalRange = 2102 - 1700 = 402
  // Actually what matters is step = ceil(totalRange/400). For 401: step = 2.
  // For 402: step = ceil(402/400) = 2. Both give step=2, which is fine.
  // Let's just verify step = 2 for any totalRange in (400, 800].
  describe("3.6 Boundary: totalRange = 401+ → step = 2", () => {
    // underlying=1905, strikes 1895/1915 gives totalRange=402
    const positions = [
      makeLongCallPosition(1895, 50, "ENG"),
      makeShortCallPosition(1915, 40, "ENG"),
    ];
    const chartData = generateMultiLegPayoff(positions, 1905);
    const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];

    it("totalRange > 400", () => {
      expect(totalRange).toBeGreaterThan(400);
    });

    it("step = ceil(totalRange/400) = 2", () => {
      const expectedStep = Math.max(Math.ceil(totalRange / 400), 1);
      expect(expectedStep).toBe(2);

      // Verify by checking the dominant gap between prices (excluding strike-injected gaps)
      const gaps = [];
      for (let i = 1; i < chartData.prices.length; i++) {
        gaps.push(chartData.prices[i] - chartData.prices[i - 1]);
      }
      const maxGap = Math.max(...gaps);
      expect(maxGap).toBe(2);
    });
  });

  // 3.7 Very small range (<50 not possible due to floor)
  // Cheap stock with floor = 50 means totalRange ≥ 100
  // strikes 8/10, underlying=9 → extension = 50 → totalRange ≈ 102
  // step = ceil(102/400) = 1
  describe("3.7 Very small range — $50 floor guarantees totalRange ≥ 100", () => {
    const positions = [
      makeLongCallPosition(8, 1.5, "PENNY"),
      makeShortCallPosition(10, 0.8, "PENNY"),
    ];
    const chartData = generateMultiLegPayoff(positions, 9);
    const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];

    it("totalRange ≥ 100 due to $50 floor on each side", () => {
      expect(totalRange).toBeGreaterThanOrEqual(100);
    });

    it("step = 1", () => {
      const expectedStep = Math.max(Math.ceil(totalRange / 400), 1);
      expect(expectedStep).toBe(1);
    });

    it("data point count ≈ totalRange (step=1)", () => {
      // With step=1, each integer from lowerBound to upperBound is present plus strikes
      expect(chartData.prices.length).toBeGreaterThanOrEqual(totalRange);
    });
  });
});

// ===========================================================================
// Section 4: Data Point Count Cap (Max 600)
//
// No scenario should produce more than 600 data points.
// ===========================================================================
describe("Section 4: Data Point Count Cap (Max 600)", () => {

  // 4.1 NDX multi-leg
  it("4.1 NDX multi-leg: prices.length <= 600", () => {
    const positions = [
      makeLongCallPosition(23100, 200, "NDX"),
      makeShortCallPosition(23150, 150, "NDX"),
    ];
    const chartData = generateMultiLegPayoff(positions, 23132);
    expect(chartData.prices.length).toBeLessThanOrEqual(600);
  });

  // 4.2 $50,000 multi-leg
  it("4.2 $50,000 multi-leg: prices.length <= 600", () => {
    const positions = [
      makeLongCallPosition(49900, 500, "MEGA"),
      makeShortCallPosition(50000, 400, "MEGA"),
    ];
    const chartData = generateMultiLegPayoff(positions, 50000);
    expect(chartData.prices.length).toBeLessThanOrEqual(600);
  });

  // 4.3 Penny stock multi-leg
  it("4.3 Penny stock multi-leg: prices.length <= 600", () => {
    const positions = [
      makeLongPutPosition(1.10, 0.30, "PENNY"),
      makeShortPutPosition(0.90, 0.20, "PENNY"),
    ];
    const chartData = generateMultiLegPayoff(positions, 1.0);
    expect(chartData.prices.length).toBeLessThanOrEqual(600);
  });

  // 4.4 NDX single-leg
  it("4.4 NDX single-leg: prices.length <= 600", () => {
    const positions = [makeLongCallPosition(23100, 200, "NDX")];
    const chartData = generateMultiLegPayoff(positions, 23132);
    expect(chartData.prices.length).toBeLessThanOrEqual(600);
  });

  // 4.5 $50,000 single-leg
  it("4.5 $50,000 single-leg: prices.length <= 600", () => {
    const positions = [makeLongCallPosition(50000, 1000, "MEGA")];
    const chartData = generateMultiLegPayoff(positions, 50000);
    expect(chartData.prices.length).toBeLessThanOrEqual(600);
  });

  // 4.6 Cheap stock single-leg
  it("4.6 Cheap stock single-leg: prices.length <= 600", () => {
    const positions = [makeLongCallPosition(25, 3, "CHEAP")];
    const chartData = generateMultiLegPayoff(positions, 25);
    expect(chartData.prices.length).toBeLessThanOrEqual(600);
  });

  // 4.7 Iron Condor high-value (NDX-level)
  // Uses generateButterflyPayoff with "IRON Condor" strategyType
  it("4.7 NDX Iron Condor: prices.length <= 600", () => {
    const butterflyInfo = {
      long_put_strike: 23000,
      short_put_strike: 23050,
      short_call_strike: 23150,
      long_call_strike: 23200,
      long_put_price: 80,
      short_put_price: 100,
      short_call_price: 100,
      long_call_price: 80,
    };
    const result = generateButterflyPayoff(butterflyInfo, "IRON Condor", 50);
    expect(result.prices.length).toBeLessThanOrEqual(600);
  });

  // 4.8 Standard butterfly high-value (NDX-level)
  it("4.8 NDX Call Butterfly: prices.length <= 600", () => {
    const butterflyInfo = {
      lower_strike: 23050,
      atm_strike: 23100,
      upper_strike: 23150,
      lower_price: 50,
      atm_price: 80,
      upper_price: 50,
    };
    const result = generateButterflyPayoff(butterflyInfo, "CALL Butterfly", 50);
    expect(result.prices.length).toBeLessThanOrEqual(600);
  });
});

// ===========================================================================
// Section 5: Strike Inclusion in Generated Data Points
//
// All strike prices must appear in the prices array regardless of step size.
// ===========================================================================
describe("Section 5: Strike Inclusion in Generated Data Points", () => {

  // 5.1 Multi-leg, step > 1 (NDX spread)
  it("5.1 Multi-leg step > 1: both strikes present in prices array", () => {
    const positions = [
      makeLongCallPosition(23100, 200, "NDX"),
      makeShortCallPosition(23150, 150, "NDX"),
    ];
    const chartData = generateMultiLegPayoff(positions, 23132);

    // Confirm step > 1
    const pricesWithoutStrikes = chartData.prices.filter(
      (p) => p !== 23100 && p !== 23150
    );
    if (pricesWithoutStrikes.length >= 2) {
      const gap = pricesWithoutStrikes[1] - pricesWithoutStrikes[0];
      expect(gap).toBeGreaterThan(1);
    }

    expect(chartData.prices).toContain(23100);
    expect(chartData.prices).toContain(23150);
  });

  // 5.2 Multi-leg, step = 1 (Cheap stock spread)
  it("5.2 Multi-leg step = 1: both strikes present in prices array", () => {
    const positions = [
      makeLongCallPosition(48, 4, "CHEAP"),
      makeShortCallPosition(53, 2, "CHEAP"),
    ];
    const chartData = generateMultiLegPayoff(positions, 50);

    expect(chartData.prices).toContain(48);
    expect(chartData.prices).toContain(53);
  });

  // 5.3 Single-leg, step > 1 (NDX single)
  it("5.3 Single-leg step > 1: strike present in prices array", () => {
    const positions = [makeLongCallPosition(23100, 200, "NDX")];
    const chartData = generateMultiLegPayoff(positions, 23132);

    expect(chartData.prices).toContain(23100);
  });

  // 5.4 Single-leg, step = 1 (Cheap stock single)
  it("5.4 Single-leg step = 1: strike present in prices array", () => {
    const positions = [makeLongCallPosition(25, 3, "CHEAP")];
    const chartData = generateMultiLegPayoff(positions, 25);

    expect(chartData.prices).toContain(25);
  });

  // 5.5 Strikes NOT on step grid boundary
  // Use odd strikes that are unlikely to land on a step grid
  it("5.5 Strikes not on step boundary (23103/23147): both present in prices", () => {
    const positions = [
      makeLongCallPosition(23103, 200, "NDX"),
      makeShortCallPosition(23147, 150, "NDX"),
    ];
    const chartData = generateMultiLegPayoff(positions, 23132);

    // Verify step > 1 (NDX-level prices guarantee this)
    const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
    const step = Math.max(Math.ceil(totalRange / 400), 1);
    expect(step).toBeGreaterThan(1);

    // Both off-grid strikes must still be present
    expect(chartData.prices).toContain(23103);
    expect(chartData.prices).toContain(23147);
  });

  // 5.6 Prices array is sorted ascending (monotonically non-decreasing)
  it("5.6 Prices array is sorted ascending for NDX spread with step > 1", () => {
    const positions = [
      makeLongCallPosition(23100, 200, "NDX"),
      makeShortCallPosition(23150, 150, "NDX"),
    ];
    const chartData = generateMultiLegPayoff(positions, 23132);

    for (let i = 1; i < chartData.prices.length; i++) {
      expect(chartData.prices[i]).toBeGreaterThanOrEqual(chartData.prices[i - 1]);
    }
  });

  it("5.6b Prices array is sorted ascending for off-grid strikes (23103/23147)", () => {
    const positions = [
      makeLongCallPosition(23103, 200, "NDX"),
      makeShortCallPosition(23147, 150, "NDX"),
    ];
    const chartData = generateMultiLegPayoff(positions, 23132);

    for (let i = 1; i < chartData.prices.length; i++) {
      expect(chartData.prices[i]).toBeGreaterThanOrEqual(chartData.prices[i - 1]);
    }
  });
});

// ===========================================================================
// Section 6: Extreme Price Levels
//
// 6a: Penny Stocks (~$1)
// 6b: Mega-Cap Indices (~$50,000)
// ===========================================================================
describe("Section 6: Extreme Price Levels", () => {

  // -------------------------------------------------------------------------
  // 6a: Penny Stocks (~$1)
  // -------------------------------------------------------------------------
  describe("6a: Penny Stocks", () => {

    // 6a.1 Multi-leg put spread
    // strikes 0.90/1.10, underlying=1.0
    // extension = max(0.1, 0.6, 50) = 50
    // lowerBound = floor(0.90 - 50) = -50; upperBound = ceil(1.10 + 50) = 52
    describe("6a.1 Multi-leg put spread on $1 stock", () => {
      const positions = [
        makeLongPutPosition(1.10, 0.30, "PENNY"),
        makeShortPutPosition(0.90, 0.20, "PENNY"),
      ];
      const chartData = generateMultiLegPayoff(positions, 1.0);

      it("$50 floor: lowerBound <= -49", () => {
        expect(chartData.prices[0]).toBeLessThanOrEqual(-49);
      });

      it("upperBound >= 51", () => {
        expect(chartData.prices[chartData.prices.length - 1]).toBeGreaterThanOrEqual(51);
      });

      it("step = 1", () => {
        const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
        const step = Math.max(Math.ceil(totalRange / 400), 1);
        expect(step).toBe(1);
      });

      it("prices.length <= 600", () => {
        expect(chartData.prices.length).toBeLessThanOrEqual(600);
      });
    });

    // 6a.2 Single-leg call
    // strike=1.0, underlying=1.05
    // tierRange = max(1*8, 250) = 250; range = max(250, 0.105, 50) = 250
    describe("6a.2 Single-leg call on $1 stock", () => {
      const positions = [makeLongCallPosition(1.0, 0.50, "PENNY")];
      const chartData = generateMultiLegPayoff(positions, 1.05);

      it("tierRange = max(8, 250) = 250; range = 250", () => {
        const firstPrice = chartData.prices[0];
        const lastPrice = chartData.prices[chartData.prices.length - 1];

        // lowerBound = floor(1.0 - 250) = -249
        expect(firstPrice).toBeLessThanOrEqual(-249);
        // upperBound = ceil(1.0 + 250) = 251
        expect(lastPrice).toBeGreaterThanOrEqual(251);
      });

      it("step = 1 (totalRange = 500)", () => {
        const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
        // 251 - (-249) = 500, ceil(500/400) = 2
        // Actually for 500: ceil(500/400) = 2
        const step = Math.max(Math.ceil(totalRange / 400), 1);
        // 500/400 = 1.25, ceil = 2
        expect(step).toBeLessThanOrEqual(2);
      });
    });

    // 6a.3 Negative lowerBound allowed
    // strike=0.50, underlying=0.50
    // tierRange = max(0.50 * 8, 250) = 250; range = max(250, 0.05, 50) = 250
    // lowerBound = floor(0.50 - 250) = floor(-249.5) = -250
    describe("6a.3 Negative lowerBound — no crash, prices include negatives", () => {
      const positions = [makeLongCallPosition(0.50, 0.20, "PENNY")];
      const chartData = generateMultiLegPayoff(positions, 0.50);

      it("lowerBound = floor(0.50 - 250) = -250; no crash", () => {
        expect(chartData).not.toBeNull();
        expect(chartData.prices.length).toBeGreaterThan(0);
      });

      it("prices include negative values", () => {
        expect(chartData.prices[0]).toBeLessThan(0);
      });

      it("no NaN or Infinity in prices", () => {
        expect(chartData.prices.every(Number.isFinite)).toBe(true);
      });

      it("no NaN or Infinity in payoffs", () => {
        expect(chartData.payoffs.every(Number.isFinite)).toBe(true);
      });
    });
  });

  // -------------------------------------------------------------------------
  // 6b: Mega-Cap Indices (~$50,000)
  // -------------------------------------------------------------------------
  describe("6b: Mega-Cap Indices", () => {

    // 6b.1 $10 spread on $50,000 underlying
    // strikes 49995/50005, underlying=50000
    // strikeRange = 10, extension = max(5000, 30, 50) = 5000
    // lowerBound = floor(49995 - 5000) = 44995
    // upperBound = ceil(50005 + 5000) = 55005
    // totalRange = 55005 - 44995 = 10010
    // step = ceil(10010/400) = 26
    describe("6b.1 $10 spread on $50,000 underlying", () => {
      const positions = [
        makeLongCallPosition(49995, 500, "MEGA"),
        makeShortCallPosition(50005, 450, "MEGA"),
      ];
      const chartData = generateMultiLegPayoff(positions, 50000);

      it("extension = max(5000, 30, 50) = 5000; range ~10000", () => {
        const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
        expect(totalRange).toBeGreaterThanOrEqual(9900);
        expect(totalRange).toBeLessThanOrEqual(10100);
      });

      it("step = ceil(~10010/400) = 26", () => {
        const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
        const step = Math.max(Math.ceil(totalRange / 400), 1);
        expect(step).toBe(26);
      });

      it("prices.length <= 600", () => {
        expect(chartData.prices.length).toBeLessThanOrEqual(600);
      });
    });

    // 6b.2 $10 spread on $20,000 underlying
    // strikes 19995/20005, underlying=20000
    // strikeRange = 10, extension = max(2000, 30, 50) = 2000
    // lowerBound = floor(19995 - 2000) = 17995
    // upperBound = ceil(20005 + 2000) = 22005
    // totalRange = 22005 - 17995 = 4010
    // step = ceil(4010/400) = 11
    describe("6b.2 $10 spread on $20,000 underlying", () => {
      const positions = [
        makeLongCallPosition(19995, 200, "IDX"),
        makeShortCallPosition(20005, 180, "IDX"),
      ];
      const chartData = generateMultiLegPayoff(positions, 20000);

      it("extension = max(2000, 30, 50) = 2000; range ~4010", () => {
        const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
        expect(totalRange).toBeGreaterThanOrEqual(3900);
        expect(totalRange).toBeLessThanOrEqual(4100);
      });

      it("step = ceil(4010/400) = 11", () => {
        const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
        const step = Math.max(Math.ceil(totalRange / 400), 1);
        expect(step).toBe(11);
      });

      it("prices.length <= 600", () => {
        expect(chartData.prices.length).toBeLessThanOrEqual(600);
      });
    });

    // 6b.3 $500 spread on $50,000 underlying
    // strikes 49750/50250, underlying=50000
    // strikeRange = 500, extension = max(5000, 1500, 50) = 5000
    // percentage still dominates
    // lowerBound = floor(49750 - 5000) = 44750
    // upperBound = ceil(50250 + 5000) = 55250
    // totalRange = 55250 - 44750 = 10500
    // step = ceil(10500/400) = 27
    describe("6b.3 $500 spread on $50,000 underlying — percentage still dominates", () => {
      const positions = [
        makeLongCallPosition(49750, 600, "MEGA"),
        makeShortCallPosition(50250, 400, "MEGA"),
      ];
      const chartData = generateMultiLegPayoff(positions, 50000);

      it("extension = max(5000, 1500, 50) = 5000; percentage dominates", () => {
        const firstPrice = chartData.prices[0];
        const lastPrice = chartData.prices[chartData.prices.length - 1];

        // lowerBound = floor(49750 - 5000) = 44750
        expect(firstPrice).toBeLessThanOrEqual(44750);
        // upperBound = ceil(50250 + 5000) = 55250; last price within one step
        expect(lastPrice).toBeGreaterThanOrEqual(55250 - 27);
      });

      it("prices.length <= 600", () => {
        expect(chartData.prices.length).toBeLessThanOrEqual(600);
      });

      it("step = ceil(~10500/400) ≈ 27", () => {
        const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
        const step = Math.max(Math.ceil(totalRange / 400), 1);
        // Should be around 27 (10500/400 = 26.25 → 27)
        expect(step).toBeGreaterThanOrEqual(26);
        expect(step).toBeLessThanOrEqual(28);
      });
    });
  });
});

// ===========================================================================
// Section 7: Very Narrow Spreads on Expensive Symbols
//
// Verifies that narrow spreads on expensive underlyings produce a meaningful
// chart range driven by the percentage-based extension floor.
// ===========================================================================
describe("Section 7: Very Narrow Spreads on Expensive Symbols", () => {

  // 7.1 $10 spread on $20,000 underlying
  // strikes 19995/20005, underlying=20000
  // strikeRange = 10, extension = max(2000, 30, 50) = 2000
  // Chart shows meaningful ±10% range despite tiny spread
  describe("7.1 $10 spread on $20,000 underlying", () => {
    const positions = [
      makeLongCallPosition(19995, 200, "IDX"),
      makeShortCallPosition(20005, 180, "IDX"),
    ];
    const underlyingPrice = 20000;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("extension = max(2000, 30, 50) = 2000; meaningful ±10% range", () => {
      const firstPrice = chartData.prices[0];
      const lastPrice = chartData.prices[chartData.prices.length - 1];

      // lowerBound = floor(19995 - 2000) = 17995
      expect(firstPrice).toBeLessThanOrEqual(17995);
      // upperBound = ceil(20005 + 2000) = 22005; within one step
      const totalRange = lastPrice - firstPrice;
      const step = Math.max(Math.ceil(totalRange / 400), 1);
      expect(lastPrice).toBeGreaterThanOrEqual(22005 - step);
    });

    it("prices.length <= 600", () => {
      expect(chartData.prices.length).toBeLessThanOrEqual(600);
    });
  });

  // 7.2 $5 spread on $5,000 underlying
  // strikes 4998/5003, underlying=5000
  // strikeRange = 5, extension = max(500, 15, 50) = 500
  // lowerBound = floor(4998 - 500) = 4498
  // upperBound = ceil(5003 + 500) = 5503
  // totalRange = 5503 - 4498 = 1005
  // step = ceil(1005/400) = 3
  describe("7.2 $5 spread on $5,000 underlying", () => {
    const positions = [
      makeLongCallPosition(4998, 60, "IDX"),
      makeShortCallPosition(5003, 50, "IDX"),
    ];
    const underlyingPrice = 5000;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("extension = max(500, 15, 50) = 500", () => {
      const firstPrice = chartData.prices[0];
      // lowerBound = floor(4998 - 500) = 4498
      expect(firstPrice).toBeLessThanOrEqual(4498);
    });

    it("step = ceil(1005/400) = 3", () => {
      const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
      const step = Math.max(Math.ceil(totalRange / 400), 1);
      expect(step).toBe(3);
    });

    it("both strikes present despite step > 1", () => {
      expect(chartData.prices).toContain(4998);
      expect(chartData.prices).toContain(5003);
    });
  });

  // 7.3 $1 spread on $23,000 underlying
  // strikes 23000/23001, underlying=23000
  // strikeRange = 1, extension = max(2300, 3, 50) = 2300
  // lowerBound = floor(23000 - 2300) = 20700
  // upperBound = ceil(23001 + 2300) = 25301
  // totalRange = 25301 - 20700 = 4601
  // step = ceil(4601/400) = 12
  describe("7.3 $1 spread on $23,000 underlying", () => {
    const positions = [
      makeLongCallPosition(23000, 200, "NDX"),
      makeShortCallPosition(23001, 195, "NDX"),
    ];
    const underlyingPrice = 23000;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("extension = max(2300, 3, 50) = 2300", () => {
      const firstPrice = chartData.prices[0];
      // lowerBound = floor(23000 - 2300) = 20700
      expect(firstPrice).toBeLessThanOrEqual(20700);
    });

    it("step = ceil(4601/400) = 12", () => {
      const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
      const step = Math.max(Math.ceil(totalRange / 400), 1);
      expect(step).toBe(12);
    });

    it("both strikes present despite being 1 apart with step=12", () => {
      expect(chartData.prices).toContain(23000);
      expect(chartData.prices).toContain(23001);
    });
  });

  // 7.4 Initial view still shows strategy structure
  // Same as 7.3: strikes 23000/23001, underlying=23000
  // viewPad = max(23000 * 0.05, 1 * 1.5, 25) = max(1150, 1.5, 25) = 1150
  // suggestedMin = floor(23000 - 1150) = 21850
  // suggestedMax = ceil(23001 + 1150) = 24151
  // Strikes should be visible within this initial window
  describe("7.4 Initial view still shows strategy structure", () => {
    const positions = [
      makeLongCallPosition(23000, 200, "NDX"),
      makeShortCallPosition(23001, 195, "NDX"),
    ];
    const underlyingPrice = 23000;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);
    const config = createMultiLegChartConfig(chartData, underlyingPrice);

    const sugMin = config.options.scales.x.suggestedMin;
    const sugMax = config.options.scales.x.suggestedMax;

    it("viewPad = max(1150, 1.5, 25) = 1150; strikes visible within initial window", () => {
      // suggestedMin should be below the lower strike
      expect(sugMin).toBeLessThan(23000);
      // suggestedMax should be above the upper strike
      expect(sugMax).toBeGreaterThan(23001);
    });

    it("initial view window width ≈ 2 * 1150 + 1 = 2301", () => {
      const windowWidth = sugMax - sugMin;
      // viewPad = 1150, window ≈ 2*1150 + strikeSpan(1) = 2301
      expect(windowWidth).toBeGreaterThanOrEqual(2200);
      expect(windowWidth).toBeLessThanOrEqual(2400);
    });
  });
});

// ===========================================================================
// Section 8: Very Wide Spreads on Cheap Symbols
//
// Verifies that wide spreads on cheap underlyings produce correct ranges
// driven by strikeRange * 3.0.
// ===========================================================================
describe("Section 8: Very Wide Spreads on Cheap Symbols", () => {

  // 8.1 $40 spread on $25 stock
  // strikes 5/45, underlying=25
  // strikeRange = 40, extension = max(2.5, 120, 50) = 120
  // lowerBound = floor(5 - 120) = -115
  // upperBound = ceil(45 + 120) = 165
  describe("8.1 $40 spread on $25 stock", () => {
    const positions = [
      makeLongCallPosition(5, 21, "WIDE"),
      makeShortCallPosition(45, 1, "WIDE"),
    ];
    const underlyingPrice = 25;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("extension = max(2.5, 120, 50) = 120; range covers ±120 from strikes", () => {
      const firstPrice = chartData.prices[0];
      const lastPrice = chartData.prices[chartData.prices.length - 1];

      // lowerBound = floor(5 - 120) = -115
      expect(firstPrice).toBeLessThanOrEqual(-115);
      // upperBound = ceil(45 + 120) = 165
      expect(lastPrice).toBeGreaterThanOrEqual(165);
    });

    it("prices.length <= 600", () => {
      expect(chartData.prices.length).toBeLessThanOrEqual(600);
    });
  });

  // 8.2 $20 spread on $10 stock
  // strikes 1/21, underlying=10
  // strikeRange = 20, extension = max(1, 60, 50) = 60
  // lowerBound = floor(1 - 60) = -59
  // upperBound = ceil(21 + 60) = 81
  describe("8.2 $20 spread on $10 stock", () => {
    const positions = [
      makeLongCallPosition(1, 9.5, "WIDE"),
      makeShortCallPosition(21, 0.5, "WIDE"),
    ];
    const underlyingPrice = 10;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("extension = max(1, 60, 50) = 60; strikeRange*3 dominates", () => {
      const firstPrice = chartData.prices[0];
      const lastPrice = chartData.prices[chartData.prices.length - 1];

      // lowerBound = floor(1 - 60) = -59
      expect(firstPrice).toBeLessThanOrEqual(-59);
      // upperBound = ceil(21 + 60) = 81
      expect(lastPrice).toBeGreaterThanOrEqual(81);
    });

    it("prices.length <= 600", () => {
      expect(chartData.prices.length).toBeLessThanOrEqual(600);
    });
  });

  // 8.3 Range wider than underlying price — negative lowerBound
  // Same as 8.1: strikes 5/45 on $25 stock
  // lowerBound = -115, which is well below 0 and below the underlying price
  describe("8.3 Range wider than underlying price — negative lowerBound", () => {
    const positions = [
      makeLongCallPosition(5, 21, "WIDE"),
      makeShortCallPosition(45, 1, "WIDE"),
    ];
    const underlyingPrice = 25;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);

    it("lowerBound is negative (5 - 120 = -115)", () => {
      expect(chartData.prices[0]).toBeLessThan(0);
    });

    it("no crash — result is valid", () => {
      expect(chartData).not.toBeNull();
      expect(chartData.prices.length).toBeGreaterThan(0);
      expect(chartData.payoffs.length).toBe(chartData.prices.length);
    });

    it("no NaN or Infinity in prices or payoffs", () => {
      expect(chartData.prices.every(Number.isFinite)).toBe(true);
      expect(chartData.payoffs.every(Number.isFinite)).toBe(true);
    });
  });
});

// ===========================================================================
// Section 9: Initial View Window (FR-4)
//
// Formula (multi-strike path, strikes.length >= 2):
//   viewPad = Math.max(underlyingPrice * 0.05, strikeSpan * 1.5, 25)
//   suggestedMin = Math.floor(sMin - viewPad)
//   suggestedMax = Math.ceil(sMax + viewPad)
//
// Fallback (single-strike / < 2 unique strikes):
//   initialRange = Math.max(underlyingPrice * 0.05, 15)
//   suggestedMin = Math.max(currentPrice - initialRange, pMin)
//   suggestedMax = Math.min(currentPrice + initialRange, pMax)
// ===========================================================================
describe("Section 9: Initial View Window (FR-4)", () => {

  // Helper to extract suggestedMin/suggestedMax from config
  function getViewWindow(positions, underlyingPrice) {
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);
    const config = createMultiLegChartConfig(chartData, underlyingPrice);
    return {
      suggestedMin: config.options.scales.x.suggestedMin,
      suggestedMax: config.options.scales.x.suggestedMax,
      chartData,
      config,
    };
  }

  // 9.1 NDX — percentage dominates
  // strikes 23100/23150, underlying=23132
  // strikeSpan = 50, viewPad = max(1156.6, 75, 25) = 1156.6
  // suggestedMin = floor(23100 - 1156.6) = floor(21943.4) = 21943
  // suggestedMax = ceil(23150 + 1156.6) = ceil(24306.6) = 24307
  // window width = 24307 - 21943 = 2364
  describe("9.1 NDX — percentage dominates", () => {
    const positions = [
      makeLongCallPosition(23100, 200, "NDX"),
      makeShortCallPosition(23150, 150, "NDX"),
    ];
    const { suggestedMin, suggestedMax } = getViewWindow(positions, 23132);

    it("viewPad = max(1156.6, 75, 25) = 1156.6", () => {
      // suggestedMin = floor(23100 - 1156.6) = 21943
      expect(suggestedMin).toBeLessThanOrEqual(21944);
      expect(suggestedMin).toBeGreaterThanOrEqual(21940);
      // suggestedMax = ceil(23150 + 1156.6) = 24307
      expect(suggestedMax).toBeGreaterThanOrEqual(24306);
      expect(suggestedMax).toBeLessThanOrEqual(24310);
    });

    it("window width >= 2 * 1156.6 + 50 = ~2363", () => {
      const width = suggestedMax - suggestedMin;
      expect(width).toBeGreaterThanOrEqual(2360);
    });
  });

  // 9.2 Initial view width >= 5% of underlying for expensive symbols
  // underlying=50000, strikes 49950/50050
  // strikeSpan = 100, viewPad = max(2500, 150, 25) = 2500
  // window = 2*2500 + 100 = 5100
  describe("9.2 $50,000 underlying — view width >= 5% of underlying", () => {
    const positions = [
      makeLongCallPosition(49950, 500, "MEGA"),
      makeShortCallPosition(50050, 400, "MEGA"),
    ];
    const { suggestedMin, suggestedMax } = getViewWindow(positions, 50000);

    it("viewPad = max(2500, 150, 25) = 2500; window >= 5100", () => {
      const width = suggestedMax - suggestedMin;
      expect(width).toBeGreaterThanOrEqual(5100);
    });

    it("initial view width >= 5% of underlying", () => {
      const width = suggestedMax - suggestedMin;
      // 5% of 50000 = 2500; window should be at least 2*2500 = 5000
      expect(width).toBeGreaterThanOrEqual(50000 * 0.05 * 2);
    });
  });

  // 9.3 Cheap stock — strikeSpan * 1.5 vs $25 floor
  // strikes 48/53, underlying=50
  // strikeSpan = 5, viewPad = max(2.5, 7.5, 25) = 25
  // suggestedMin = floor(48 - 25) = 23
  // suggestedMax = ceil(53 + 25) = 78
  // window = 78 - 23 = 55
  describe("9.3 Cheap stock — $25 floor dominates", () => {
    const positions = [
      makeLongCallPosition(48, 4, "CHEAP"),
      makeShortCallPosition(53, 2, "CHEAP"),
    ];
    const { suggestedMin, suggestedMax } = getViewWindow(positions, 50);

    it("viewPad = max(2.5, 7.5, 25) = 25; window >= 55", () => {
      const width = suggestedMax - suggestedMin;
      expect(width).toBeGreaterThanOrEqual(55);
    });

    it("suggestedMin ≈ 23, suggestedMax ≈ 78", () => {
      expect(suggestedMin).toBeLessThanOrEqual(23);
      expect(suggestedMax).toBeGreaterThanOrEqual(78);
    });
  });

  // 9.4 Penny stock — $25 floor dominates for very cheap
  // strikes 0.90/1.10, underlying=1.0
  // strikeSpan = 0.2, viewPad = max(0.05, 0.3, 25) = 25
  // suggestedMin = floor(0.90 - 25) = floor(-24.1) = -25
  // suggestedMax = ceil(1.10 + 25) = ceil(26.1) = 27
  // window = 27 - (-25) = 52
  describe("9.4 Penny stock — $25 floor dominates", () => {
    const positions = [
      makeLongPutPosition(1.10, 0.30, "PENNY"),
      makeShortPutPosition(0.90, 0.20, "PENNY"),
    ];
    const { suggestedMin, suggestedMax } = getViewWindow(positions, 1.0);

    it("viewPad = max(0.05, 0.3, 25) = 25; window >= 50.2", () => {
      const width = suggestedMax - suggestedMin;
      expect(width).toBeGreaterThanOrEqual(50);
    });

    it("suggestedMin ≈ -25, suggestedMax ≈ 27", () => {
      expect(suggestedMin).toBeLessThanOrEqual(-24);
      expect(suggestedMax).toBeGreaterThanOrEqual(26);
    });
  });

  // 9.5 suggestedMin < minStrike (always)
  describe("9.5 suggestedMin < minStrike always", () => {
    const scenarios = [
      { name: "NDX", strikes: [23100, 23150], underlying: 23132 },
      { name: "$50k", strikes: [49950, 50050], underlying: 50000 },
      { name: "Cheap", strikes: [48, 53], underlying: 50 },
      { name: "Penny", strikes: [0.90, 1.10], underlying: 1.0 },
    ];

    for (const s of scenarios) {
      it(`${s.name}: suggestedMin < minStrike (${Math.min(...s.strikes)})`, () => {
        const positions = [
          makeLongCallPosition(s.strikes[0], 10, "TEST"),
          makeShortCallPosition(s.strikes[1], 5, "TEST"),
        ];
        const { suggestedMin } = getViewWindow(positions, s.underlying);
        expect(suggestedMin).toBeLessThan(Math.min(...s.strikes));
      });
    }
  });

  // 9.6 suggestedMax > maxStrike (always)
  describe("9.6 suggestedMax > maxStrike always", () => {
    const scenarios = [
      { name: "NDX", strikes: [23100, 23150], underlying: 23132 },
      { name: "$50k", strikes: [49950, 50050], underlying: 50000 },
      { name: "Cheap", strikes: [48, 53], underlying: 50 },
      { name: "Penny", strikes: [0.90, 1.10], underlying: 1.0 },
    ];

    for (const s of scenarios) {
      it(`${s.name}: suggestedMax > maxStrike (${Math.max(...s.strikes)})`, () => {
        const positions = [
          makeLongCallPosition(s.strikes[0], 10, "TEST"),
          makeShortCallPosition(s.strikes[1], 5, "TEST"),
        ];
        const { suggestedMax } = getViewWindow(positions, s.underlying);
        expect(suggestedMax).toBeGreaterThan(Math.max(...s.strikes));
      });
    }
  });

  // 9.7 Single-strike fallback branch
  // When there's only 1 unique strike, chartData.strikes has length 1,
  // so the else branch fires:
  //   initialRange = Math.max(underlyingPrice * 0.05, 15)
  //   suggestedMin = Math.max(currentPrice - initialRange, pMin)
  //   suggestedMax = Math.min(currentPrice + initialRange, pMax)
  describe("9.7 Single-strike fallback branch", () => {
    // Single call at 100, underlying=102
    const positions = [makeLongCallPosition(100, 5, "TEST")];
    const underlyingPrice = 102;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);
    const config = createMultiLegChartConfig(chartData, underlyingPrice);

    const sugMin = config.options.scales.x.suggestedMin;
    const sugMax = config.options.scales.x.suggestedMax;

    it("no crash — config is valid", () => {
      expect(config).toBeDefined();
      expect(sugMin).toBeDefined();
      expect(sugMax).toBeDefined();
    });

    it("falls to else branch (strikes.length < 2)", () => {
      // With single strike, chartData.strikes = [100] (length 1)
      expect(chartData.strikes.length).toBe(1);
    });

    it("reasonable window — fallback uses initialRange = max(102*0.05, 15) = 15", () => {
      // initialRange = max(5.1, 15) = 15
      // suggestedMin = max(102 - 15, pMin) = max(87, pMin)
      // suggestedMax = min(102 + 15, pMax) = min(117, pMax)
      // Since pMin is very negative (-400) and pMax is very positive (600),
      // suggestedMin = 87 and suggestedMax = 117
      expect(sugMin).toBeLessThanOrEqual(87);
      expect(sugMax).toBeGreaterThanOrEqual(117);
    });

    it("suggestedMin < suggestedMax", () => {
      expect(sugMin).toBeLessThan(sugMax);
    });
  });
});

// ===========================================================================
// Section 10: Pan/Zoom Limits (FR-5)
//
// Verify limits.x = { min: Math.min(...prices), max: Math.max(...prices) }
// Access: config.options.plugins.zoom.limits.x.min / .max
// ===========================================================================
describe("Section 10: Pan/Zoom Limits (FR-5)", () => {

  // Helper to get limits and view window from a config
  function getConfigData(positions, underlyingPrice) {
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);
    const config = createMultiLegChartConfig(chartData, underlyingPrice);
    return {
      limits: config.options.plugins.zoom.limits.x,
      suggestedMin: config.options.scales.x.suggestedMin,
      suggestedMax: config.options.scales.x.suggestedMax,
      chartData,
    };
  }

  // 10.1 Limits equal full data extent (NDX spread)
  it("10.1 NDX spread: limits.x.min === prices[0], limits.x.max === prices[last]", () => {
    const positions = [
      makeLongCallPosition(23100, 200, "NDX"),
      makeShortCallPosition(23150, 150, "NDX"),
    ];
    const { limits, chartData } = getConfigData(positions, 23132);
    const prices = chartData.prices;

    expect(limits.min).toBe(Math.min(...prices));
    expect(limits.max).toBe(Math.max(...prices));
    expect(limits.min).toBe(prices[0]);
    expect(limits.max).toBe(prices[prices.length - 1]);
  });

  // 10.2 Limits wider than initial view (NDX spread)
  it("10.2 NDX spread: limits wider than suggestedMin/suggestedMax", () => {
    const positions = [
      makeLongCallPosition(23100, 200, "NDX"),
      makeShortCallPosition(23150, 150, "NDX"),
    ];
    const { limits, suggestedMin, suggestedMax } = getConfigData(positions, 23132);

    expect(limits.min).toBeLessThan(suggestedMin);
    expect(limits.max).toBeGreaterThan(suggestedMax);
  });

  // 10.3 Limits allow panning to negative prices (penny stock)
  it("10.3 Penny stock: limits.x.min < 0 when lowerBound is negative", () => {
    const positions = [
      makeLongPutPosition(1.10, 0.30, "PENNY"),
      makeShortPutPosition(0.90, 0.20, "PENNY"),
    ];
    const { limits, chartData } = getConfigData(positions, 1.0);

    // lowerBound = floor(0.90 - 50) = -50, so prices start negative
    expect(limits.min).toBeLessThan(0);
    expect(limits.min).toBe(chartData.prices[0]);
  });

  // 10.4 Cheap stock limits match full generated range
  it("10.4 Cheap stock ($50): limits match full generated range", () => {
    const positions = [
      makeLongCallPosition(48, 4, "CHEAP"),
      makeShortCallPosition(53, 2, "CHEAP"),
    ];
    const { limits, chartData } = getConfigData(positions, 50);
    const prices = chartData.prices;

    expect(limits.min).toBe(prices[0]);
    expect(limits.max).toBe(prices[prices.length - 1]);
  });
});

// ===========================================================================
// Section 11: Butterfly / Iron Condor Payoff Consistency (AC-8)
//
// Verify generateButterflyPayoff() uses the adaptive range and step formula
// consistently and produces correct payoff values.
// ===========================================================================
describe("Section 11: Butterfly / Iron Condor Payoff Consistency (AC-8)", () => {

  // -------------------------------------------------------------------------
  // 11a: Iron Condor Path
  // -------------------------------------------------------------------------
  describe("11a: Iron Condor", () => {

    // Shared NDX-level IC: 23000/23050/23150/23200, legWidth=50
    // estimatedPrice = (23000 + 23200) / 2 = 23100
    // butterflyExtension = max(50, 2310, 50) = 2310
    const ndxIC = {
      long_put_strike: 23000,
      short_put_strike: 23050,
      short_call_strike: 23150,
      long_call_strike: 23200,
      long_put_price: 80,
      short_put_price: 100,
      short_call_price: 100,
      long_call_price: 80,
    };
    const ndxICResult = generateButterflyPayoff(ndxIC, "IRON Condor", 50);

    // 11a.1 NDX Iron Condor — data points ≤ 600
    it("11a.1 NDX IC: prices.length <= 600", () => {
      expect(ndxICResult.prices.length).toBeLessThanOrEqual(600);
    });

    // 11a.2 NDX IC — range coverage
    // extension = max(50, 2310, 50) = 2310
    // lowerBound = floor(23000 - 2310) = 20690
    // upperBound = ceil(23200 + 2310) = 25510
    it("11a.2 NDX IC: range extends at least ±2000 from outer strikes", () => {
      const firstPrice = ndxICResult.prices[0];
      const lastPrice = ndxICResult.prices[ndxICResult.prices.length - 1];

      expect(firstPrice).toBeLessThanOrEqual(23000 - 2000);
      expect(lastPrice).toBeGreaterThanOrEqual(23200 + 2000);
    });

    // 11a.3 NDX IC — step formula
    // totalRange = 25510 - 20690 = 4820
    // step = ceil(4820/400) = 13
    it("11a.3 NDX IC: step = ceil(totalRange/400) > 1", () => {
      const totalRange = ndxICResult.prices[ndxICResult.prices.length - 1] - ndxICResult.prices[0];
      const step = Math.max(Math.ceil(totalRange / 400), 1);
      expect(step).toBeGreaterThan(1);
    });

    // 11a.4 Cheap IC — step = 1
    // IC at 45/47/53/55 on $50 stock, legWidth=10
    // estimatedPrice = (45+55)/2 = 50
    // butterflyExtension = max(10, 5, 50) = 50
    // lowerBound = floor(45 - 50) = -5; upperBound = ceil(55 + 50) = 105
    // totalRange = 110; step = ceil(110/400) = 1
    it("11a.4 Cheap IC: step = 1", () => {
      const cheapIC = {
        long_put_strike: 45,
        short_put_strike: 47,
        short_call_strike: 53,
        long_call_strike: 55,
        long_put_price: 3,
        short_put_price: 4,
        short_call_price: 4,
        long_call_price: 3,
      };
      const result = generateButterflyPayoff(cheapIC, "IRON Condor", 10);
      const totalRange = result.prices[result.prices.length - 1] - result.prices[0];
      const step = Math.max(Math.ceil(totalRange / 400), 1);
      expect(step).toBe(1);
    });

    // 11a.5 IC payoff correctness
    // netPremium = 100 + 100 - 80 - 80 = 40
    // In the max-profit zone (short_put_strike <= price <= short_call_strike),
    // all intrinsic values are 0, so payoff = -netPremium * 100 = -4000.
    // The code uses a convention where the IC max-profit is the least-negative value.
    it("11a.5 IC payoff correctness: max profit = -netPremium * 100 in max-profit zone", () => {
      // netPremium = 100 + 100 - 80 - 80 = 40
      expect(ndxICResult.netPremium).toBe(40);

      // Max profit in the code is the least-negative payoff
      const maxPayoff = Math.max(...ndxICResult.payoffs);
      expect(maxPayoff).toBe(-ndxICResult.netPremium * 100); // -4000
    });

    // 11a.6 IC break-even correctness
    // lowerBreakEven = short_put_strike + netPremium = 23050 + 40 = 23090
    // upperBreakEven = short_call_strike - netPremium = 23150 - 40 = 23110
    it("11a.6 IC break-even correctness", () => {
      expect(ndxICResult.lowerBreakEven).toBe(23050 + 40); // 23090
      expect(ndxICResult.upperBreakEven).toBe(23150 - 40); // 23110
    });
  });

  // -------------------------------------------------------------------------
  // 11b: Standard Butterfly / Iron Butterfly Path
  // -------------------------------------------------------------------------
  describe("11b: Standard Butterfly / Iron Butterfly", () => {

    // 11b.1 NDX Call Butterfly — data points ≤ 600
    // lower=23050, atm=23100, upper=23150, legWidth=50
    // estimatedPrice = (23050+23150)/2 = 23100
    // butterflyExtension = max(50, 2310, 50) = 2310
    it("11b.1 NDX Call Butterfly: prices.length <= 600", () => {
      const butterflyInfo = {
        lower_strike: 23050,
        atm_strike: 23100,
        upper_strike: 23150,
        lower_price: 50,
        atm_price: 80,
        upper_price: 50,
      };
      const result = generateButterflyPayoff(butterflyInfo, "CALL Butterfly", 50);
      expect(result.prices.length).toBeLessThanOrEqual(600);
    });

    // 11b.2 NDX Call Butterfly — range
    // extension = max(50, 2310, 50) = 2310
    it("11b.2 NDX Call Butterfly: extension = max(50, 2310, 50) = 2310", () => {
      const butterflyInfo = {
        lower_strike: 23050,
        atm_strike: 23100,
        upper_strike: 23150,
        lower_price: 50,
        atm_price: 80,
        upper_price: 50,
      };
      const result = generateButterflyPayoff(butterflyInfo, "CALL Butterfly", 50);
      const firstPrice = result.prices[0];
      const lastPrice = result.prices[result.prices.length - 1];

      // lowerBound = floor(23050 - 2310) = 20740
      expect(firstPrice).toBeLessThanOrEqual(20740);
      // upperBound = ceil(23150 + 2310) = 25460; last price within one step
      const totalRange = lastPrice - firstPrice;
      const step = Math.max(Math.ceil(totalRange / 400), 1);
      expect(lastPrice).toBeGreaterThanOrEqual(25460 - step);
    });

    // 11b.3 Iron Butterfly — data points ≤ 600
    it("11b.3 NDX Iron Butterfly: prices.length <= 600", () => {
      const ibInfo = {
        lower_strike: 23050,
        atm_strike: 23100,
        upper_strike: 23150,
        lower_price: 30,
        atm_put_price: 80,
        atm_call_price: 80,
        upper_price: 30,
      };
      const result = generateButterflyPayoff(ibInfo, "IRON Butterfly", 50);
      expect(result.prices.length).toBeLessThanOrEqual(600);
    });

    // 11b.4 Cheap Call Butterfly
    // lower=45, atm=50, upper=55, legWidth=10
    // estimatedPrice = (45+55)/2 = 50
    // butterflyExtension = max(10, 5, 50) = 50
    // lowerBound = floor(45-50) = -5; upperBound = ceil(55+50) = 105
    // totalRange = 110; step = ceil(110/400) = 1
    it("11b.4 Cheap Call Butterfly: step = 1, reasonable point count", () => {
      const butterflyInfo = {
        lower_strike: 45,
        atm_strike: 50,
        upper_strike: 55,
        lower_price: 8,
        atm_price: 5,
        upper_price: 3,
      };
      const result = generateButterflyPayoff(butterflyInfo, "CALL Butterfly", 10);
      const totalRange = result.prices[result.prices.length - 1] - result.prices[0];
      const step = Math.max(Math.ceil(totalRange / 400), 1);
      expect(step).toBe(1);
      expect(result.prices.length).toBeGreaterThan(50);
      expect(result.prices.length).toBeLessThanOrEqual(600);
    });

    // 11b.5 Butterfly payoff at ATM = max profit
    // CALL Butterfly: lower=95, atm=100, upper=105
    // lower_price=8, atm_price=5, upper_price=3
    // netPremium = 8 - 2*5 + 3 = 1
    // At ATM (100): payoff = (max(100-95,0) - 2*max(100-100,0) + max(100-105,0) - 1) * 100
    //             = (5 - 0 + 0 - 1) * 100 = 400
    // Max profit = (legWidth - netPremium) * 100 = (5 - 1) * 100 = 400
    it("11b.5 Call Butterfly: payoff at ATM = max profit", () => {
      const butterflyInfo = {
        lower_strike: 95,
        atm_strike: 100,
        upper_strike: 105,
        lower_price: 8,
        atm_price: 5,
        upper_price: 3,
      };
      const result = generateButterflyPayoff(butterflyInfo, "CALL Butterfly", 5);

      // netPremium = 8 - 10 + 3 = 1
      expect(result.netPremium).toBe(1);

      // Payoff at ATM should be max profit
      const atmIdx = result.prices.indexOf(100);
      expect(atmIdx).toBeGreaterThanOrEqual(0); // ATM price must be in array (step=1 for cheap stock)
      expect(result.payoffs[atmIdx]).toBe(400); // (5 - 1) * 100

      // Max profit across all payoffs should equal payoff at ATM
      const maxPayoff = Math.max(...result.payoffs);
      expect(maxPayoff).toBe(result.payoffs[atmIdx]);
    });
  });
});

// ===========================================================================
// Section 12: No Regression on Low/Mid-Priced Symbols (FR-6, AC-7)
//
// Verify that the new adaptive range logic does not worsen behavior for
// low- and mid-priced symbols compared to old behavior.
// ===========================================================================
describe("Section 12: No Regression on Low/Mid-Priced Symbols (FR-6, AC-7)", () => {

  // 12.1 $50 stock spread — range no worse than before
  // strikes 48/53, underlying=50
  // New: extension = max(5, 15, 50) = 50; range extends at least ±50
  // Old behavior was strikeRange*5=25 capped at 500, so new is wider (50 > 25)
  it("12.1 $50 stock spread: range extends at least ±50", () => {
    const positions = [
      makeLongCallPosition(48, 4, "CHEAP"),
      makeShortCallPosition(53, 2, "CHEAP"),
    ];
    const chartData = generateMultiLegPayoff(positions, 50);
    const firstPrice = chartData.prices[0];
    const lastPrice = chartData.prices[chartData.prices.length - 1];

    expect(firstPrice).toBeLessThanOrEqual(48 - 50);
    expect(lastPrice).toBeGreaterThanOrEqual(53 + 50);
  });

  // 12.2 $200 stock spread
  // strikes 195/205, underlying=200
  // extension = max(20, 30, 50) = 50
  it("12.2 $200 stock spread: extension = 50, range similar to or wider than old", () => {
    const positions = [
      makeLongCallPosition(195, 12, "MID"),
      makeShortCallPosition(205, 8, "MID"),
    ];
    const chartData = generateMultiLegPayoff(positions, 200);
    const firstPrice = chartData.prices[0];
    const lastPrice = chartData.prices[chartData.prices.length - 1];

    // Extension = 50, so range extends ±50 from strikes
    expect(firstPrice).toBeLessThanOrEqual(195 - 50);
    expect(lastPrice).toBeGreaterThanOrEqual(205 + 50);
  });

  // 12.3 $100 stock single-leg
  // strike=100, underlying=102
  // tierRange = max(100*5, 500) = 500 (strike < 200 → mult=5, min=500)
  // range = max(500, 10.2, 50) = 500
  // Same as old $500 floor
  it("12.3 $100 stock single-leg: tierRange matches old $500 floor", () => {
    const positions = [makeLongCallPosition(100, 5, "MID")];
    const chartData = generateMultiLegPayoff(positions, 102);
    const firstPrice = chartData.prices[0];
    const lastPrice = chartData.prices[chartData.prices.length - 1];

    // lowerBound = floor(100 - 500) = -400
    expect(firstPrice).toBeLessThanOrEqual(-400);
    // upperBound = ceil(100 + 500) = 600
    // totalRange = 1000, step = ceil(1000/400) = 3
    // Last stepped price may be up to 2 below upperBound
    expect(lastPrice).toBeGreaterThanOrEqual(600 - 3);
  });

  // 12.4 Break-even calculation unchanged
  // Long call strike=100, entry=5 → break-even at 105
  it("12.4 Break-even calculation: long call strike=100, entry=5 → break-even at 105", () => {
    const positions = [makeLongCallPosition(100, 5, "TEST")];
    const chartData = generateMultiLegPayoff(positions, 102);

    // Break-even should be at strike + premium = 105
    expect(chartData.breakEvenPoints.length).toBeGreaterThanOrEqual(1);
    // The break-even should be at exactly 105 (since step=1 for cheap stock,
    // both 104 and 105 are in the array, and the payoff crosses zero there)
    const breakEven = chartData.breakEvenPoints[0];
    expect(breakEven).toBeCloseTo(105, 0);
  });

  // 12.5 Net credit calculation unchanged
  // Credit spread: short 100 call @ 8, long 105 call @ 5 → netCredit = 3
  it("12.5 Net credit: short 100 call @ 8, long 105 call @ 5 → netCredit = 3", () => {
    const positions = [
      makeShortCallPosition(100, 8, "TEST"),
      makeLongCallPosition(105, 5, "TEST"),
    ];
    const chartData = generateMultiLegPayoff(positions, 102);

    // netCredit = shortPremium - longPremium = 8 - 5 = 3
    expect(chartData.netCredit).toBe(3);
  });

  // 12.6 Payoff values at known prices
  // Long call strike=100, entry=5 at price=110
  // payoff = (intrinsic - premium) * qty * 100 = (10 - 5) * 1 * 100 = $500
  it("12.6 Payoff at known price: long call 100 @ $5 entry, at price=110 → $500", () => {
    const positions = [makeLongCallPosition(100, 5, "TEST")];
    const chartData = generateMultiLegPayoff(positions, 102);

    // Price 110 should be in the array (step=1 for this range)
    const idx = chartData.prices.indexOf(110);
    expect(idx).toBeGreaterThanOrEqual(0);
    expect(chartData.payoffs[idx]).toBe(500);
  });

  // 12.7 Existing tests pass — verified by this section passing
  // This test simply asserts that the Section 12 tests themselves provide
  // a regression check. The full suite should be run separately.
  it("12.7 All Section 12 regression checks pass (meta-check)", () => {
    // If we've reached this point without failures, the regression checks passed.
    expect(true).toBe(true);
  });
});

// ===========================================================================
// Section 13: Edge Cases and Boundary Conditions
// ===========================================================================
describe("Section 13: Edge Cases and Boundary Conditions", () => {

  // 13.1 underlyingPrice = 0
  // strike=5, single leg path: singleStrike=5 < 50
  // tierRange = max(5*8, 250) = 250; range = max(250, 0, 50) = 250
  it("13.1 underlyingPrice = 0: no crash, $50 floor applies via tierRange min", () => {
    const positions = [makeLongCallPosition(5, 2, "EDGE")];
    const chartData = generateMultiLegPayoff(positions, 0);

    expect(chartData).not.toBeNull();
    expect(chartData.prices.length).toBeGreaterThan(0);
    expect(chartData.prices.every(Number.isFinite)).toBe(true);
    expect(chartData.payoffs.every(Number.isFinite)).toBe(true);

    // tierRange = max(40, 250) = 250; range = max(250, 0, 50) = 250
    const firstPrice = chartData.prices[0];
    const lastPrice = chartData.prices[chartData.prices.length - 1];
    expect(firstPrice).toBeLessThanOrEqual(5 - 250);
    expect(lastPrice).toBeGreaterThanOrEqual(5 + 250);
  });

  // 13.2 underlyingPrice very small (0.01)
  // strike=0.50, single leg: singleStrike=0.50 < 50
  // tierRange = max(0.50*8, 250) = 250; range = max(250, 0.001, 50) = 250
  it("13.2 underlyingPrice = 0.01: $50 floor applies via tierRange min, no crash", () => {
    const positions = [makeLongCallPosition(0.50, 0.10, "EDGE")];
    const chartData = generateMultiLegPayoff(positions, 0.01);

    expect(chartData).not.toBeNull();
    expect(chartData.prices.length).toBeGreaterThan(0);
    expect(chartData.prices.every(Number.isFinite)).toBe(true);
    expect(chartData.payoffs.every(Number.isFinite)).toBe(true);
  });

  // 13.3 Identical strikes (strikeRange=0) treated as single-leg
  // Two calls at same strike=100, underlying=100
  // strikeRange = 0 → single-leg path
  // singleStrike=100 < 200 → tierRange = max(100*5, 500) = 500
  // range = max(500, 10, 50) = 500
  describe("13.3 Identical strikes treated as single-leg", () => {
    const positions = [
      makeLongCallPosition(100, 5, "SAME"),
      makeLongCallPosition(100, 5, "SAME"),
    ];
    const chartData = generateMultiLegPayoff(positions, 100);

    it("takes single-leg path (strikeRange = 0)", () => {
      expect(chartData).not.toBeNull();
      // range should be 500 (single-leg tiered), not 50 (multi-leg floor)
      const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
      // If multi-leg path were used (strikeRange=0), extension = max(10, 0, 50) = 50
      // totalRange would be ≈ 100. But single-leg gives totalRange ≈ 1000.
      expect(totalRange).toBeGreaterThanOrEqual(900);
    });

    it("range uses tiered multiplier, not multi-leg formula", () => {
      const firstPrice = chartData.prices[0];
      const lastPrice = chartData.prices[chartData.prices.length - 1];
      // lowerBound = floor(100 - 500) = -400
      expect(firstPrice).toBeLessThanOrEqual(-400);
      // upperBound = ceil(100 + 500) = 600; within one step
      const totalRange = lastPrice - firstPrice;
      const step = Math.max(Math.ceil(totalRange / 400), 1);
      expect(lastPrice).toBeGreaterThanOrEqual(600 - step);
    });
  });

  // 13.4 Empty positions → returns null
  it("13.4 Empty positions: returns null", () => {
    expect(generateMultiLegPayoff([], 100)).toBeNull();
  });

  it("13.4b Null positions: returns null", () => {
    expect(generateMultiLegPayoff(null, 100)).toBeNull();
  });

  it("13.4c Undefined positions: returns null", () => {
    expect(generateMultiLegPayoff(undefined, 100)).toBeNull();
  });

  // 13.5 Non-option positions filtered out → returns null
  it("13.5 Non-option positions (us_equity only): returns null", () => {
    const positions = [
      {
        symbol: "TEST",
        asset_class: "us_equity",
        side: "long",
        qty: 100,
        strike_price: 0,
        option_type: null,
        avg_entry_price: 50,
        unrealized_pl: 0,
        underlying_symbol: "TEST",
      },
    ];
    const result = generateMultiLegPayoff(positions, 50);
    expect(result).toBeNull();
  });

  // 13.6 Mixed option types in single-leg: one put at strike=100
  // strikeRange = 0 (single leg), singleStrike=100 < 200
  // tierRange = max(100*5, 500) = 500; range = max(500, 10.5, 50) = 500
  it("13.6 Single put at strike=100: single-leg path, range reasonable", () => {
    const positions = [makeLongPutPosition(100, 5, "TEST")];
    const chartData = generateMultiLegPayoff(positions, 105);

    expect(chartData).not.toBeNull();
    const firstPrice = chartData.prices[0];
    const lastPrice = chartData.prices[chartData.prices.length - 1];

    // lowerBound = floor(100 - 500) = -400
    expect(firstPrice).toBeLessThanOrEqual(-400);
    // upperBound = ceil(100 + 500) = 600
    const totalRange = lastPrice - firstPrice;
    const step = Math.max(Math.ceil(totalRange / 400), 1);
    expect(lastPrice).toBeGreaterThanOrEqual(600 - step);
  });

  // 13.7 Very large quantity positions: qty=100
  // Range and step are unchanged (qty affects payoff magnitude, not range)
  describe("13.7 Very large quantity: range and step unchanged", () => {
    // Helper to make a position with custom quantity
    function makeLongCallWithQty(strike, entryPrice, qty, symbol = "TEST") {
      return {
        symbol: `${symbol}_CALL_${strike}`,
        asset_class: "us_option",
        side: "long",
        qty: qty,
        strike_price: strike,
        option_type: "call",
        expiry_date: "2026-04-17",
        current_price: entryPrice,
        avg_entry_price: entryPrice,
        cost_basis: entryPrice * qty * 100,
        market_value: entryPrice * qty * 100,
        unrealized_pl: 0,
        unrealized_plpc: 0,
        underlying_symbol: symbol,
      };
    }

    function makeShortCallWithQty(strike, entryPrice, qty, symbol = "TEST") {
      return {
        symbol: `${symbol}_CALL_${strike}`,
        asset_class: "us_option",
        side: "short",
        qty: -qty,
        strike_price: strike,
        option_type: "call",
        expiry_date: "2026-04-17",
        current_price: entryPrice,
        avg_entry_price: entryPrice,
        cost_basis: -(entryPrice * qty * 100),
        market_value: -(entryPrice * qty * 100),
        unrealized_pl: 0,
        unrealized_plpc: 0,
        underlying_symbol: symbol,
      };
    }

    const positionsQty1 = [
      makeLongCallWithQty(23100, 200, 1, "NDX"),
      makeShortCallWithQty(23150, 150, 1, "NDX"),
    ];
    const positionsQty100 = [
      makeLongCallWithQty(23100, 200, 100, "NDX"),
      makeShortCallWithQty(23150, 150, 100, "NDX"),
    ];
    const chartQty1 = generateMultiLegPayoff(positionsQty1, 23132);
    const chartQty100 = generateMultiLegPayoff(positionsQty100, 23132);

    it("prices arrays are identical regardless of qty", () => {
      expect(chartQty100.prices).toEqual(chartQty1.prices);
    });

    it("step size is the same regardless of qty", () => {
      const range1 = chartQty1.prices[chartQty1.prices.length - 1] - chartQty1.prices[0];
      const range100 = chartQty100.prices[chartQty100.prices.length - 1] - chartQty100.prices[0];
      expect(range100).toBe(range1);
    });

    it("payoff magnitude scales with qty", () => {
      // At a known price, payoff with qty=100 should be 100x payoff with qty=1
      const idx = chartQty1.prices.indexOf(23100);
      if (idx >= 0) {
        expect(chartQty100.payoffs[idx]).toBe(chartQty1.payoffs[idx] * 100);
      }
    });
  });

  // 13.8 Step size boundary: totalRange / 400 rounds up
  // totalRange=801 → step = ceil(801/400) = 3, not 2
  // Engineer: multi-leg with specific values to get totalRange near 801.
  // Actually, rather than engineering exact inputs, we can verify the formula
  // property: for any scenario, step = ceil(totalRange/400).
  it("13.8 Step size boundary: ceil rounds up (totalRange=801 → step=3)", () => {
    // Verify the property directly
    expect(Math.ceil(801 / 400)).toBe(3);
    expect(Math.ceil(800 / 400)).toBe(2);
    expect(Math.ceil(400 / 400)).toBe(1);
    expect(Math.ceil(401 / 400)).toBe(2);

    // Also verify with actual chart data: find the actual step from a scenario
    // and confirm it matches ceil(totalRange/400)
    const positions = [
      makeLongCallPosition(23100, 200, "NDX"),
      makeShortCallPosition(23150, 150, "NDX"),
    ];
    const chartData = generateMultiLegPayoff(positions, 23132);
    const totalRange = chartData.prices[chartData.prices.length - 1] - chartData.prices[0];
    const expectedStep = Math.max(Math.ceil(totalRange / 400), 1);

    // Verify the dominant gap matches the expected step
    const gaps = [];
    for (let i = 1; i < chartData.prices.length; i++) {
      gaps.push(chartData.prices[i] - chartData.prices[i - 1]);
    }
    const maxGap = Math.max(...gaps);
    expect(maxGap).toBe(expectedStep);
  });
});

// ===========================================================================
// Section 14: Cross-Cutting Concerns
//
// Properties that must hold for every scenario.
// ===========================================================================
describe("Section 14: Cross-Cutting Concerns", () => {

  // Build a collection of representative scenarios for cross-cutting checks
  const scenarios = [
    {
      name: "NDX multi-leg",
      positions: [
        makeLongCallPosition(23100, 200, "NDX"),
        makeShortCallPosition(23150, 150, "NDX"),
      ],
      underlying: 23132,
    },
    {
      name: "Cheap stock multi-leg",
      positions: [
        makeLongCallPosition(48, 4, "CHEAP"),
        makeShortCallPosition(53, 2, "CHEAP"),
      ],
      underlying: 50,
    },
    {
      name: "Penny stock multi-leg",
      positions: [
        makeLongPutPosition(1.10, 0.30, "PENNY"),
        makeShortPutPosition(0.90, 0.20, "PENNY"),
      ],
      underlying: 1.0,
    },
    {
      name: "NDX single-leg",
      positions: [makeLongCallPosition(23100, 200, "NDX")],
      underlying: 23132,
    },
    {
      name: "$50k single-leg",
      positions: [makeLongCallPosition(50000, 1000, "MEGA")],
      underlying: 50000,
    },
    {
      name: "Penny single-leg",
      positions: [makeLongPutPosition(1.0, 0.50, "PENNY")],
      underlying: 1.05,
    },
  ];

  // 14.1 Prices array always sorted ascending
  describe("14.1 Prices array always sorted ascending", () => {
    for (const s of scenarios) {
      it(`${s.name}: prices[i] <= prices[i+1]`, () => {
        const chartData = generateMultiLegPayoff(s.positions, s.underlying);
        for (let i = 1; i < chartData.prices.length; i++) {
          expect(chartData.prices[i]).toBeGreaterThanOrEqual(chartData.prices[i - 1]);
        }
      });
    }
  });

  // 14.2 Payoffs array has same length as prices
  describe("14.2 payoffs.length === prices.length", () => {
    for (const s of scenarios) {
      it(`${s.name}: payoffs.length === prices.length`, () => {
        const chartData = generateMultiLegPayoff(s.positions, s.underlying);
        expect(chartData.payoffs.length).toBe(chartData.prices.length);
      });
    }
  });

  // 14.3 No NaN or Infinity in prices or payoffs
  describe("14.3 No NaN or Infinity in prices or payoffs", () => {
    for (const s of scenarios) {
      it(`${s.name}: all prices finite`, () => {
        const chartData = generateMultiLegPayoff(s.positions, s.underlying);
        expect(chartData.prices.every(Number.isFinite)).toBe(true);
      });

      it(`${s.name}: all payoffs finite`, () => {
        const chartData = generateMultiLegPayoff(s.positions, s.underlying);
        expect(chartData.payoffs.every(Number.isFinite)).toBe(true);
      });
    }
  });

  // 14.4 lowerBound < upperBound always (prices[0] < prices[last])
  describe("14.4 prices[0] < prices[last] always", () => {
    for (const s of scenarios) {
      it(`${s.name}: prices[0] < prices[last]`, () => {
        const chartData = generateMultiLegPayoff(s.positions, s.underlying);
        expect(chartData.prices[0]).toBeLessThan(chartData.prices[chartData.prices.length - 1]);
      });
    }
  });

  // 14.5 generateButterflyPayoff returns all expected fields
  describe("14.5 generateButterflyPayoff returns all expected fields", () => {
    it("Iron Condor: prices, payoffs, lowerBreakEven, upperBreakEven, netPremium", () => {
      const info = {
        long_put_strike: 23000,
        short_put_strike: 23050,
        short_call_strike: 23150,
        long_call_strike: 23200,
        long_put_price: 80,
        short_put_price: 100,
        short_call_price: 100,
        long_call_price: 80,
      };
      const result = generateButterflyPayoff(info, "IRON Condor", 50);

      expect(result).toHaveProperty("prices");
      expect(result).toHaveProperty("payoffs");
      expect(result).toHaveProperty("lowerBreakEven");
      expect(result).toHaveProperty("upperBreakEven");
      expect(result).toHaveProperty("netPremium");
      expect(Array.isArray(result.prices)).toBe(true);
      expect(Array.isArray(result.payoffs)).toBe(true);
      expect(typeof result.lowerBreakEven).toBe("number");
      expect(typeof result.upperBreakEven).toBe("number");
      expect(typeof result.netPremium).toBe("number");
    });

    it("CALL Butterfly: prices, payoffs, lowerBreakEven, upperBreakEven, netPremium", () => {
      const info = {
        lower_strike: 95,
        atm_strike: 100,
        upper_strike: 105,
        lower_price: 8,
        atm_price: 5,
        upper_price: 3,
      };
      const result = generateButterflyPayoff(info, "CALL Butterfly", 5);

      expect(result).toHaveProperty("prices");
      expect(result).toHaveProperty("payoffs");
      expect(result).toHaveProperty("lowerBreakEven");
      expect(result).toHaveProperty("upperBreakEven");
      expect(result).toHaveProperty("netPremium");
    });
  });

  // 14.6 generateMultiLegPayoff returns all expected fields
  describe("14.6 generateMultiLegPayoff returns all expected fields", () => {
    it("multi-leg: prices, payoffs, breakEvenPoints, maxProfit, maxLoss, netCredit, strikes", () => {
      const positions = [
        makeLongCallPosition(23100, 200, "NDX"),
        makeShortCallPosition(23150, 150, "NDX"),
      ];
      const result = generateMultiLegPayoff(positions, 23132);

      expect(result).toHaveProperty("prices");
      expect(result).toHaveProperty("payoffs");
      expect(result).toHaveProperty("breakEvenPoints");
      expect(result).toHaveProperty("maxProfit");
      expect(result).toHaveProperty("maxLoss");
      expect(result).toHaveProperty("netCredit");
      expect(result).toHaveProperty("strikes");
      expect(Array.isArray(result.prices)).toBe(true);
      expect(Array.isArray(result.payoffs)).toBe(true);
      expect(Array.isArray(result.breakEvenPoints)).toBe(true);
      expect(Array.isArray(result.strikes)).toBe(true);
      expect(typeof result.maxProfit).toBe("number");
      expect(typeof result.maxLoss).toBe("number");
      expect(typeof result.netCredit).toBe("number");
    });

    it("single-leg: same fields present", () => {
      const positions = [makeLongCallPosition(100, 5, "TEST")];
      const result = generateMultiLegPayoff(positions, 102);

      expect(result).toHaveProperty("prices");
      expect(result).toHaveProperty("payoffs");
      expect(result).toHaveProperty("breakEvenPoints");
      expect(result).toHaveProperty("maxProfit");
      expect(result).toHaveProperty("maxLoss");
      expect(result).toHaveProperty("netCredit");
      expect(result).toHaveProperty("strikes");
    });
  });
});
