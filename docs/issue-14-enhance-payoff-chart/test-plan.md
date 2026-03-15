# Test Plan: Enhance Payoff Chart with T+0 (Today) Line

**Issue:** [#14 — Enhance Payoff Chart](https://github.com/schardosin/juicytrade/issues/14)
**Requirements:** [requirements.md](./requirements.md)
**Architecture:** [architecture.md](./architecture.md)
**Date:** 2025-01-28
**QA Engineer:** @qa

---

## Test Strategy

This plan validates the implementation against all 10 acceptance criteria (AC-1 through AC-10) from the requirements document. Testing combines:
- **Review of existing dev tests** (54 blackScholes tests, 19 PayoffChart tests, 16 ChartDataFlow tests)
- **New QA tests** covering gaps, edge cases, and integration points
- **Code quality review** for architectural compliance
- **Full regression suite** to confirm no breakage

---

## Step 1: Review Existing blackScholes.js Unit Tests

**Objective:** Verify the dev's 54 tests in `trade-app/tests/blackScholes.test.js` adequately cover the Black-Scholes implementation.

**What to check:**
- CND accuracy against known statistical values (e.g., N(0)=0.5, N(1.96)≈0.975)
- CND symmetry: N(-x) = 1 - N(x)
- CND boundary: N(large positive) → 1, N(large negative) → 0
- Black-Scholes call pricing against known reference values
- Black-Scholes put pricing against known reference values
- Edge case: T ≤ 0 returns intrinsic value (call: max(S-K,0), put: max(K-S,0))
- Edge case: σ ≤ 0 returns intrinsic value
- Edge case: S ≤ 0 handled without NaN/Infinity
- Put-call parity: C - P = S - K*e^(-rT)
- `calculateT0Payoffs()` with multi-leg scenarios (iron condor, spreads)
- `calculateT0Payoffs()` credit adjustment applied correctly
- `calculateT0Payoffs()` empty legs returns all zeros
- 0 DTE convergence: T+0 matches expiration payoff when T=0
- Performance: 500 prices × 4 legs under 50ms

**AC Mapping:** AC-3 (correct Black-Scholes pricing), AC-4 (0 DTE convergence)

---

## Step 2: Review Existing chartUtils/PayoffChart/ChartDataFlow Tests

**Objective:** Verify the dev's tests in `trade-app/tests/PayoffChart.test.js` and `trade-app/tests/ChartDataFlow.test.js` cover the chart integration.

**What to check:**
- T+0 dataset conditionally added when `chartData.t0Payoffs` is present
- T+0 dataset absent when `t0Payoffs` is null/undefined
- Visual style: dashed blue `rgb(0, 150, 255)`, `borderDash: [6, 3]`, no fill
- Dataset render order: T+0 has `order: 0` (above expiration)
- Crosshair plugin receives `t0Payoffs` in plugin options
- Y-axis dynamic range includes T+0 values
- `_creditAdjustment` exposed from `generateMultiLegPayoff()`
- Backward compatibility: chart works identically without T+0 data

**AC Mapping:** AC-1, AC-2, AC-5, AC-8, AC-9

---

## Step 3: Write QA Tests for blackScholes.js — Additional Edge Cases

**Objective:** Write additional tests covering edge cases the dev may have missed.

**Tests to write:**
1. CND at x=0 returns exactly 0.5
2. CND symmetry property: N(-x) = 1 - N(x) for multiple x values
3. CND at extreme values: N(10) ≈ 1, N(-10) ≈ 0
4. BS with very large T (e.g., T=10 years) — should not produce NaN
5. BS with very large sigma (e.g., sigma=5.0) — should not produce NaN
6. BS at-the-money call vs put symmetry check
7. BS call price always ≥ 0
8. BS put price always ≥ 0 (non-negative prices)
9. `calculateT0Payoffs()` with empty legs array returns array of zeros
10. `calculateT0Payoffs()` single long call leg — verify P&L direction
11. `calculateT0Payoffs()` single short put leg — verify P&L direction
12. `calculateT0Payoffs()` legs with T=0 should produce intrinsic-value-based payoffs matching expiration
13. `calculateT0Payoffs()` with creditAdjustment shifts all values by the adjustment amount
14. Verify blackScholes.js has zero imports (no Vue, no Chart.js, no external deps)

**AC Mapping:** AC-3, AC-4, AC-6

---

## Step 4: Write QA Tests for chartUtils.js — Integration and Backward Compatibility

**Objective:** Write tests verifying the chart configuration correctly handles T+0 data.

**Tests to write:**
1. `createMultiLegChartConfig()` with `t0Payoffs` present: dataset with label "Today (T+0)" exists
2. `createMultiLegChartConfig()` with `t0Payoffs: null`: no "Today (T+0)" dataset
3. `createMultiLegChartConfig()` with `t0Payoffs: undefined`: no "Today (T+0)" dataset
4. T+0 dataset has exact color `rgb(0, 150, 255)`
5. T+0 dataset has `borderDash: [6, 3]`
6. T+0 dataset has `fill: false`
7. T+0 dataset has `order: 0` (renders above expiration)
8. T+0 dataset has `pointRadius: 0`
9. Expiration dataset unchanged (still solid, gradient fill, `rgb(33, 37, 41)`)
10. Legend label is exactly "Today (T+0)" (not "T+0", "Today", etc.)
11. Plugin options include `t0Payoffs` when provided
12. Plugin options have `t0Payoffs: null` when not provided
13. `_creditAdjustment` is present in `generateMultiLegPayoff()` return value
14. Y-axis range extends to include T+0 min/max when T+0 values outside expiration range
15. Backward compatibility: crosshair tooltip label is "P&L:" when no T+0 (verify in plugin options structure)
16. When T+0 present: tooltip uses "Expiry P&L:" and "Today P&L:" labels

**AC Mapping:** AC-1, AC-2, AC-5, AC-6, AC-9

---

## Step 5: Write QA Tests for OptionsTrading.vue — computeT0ForPositions

**Objective:** Verify the IV fetching logic and T+0 computation bridge.

**Tests to write:**
1. Returns `null` when any leg is missing IV (implied_volatility)
2. Returns `null` when no option positions (all positions are stock)
3. Returns `null` when Greeks object exists but `implied_volatility` is falsy (0 or null)
4. Correctly builds legs array with proper fields: strike_price, option_type, qty, avg_entry_price, iv, T
5. Time-to-expiry (T) is positive for future dates and clamped to 0 for past dates
6. Uses `_creditAdjustment` from payoffData
7. Calls `calculateT0Payoffs` with correct parameters
8. Returns valid array when all legs have IV data

**AC Mapping:** AC-6 (graceful degradation)

---

## Step 6: Write QA Tests for PayoffChart.vue — Dataset Count Guard

**Objective:** Verify the dataset count guard prevents flickering when T+0 appears/disappears.

**Tests to write:**
1. `performMinorUpdate()` detects dataset count change (3→4) and falls back to full update
2. `performMinorUpdate()` detects dataset count change (4→3) and falls back to full update
3. `performMinorUpdate()` proceeds normally when dataset count is unchanged (4→4)
4. Guard is in the correct location (before label-matching loop)

**AC Mapping:** AC-8 (no flickering)

---

## Step 7: Code Quality Review (Manual Checks)

**Objective:** Verify architectural compliance and code quality.

**Checks:**
1. `blackScholes.js` has zero imports — no Vue, no Chart.js, no external libraries
2. `blackScholes.js` exports exactly 3 functions: `cumulativeNormalDistribution`, `blackScholesPrice`, `calculateT0Payoffs`
3. Label-based matching in `PayoffChart.vue` uses exact label strings from architecture
4. Backward compatibility: when T+0 absent, tooltip label is "P&L:" (unchanged from before)
5. No new Chart.js re-renders or destructions introduced
6. `computeT0ForPositions()` is a local helper in OptionsTrading.vue (not exported)
7. Risk-free rate is hardcoded at 0.05
8. `_creditAdjustment` uses underscore prefix convention

**AC Mapping:** AC-7 (zoom/pan preserved), AC-10 (existing tests pass)

---

## Step 8: Run Full Test Suite — Regression Check

**Objective:** Run the complete test suite and confirm no regressions.

**Actions:**
1. Run `cd trade-app && npx vitest run`
2. Confirm all payoff-chart-related tests pass (blackScholes, PayoffChart, ChartDataFlow)
3. Confirm the only failures are pre-existing in `CollapsibleOptionsChain.test.js`
4. Document total pass/fail counts

**AC Mapping:** AC-10 (all existing frontend tests continue to pass)

---

## Acceptance Criteria Traceability Matrix

| AC# | Criterion | Test Steps |
|-----|-----------|------------|
| AC-1 | Two lines when IV available | Steps 2, 4 |
| AC-2 | Visually distinct style | Steps 2, 4 |
| AC-3 | Correct Black-Scholes pricing | Steps 1, 3 |
| AC-4 | 0 DTE convergence | Steps 1, 3 |
| AC-5 | Dual tooltip values | Steps 2, 4 |
| AC-6 | Graceful degradation | Steps 3, 4, 5 |
| AC-7 | Zoom/pan preserved | Step 7 |
| AC-8 | No flickering | Steps 2, 6 |
| AC-9 | Legend entries | Steps 2, 4 |
| AC-10 | No regressions | Steps 1, 7, 8 |
