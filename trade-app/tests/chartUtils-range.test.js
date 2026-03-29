/**
 * Range and step-size tests for chartUtils.js
 *
 * Verifies that generateMultiLegPayoff, createMultiLegChartConfig, and
 * generateButterflyPayoff produce correct data ranges and step sizes
 * for both high-priced (NDX-level) and low-priced symbols.
 *
 * Covers Steps 8-13 of the implementation plan for issue #42.
 */

import { describe, it, expect } from "vitest";
import {
  generateMultiLegPayoff,
  createMultiLegChartConfig,
  generateButterflyPayoff,
} from "../src/utils/chartUtils.js";

// ---------------------------------------------------------------------------
// Helpers — build option position objects
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

// ---------------------------------------------------------------------------
// Step 8 — NDX-level multi-leg ($50-wide spread, underlyingPrice ~$23,132)
// ---------------------------------------------------------------------------
describe("NDX-level multi-leg ($50-wide spread, underlyingPrice ~$23,132)", () => {
  const positions = [
    makeLongCallPosition(23100, 200, "NDX"),
    makeShortCallPosition(23150, 150, "NDX"),
  ];
  const underlyingPrice = 23132;
  const chartData = generateMultiLegPayoff(positions, underlyingPrice);

  it("data range extends at least ±$2,000 from the strikes (AC-1)", () => {
    const firstPrice = chartData.prices[0];
    const lastPrice = chartData.prices[chartData.prices.length - 1];

    expect(firstPrice).toBeLessThanOrEqual(23100 - 2000);
    expect(lastPrice).toBeGreaterThanOrEqual(23150 + 2000);
  });

  it("generates ≤600 data points (AC-4)", () => {
    expect(chartData.prices.length).toBeLessThanOrEqual(600);
  });

  it("all strike prices are included in the prices array (AC-6)", () => {
    expect(chartData.prices).toContain(23100);
    expect(chartData.prices).toContain(23150);
  });
});

// ---------------------------------------------------------------------------
// Step 9 — Cheap stock multi-leg ($5-wide spread on $50 stock)
// ---------------------------------------------------------------------------
describe("Cheap stock multi-leg ($5-wide spread on $50 stock)", () => {
  const positions = [
    makeLongCallPosition(48, 4, "CHEAP"),
    makeShortCallPosition(53, 2, "CHEAP"),
  ];
  const underlyingPrice = 50;
  const chartData = generateMultiLegPayoff(positions, underlyingPrice);

  it("data range extends at least ±$50 from the strikes (AC-2)", () => {
    const firstPrice = chartData.prices[0];
    const lastPrice = chartData.prices[chartData.prices.length - 1];

    expect(firstPrice).toBeLessThanOrEqual(48 - 50);
    expect(lastPrice).toBeGreaterThanOrEqual(53 + 50);
  });

  it("generates a reasonable number of data points", () => {
    expect(chartData.prices.length).toBeGreaterThan(10);
    expect(chartData.prices.length).toBeLessThanOrEqual(600);
  });

  it("range is not excessively wide (≤ ±$500 from strikes for a $50 stock)", () => {
    const firstPrice = chartData.prices[0];
    const lastPrice = chartData.prices[chartData.prices.length - 1];

    expect(firstPrice).toBeGreaterThanOrEqual(48 - 500);
    expect(lastPrice).toBeLessThanOrEqual(53 + 500);
  });
});

// ---------------------------------------------------------------------------
// Step 10 — NDX single-leg (single call at 23,100)
// ---------------------------------------------------------------------------
describe("NDX single-leg (single call at 23,100, underlyingPrice ~$23,132)", () => {
  const positions = [makeLongCallPosition(23100, 200, "NDX")];
  const underlyingPrice = 23132;
  const chartData = generateMultiLegPayoff(positions, underlyingPrice);

  it("data range extends at least ±$2,000 from the strike", () => {
    const firstPrice = chartData.prices[0];
    const lastPrice = chartData.prices[chartData.prices.length - 1];

    expect(firstPrice).toBeLessThanOrEqual(23100 - 2000);
    expect(lastPrice).toBeGreaterThanOrEqual(23100 + 2000);
  });

  it("generates ≤600 data points", () => {
    expect(chartData.prices.length).toBeLessThanOrEqual(600);
  });

  it("strike price is included in the prices array", () => {
    expect(chartData.prices).toContain(23100);
  });
});

