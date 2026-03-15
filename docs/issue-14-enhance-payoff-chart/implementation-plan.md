# Implementation Plan: Enhance Payoff Chart with T+0 (Today) Line

**Issue:** [#14 — Enhance Payoff Chart](https://github.com/schardosin/juicytrade/issues/14)
**Requirements:** [requirements.md](./requirements.md)
**Architecture:** [architecture.md](./architecture.md)
**Date:** 2025-01-28

---

## Pre-Implementation State

**Existing test suite:** 20 test files in `trade-app/tests/` using Vitest + happy-dom + @vue/test-utils.
Key related tests:
- `PayoffChart.test.js` — 819 lines, tests payoff math validation, chart controls, quantity scaling, edge cases
- `ChartDataFlowIntegration.test.js` — 489 lines, tests position ID uniqueness, chart data source priority, payoff calculations
- `optionsCalculator.test.js` — options P&L calculation tests

**Test command:** `npm test` (runs `vitest`) from `trade-app/`

**Files to create:** 1 new source file + 1 new test file
**Files to modify:** 3 source files

---

## Step 1: Create `blackScholes.js` with CND, Black-Scholes pricing, and `calculateT0Payoffs`

### Goal
Create a pure utility module implementing the Black-Scholes option pricing model. This module has zero Vue/Chart.js dependencies, making it fully unit-testable in isolation.

### Files to Create

**`trade-app/src/utils/blackScholes.js`**

Implement three exported functions:

1. **`cumulativeNormalDistribution(x)`**
   - Abramowitz & Stegun approximation (formula 26.2.17)
   - Constants: `a1 = 0.319381530`, `a2 = -0.356563782`, `a3 = 1.781477937`, `a4 = -1.821255978`, `a5 = 1.330274429`, `gamma = 0.2316419`
   - For `x >= 0`: `N(x) = 1 - n(x) * (a1*k + a2*k² + a3*k³ + a4*k⁴ + a5*k⁵)` where `k = 1/(1 + gamma*x)` and `n(x) = (1/√(2π)) * e^(-x²/2)`
   - For `x < 0`: `N(x) = 1 - N(-x)`

2. **`blackScholesPrice({ optionType, S, K, T, r, sigma })`**
   - Standard Black-Scholes formula for European options
   - Edge cases: `T <= 0` → return intrinsic value; `sigma <= 0` → return intrinsic value; `S <= 0` → return 0 for calls, `K * e^(-rT)` for puts
   - Pre-compute `sqrtT`, `sigmaSqrtT`, `d1Constant`, `discountFactor` for performance

3. **`calculateT0Payoffs({ prices, legs, riskFreeRate = 0.05, creditAdjustment = 0 })`**
   - For each price point in `prices[]`, for each leg, compute BS theoretical price, then P&L: `(theoreticalPrice - entryPrice) * qty * 100` (long) or `(entryPrice - theoreticalPrice) * |qty| * 100` (short)
   - Sum across all legs, add `creditAdjustment`
   - Return `number[]` of T+0 P&L values (same length as `prices[]`)
   - Performance: pre-compute per-leg constants (`sqrtT`, `sigmaSqrtT`, `d1Constant`, `discountFactor`) outside the price loop

**`trade-app/tests/blackScholes.test.js`**

Unit tests covering:

1. **`cumulativeNormalDistribution` tests:**
   - `CND(0) === 0.5`
   - `CND(large positive) ≈ 1.0` (e.g., `CND(10) ≈ 1.0`)
   - `CND(large negative) ≈ 0.0` (e.g., `CND(-10) ≈ 0.0`)
   - Symmetry: `CND(x) + CND(-x) ≈ 1.0` for several values
   - Known values: `CND(1.0) ≈ 0.8413`, `CND(-1.0) ≈ 0.1587`, `CND(1.96) ≈ 0.975`

2. **`blackScholesPrice` tests:**
   - ATM call with known parameters — verify against textbook Black-Scholes value
   - ATM put with same parameters — verify put-call parity: `C - P = S - K*e^(-rT)`
   - Deep ITM call → price ≈ `S - K*e^(-rT)` (approaches intrinsic + discount)
   - Deep OTM call → price ≈ 0
   - Edge case: `T = 0` → returns intrinsic value exactly
   - Edge case: `sigma = 0` → returns intrinsic value exactly
   - Edge case: `S = 0` → call returns 0, put returns `K*e^(-rT)`
   - Edge case: `S < 0` (from wide price ranges) → call returns 0

3. **`calculateT0Payoffs` tests:**
   - Single long call leg → verify T+0 P&L at several price points matches manual Black-Scholes calculation
   - Iron condor (4 legs) → verify T+0 curve has expected shape (curved between strikes, converging to intrinsic at extremes)
   - 0 DTE (`T = 0`) → T+0 payoffs match expiration payoffs exactly
   - Mixed expirations → each leg uses its own `T` value
   - With `creditAdjustment > 0` → all payoffs shifted by that amount
   - Empty legs array → returns array of zeros
   - Verify output array length matches input `prices` array length

### Verification
```bash
cd trade-app && npx vitest run tests/blackScholes.test.js
```

### Commit Message
```
feat: add Black-Scholes pricing utility for T+0 payoff calculation

Implement cumulativeNormalDistribution, blackScholesPrice, and
calculateT0Payoffs as pure functions with zero external dependencies.
Includes comprehensive unit tests covering edge cases (0 DTE, S<=0,
sigma=0) and numerical accuracy validation.
```

---

## Step 2: Modify `chartUtils.js` — expose `_creditAdjustment`, add T+0 dataset, update Y-axis, update crosshair plugin

### Goal
Extend the chart configuration to conditionally render a T+0 line when `chartData.t0Payoffs` is present. Update the crosshair plugin to show dual values. Expose `_creditAdjustment` from the payoff generator so the T+0 calculation can use it.

### File to Modify

**`trade-app/src/utils/chartUtils.js`**

#### Change 1: Expose `_creditAdjustment` from `generateMultiLegPayoff()`

In the `result` object at the end of `generateMultiLegPayoff()` (around line 608), add `_creditAdjustment: creditAdjustment` to the return object. The `creditAdjustment` variable is already computed at line 496. The underscore prefix signals it is an internal implementation detail.

**Exact location:** Line 608-618 — add `_creditAdjustment: creditAdjustment,` to the existing `result` object.

#### Change 2: Conditionally add T+0 dataset in `createMultiLegChartConfig()`

After building the expiration dataset (line 797, the `datasets` array declaration ending around line 854), and **before** pushing the Zero Line dataset (line 835), conditionally push the T+0 dataset:

```javascript
if (chartData.t0Payoffs && chartData.t0Payoffs.length === prices.length) {
  datasets.push({
    label: "Today (T+0)",
    data: chartData.t0Payoffs,
    borderColor: "rgb(0, 150, 255)",
    borderWidth: 2,
    borderDash: [6, 3],
    pointRadius: 0,
    fill: false,
    tension: 0,
    order: 0,  // Render above the expiration line
  });
}
```

**Key detail:** The T+0 dataset uses raw numeric `data` (not `{x,y}` objects) because `createMultiLegChartConfig` uses labels-based indexing (line 952: `data: { labels: prices, datasets }`). The expiration payoff dataset at index 0 already stores `{x,y}` objects as `chartPoints` — but since `labels` is set to `prices`, both formats work. For consistency and simplicity, use the raw numeric array (`chartData.t0Payoffs`) directly since it corresponds 1:1 with `labels`.

**Wait — correction.** Looking at the existing code more carefully: dataset 0 (`chartPoints`) uses `{x,y}` objects (line 788-791). The zero line also uses `{x,y}` (line 792). The current price line uses `{x,y}` (line 793-796). So we should match the format:

```javascript
const t0ChartPoints = prices.map((price, index) => ({
  x: price,
  y: chartData.t0Payoffs[index],
}));
```

Use `t0ChartPoints` as the `data` value in the T+0 dataset.

#### Change 3: Update Y-axis dynamic `min`/`max` functions

The Y-axis `min` function (lines 1063-1112) and `max` function (lines 1114-1164) iterate over `payoffs[]` to find visible values. After the existing loop that collects `visiblePayoffs` from expiration payoffs, add:

```javascript
// Also consider T+0 payoffs for Y-axis range (if present)
if (chartData.t0Payoffs) {
  for (let i = 0; i < prices.length; i++) {
    if (prices[i] >= visibleMin && prices[i] <= visibleMax) {
      visiblePayoffs.push(chartData.t0Payoffs[i]);
    }
  }
}
```

Add this in **both** the `min` and `max` functions, right after the existing `for` loop that populates `visiblePayoffs`.

#### Change 4: Pass T+0 data to crosshair plugin options

In the plugin options block (around line 964-967), extend `customCrosshair` to include `t0Payoffs`:

```javascript
customCrosshair: {
  prices: prices,
  payoffs: payoffs,
  t0Payoffs: chartData.t0Payoffs || null,  // NEW
},
```

#### Change 5: Enhance `customCrosshairPlugin.afterDraw`

In the `afterDraw` hook (lines 871-948):

**5a. Add T+0 interpolation** (after existing interpolation at line 903):
```javascript
let t0YVal = null;
let t0YPixel = null;
if (options.t0Payoffs && options.t0Payoffs.length > 0) {
  const t0Y1 = options.t0Payoffs[i - 1];
  const t0Y2 = options.t0Payoffs[i];
  t0YVal = t0Y1 + t * (t0Y2 - t0Y1);
  t0YPixel = scales.y.getPixelForValue(t0YVal);
}
```

**5b. Draw T+0 intersection circle** (after existing circle at lines 906-910):
```javascript
if (t0YPixel !== null && t0YPixel >= top && t0YPixel <= bottom) {
  ctx.beginPath();
  ctx.fillStyle = "rgb(0, 150, 255)";
  ctx.arc(crosshair.x, t0YPixel, 5, 0, 2 * Math.PI);
  ctx.fill();
}
```

**5c. Enhance tooltip box** (lines 914-947):
- Change label from `P&L:` to `Expiry P&L:` when T+0 is present
- Add third line `Today P&L: $XXX` in blue color when T+0 is present
- Increase `boxHeight` from 40 to 56 when T+0 is present
- Measure all three text widths for `boxWidth`

### Tests to Run

Existing tests that exercise these functions (should still pass):
```bash
cd trade-app && npx vitest run tests/PayoffChart.test.js tests/ChartDataFlowIntegration.test.js
```

**No new test file needed for this step.** The `PayoffChart.test.js` already tests `generateMultiLegPayoff` and chart rendering. The changes are additive (new optional field in return object, conditional dataset) and backward-compatible. The existing tests verify that the chart still works correctly without T+0 data.

However, add the following to `tests/blackScholes.test.js` (extending from Step 1):

- Test that `generateMultiLegPayoff()` now returns `_creditAdjustment` field
- Test that `createMultiLegChartConfig()` produces 3 datasets when `t0Payoffs` is `null`
- Test that `createMultiLegChartConfig()` produces 4 datasets when `t0Payoffs` is a valid array
- Test that the 4th dataset (index 1) has label "Today (T+0)", correct color, dashed style

**Note:** These chart config tests should go in a new section at the end of `tests/blackScholes.test.js` or in the existing `tests/PayoffChart.test.js` — whichever is cleaner. Recommendation: add them to `tests/PayoffChart.test.js` since that file already imports `generateMultiLegPayoff` and `createMultiLegChartConfig` from `chartUtils.js`.

### Verification
```bash
cd trade-app && npx vitest run tests/PayoffChart.test.js tests/ChartDataFlowIntegration.test.js tests/blackScholes.test.js
```

### Commit Message
```
feat: add T+0 dataset to payoff chart config and enhance crosshair tooltip

- Expose _creditAdjustment from generateMultiLegPayoff() for T+0 reuse
- Conditionally add "Today (T+0)" dataset when chartData.t0Payoffs is
  present (dashed blue line, no fill, renders above expiration line)
- Update Y-axis dynamic min/max to include T+0 values in range calc
- Extend crosshair plugin: dual interpolation, dual intersection circles,
  3-line tooltip showing both Expiry P&L and Today P&L
- All changes are backward-compatible: without t0Payoffs, chart renders
  exactly as before
```

---

## Step 3: Modify `OptionsTrading.vue` — import `calculateT0Payoffs`, add `computeT0ForPositions()`, extend `updateChartData()`

### Goal
Wire up the T+0 calculation into the existing data flow. Fetch IV from Greeks, compute time-to-expiration per leg, call `calculateT0Payoffs()`, and attach the result to `chartData`.

### File to Modify

**`trade-app/src/views/OptionsTrading.vue`**

#### Change 1: Add import for `calculateT0Payoffs`

At line 186 (existing import from `chartUtils.js`), add a new import:

```javascript
import { calculateT0Payoffs } from "../utils/blackScholes";
```

#### Change 2: Destructure `getOptionGreeks` from `useMarketData()`

The `useMarketData()` composable is already called at line 223:
```javascript
const { getIvxDataStreaming } = useMarketData();
```

Change to:
```javascript
const { getIvxDataStreaming, getOptionGreeks } = useMarketData();
```

#### Change 3: Add `computeT0ForPositions()` helper function

Add this function near the other helper functions (e.g., after `updateChartData` around line 783):

```javascript
const computeT0ForPositions = (positions, payoffData) => {
  const optionPositions = positions.filter(
    (pos) => pos.asset_class === "us_option"
  );
  if (optionPositions.length === 0) return null;

  const now = new Date();
  const legs = [];

  for (const pos of optionPositions) {
    const greeksRef = getOptionGreeks(pos.symbol);
    const greeks = greeksRef.value;

    if (!greeks || !greeks.implied_volatility) {
      return null;  // Per FR-6: if ANY leg lacks IV, skip T+0 entirely
    }

    const expiryDate = new Date(pos.expiry_date + "T16:00:00");
    const msToExpiry = expiryDate.getTime() - now.getTime();
    const T = Math.max(msToExpiry / (365.25 * 24 * 60 * 60 * 1000), 0);

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
};
```

#### Change 4: Extend `updateChartData()` to compute T+0

Modify the existing `updateChartData()` function (line 758) to compute T+0 payoffs after the expiration payoff and attach them to `chartData`:

Replace the current block (lines 768-778):
```javascript
const payoffData = generateMultiLegPayoff(
  positions,
  currentPrice.value,
  adjustedNetCredit.value
);

// Force reactivity by creating a new object reference
chartData.value = {
  ...payoffData,
  timestamp: Date.now(),
};
```

With:
```javascript
const payoffData = generateMultiLegPayoff(
  positions,
  currentPrice.value,
  adjustedNetCredit.value
);

if (payoffData) {
  // Attempt T+0 calculation (optional — fails silently)
  let t0Payoffs = null;
  try {
    t0Payoffs = computeT0ForPositions(positions, payoffData);
  } catch (e) {
    console.warn("T+0 calculation failed:", e.message);
  }
  payoffData.t0Payoffs = t0Payoffs;

  chartData.value = {
    ...payoffData,
    timestamp: Date.now(),
  };
} else {
  chartData.value = null;
}
```

### Tests

This step modifies `OptionsTrading.vue`, which is a heavily mocked component in tests. The existing `tests/OptionsTrading.test.js` mocks most composables. No new test file is needed for this step because:

1. The `computeT0ForPositions()` function is a local helper that calls `getOptionGreeks()` (mocked in tests) and `calculateT0Payoffs()` (already tested in Step 1).
2. The integration between these components is validated by the existing `PayoffChart.test.js` and `ChartDataFlowIntegration.test.js` tests, which verify that `chartData` objects with various shapes are handled correctly.
3. The try/catch ensures that any failure in T+0 computation is non-fatal.

**However**, add integration tests to `tests/ChartDataFlowIntegration.test.js` to verify:
- `generateMultiLegPayoff` result now has `_creditAdjustment` property
- When `t0Payoffs` is attached to `chartData`, the chart data flow continues to work correctly
- `t0Payoffs: null` does not break any existing chart data flow tests

### Verification
```bash
cd trade-app && npx vitest run tests/OptionsTrading.test.js tests/ChartDataFlowIntegration.test.js
```

### Commit Message
```
feat: integrate T+0 payoff calculation into OptionsTrading data flow

- Import calculateT0Payoffs from blackScholes.js
- Destructure getOptionGreeks from useMarketData composable
- Add computeT0ForPositions() helper: reads IV per leg from Greeks,
  calculates time-to-expiry, calls calculateT0Payoffs()
- Extend updateChartData() to compute and attach t0Payoffs to chartData
- Graceful degradation: if any leg lacks IV, t0Payoffs is null and
  chart renders only the expiration line (unchanged behavior)
```

---

## Step 4: Modify `PayoffChart.vue` — add dataset count check in `performMinorUpdate()`

### Goal
Ensure that when the T+0 dataset appears or disappears (e.g., IV data becomes available or is lost), the chart falls back to a full update instead of a potentially broken minor update.

### File to Modify

**`trade-app/src/components/PayoffChart.vue`**

#### Change

In the `performMinorUpdate()` method (around line 500), after the existing labels-length check (lines 515-519):

```javascript
if (config.data.labels.length !== chart.value.data.labels.length) {
  console.warn("Labels length changed, falling back to full update");
  await performChartUpdate();
  return;
}
```

Add a new dataset count check immediately after:

```javascript
// Fall back to full update if dataset count changed (T+0 appeared/disappeared)
if (config.data.datasets.length !== chart.value.data.datasets.length) {
  console.log("Dataset count changed, falling back to full update");
  await performChartUpdate();
  return;
}
```

### Tests

The existing `PayoffChart.test.js` mocks Chart.js and tests component rendering. Add a targeted test:

- Verify that `PayoffChart` component handles `chartData` with `t0Payoffs` property without errors
- Verify that `PayoffChart` component handles `chartData` with `t0Payoffs: null` without errors

These can be added to the existing "Edge Cases and Error Handling" describe block in `tests/PayoffChart.test.js`.

### Verification
```bash
cd trade-app && npx vitest run tests/PayoffChart.test.js
```

### Commit Message
```
fix: add dataset count guard in PayoffChart minor update path

When the T+0 dataset appears or disappears (IV data becomes available
or is lost), the number of Chart.js datasets changes. The minor update
path matches datasets by label+index and would silently skip the
new/removed dataset. This guard detects the count mismatch and falls
back to a full chart update for a clean transition.
```

---

## Step 5: Run all existing tests, verify no regressions, final review

### Goal
Confirm that all 20 existing test files plus the new `blackScholes.test.js` pass. Review the complete changeset for correctness.

### Verification Steps

1. **Run full test suite:**
   ```bash
   cd trade-app && npm test -- --run
   ```
   All 21 test files (20 existing + 1 new) must pass.

2. **Run the build to verify no compilation errors:**
   ```bash
   cd trade-app && npm run build
   ```

3. **Manual review checklist:**
   - [ ] `blackScholes.js` has no Vue/Chart.js imports (pure utility)
   - [ ] `cumulativeNormalDistribution` handles negative inputs correctly
   - [ ] `blackScholesPrice` handles all edge cases: `T<=0`, `sigma<=0`, `S<=0`
   - [ ] `calculateT0Payoffs` returns array of same length as input `prices`
   - [ ] `generateMultiLegPayoff` return object includes `_creditAdjustment`
   - [ ] `createMultiLegChartConfig` only adds T+0 dataset when `t0Payoffs` is valid array of correct length
   - [ ] Crosshair plugin gracefully handles `t0Payoffs: null` (shows 2-line tooltip, single circle)
   - [ ] Crosshair plugin handles `t0Payoffs` present (shows 3-line tooltip, dual circles)
   - [ ] Y-axis dynamic range considers T+0 values when present
   - [ ] `computeT0ForPositions` returns `null` when any leg lacks IV data
   - [ ] `updateChartData` wraps T+0 in try/catch (non-fatal failure)
   - [ ] `performMinorUpdate` has dataset count check before label-matching loop
   - [ ] No existing tests are modified in a way that reduces coverage
   - [ ] No existing component behavior is changed when T+0 data is absent

4. **Verify acceptance criteria mapping:**

   | AC# | Verification |
   |-----|-------------|
   | AC-1 | T+0 dataset conditionally added when `t0Payoffs` present |
   | AC-2 | Dashed blue line, no fill, distinct from solid dark expiration line |
   | AC-3 | Black-Scholes math validated in `blackScholes.test.js` |
   | AC-4 | `T <= 0` returns intrinsic value → T+0 converges with expiration |
   | AC-5 | Crosshair shows both Expiry P&L and Today P&L in tooltip |
   | AC-6 | `computeT0ForPositions` returns `null` on missing IV → no T+0 dataset |
   | AC-7 | No new update mechanisms; T+0 uses existing lifecycle |
   | AC-8 | Dataset count check prevents broken minor updates |
   | AC-9 | Legend entry "Today (T+0)" appears when dataset present |
   | AC-10 | Full test suite passes |

### Commit Message
```
test: verify T+0 payoff chart enhancement — no regressions

Run full test suite (21 files) confirming all existing behavior is
preserved and the new T+0 line integrates cleanly. Build succeeds
with no errors.
```

---

## Summary

| Step | Files Created | Files Modified | Tests | Focus |
|------|--------------|----------------|-------|-------|
| 1 | `src/utils/blackScholes.js`, `tests/blackScholes.test.js` | — | New unit tests for BS math | Pure computation |
| 2 | — | `src/utils/chartUtils.js` | Extend `PayoffChart.test.js` | Chart config + crosshair |
| 3 | — | `src/views/OptionsTrading.vue` | Extend `ChartDataFlowIntegration.test.js` | Data flow wiring |
| 4 | — | `src/components/PayoffChart.vue` | Extend `PayoffChart.test.js` | Update lifecycle guard |
| 5 | — | — | Run all tests | Regression check |

**Total changes:** 1 new source file, 3 modified source files, 1 new test file, 2 extended test files.
