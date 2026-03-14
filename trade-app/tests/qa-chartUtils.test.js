/**
 * QA-specific integration tests for chartUtils.js
 *
 * Verifies T+0 dataset configuration, crosshair plugin, Y-axis range,
 * backward compatibility, and _creditAdjustment exposure.
 *
 * Covers:
 *  1.  _creditAdjustment exposed by generateMultiLegPayoff()
 *  2.  T+0 dataset present when t0Payoffs provided
 *  3.  T+0 dataset absent when t0Payoffs is null
 *  4.  T+0 dataset absent when t0Payoffs is undefined
 *  5.  T+0 dataset visual properties (exact values)
 *  6.  T+0 label is exactly "Today (T+0)"
 *  7.  Expiration dataset unchanged when T+0 is present
 *  8.  Plugin options include t0Payoffs when provided
 *  9.  Plugin options t0Payoffs is null when not provided
 *  10. T+0 dataset length matches prices length
 *  11. Backward compatibility: 3 datasets when no t0Payoffs
 *  12. With t0Payoffs: exactly 4 datasets
 */

import { describe, it, expect } from "vitest";
import {
  generateMultiLegPayoff,
  createMultiLegChartConfig,
} from "../src/utils/chartUtils.js";

// ---------------------------------------------------------------------------
// Helpers — build minimal position data for generateMultiLegPayoff
// ---------------------------------------------------------------------------

/**
 * Build a simple long call position suitable for generateMultiLegPayoff.
 */
function makeLongCallPosition(strike, entryPrice, currentPrice) {
  return {
    symbol: `TEST_CALL_${strike}`,
    asset_class: "us_option",
    side: "long",
    qty: 1,
    strike_price: strike,
    option_type: "call",
    expiry_date: "2026-04-17",
    current_price: currentPrice ?? entryPrice,
    avg_entry_price: entryPrice,
    cost_basis: entryPrice * 100,
    market_value: (currentPrice ?? entryPrice) * 100,
    unrealized_pl: 0,
    unrealized_plpc: 0,
    underlying_symbol: "TEST",
  };
}

function makeShortCallPosition(strike, entryPrice, currentPrice) {
  return {
    symbol: `TEST_CALL_${strike}`,
    asset_class: "us_option",
    side: "short",
    qty: -1,
    strike_price: strike,
    option_type: "call",
    expiry_date: "2026-04-17",
    current_price: currentPrice ?? entryPrice,
    avg_entry_price: entryPrice,
    cost_basis: -(entryPrice * 100),
    market_value: -(currentPrice ?? entryPrice) * 100,
    unrealized_pl: 0,
    unrealized_plpc: 0,
    underlying_symbol: "TEST",
  };
}

/**
 * Generate chartData from a bull call spread (long 100 call + short 110 call)
 * and return both the raw chartData and the underlying price.
 */
function makeBaseChartData() {
  const positions = [
    makeLongCallPosition(100, 8),
    makeShortCallPosition(110, 3),
  ];
  const underlyingPrice = 105;
  const chartData = generateMultiLegPayoff(positions, underlyingPrice);
  return { chartData, underlyingPrice };
}

/**
 * Attach synthetic t0Payoffs of the correct length to chartData.
 */
function withT0Payoffs(chartData) {
  // Create a simple synthetic T+0 payoff array (same length as prices)
  const t0Payoffs = chartData.prices.map((p, i) => chartData.payoffs[i] * 0.6);
  return { ...chartData, t0Payoffs };
}

