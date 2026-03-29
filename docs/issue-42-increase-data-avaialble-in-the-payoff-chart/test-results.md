# QA Test Results: Issue #42 — Increase Data Available in the Payoff Chart

**Date:** 2026-03-29
**Tester:** @qa
**Status:** ✅ **ALL TESTS PASS — QA APPROVED**

---

## Summary

| Metric | Value |
|--------|-------|
| **QA test file** | `trade-app/tests/qa-chartUtils-range.test.js` |
| **QA tests written** | 190 |
| **QA tests passing** | 190 (100%) |
| **QA tests failing** | 0 |
| **Full suite (baseline)** | 1166 tests (1164 pass, 2 known failures) |
| **Full suite (after QA)** | 1356 tests (1354 pass, 2 known failures) |
| **Regressions introduced** | 0 |
| **Duration (QA tests)** | ~393ms |
| **Duration (full suite)** | ~37.8s |

---

## Acceptance Criteria Verification

| AC# | Criterion | Status | Tests |
|-----|-----------|--------|-------|
| AC-1 | NDX-level prices (~$23,000): multi-leg range extends ≥ ±$2,000 from strikes | ✅ Pass | 1.1, 1.5, 3.1, 4.1, 7.3 |
| AC-2 | Cheap stocks (~$50): range still ≥ ±$50 (no regression) | ✅ Pass | 1.2, 1.3, 1.4, 3.2, 12.1, 12.2 |
| AC-3 | Data points never exceed 600 for any price level | ✅ Pass | 4.1–4.8 (all price tiers and strategy types) |
| AC-4 | Strike prices always included in generated data points | ✅ Pass | 5.1–5.6 (on-grid and off-grid strikes) |
| AC-5 | Initial view window width ≥ 5% of underlying for expensive symbols | ✅ Pass | 9.1–9.7 |
| AC-6 | Pan/zoom limits allow panning to full data range boundaries | ✅ Pass | 10.1–10.4 |
| AC-7 | Butterfly/IC payoff: same percentage-based logic applied consistently | ✅ Pass | 11a.1–11a.6, 11b.1–11b.5 |
| AC-8 | No regression: all existing tests still pass | ✅ Pass | Full suite: 1354/1356 (2 known pre-existing) |

---

## Test Sections Breakdown

### Section 1: Multi-Leg Range Formula Verification (18 tests) ✅
Verified `extension = Math.max(underlyingPrice * 0.10, strikeRange * 3.0, 50)` across 7 price tiers: NDX (~$23k), cheap stock ($50), penny-like ($9), mid-price ($200), SPX (~$5.9k), mega-cap ($50k), penny stock ($1).

### Section 2: Single-Leg Range Formula Verification (12 tests) ✅
Verified tiered multiplier: strike < $50 → mult=8/min=250; strike < $200 → mult=5/min=500; else → mult=3/min=1000. Tested across 6 price tiers from $1 to $50,000.

### Section 3: Adaptive Step Size Verification (20 tests) ✅
Verified `step = Math.max(Math.ceil(totalRange / 400), 1)` with boundary cases at totalRange=400 (step=1) and totalRange>400 (step=2). Confirmed dominant gap in prices array matches expected step.

### Section 4: Data Point Count Cap — Max 600 (8 tests) ✅
Verified ≤600 data points across NDX/mega-cap/penny multi-leg, NDX/mega-cap/cheap single-leg, NDX Iron Condor, and NDX Call Butterfly.

### Section 5: Strike Inclusion in Generated Data Points (7 tests) ✅
Verified all strike prices present in prices array for step>1 (NDX) and step=1 (cheap stock). Tested off-grid strikes (23103/23147) that don't align with step boundary. Verified sorted ascending order.

### Section 6: Extreme Price Levels (19 tests) ✅
- **Penny stocks (~$1):** $50 floor prevents absurdly narrow range; negative lowerBound handled without crash; no NaN/Infinity in output.
- **Mega-cap (~$50,000):** $10 spread on $50k/$20k underlying produces meaningful ±10% range; $500 spread correctly uses percentage-based extension.

### Section 7: Very Narrow Spreads on Expensive Symbols (8 tests) ✅
$10 spread on $20k, $5 spread on $5k, $1 spread on $23k — all produce meaningful chart ranges. Initial view window still shows strategy structure (strikes visible within view).

### Section 8: Very Wide Spreads on Cheap Symbols (7 tests) ✅
$40 spread on $25 stock, $20 spread on $10 stock — strikeRange×3 dominates extension. Negative lowerBound handled gracefully.

### Section 9: Initial View Window — FR-4 (22 tests) ✅
Verified `viewPad = Math.max(underlyingPrice * 0.05, strikeSpan * 1.5, 25)` across NDX, $50k, cheap stock, penny stock. Confirmed suggestedMin < minStrike and suggestedMax > maxStrike in all scenarios. Single-strike fallback branch tested.

### Section 10: Pan/Zoom Limits — FR-5 (4 tests) ✅
Verified `limits.x.min === prices[0]` and `limits.x.max === prices[last]`. Limits wider than initial view. Negative price limits for penny stocks.

### Section 11: Butterfly/IC Payoff Consistency — AC-8 (11 tests) ✅
- **Iron Condor:** NDX-level IC ≤600 points, range extends ≥±2000, step>1, payoff at max-profit zone = -netPremium×100, break-even correctness verified.
- **Call Butterfly:** NDX-level ≤600 points, range coverage correct, payoff at ATM = (legWidth - netPremium)×100.
- **Cheap IC and Butterfly:** step=1, ≤600 points.

### Section 12: No Regression on Low/Mid-Priced Symbols (7 tests) ✅
$50 stock range ≥±50, $200 stock range reasonable, $100 single-leg tierRange=500. Payoff correctness: long call strike=100 entry=5 at price=110 → payoff=$500. Net credit calculation verified for credit spread.

### Section 13: Edge Cases and Boundary Conditions (15 tests) ✅
- underlyingPrice=0 and 0.01: no crash, tierRange floors apply
- Identical strikes: correctly uses single-leg path
- Empty/null/undefined positions: returns null
- Non-option positions (us_equity): filtered out, returns null
- qty=100 vs qty=1: identical range/step, payoff scales by 100x
- Step rounding: ceil(totalRange/400) verified

### Section 14: Cross-Cutting Concerns (32 tests) ✅
Across 6 representative scenarios (NDX multi, cheap multi, penny multi, NDX single, $50k single, penny single):
- Prices always sorted ascending
- payoffs.length === prices.length
- No NaN or Infinity in prices or payoffs
- prices[0] < prices[last] always
- Both generateButterflyPayoff and generateMultiLegPayoff return all expected fields

---

## Known Pre-Existing Failures (Unrelated)

2 failures in `tests/CollapsibleOptionsChain.test.js`:
1. "displays strike count filter with correct options" — component now has 10 options, test expects 6
2. "provides all expected strike count options" — component added 150, 200, 250, 300 values

These are test/code mismatches from a prior update to the CollapsibleOptionsChain component. **Not related to Issue #42.**

---

## Regression Summary

| Suite | Before QA | After QA | Delta |
|-------|-----------|----------|-------|
| Test files | 36 | 37 | +1 |
| Total tests | 1166 | 1356 | +190 |
| Passing | 1164 | 1354 | +190 |
| Failing | 2 | 2 | 0 |

**Zero regressions introduced.**
