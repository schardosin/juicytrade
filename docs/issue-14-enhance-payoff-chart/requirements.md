# Requirements: Enhance Payoff Chart — Add T+0 (Today) Line

**Issue:** [#14 — Enhance Payoff Chart](https://github.com/schardosin/juicytrade/issues/14)  
**Date:** 2025-01-28  
**Status:** Draft — awaiting customer approval

---

## 1. Overview

Currently, the payoff chart in the Analysis tab (right panel) displays only the **expiration payoff line** — the theoretical P&L assuming all options are held to expiration. This request adds a **T+0 line (Today line)** that shows the estimated P&L if the underlying price moved right now, accounting for time value and implied volatility.

The T+0 line is a standard feature in professional options analysis tools and provides traders with critical insight into how their position performs *today*, not just at expiration.

## 2. Motivation & Business Value

- **Real-time decision support:** Traders need to see what happens to their P&L if the underlying moves now — not just at an abstract future expiration date. The T+0 line answers "if the stock moves to $X right now, what's my P&L?"
- **Time value visibility:** The gap between the T+0 line and the expiration line reveals the time value (theta) in the position. This is essential for strategies that profit from time decay (iron condors, butterflies, credit spreads).
- **Standard expectation:** Every professional options platform (ThinkorSwim, TastyTrade, IBKR) shows a T+0 line. Its absence is a significant gap.

## 3. Functional Requirements

### FR-1: T+0 Line Calculation

The system shall compute a T+0 payoff line using the **Black-Scholes model** to calculate theoretical option prices at the current moment in time for a range of underlying prices.

**Calculation approach:**

For each price point on the x-axis, the system shall:

1. For each leg in the position, calculate the theoretical option price using Black-Scholes with:
   - **Strike price (K):** from the leg data (`strike_price`)
   - **Underlying price (S):** the x-axis price point being evaluated
   - **Time to expiration (T):** calculated from today to the leg's `expiry_date`, expressed in years (calendar days / 365)
   - **Implied volatility (σ):** from the leg's current Greeks data via `smartMarketDataStore.getOptionGreeks(symbol)` → `implied_volatility`
   - **Risk-free rate (r):** use a reasonable constant (e.g., 0.05 / 5%), or optionally make it configurable in a future iteration
   - **Option type:** call or put from the leg data
2. Compute the P&L for each leg at that price point: `(theoreticalPrice - entryPrice) × quantity × 100` (long) or `(entryPrice - theoreticalPrice) × |quantity| × 100` (short)
3. Sum across all legs to get the total position T+0 P&L at that price point
4. Apply the same credit/debit adjustment logic as the expiration line (`adjustedNetCredit`)

**IV handling:**
- Use the **current implied volatility** from the Greeks data for each leg. If IV is not available for a leg, fall back to a reasonable default (e.g., average IV of other legs in the position, or 0.30 as a last resort).
- IV is assumed constant across all hypothetical underlying prices (flat vol assumption). This is a standard simplification.

### FR-2: T+0 Line Display

The T+0 line shall be rendered on the **same chart** as the existing expiration payoff line.

- **Visual style:** A distinct line that is clearly differentiable from the expiration line:
  - Use a **dashed line style** (e.g., `borderDash: [6, 3]`)
  - Use a **different color** — suggest a blue/teal color (e.g., `rgb(0, 150, 255)`) to contrast with the dark expiration line (`rgb(33, 37, 41)`)
  - Line width: 2px (same as expiration line)
  - No point markers (`pointRadius: 0`) to match the expiration line style
  - No fill/gradient beneath the T+0 line (to avoid visual clutter with the existing gradient fill on the expiration line)
- **Dataset label:** "Today (T+0)" in the Chart.js legend
- **Rendering order:** The T+0 line should render **above** the expiration line (lower `order` value) so it's visually prominent

### FR-3: Legend Update

The chart legend shall display both lines:
- "Position Payoff (N legs)" — existing expiration line (unchanged)
- "Today (T+0)" — the new T+0 line

The legend should remain in the same position ("top") and use the standard Chart.js legend behavior.

### FR-4: Crosshair / Tooltip Update

The existing custom crosshair plugin (`customCrosshair`) shall be updated to show **both** values when the user hovers over the chart:

- The tooltip box shall display:
  - Price: `$XXX` (the x-axis price being hovered)
  - Expiry P&L: `$XXX` (the existing expiration payoff value)
  - Today P&L: `$XXX` (the T+0 payoff value at the same price point)
- The crosshair shall draw **two** intersection circles on the vertical line — one on the expiration line and one on the T+0 line, each in their respective colors.

### FR-5: T+0 Data Flow & Updates

The T+0 line data shall be generated alongside the expiration payoff data:

- The `generateMultiLegPayoff()` function in `chartUtils.js` (or a new companion function) shall compute and return T+0 payoff arrays in addition to the existing expiration payoff arrays
- The T+0 data must flow through the same reactive update pipeline (debounced updates, frozen state during interaction, view state preservation)
- When IV or time data is unavailable, the chart shall gracefully show only the expiration line (current behavior) without errors

### FR-6: Handling Edge Cases

- **0 DTE (expiration day):** When time to expiration is 0 (or very close to 0), the T+0 line will converge with the expiration line. This is correct behavior — the system shall not special-case this.
- **Mixed expirations:** If legs have different expiration dates, each leg's T+0 theoretical price shall be calculated using its own time-to-expiration. The total T+0 payoff is still the sum across all legs.
- **Missing IV data:** If implied volatility is not available for any leg (e.g., Greeks subscription not yet active, market closed), the T+0 line shall not be rendered. The chart should display only the expiration line, with no errors or visual artifacts.
- **Negative time to expiration:** If an option is past expiration, treat its theoretical value as intrinsic value only (same as expiration payoff).

## 4. Non-Functional Requirements

### NFR-1: Chart Stability

The customer explicitly noted: *"We need to be very careful with the modification, once the chart is very sensitive."*

- All existing chart behaviors must be preserved: zoom, pan, debounced updates, frozen state during interaction, view state save/restore
- The T+0 line must participate in the same update lifecycle (minor updates, price-only updates, full updates) as the existing expiration line
- No new Chart.js re-renders or destructions shall be introduced by this feature — the T+0 dataset shall be added/updated alongside existing datasets

### NFR-2: Performance

- The Black-Scholes calculation for T+0 adds computation for each price point × each leg. The implementation must be efficient:
  - Pre-compute shared values (e.g., `d1`, `d2` terms, `sqrt(T)`) per leg, then iterate over price points
  - The existing price array (which can have hundreds of points for wide strategies) is the iteration domain
- The T+0 calculation should not add perceptible latency to chart rendering (<50ms for typical 4-leg strategies)

### NFR-3: Code Organization

- The Black-Scholes calculation logic shall be implemented as a **pure utility function** (e.g., in `chartUtils.js` or a new `blackScholes.js` utility file) — not embedded in component code
- The T+0 payoff generation shall follow the same patterns as the existing expiration payoff generation

## 5. Scope Boundaries — What Is NOT Included

- **T+N lines** (intermediate dates between today and expiration): Out of scope. Only T+0 (today) is needed now.
- **IV smile/skew modeling:** The T+0 line uses flat volatility (current IV per leg). No IV surface modeling.
- **Toggle to show/hide the T+0 line:** Not required in this iteration. The line is always shown when data is available.
- **Greeks display on the chart:** Showing delta, gamma, theta, vega values is not part of this request.
- **Changes to any other chart** (PositionsView, AutomationView, mobile overlay): Only the Analysis tab in the RightPanel on the OptionsTrading view is in scope. If other views use the same `PayoffChart` component and `chartUtils.js` functions, they will automatically benefit, but no view-specific changes are required elsewhere.
- **Risk-free rate configuration:** Use a hardcoded constant for now. Making it configurable is a future enhancement.

## 6. Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|--------------|
| AC-1 | When at least one option leg is selected and Greeks data (including IV) is available, the chart displays two lines: the existing expiration payoff line AND a new T+0 line | Visual inspection: select legs in options chain, open Analysis tab, confirm two distinct lines appear |
| AC-2 | The T+0 line uses a visually distinct style (different color, dashed) that is clearly distinguishable from the expiration line | Visual inspection |
| AC-3 | The T+0 line correctly represents theoretical P&L at current time using Black-Scholes pricing | Manual verification: for a simple single-leg call, compare the T+0 value at-the-money with the option's current market price minus entry price |
| AC-4 | For 0 DTE options, the T+0 line closely matches (converges with) the expiration line | Select a 0DTE option and verify the lines overlap |
| AC-5 | The custom crosshair tooltip displays both Expiry P&L and Today P&L values when hovering | Hover over the chart and verify both values appear in the tooltip |
| AC-6 | When IV data is not available for any leg, the chart displays only the expiration line with no errors | Disconnect Greeks data or test with a symbol that has no IV, verify no errors in console |
| AC-7 | All existing chart interactions (zoom, pan, reset) continue to work correctly with the T+0 line present | Zoom in/out, pan left/right, reset zoom — verify both lines respond correctly |
| AC-8 | Chart update behavior is preserved: no flickering, no unnecessary re-renders during price ticks or pan/zoom interactions | Stream live price updates and verify smooth chart behavior |
| AC-9 | The chart legend shows entries for both "Position Payoff (N legs)" and "Today (T+0)" | Visual inspection of legend |
| AC-10 | All existing frontend tests continue to pass (`npm test` in `trade-app/`) | Run test suite and verify |

## 7. Technical Context (for implementation reference)

**Key files involved:**
- `trade-app/src/utils/chartUtils.js` — `generateMultiLegPayoff()` generates payoff data; `createMultiLegChartConfig()` creates Chart.js config with datasets, custom crosshair plugin, annotations
- `trade-app/src/components/PayoffChart.vue` — Chart component with lifecycle management, debouncing, view state
- `trade-app/src/components/RightPanel.vue` — Analysis tab that hosts PayoffChart, has access to position data and Greeks via `smartMarketDataStore`
- `trade-app/src/services/smartMarketDataStore.js` — `getOptionGreeks(symbol)` returns `{ delta, gamma, theta, vega, implied_volatility, volume, timestamp }`
- `trade-app/src/views/OptionsTrading.vue` — Parent view that generates `chartData` via `generateMultiLegPayoff()` and passes it to RightPanel

**Data availability:**
- Each leg has: `symbol`, `strike_price`, `option_type` (call/put), `expiry_date`, `avg_entry_price`, `qty`
- Greeks per option symbol: `{ delta, gamma, theta, vega, implied_volatility }` — available via `smartMarketDataStore.getOptionGreeks(symbol)`
- Underlying price: available as `currentPrice` prop

**Existing chart datasets (indices matter for update logic):**
- Dataset 0: "Position Payoff (N legs)" — main expiration payoff line with gradient fill
- Dataset 1: "Zero Line" — dashed horizontal line at y=0
- Dataset 2: "Current Price" — dashed vertical line at underlying price