// ---------------------------------------------------------------------------
// 1. _creditAdjustment exposed
// ---------------------------------------------------------------------------
describe("QA: _creditAdjustment exposed by generateMultiLegPayoff", () => {
  it("return object contains _creditAdjustment field", () => {
    const { chartData } = makeBaseChartData();
    expect(chartData).toHaveProperty("_creditAdjustment");
  });

  it("_creditAdjustment is a number", () => {
    const { chartData } = makeBaseChartData();
    expect(typeof chartData._creditAdjustment).toBe("number");
  });

  it("_creditAdjustment is 0 when adjustedNetCredit is null (default)", () => {
    const { chartData } = makeBaseChartData();
    expect(chartData._creditAdjustment).toBe(0);
  });

  it("_creditAdjustment is non-zero when adjustedNetCredit is provided", () => {
    const positions = [
      makeLongCallPosition(100, 8),
      makeShortCallPosition(110, 3),
    ];
    const chartData = generateMultiLegPayoff(positions, 105, 10);
    // adjustedNetCredit=10, natural netCredit = 3 - 8 = -5,
    // totalAdjustment = 10 - (-5) = 15, creditAdjustment = 15 * 100 = 1500
    expect(chartData._creditAdjustment).not.toBe(0);
    expect(Number.isFinite(chartData._creditAdjustment)).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// 2. T+0 dataset present when t0Payoffs provided
// ---------------------------------------------------------------------------
describe("QA: T+0 dataset present when t0Payoffs provided", () => {
  it('a dataset with label "Today (T+0)" exists in datasets', () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const chartDataWithT0 = withT0Payoffs(chartData);
    const config = createMultiLegChartConfig(chartDataWithT0, underlyingPrice);

    const t0Dataset = config.data.datasets.find(
      (ds) => ds.label === "Today (T+0)"
    );
    expect(t0Dataset).toBeDefined();
  });
});

// ---------------------------------------------------------------------------
// 3. T+0 dataset absent when t0Payoffs is null
// ---------------------------------------------------------------------------
describe("QA: T+0 dataset absent when t0Payoffs is null", () => {
  it('no dataset with label "Today (T+0)" when t0Payoffs is null', () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const chartDataNull = { ...chartData, t0Payoffs: null };
    const config = createMultiLegChartConfig(chartDataNull, underlyingPrice);

    const t0Dataset = config.data.datasets.find(
      (ds) => ds.label === "Today (T+0)"
    );
    expect(t0Dataset).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// 4. T+0 dataset absent when t0Payoffs is undefined
// ---------------------------------------------------------------------------
describe("QA: T+0 dataset absent when t0Payoffs is undefined", () => {
  it('no dataset with label "Today (T+0)" when t0Payoffs is not set', () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    // chartData from generateMultiLegPayoff does not have t0Payoffs by default
    expect(chartData.t0Payoffs).toBeUndefined();

    const config = createMultiLegChartConfig(chartData, underlyingPrice);
    const t0Dataset = config.data.datasets.find(
      (ds) => ds.label === "Today (T+0)"
    );
    expect(t0Dataset).toBeUndefined();
  });
});

// ---------------------------------------------------------------------------
// 5. T+0 dataset visual properties
// ---------------------------------------------------------------------------
describe("QA: T+0 dataset visual properties", () => {
  let t0Dataset;

  // Set up once for all property checks
  const { chartData, underlyingPrice } = makeBaseChartData();
  const chartDataWithT0 = withT0Payoffs(chartData);
  const config = createMultiLegChartConfig(chartDataWithT0, underlyingPrice);
  t0Dataset = config.data.datasets.find((ds) => ds.label === "Today (T+0)");

  it('borderColor is exactly "rgb(0, 150, 255)"', () => {
    expect(t0Dataset.borderColor).toBe("rgb(0, 150, 255)");
  });

  it("borderDash is [6, 3]", () => {
    expect(t0Dataset.borderDash).toEqual([6, 3]);
  });

  it("fill is false", () => {
    expect(t0Dataset.fill).toBe(false);
  });

  it("order is 0 (renders above expiration line)", () => {
    expect(t0Dataset.order).toBe(0);
  });

  it("pointRadius is 0", () => {
    expect(t0Dataset.pointRadius).toBe(0);
  });

  it("borderWidth is 2", () => {
    expect(t0Dataset.borderWidth).toBe(2);
  });
});

// ---------------------------------------------------------------------------
// 6. T+0 label is exactly "Today (T+0)"
// ---------------------------------------------------------------------------
describe('QA: T+0 label is exactly "Today (T+0)"', () => {
  it('label is not "T+0", "Today", or any other variant', () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const chartDataWithT0 = withT0Payoffs(chartData);
    const config = createMultiLegChartConfig(chartDataWithT0, underlyingPrice);

    const t0Dataset = config.data.datasets.find(
      (ds) => ds.label === "Today (T+0)"
    );
    expect(t0Dataset).toBeDefined();
    expect(t0Dataset.label).toBe("Today (T+0)");

    // Ensure no other T+0-like labels exist
    const otherT0 = config.data.datasets.filter(
      (ds) =>
        ds.label !== "Today (T+0)" &&
        (ds.label === "T+0" ||
          ds.label === "Today" ||
          ds.label === "T0" ||
          ds.label === "Today T+0")
    );
    expect(otherT0).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// 7. Expiration dataset unchanged when T+0 is present
// ---------------------------------------------------------------------------
describe("QA: Expiration dataset unchanged when T+0 is present", () => {
  it('expiration dataset label still starts with "Position Payoff"', () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const chartDataWithT0 = withT0Payoffs(chartData);
    const config = createMultiLegChartConfig(chartDataWithT0, underlyingPrice);

    const expiryDataset = config.data.datasets.find((ds) =>
      ds.label.startsWith("Position Payoff")
    );
    expect(expiryDataset).toBeDefined();
    expect(expiryDataset.label).toMatch(/^Position Payoff/);
  });

  it("expiration dataset is the first dataset (index 0)", () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const chartDataWithT0 = withT0Payoffs(chartData);
    const config = createMultiLegChartConfig(chartDataWithT0, underlyingPrice);

    expect(config.data.datasets[0].label).toMatch(/^Position Payoff/);
  });

  it("expiration dataset has order=1 (below T+0 which has order=0)", () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const chartDataWithT0 = withT0Payoffs(chartData);
    const config = createMultiLegChartConfig(chartDataWithT0, underlyingPrice);

    const expiryDataset = config.data.datasets.find((ds) =>
      ds.label.startsWith("Position Payoff")
    );
    expect(expiryDataset.order).toBe(1);
  });
});

// ---------------------------------------------------------------------------
// 8. Plugin options include t0Payoffs when provided
// ---------------------------------------------------------------------------
describe("QA: plugin options include t0Payoffs when provided", () => {
  it("config.options.plugins.customCrosshair.t0Payoffs is set", () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const chartDataWithT0 = withT0Payoffs(chartData);
    const config = createMultiLegChartConfig(chartDataWithT0, underlyingPrice);

    const crosshairOpts = config.options.plugins.customCrosshair;
    expect(crosshairOpts).toBeDefined();
    expect(crosshairOpts.t0Payoffs).toBeDefined();
    expect(crosshairOpts.t0Payoffs).not.toBeNull();
  });

  it("plugin t0Payoffs matches the input t0Payoffs array", () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const chartDataWithT0 = withT0Payoffs(chartData);
    const config = createMultiLegChartConfig(chartDataWithT0, underlyingPrice);

    expect(config.options.plugins.customCrosshair.t0Payoffs).toEqual(
      chartDataWithT0.t0Payoffs
    );
  });

  it("plugin options also include prices and payoffs arrays", () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const chartDataWithT0 = withT0Payoffs(chartData);
    const config = createMultiLegChartConfig(chartDataWithT0, underlyingPrice);

    const crosshairOpts = config.options.plugins.customCrosshair;
    expect(crosshairOpts.prices).toEqual(chartDataWithT0.prices);
    expect(crosshairOpts.payoffs).toEqual(chartDataWithT0.payoffs);
  });
});

// ---------------------------------------------------------------------------
// 9. Plugin options t0Payoffs is null when not provided
// ---------------------------------------------------------------------------
describe("QA: plugin options t0Payoffs is null when not provided", () => {
  it("t0Payoffs is null when chartData has no t0Payoffs", () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const config = createMultiLegChartConfig(chartData, underlyingPrice);

    expect(config.options.plugins.customCrosshair.t0Payoffs).toBeNull();
  });

  it("t0Payoffs is null when chartData.t0Payoffs is explicitly null", () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const chartDataNull = { ...chartData, t0Payoffs: null };
    const config = createMultiLegChartConfig(chartDataNull, underlyingPrice);

    expect(config.options.plugins.customCrosshair.t0Payoffs).toBeNull();
  });
});

