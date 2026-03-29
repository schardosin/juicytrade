# Requirements: Increase Data Available in the Payoff Chart

**Issue:** [#42 — Increase data available in the payoff chart](https://github.com/schardosin/juicytrade/issues/42)
**Status:** Draft — awaiting customer approval

---

## 1. Problem Statement

The payoff chart currently calculates its price range based on the **strike range** (distance between the widest selected strikes), not the **underlying price level**. This produces adequate ranges for low/mid-priced stocks but far too narrow ranges for high-value symbols.

### Root Cause Analysis

The core logic in `generateMultiLegPayoff()` (`trade-app/src/utils/chartUtils.js`) computes the data range as follows:

**Multi-leg strategies:**
```
baseExtension = max(strikeRange × 5.0, $500)
maxExtension  = min(baseExtension, $2000)
lowerBound    = minStrike − maxExtension
upperBound    = maxStrike + maxExtension
```

**Single-leg strategies:**
```
if strike < 50:   range = max(strike × 8.0, $250)
if strike < 200:  range = max(strike × 5.0, $500)
else:             range = max(strike × 3.0, $1000)
```

The **multi-leg path** is entirely strike-range-dependent and has no awareness of the underlying price. For NDX (~$23,132) with a $50-wide spread:
- `baseExtension` = max(50 × 5, 500) = 500
- `maxExtension` = min(500, 2000) = 500
- Chart shows ±$500 around the strikes → only **±2.2%** of NDX's price

For comparison, the same $50-wide spread on a $200 stock would show ±$500 → **±250%** of the stock price. This disparity is the core of the problem.

Additionally, three related constraints compound the issue:

1. **Initial view window** (`suggestedMin`/`suggestedMax`) uses `strikeSpan × 1.5` with padding of `max(0.25 × strikeSpan, $5)` — far too tight for high-priced symbols.
2. **Pan/zoom limits** are derived from the same tight strike-based calculation, preventing users from scrolling to wider ranges even if more data existed.
3. **Step size** for narrow strategies (≤5 wide) is fixed at 1, which is fine for low-priced stocks but creates excessive data points for high-priced symbols (e.g., a $6000 range at step=1 = 6000 points).

### Example: NDX at ~$23,132

| Metric | Current | Typical Broker Tool |
|--------|---------|-------------------|
| Chart range | ±$500 (~±2.2%) | ±$2,300–$4,600 (~±10–20%) |
| Initial view | ~$75 window (~0.3%) | ~$2,300 window (~10%) |
| Pan limit | ~$100 beyond initial view | Full generated range |

---

## 2. Context and Motivation

Options payoff charts are a core analytical tool for traders. Broker platforms (Schwab, Tastytrade, etc.) typically show price ranges of **±10–20%** of the underlying price, giving traders context for how the position behaves across meaningful price movements. The current implementation fails to provide this context for higher-priced symbols like NDX, SPX, AMZN, etc., making the chart significantly less useful compared to broker tools.

---

## 3. Functional Requirements

### FR-1: Price-Level-Aware Range Calculation (Multi-Leg)

The `generateMultiLegPayoff()` function SHALL compute the price range extension using **both** the strike range and the underlying price level.

**Proposed approach:** Use a **percentage of the underlying price** as the primary range driver, with a floor based on the strike range to ensure the full strategy structure is always visible.

```
percentageRange = underlyingPrice × 0.10   (10% of underlying price on each side)
strikeBasedRange = strikeRange × 3.0       (3× the strike span for strategy visibility)
extension = max(percentageRange, strikeBasedRange, 50)   (floor of $50 for very cheap symbols)
```

For NDX ($23,132) with a $50 spread:
- `percentageRange` = 23,132 × 0.10 = $2,313
- `strikeBasedRange` = 50 × 3 = $150
- `extension` = $2,313
- Chart range: ±$2,313 → **±10%** ✓

For a $50 stock with a $5 spread:
- `percentageRange` = 50 × 0.10 = $5
- `strikeBasedRange` = 5 × 3 = $15
- `extension` = max(5, 15, 50) = $50
- Chart range: ±$50 → still reasonable ✓

> **Note:** The 10% value is a proposed starting point. The exact percentage should be validated during implementation against multiple real-world scenarios (NDX, SPX, AMZN, low-priced stocks) to ensure it looks good across the board. Small adjustments are acceptable without re-approval.

### FR-2: Price-Level-Aware Range Calculation (Single-Leg)

The single-leg path SHALL also benefit from a percentage-based approach. The current tiered multipliers (8×, 5×, 3× the strike) already scale with price, but the hard-coded dollar minimums ($250, $500, $1000) can be replaced with a percentage-based minimum for consistency.

**Proposed approach:**
```
range = max(strike × currentMultiplier, underlyingPrice × 0.10, 50)
```

This keeps the existing behavior for most stocks while ensuring high-priced single-leg options also get meaningful range.

### FR-3: Adaptive Step Size

The step size SHALL adapt to the underlying price level to avoid generating excessive data points for high-priced symbols while maintaining sufficient resolution.

**Proposed approach:** Scale step size based on the total range, targeting approximately **200–500 data points** regardless of price level.

```
totalRange = upperBound − lowerBound
step = max(Math.ceil(totalRange / 400), 1)
```

This ensures:
- NDX ($4,626 range): step ≈ 12 → ~386 points ✓
- $50 stock ($100 range): step = 1 → 100 points ✓

Strike prices MUST still be included in the generated price set (existing `priceSet` logic) to ensure accuracy at critical points.

### FR-4: Wider Initial View Window

The `createMultiLegChartConfig()` function's initial view (`suggestedMin`/`suggestedMax`) SHALL be widened to show a meaningful percentage of the underlying price, not just the strike span.

**Proposed approach:**
```
viewPad = max(underlyingPrice × 0.05, strikeSpan × 1.5, 25)
suggestedMin = floor(minStrike − viewPad)
suggestedMax = ceil(maxStrike + viewPad)
```

For NDX: viewPad = max(1,157, 75, 25) = $1,157 → initial view shows ~$2,364 range → **~10%** ✓

### FR-5: Wider Pan/Zoom Limits

The chart's pan/zoom limits SHALL be expanded to allow users to explore the full generated data range (or close to it), rather than being restricted to a tight strike-based window.

**Proposed approach:** Set pan/zoom limits to the full extent of the generated `prices` array (i.e., `lowerBound` to `upperBound`), allowing users to scroll through the entire computed range.

### FR-6: No Regression on Low-Priced Symbols

The changes MUST NOT degrade the payoff chart experience for low/mid-priced symbols ($5–$200 range). The $50 minimum floor and strike-based floors ensure that cheap symbols still get adequate range.

---

## 4. Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| AC-1 | For NDX (~$23,132) with a $50-wide spread, the chart data range extends at least ±$2,000 from the strikes | Unit test with mock positions |
| AC-2 | For a $50 stock with a $5-wide spread, the chart data range extends at least ±$50 from the strikes | Unit test with mock positions |
| AC-3 | For NDX, the initial view window (`suggestedMin`/`suggestedMax`) spans at least $2,000 | Unit test on chart config |
| AC-4 | For NDX, the chart generates ≤600 data points (no performance degradation) | Unit test checking `prices.length` |
| AC-5 | Pan/zoom limits allow exploring the full generated data range | Manual verification / code review |
| AC-6 | All strike prices are still included in the generated data points (existing behavior preserved) | Unit test |
| AC-7 | Break-even calculation, net credit calculation, and payoff calculation logic remain unchanged (only range/step changes) | Existing tests pass |
| AC-8 | The `generateButterflyPayoff()` function is also updated consistently (it has its own range logic) | Code review |

---

## 5. Scope

### In Scope
- `generateMultiLegPayoff()` in `trade-app/src/utils/chartUtils.js` — range and step logic
- `generateButterflyPayoff()` in `trade-app/src/utils/chartUtils.js` — range and step logic (for consistency)
- `createMultiLegChartConfig()` in `trade-app/src/utils/chartUtils.js` — initial view and pan/zoom limits
- Unit tests covering the new range behavior across different price levels

### Out of Scope
- Changes to the payoff calculation formulas themselves (intrinsic value, break-even, etc.)
- Changes to the theoretical payoff / T+0 line calculation (`theoreticalPayoff.js`, `blackScholes.js`)
- Changes to the PayoffChart.vue component rendering logic
- Adding user-configurable range settings (potential future enhancement)
- Backend changes (this is entirely a frontend calculation change)

---

## 6. Affected Files

| File | Change Type |
|------|------------|
| `trade-app/src/utils/chartUtils.js` | Modify — range calculation, step size, initial view, pan/zoom limits |
| `trade-app/tests/` (new test file) | Add — unit tests for range behavior across price levels |

---

## 7. Risks and Considerations

- **Performance:** Increasing the data range could increase the number of chart data points. The adaptive step size (FR-3) mitigates this by targeting a consistent point count regardless of price level.
- **Visual consistency:** The chart should still look "zoomed in" enough to show the strategy structure clearly on initial load. The initial view window (FR-4) handles this by starting with a ~10% view centered on the strikes.
- **Butterfly strategies:** `generateButterflyPayoff()` has its own separate range logic with the same issue. It should be updated consistently (AC-8).
