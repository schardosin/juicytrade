/**
 * QA tests for PayoffChart.vue dataset count guard in performMinorUpdate().
 *
 * Since PayoffChart.vue relies heavily on Chart.js internals and the guard
 * logic triggers a fallback to performChartUpdate(), we use two approaches:
 *
 * A. **Source code analysis** — Read the .vue file and verify the guard code
 *    is present with correct patterns (similar to the zero-dependency check).
 *
 * B. **Component mount tests** — Mount PayoffChart with the Chart.js mock from
 *    the dev tests to verify the component renders correctly with different
 *    dataset configurations (3 vs 4 datasets).
 *
 * Covers:
 *  1. Dataset count guard exists in source code
 *  2. Label-based matching uses exact string comparison (ds.label === newDs.label)
 *  3. performMinorUpdate has the guard checking dataset length mismatch
 *  4. customCrosshair plugin sync line exists in performMinorUpdate
 *  5. customCrosshair plugin sync line exists in minimalPriceUpdate
 *  6. customCrosshair plugin sync line exists in updateChartData (full update)
 *  7. Guard falls back to performChartUpdate on dataset count change
 *  8. Labels length guard also exists (separate from dataset count)
 */

import { describe, it, expect, vi } from "vitest";
import { readFileSync } from "fs";
import { resolve } from "path";
import { mount } from "@vue/test-utils";
import PayoffChart from "../src/components/PayoffChart.vue";
import { generateMultiLegPayoff, createMultiLegChartConfig } from "../src/utils/chartUtils.js";

// ---------------------------------------------------------------------------
// Mock Chart.js (same pattern as dev PayoffChart.test.js)
// ---------------------------------------------------------------------------
vi.mock("chart.js", () => {
  const Chart = class Chart {
    constructor(ctx, config) {
      this.ctx = ctx;
      this.config = config;
      this.data = config.data || { labels: [], datasets: [] };
      this.options = config.options || {};
      this.scales = { x: { min: 0, max: 100 } };
    }
    update() {}
    destroy() {}
    zoom() {}
    resetZoom() {}
    zoomScale() {}
  };

  Chart.register = vi.fn();
  Chart.getChart = vi.fn(() => null);

  return {
    Chart,
    registerables: [],
  };
});

vi.mock("chartjs-plugin-zoom", () => ({
  default: {},
}));

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const SOURCE_PATH = resolve(
  __dirname,
  "../src/components/PayoffChart.vue"
);

function readSource() {
  return readFileSync(SOURCE_PATH, "utf-8");
}

function createWrapper(chartData, props = {}) {
  return mount(PayoffChart, {
    props: {
      chartData,
      underlyingPrice: 500,
      title: "Test Payoff Chart",
      showInfo: false,
      ...props,
    },
    global: {
      stubs: {
        Card: {
          template:
            '<div class="p-card"><div class="p-card-title"><slot name="title"></slot></div><div class="p-card-content"><slot name="content"></slot></div></div>',
        },
      },
    },
  });
}

function makeIronCondorPositions() {
  return [
    { symbol: "SPY_PUT_480", asset_class: "us_option", side: "long",  qty:  1, strike_price: 480, option_type: "put",  expiry_date: "2026-06-19", current_price: 2, avg_entry_price: 2, cost_basis:  200, market_value:  200, unrealized_pl: 0, underlying_symbol: "SPY" },
    { symbol: "SPY_PUT_490", asset_class: "us_option", side: "short", qty: -1, strike_price: 490, option_type: "put",  expiry_date: "2026-06-19", current_price: 4, avg_entry_price: 4, cost_basis: -400, market_value: -400, unrealized_pl: 0, underlying_symbol: "SPY" },
    { symbol: "SPY_CALL_510", asset_class: "us_option", side: "short", qty: -1, strike_price: 510, option_type: "call", expiry_date: "2026-06-19", current_price: 4, avg_entry_price: 4, cost_basis: -400, market_value: -400, unrealized_pl: 0, underlying_symbol: "SPY" },
    { symbol: "SPY_CALL_520", asset_class: "us_option", side: "long",  qty:  1, strike_price: 520, option_type: "call", expiry_date: "2026-06-19", current_price: 2, avg_entry_price: 2, cost_basis:  200, market_value:  200, unrealized_pl: 0, underlying_symbol: "SPY" },
  ];
}

