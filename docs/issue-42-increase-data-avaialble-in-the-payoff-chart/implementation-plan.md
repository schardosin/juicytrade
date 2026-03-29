# Implementation Plan: Increase Data Available in the Payoff Chart

**Issue:** [#42](https://github.com/schardosin/juicytrade/issues/42)
**Requirements:** [requirements.md](./requirements.md)

---

## Files

| File | Action |
|------|--------|
| `trade-app/src/utils/chartUtils.js` | Modify |
| `trade-app/tests/chartUtils-range.test.js` | Create |

---

## Steps

### Step 1 — `generateMultiLegPayoff()`: Replace multi-leg range calculation with percentage-based extension

**File:** `trade-app/src/utils/chartUtils.js`, lines 420–425

Replace the strike-based `baseExtension` / `maxExtension` logic in the `else` (multi-leg) branch with:

```js
const percentageRange = underlyingPrice * 0.10;
const strikeBasedRange = strikeRange * 3.0;
const extension = Math.max(percentageRange, strikeBasedRange, 50);

lowerBound = Math.floor(minStrike - extension);
upperBound = Math.ceil(maxStrike + extension);
```

**Why:** The current `max(strikeRange * 5, 500)` capped at `$2000` produces only ±2.2% for NDX. The new formula uses 10% of underlyingPrice as the primary driver, with `strikeRange * 3` and `$50` as floors, matching FR-1.

**Verify:** No test changes yet — existing tests should still pass since cheap-stock ranges will be similar or slightly wider.

---

### Step 2 — `generateMultiLegPayoff()`: Add percentage-based floor to single-leg range

**File:** `trade-app/src/utils/chartUtils.js`, lines 402–418

In the `if (strikeRange === 0)` branch, after computing `range` from the existing tiered multiplier logic, add `underlyingPrice * 0.10` as an additional candidate in the `Math.max`:

```js
let range;
if (singleStrike < 50) {
  range = Math.max(singleStrike * 8.0, underlyingPrice * 0.10, 50);
} else if (singleStrike < 200) {
  range = Math.max(singleStrike * 5.0, underlyingPrice * 0.10, 50);
} else {
  range = Math.max(singleStrike * 3.0, underlyingPrice * 0.10, 50);
}
```

**Why:** The existing dollar floors (`$250`, `$500`, `$1000`) are replaced with a unified percentage floor and a `$50` absolute minimum, matching FR-2. For most stocks the multiplier still dominates; for high-priced symbols the percentage floor kicks in.

**Verify:** Existing tests should still pass since this only widens (never narrows) ranges.

---

### Step 3 — `generateMultiLegPayoff()`: Replace all step-size logic with adaptive formula

**File:** `trade-app/src/utils/chartUtils.js`, lines 418 and 427–443

Replace the step-size computation in **both** the single-leg and multi-leg branches with a single adaptive formula applied after `lowerBound` / `upperBound` are set:

```js
const totalRange = upperBound - lowerBound;
step = Math.max(Math.ceil(totalRange / 400), 1);
```

Remove the old step logic entirely: the single-leg `range > 200 ? 5 : range > 100 ? 2 : 1` block (line 418) and the multi-leg `strikeRange <= 2 / <= 5 / else` block (lines 427–443).

**Why:** The old step logic used fixed thresholds that produced either too few or too many points for extreme price levels. The adaptive formula targets ~400 data points regardless of price level (FR-3). A $100 range yields step=1 (100 pts); a $4,626 range yields step≈12 (~386 pts).

**Verify:** Existing tests should still pass. The `priceSet` logic (line 447) already ensures strike prices are included regardless of step size.

---

### Step 4 — `createMultiLegChartConfig()`: Use percentage-based initial view window

**File:** `trade-app/src/utils/chartUtils.js`, lines 761–776

In the `if (chartData.strikes.length >= 2)` branch, replace the strike-span-based `width` / `pad` calculation with a percentage-based `viewPad`:

```js
if (chartData && chartData.strikes && chartData.strikes.length >= 2) {
  const sMin = Math.min(...chartData.strikes);
  const sMax = Math.max(...chartData.strikes);
  strikeSpan = Math.max(sMax - sMin, 1);

  const viewPad = Math.max(underlyingPrice * 0.05, strikeSpan * 1.5, 25);
  suggestedMin = Math.floor(sMin - viewPad);
  suggestedMax = Math.ceil(sMax + viewPad);
}
```

**Why:** The current `pad = max(0.25 * strikeSpan, 5)` produces a ~$17 initial window for a $50 NDX spread. The new formula uses 5% of underlyingPrice as the primary driver (~$1,157 for NDX), matching FR-4.

**Verify:** Existing QA tests in `qa-chartUtils.test.js` don't assert on `suggestedMin`/`suggestedMax` values, so they should pass unchanged.

---

### Step 5 — `createMultiLegChartConfig()`: Expand pan/zoom limits to full data range

**File:** `trade-app/src/utils/chartUtils.js`, lines 1076–1085

Replace the strike-based pan/zoom `limits.x` block with simple limits based on the full generated data extent:

```js
limits: {
  x: {
    min: Math.min(...prices),
    max: Math.max(...prices),
  },
},
```

**Why:** The current limits are derived from `suggestedMin`/`suggestedMax` plus a small strike-based buffer, preventing users from panning to the wider data range that now exists. Since Step 1 already generates a meaningful ±10% range, letting users pan to the full extent is safe (FR-5).

**Verify:** Existing QA tests don't assert on zoom limits. Manual verification recommended.

---

### Step 6 — `generateButterflyPayoff()`: Update Iron Condor path range and step size

**File:** `trade-app/src/utils/chartUtils.js`, lines 17–24

In the Iron Condor branch, replace the range and step logic:

```js
// Before (lines 17-24):
const lowerBound = Math.floor(long_put_strike - legWidth);
const upperBound = Math.ceil(long_call_strike + legWidth);
const step = upperBound - lowerBound > 100 ? (upperBound - lowerBound > 200 ? 5 : 2) : 1;

// After:
const lowerBound = Math.floor(long_put_strike - legWidth);
const upperBound = Math.ceil(long_call_strike + legWidth);
const totalRange = upperBound - lowerBound;
const step = Math.max(Math.ceil(totalRange / 400), 1);
```

**Why:** Consistency with `generateMultiLegPayoff()`. The butterfly function has its own range/step logic that suffers from the same fixed-threshold problem (AC-8). The `legWidth` parameter already controls the range extent here, so only the step formula needs updating.

**Verify:** No existing unit tests cover `generateButterflyPayoff()` range/step specifically.

---

### Step 7 — `generateButterflyPayoff()`: Update standard butterfly/iron butterfly path range and step size

**File:** `trade-app/src/utils/chartUtils.js`, lines 71–74

In the non-Iron-Condor branch, apply the same step fix:

```js
// Before (lines 71-74):
const lowerBound = Math.floor(lower_strike - legWidth);
const upperBound = Math.ceil(upper_strike + legWidth);
const step = upperBound - lowerBound > 100 ? (upperBound - lowerBound > 200 ? 5 : 2) : 1;

// After:
const lowerBound = Math.floor(lower_strike - legWidth);
const upperBound = Math.ceil(upper_strike + legWidth);
const totalRange = upperBound - lowerBound;
const step = Math.max(Math.ceil(totalRange / 400), 1);
```

**Why:** Same rationale as Step 6 — consistency across all payoff generators.

**Verify:** No existing unit tests cover this path's step logic.

---

### Step 8 — Write unit tests: NDX-level multi-leg spread

**File:** `trade-app/tests/chartUtils-range.test.js` (new)

Create the test file with helpers similar to `qa-chartUtils.test.js`. First test group:

```
describe("NDX-level multi-leg ($50-wide spread, underlyingPrice ~$23,132)")
  - data range extends at least ±$2,000 from the strikes (AC-1)
  - generates ≤600 data points (AC-4)
  - all strike prices included in the prices array (AC-6)
```

Use positions: long call at 23,100, short call at 23,150, underlyingPrice = 23,132.

---

### Step 9 — Write unit tests: cheap stock multi-leg spread (no regression)

Same file. Second test group:

```
describe("Cheap stock multi-leg ($5-wide spread on $50 stock)")
  - data range extends at least ±$50 from the strikes (AC-2)
  - generates a reasonable number of data points (not excessive, not too few)
  - range is not excessively wide (≤ ±$500 from strikes for a $50 stock)
```

Use positions: long call at 48, short call at 53, underlyingPrice = 50.

---

### Step 10 — Write unit tests: NDX single-leg option

Same file. Third test group:

```
describe("NDX single-leg (single call at 23,100, underlyingPrice ~$23,132)")
  - data range extends at least ±$2,000 from the strike
  - generates ≤600 data points
  - strike price is included in the prices array
```

---

### Step 11 — Write unit tests: strike inclusion verification

Same file. Fourth test group:

```
describe("Strike inclusion in generated data")
  - for a multi-leg spread, every strike price appears in the prices array
  - for a single-leg option, the strike price appears in the prices array
  - strike prices are present even when step size > 1 (high-priced symbol)
```

---

### Step 12 — Write unit tests: initial view window (`createMultiLegChartConfig`)

Same file. Fifth test group:

```
describe("Initial view window (suggestedMin / suggestedMax)")
  - for NDX ($50 spread), suggestedMin/suggestedMax span at least $2,000 (AC-3)
  - for NDX, suggestedMin < minStrike and suggestedMax > maxStrike
  - for a $50 stock ($5 spread), the initial view window is reasonable (≥ $25 on each side)
  - pan/zoom limits allow the full generated data range
```

Extract `suggestedMin`/`suggestedMax` from `config.options.scales.x`.

---

### Step 13 — Write unit tests: butterfly and iron condor step size

Same file. Sixth test group:

```
describe("generateButterflyPayoff range and step consistency")
  - Iron Condor with wide legWidth: generates ≤600 data points
  - Standard butterfly with wide legWidth: generates ≤600 data points
  - payoff values and break-even calculation still correct for a known butterfly
```

---

### Step 14 — Run full test suite and verify no regressions

Run `npx vitest run` from `trade-app/` to execute all tests including:
- `tests/qa-chartUtils.test.js` (existing 12 QA tests)
- `tests/chartUtils-range.test.js` (new range tests)
- Any other existing test files

Fix any failures before considering the implementation complete.

---

## Execution Order

Steps 1–7 are code changes to `chartUtils.js`. Steps 8–13 are test additions to the new test file. Step 14 is the final verification. Steps 1–3 can be done together as a single edit session (they all modify `generateMultiLegPayoff`). Steps 4–5 can be done together (they both modify `createMultiLegChartConfig`). Steps 6–7 can be done together (they both modify `generateButterflyPayoff`). Steps 8–13 are all additions to the same new test file and can be done in a single session.