// ---------------------------------------------------------------------------
// 10. T+0 dataset length matches prices length
// ---------------------------------------------------------------------------
describe("QA: T+0 dataset length matches prices length", () => {
  it("T+0 data array has same length as prices array", () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const chartDataWithT0 = withT0Payoffs(chartData);
    const config = createMultiLegChartConfig(chartDataWithT0, underlyingPrice);

    const t0Dataset = config.data.datasets.find(
      (ds) => ds.label === "Today (T+0)"
    );
    expect(t0Dataset.data).toHaveLength(chartData.prices.length);
  });

  it("each T+0 data point has x matching prices and y matching t0Payoffs", () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const chartDataWithT0 = withT0Payoffs(chartData);
    const config = createMultiLegChartConfig(chartDataWithT0, underlyingPrice);

    const t0Dataset = config.data.datasets.find(
      (ds) => ds.label === "Today (T+0)"
    );

    for (let i = 0; i < chartData.prices.length; i++) {
      expect(t0Dataset.data[i].x).toBe(chartData.prices[i]);
      expect(t0Dataset.data[i].y).toBe(chartDataWithT0.t0Payoffs[i]);
    }
  });
});

// ---------------------------------------------------------------------------
// 11. Backward compatibility: 3 datasets when no t0Payoffs
// ---------------------------------------------------------------------------
describe("QA: backward compatibility — 3 datasets when no t0Payoffs", () => {
  it("chart config has exactly 3 datasets", () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const config = createMultiLegChartConfig(chartData, underlyingPrice);

    expect(config.data.datasets).toHaveLength(3);
  });

  it('datasets are: "Position Payoff (...)", "Zero Line", "Current Price"', () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const config = createMultiLegChartConfig(chartData, underlyingPrice);

    const labels = config.data.datasets.map((ds) => ds.label);
    expect(labels[0]).toMatch(/^Position Payoff/);
    expect(labels[1]).toBe("Zero Line");
    expect(labels[2]).toBe("Current Price");
  });
});

// ---------------------------------------------------------------------------
// 12. With t0Payoffs: exactly 4 datasets
// ---------------------------------------------------------------------------
describe("QA: 4 datasets when t0Payoffs provided", () => {
  it("chart config has exactly 4 datasets", () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const chartDataWithT0 = withT0Payoffs(chartData);
    const config = createMultiLegChartConfig(chartDataWithT0, underlyingPrice);

    expect(config.data.datasets).toHaveLength(4);
  });

  it('datasets are: "Position Payoff (...)", "Today (T+0)", "Zero Line", "Current Price"', () => {
    const { chartData, underlyingPrice } = makeBaseChartData();
    const chartDataWithT0 = withT0Payoffs(chartData);
    const config = createMultiLegChartConfig(chartDataWithT0, underlyingPrice);

    const labels = config.data.datasets.map((ds) => ds.label);
    expect(labels[0]).toMatch(/^Position Payoff/);
    expect(labels[1]).toBe("Today (T+0)");
    expect(labels[2]).toBe("Zero Line");
    expect(labels[3]).toBe("Current Price");
  });
});