// ---------------------------------------------------------------------------
// A. Source code analysis tests
// ---------------------------------------------------------------------------

describe("QA: PayoffChart source — dataset count guard exists", () => {
  const source = readSource();

  it("performMinorUpdate checks config.data.datasets.length !== chart.value.data.datasets.length", () => {
    // The exact guard from PayoffChart.vue:522
    const pattern =
      /config\.data\.datasets\.length\s*!==\s*chart\.value\.data\.datasets\.length/;
    expect(pattern.test(source)).toBe(true);
  });

  it("guard logs and falls back to performChartUpdate on dataset count change", () => {
    // The code block around line 522-525:
    //   if (config.data.datasets.length !== chart.value.data.datasets.length) {
    //     console.log("Dataset count changed, falling back to full update");
    //     await performChartUpdate();
    //     return;
    //   }
    expect(source).toContain("Dataset count changed");
    expect(source).toContain("falling back to full update");

    // Verify there is a performChartUpdate() call after this guard
    const guardIdx = source.indexOf(
      "config.data.datasets.length !== chart.value.data.datasets.length"
    );
    expect(guardIdx).toBeGreaterThan(-1);

    // Within 200 chars after the guard, there should be performChartUpdate()
    const afterGuard = source.substring(guardIdx, guardIdx + 200);
    expect(afterGuard).toContain("performChartUpdate()");
  });
});

describe("QA: PayoffChart source — label-based matching", () => {
  const source = readSource();

  it("uses exact label comparison: ds.label === newDs.label", () => {
    // From PayoffChart.vue:531
    const pattern = /ds\.label\s*===\s*newDs\.label/;
    expect(pattern.test(source)).toBe(true);
  });

  it("does NOT use index-only matching (no ds[j] without label check in minor update)", () => {
    // The forEach at line 529 uses (newDs, j) and then checks ds.label === newDs.label.
    // Verify the label check is inside the forEach body, not a raw index-based copy.
    const forEachMatch = source.match(
      /config\.data\.datasets\.forEach\(\(newDs,\s*j\)\s*=>\s*\{([^}]+)\}/s
    );
    expect(forEachMatch).not.toBeNull();

    // The captured body should include the label comparison
    const body = forEachMatch[1];
    expect(body).toContain("ds.label === newDs.label");
  });
});

describe("QA: PayoffChart source — labels length guard", () => {
  const source = readSource();

  it("also checks labels length mismatch before dataset count guard", () => {
    // From PayoffChart.vue:515
    const pattern =
      /config\.data\.labels\.length\s*!==\s*chart\.value\.data\.labels\.length/;
    expect(pattern.test(source)).toBe(true);
  });

  it("labels guard appears before dataset count guard in performMinorUpdate", () => {
    const labelsGuardIdx = source.indexOf(
      "config.data.labels.length !== chart.value.data.labels.length"
    );
    const datasetsGuardIdx = source.indexOf(
      "config.data.datasets.length !== chart.value.data.datasets.length"
    );
    expect(labelsGuardIdx).toBeGreaterThan(-1);
    expect(datasetsGuardIdx).toBeGreaterThan(-1);
    expect(labelsGuardIdx).toBeLessThan(datasetsGuardIdx);
  });
});