// ---------------------------------------------------------------------------
// Step 11 — Strike inclusion verification
// ---------------------------------------------------------------------------
describe("Strike inclusion in generated data", () => {
  it("for a multi-leg spread, every strike price appears in the prices array", () => {
    const positions = [
      makeLongCallPosition(23100, 200, "NDX"),
      makeShortCallPosition(23150, 150, "NDX"),
    ];
    const chartData = generateMultiLegPayoff(positions, 23132);

    expect(chartData.prices).toContain(23100);
    expect(chartData.prices).toContain(23150);
  });

  it("for a single-leg option, the strike price appears in the prices array", () => {
    const positions = [makeLongCallPosition(23100, 200, "NDX")];
    const chartData = generateMultiLegPayoff(positions, 23132);

    expect(chartData.prices).toContain(23100);
  });

  it("strike prices are present even when step size > 1 (high-priced symbol)", () => {
    // With NDX-level prices, the step size will be > 1 due to large range
    const positions = [
      makeLongCallPosition(23100, 200, "NDX"),
      makeShortCallPosition(23150, 150, "NDX"),
    ];
    const chartData = generateMultiLegPayoff(positions, 23132);

    // Confirm step > 1 by checking price gaps (excluding strikes which are injected)
    const pricesWithoutStrikes = chartData.prices.filter(
      (p) => p !== 23100 && p !== 23150
    );
    if (pricesWithoutStrikes.length >= 2) {
      const gap = pricesWithoutStrikes[1] - pricesWithoutStrikes[0];
      expect(gap).toBeGreaterThan(1);
    }

    // Strikes must still be present
    expect(chartData.prices).toContain(23100);
    expect(chartData.prices).toContain(23150);
  });
});

// ---------------------------------------------------------------------------
// Step 12 — Initial view window (suggestedMin / suggestedMax)
// ---------------------------------------------------------------------------
describe("Initial view window (suggestedMin / suggestedMax)", () => {
  it("for NDX ($50 spread), suggestedMin/suggestedMax span at least $2,000 (AC-3)", () => {
    const positions = [
      makeLongCallPosition(23100, 200, "NDX"),
      makeShortCallPosition(23150, 150, "NDX"),
    ];
    const underlyingPrice = 23132;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);
    const config = createMultiLegChartConfig(chartData, underlyingPrice);

    const sugMin = config.options.scales.x.suggestedMin;
    const sugMax = config.options.scales.x.suggestedMax;

    expect(sugMax - sugMin).toBeGreaterThanOrEqual(2000);
  });

  it("for NDX, suggestedMin < minStrike and suggestedMax > maxStrike", () => {
    const positions = [
      makeLongCallPosition(23100, 200, "NDX"),
      makeShortCallPosition(23150, 150, "NDX"),
    ];
    const underlyingPrice = 23132;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);
    const config = createMultiLegChartConfig(chartData, underlyingPrice);

    const sugMin = config.options.scales.x.suggestedMin;
    const sugMax = config.options.scales.x.suggestedMax;

    expect(sugMin).toBeLessThan(23100);
    expect(sugMax).toBeGreaterThan(23150);
  });

  it("for a $50 stock ($5 spread), the initial view window is reasonable (≥ $25 on each side)", () => {
    const positions = [
      makeLongCallPosition(48, 4, "CHEAP"),
      makeShortCallPosition(53, 2, "CHEAP"),
    ];
    const underlyingPrice = 50;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);
    const config = createMultiLegChartConfig(chartData, underlyingPrice);

    const sugMin = config.options.scales.x.suggestedMin;
    const sugMax = config.options.scales.x.suggestedMax;

    expect(sugMin).toBeLessThanOrEqual(48 - 25);
    expect(sugMax).toBeGreaterThanOrEqual(53 + 25);
  });

  it("pan/zoom limits allow the full generated data range", () => {
    const positions = [
      makeLongCallPosition(23100, 200, "NDX"),
      makeShortCallPosition(23150, 150, "NDX"),
    ];
    const underlyingPrice = 23132;
    const chartData = generateMultiLegPayoff(positions, underlyingPrice);
    const config = createMultiLegChartConfig(chartData, underlyingPrice);

    const limits = config.options.plugins.zoom.limits.x;
    const prices = chartData.prices;

    expect(limits.min).toBe(Math.min(...prices));
    expect(limits.max).toBe(Math.max(...prices));
  });
});

// ---------------------------------------------------------------------------
// Step 13 — generateButterflyPayoff range and step consistency
// ---------------------------------------------------------------------------
describe("generateButterflyPayoff range and step consistency", () => {
  it("Iron Condor with high-value strikes: generates ≤600 data points", () => {
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

  it("Iron Condor with high-value strikes: range covers at least ±$2,000", () => {
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

    const firstPrice = result.prices[0];
    const lastPrice = result.prices[result.prices.length - 1];

    expect(firstPrice).toBeLessThanOrEqual(23000 - 2000);
    expect(lastPrice).toBeGreaterThanOrEqual(23200 + 2000);
  });

  it("Standard butterfly with high-value strikes: generates ≤600 data points", () => {
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