describe("QA: PayoffChart source — customCrosshair plugin sync", () => {
  const source = readSource();

  it("syncs customCrosshair plugin options in performMinorUpdate", () => {
    // Find the performMinorUpdate function body
    const fnStart = source.indexOf("const performMinorUpdate");
    expect(fnStart).toBeGreaterThan(-1);

    // The next function definition after performMinorUpdate
    const fnEnd = source.indexOf("const createChart", fnStart);
    const fnBody = source.substring(fnStart, fnEnd);

    // Should contain the sync line
    expect(fnBody).toContain("chart.value.options.plugins.customCrosshair");
    expect(fnBody).toContain("config.options?.plugins?.customCrosshair");
  });

  it("syncs customCrosshair plugin options in minimalPriceUpdate", () => {
    const fnStart = source.indexOf("const minimalPriceUpdate");
    expect(fnStart).toBeGreaterThan(-1);

    const fnEnd = source.indexOf("const computeFromChart", fnStart);
    const fnBody = source.substring(fnStart, fnEnd);

    expect(fnBody).toContain("chart.value.options.plugins.customCrosshair");
  });

  it("syncs customCrosshair plugin options in updateChartData (full update)", () => {
    const fnStart = source.indexOf("const updateChartData");
    expect(fnStart).toBeGreaterThan(-1);

    // updateChartData ends before the watch() call
    const fnEnd = source.indexOf("// Separate watches", fnStart);
    const fnBody = source.substring(fnStart, fnEnd);

    expect(fnBody).toContain("chart.value.options.plugins.customCrosshair");
  });
});

// ---------------------------------------------------------------------------
// B. Integration: chart config dataset count changes
// ---------------------------------------------------------------------------

describe("QA: createMultiLegChartConfig produces correct dataset counts", () => {
  it("3 datasets when no t0Payoffs", () => {
    const positions = makeIronCondorPositions();
    const chartData = generateMultiLegPayoff(positions, 500);
    const config = createMultiLegChartConfig(chartData, 500);

    expect(config.data.datasets).toHaveLength(3);
  });

  it("4 datasets when t0Payoffs present", () => {
    const positions = makeIronCondorPositions();
    const chartData = generateMultiLegPayoff(positions, 500);
    chartData.t0Payoffs = chartData.prices.map((_, i) => chartData.payoffs[i] * 0.6);
    const config = createMultiLegChartConfig(chartData, 500);

    expect(config.data.datasets).toHaveLength(4);
  });

  it("dataset count changes from 3 to 4 when t0Payoffs is added", () => {
    const positions = makeIronCondorPositions();
    const chartData = generateMultiLegPayoff(positions, 500);

    const configWithout = createMultiLegChartConfig(chartData, 500);
    expect(configWithout.data.datasets).toHaveLength(3);

    chartData.t0Payoffs = chartData.prices.map((_, i) => chartData.payoffs[i] * 0.6);
    const configWith = createMultiLegChartConfig(chartData, 500);
    expect(configWith.data.datasets).toHaveLength(4);

    // This is the scenario that triggers the guard
    expect(configWithout.data.datasets.length).not.toBe(
      configWith.data.datasets.length
    );
  });

  it("dataset count changes from 4 to 3 when t0Payoffs is removed", () => {
    const positions = makeIronCondorPositions();
    const chartData = generateMultiLegPayoff(positions, 500);
    chartData.t0Payoffs = chartData.prices.map((_, i) => chartData.payoffs[i] * 0.6);

    const configWith = createMultiLegChartConfig(chartData, 500);
    expect(configWith.data.datasets).toHaveLength(4);

    delete chartData.t0Payoffs;
    const configWithout = createMultiLegChartConfig(chartData, 500);
    expect(configWithout.data.datasets).toHaveLength(3);
  });
});

describe("QA: PayoffChart component renders with both dataset configurations", () => {
  it("renders with 3-dataset config (no T+0)", async () => {
    const positions = makeIronCondorPositions();
    const chartData = generateMultiLegPayoff(positions, 500);

    const wrapper = createWrapper(chartData);
    await wrapper.vm.$nextTick();

    expect(wrapper.exists()).toBe(true);
    expect(wrapper.find(".chart-container").exists()).toBe(true);
    expect(wrapper.find("canvas").exists()).toBe(true);
  });

  it("renders with 4-dataset config (with T+0)", async () => {
    const positions = makeIronCondorPositions();
    const chartData = generateMultiLegPayoff(positions, 500);
    chartData.t0Payoffs = chartData.prices.map((_, i) => chartData.payoffs[i] * 0.6);

    const wrapper = createWrapper(chartData);
    await wrapper.vm.$nextTick();

    expect(wrapper.exists()).toBe(true);
    expect(wrapper.find(".chart-container").exists()).toBe(true);
    expect(wrapper.find("canvas").exists()).toBe(true);
  });
});
