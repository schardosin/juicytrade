# Requirements: Enhance Payoff Chart with T+X Line and Volatility Simulation

**Issue:** [#16 — Enhance Payoff Chart with T+X line](https://github.com/schardosin/juicytrade/issues/16)
**Date:** 2025-01-28
**Status:** Draft — awaiting customer approval

---

## 1. Overview

The payoff chart currently displays two lines:
1. **Expiration Payoff** — P&L at expiration (intrinsic value only)
2. **Today (T+0)** — Theoretical P&L at the current moment using Black-Scholes pricing

This enhancement transforms the static T+0 line into an interactive **P/L Theoretical** line that allows traders to:
- **Select a target date** between today and expiration to simulate how the position would behave at different points in time (T+X simulation)
- **Adjust implied volatility** to simulate how the position would perform under different volatility scenarios
- **Simulate spot price changes** to see P&L at different underlying prices

This feature follows the **TastyTrade Desktop Platform** approach for their Analysis Mode.

---

## 2. Motivation & Business Value

### 2.1 Time Decay Analysis (Theta)
Traders need to understand how their position's P&L will change over time. For example:
- A 45-day iron condor: What will the P&L look like in 20 days if the price stays the same?
- A calendar spread: How does the position behave as the near-term option approaches expiration?

The T+X date selection answers: *"If I hold this position for N more days, what's my theoretical P&L?"*

### 2.2 Volatility Scenario Analysis (Vega)
Implied volatility is a key driver of option prices. Traders need to stress-test their positions:
- *"What happens if IV drops 10% after earnings?"*
- *"How does my short straddle perform if IV spikes?"*

The volatility adjustment control answers: *"If IV changes to X%, what's my theoretical P&L?"*

### 2.3 Professional Standard
Every professional options platform (ThinkorSwim, TastyTrade, IBKR) provides these simulation controls. Their absence is a significant gap for serious options traders.

---

## 3. Functional Requirements

### FR-1: Rename T+0 Line to "P/L Theoretical"

The line currently labeled "Today (T+0)" shall be renamed to **"P/L Theoretical"** to reflect its new role as a simulation tool rather than a static "today" indicator.

- **Legend label:** "P/L Theoretical"
- **Tooltip label:** "Theoretical P&L: $XXX" (instead of "Today P&L: $XXX")
- **Crosshair circle color:** Remains blue (`rgb(0, 150, 255)`)

### FR-2: Date Selection Control (T+X Simulation)

The user shall be able to select a target date for the theoretical P&L calculation.

#### FR-2.1: Date Range
- **Minimum date:** Today (T+0)
- **Maximum date:** The latest expiration date among all legs in the position
- **Default:** Today (T+0) — maintains current behavior

#### FR-2.2: UI Control (TastyTrade Style)
A date selection control shall be added following TastyTrade's pattern:

```
┌─────────────────────────────────────────────────┐
│ Evaluate at Date                                │
│ ┌─────┐  ┌────────────────┐  ┌─────┐           │
│ │  ◀  │  │  Jan 30, 2025  │  │ 📅  │           │
│ └─────┘  └────────────────┘  └─────┘           │
│          (click to open calendar picker)        │
└─────────────────────────────────────────────────┘
```

- **Date display field:** Shows the currently selected date (e.g., "Jan 30, 2025")
- **Arrow buttons (◀/▶):** Increment/decrement the date by one day
- **Calendar icon (📅):** Opens a date picker to jump to any specific date
- **Label:** "Evaluate at Date" (matching TastyTrade terminology)

#### FR-2.3: Calculation
When the user selects a date:
1. Calculate time-to-expiration for each leg from the selected date
2. For legs already expired at the selected date, use intrinsic value only
3. Use Black-Scholes pricing with the adjusted time-to-expiration
4. Recalculate the theoretical P&L array

#### FR-2.4: Mixed Expirations
For positions with legs expiring on different dates (calendar spreads, diagonals):
- Each leg uses its own time-to-expiration from the selected date
- If a leg expires before the selected date, it contributes $0 to the position (closed/expired)
- The date picker maximum is the **latest** expiration date among all legs

### FR-3: Volatility Adjustment Control

The user shall be able to adjust the implied volatility used in the theoretical P&L calculation.

#### FR-3.1: Two IV Modes (TastyTrade Style)

**Mode 1: Expiration IV (Default)**
- Uses a single IV value for all legs in the same expiration
- Shows current expiration IV with increment/decrement buttons
- Label: "Expiration Implied Volatility"

**Mode 2: Per-Contract IV**
- Uses each option's individual IV
- Checkbox to enable: "Use per-contract IVs"
- When enabled, shows IV for each leg with individual adjustment controls

#### FR-3.2: UI Control - Expiration IV Mode (TastyTrade Style)

```
┌─────────────────────────────────────────────────┐
│ Implied Volatility                               │
│ ┌─────┐  ┌─────────────┐  ┌─────┐               │
│ │  −  │  │   37.7%     │  │  +  │               │
│ └─────┘  └─────────────┘  └─────┘               │
│          Current: 32.7% (+5.0%)                 │
│                                                 │
│ ☐ Use per-contract IVs                         │
└─────────────────────────────────────────────────┘
```

- **IV display field:** Shows the currently selected IV (e.g., "37.7%")
- **Increment button (+):** Increases IV by 1% per click
- **Decrement button (−):** Decreases IV by 1% per click
- **Delta indicator:** Shows difference from current IV (e.g., "+5.0%")
- **Current IV label:** Shows the actual market IV (e.g., "Current: 32.7%")
- **Per-contract checkbox:** Switches to per-leg IV mode

#### FR-3.3: UI Control - Per-Contract IV Mode

When "Use per-contract IVs" is checked:

```
┌─────────────────────────────────────────────────┐
│ Implied Volatility                               │
│ ☑ Use per-contract IVs                         │
│                                                 │
│ NFLX Feb 15 600 Call    IV: 35.2%  [−][+]      │
│ NFLX Feb 15 620 Call    IV: 32.1%  [−][+]      │
└─────────────────────────────────────────────────┘
```

- Each leg shows its own IV with individual increment/decrement buttons
- Default values are the actual IV for each option

#### FR-3.4: IV Range
- **Minimum:** 5% absolute IV
- **Maximum:** 200% absolute IV
- **Step size:** 1% increments

#### FR-3.5: Default IV Calculation
- **Expiration IV mode:** Weighted average of all legs' IVs, weighted by absolute delta or notional
- **Per-contract IV mode:** Each leg uses its own market IV as default

### FR-4: Spot Price Simulation (TastyTrade Style)

The user shall be able to simulate the P&L at a different underlying price.

#### FR-4.1: UI Control

```
┌─────────────────────────────────────────────────┐
│ Theo Spot Price                                  │
│ ┌─────┐  ┌─────────────┐  ┌─────┐               │
│ │  −  │  │   $580.00   │  │  +  │               │
│ └─────┘  └─────────────┘  └─────┘               │
│          Current: $565.50                       │
└─────────────────────────────────────────────────┘
```

- **Price display field:** Shows the theoretical spot price
- **Increment/decrement buttons:** Adjust price by $1 (or appropriate increment for the underlying)
- **Current price label:** Shows the actual market price
- **Crosshair sync:** When adjusted, the chart crosshair moves to the theoretical price position

#### FR-4.2: Behavior
- Adjusting the spot price shows the theoretical P&L at that price
- The P/L Theoretical line updates to show P&L across all prices, with a marker at the theoretical spot
- The header/metrics area shows the P&L at the theoretical spot price

### FR-5: Line Visibility Toggles

The user shall be able to toggle the visibility of each P/L line independently.

#### FR-5.1: Toggle Controls

```
┌─────────────────────────────────────────────────┐
│ Lines                                            │
│ ☑ P/L at Expiration                             │
│ ☑ P/L Theoretical                               │
└─────────────────────────────────────────────────┘
```

- **P/L at Expiration toggle:** Show/hide the expiration payoff line (green)
- **P/L Theoretical toggle:** Show/hide the theoretical payoff line (blue)
- Both are enabled by default

#### FR-5.2: Zone Display Toggle
- **"At Expiration" zones:** Green/red areas based on expiration P/L
- **"Theo" zones:** Green/red areas based on theoretical P/L
- User can switch between zone display modes

### FR-6: Greek Overlay (Optional Enhancement)

The user may optionally display Greek values as an overlay on the chart.

#### FR-6.1: Greek Selector

```
┌─────────────────────────────────────────────────┐
│ Greek Overlay                                    │
│ [No Greek ▼]                                    │
│                                                 │
│ Options: None, Delta, Gamma, Theta, Vega        │
└─────────────────────────────────────────────────┘
```

- **Dropdown selector:** Choose which Greek to display
- **Options:** None (default), Delta, Gamma, Theta, Vega
- **Display:** Purple line showing the selected Greek across price points

### FR-7: Combined Simulation

All controls work together for comprehensive scenario analysis:
- Date + IV + Spot Price can all be adjusted simultaneously
- The theoretical P&L line reflects all adjustments
- Example: *"In 20 days, if IV drops to 25% and price moves to $580, what's my P&L?"*

### FR-8: Reset to Defaults

A **"Reset"** button shall restore all controls to their default values:
- Date: Today (T+0)
- IV: Current market IV (expiration mode)
- Spot Price: Current market price

### FR-9: Data Flow & State Management

#### FR-9.1: Control State
The simulation controls state shall be:
- **Persistent during session:** Selections persist while the user views the same position
- **Reset on position change:** When the user selects different legs or changes the underlying symbol, controls reset to defaults
- **Not persisted across sessions:** No localStorage or backend persistence required

#### FR-9.2: Reactive Updates
The theoretical P&L line shall update:
- **Immediately** when the user adjusts any control
- **Smoothly** without flickering or full chart re-render
- **Efficiently** using the existing minor update path where possible

### FR-10: Graceful Degradation

#### FR-10.1: Missing IV Data
If IV data is not available for any leg:
- The volatility control shall be **disabled** (grayed out)
- A tooltip shall explain: "IV data unavailable"
- The theoretical P&L line shall not be rendered (current behavior)

#### FR-10.2: Single Expiration Date
For positions with all legs expiring on the same date:
- The date picker maximum is that expiration date
- Normal behavior

#### FR-10.3: 0 DTE Position
For positions expiring today:
- Date picker range is 0 to 0 (effectively disabled)
- Volatility control remains functional
- Theoretical P&L converges with expiration P&L

---

## 4. Non-Functional Requirements

### NFR-1: Chart Stability
The customer previously noted: *"We need to be very careful with the modification, once the chart is very sensitive."*

- All existing chart behaviors must be preserved: zoom, pan, debounced updates, frozen state during interaction, view state save/restore
- The theoretical P&L line must participate in the same update lifecycle as the expiration line
- No new Chart.js re-renders or destructions shall be introduced — the dataset shall be updated in place

### NFR-2: Performance
- Black-Scholes recalculation for control adjustments must complete in **<50ms** for typical 4-leg strategies
- Button clicks must feel responsive (no perceptible lag)
- Consider using a **debounce (100-150ms)** for rapid adjustments

### NFR-3: Code Organization
- The date/volatility/spot calculation logic shall be implemented as **pure utility functions** in `blackScholes.js`
- The UI controls shall be implemented in a new component or as part of `RightPanel.vue`'s Analysis section
- The state management shall use Vue reactivity (refs, computed) without introducing new external state management

### NFR-4: Mobile Consideration
- The controls must be usable on mobile devices (touch-friendly buttons, appropriate sizing)
- The mobile overlay chart (if applicable) should also support these controls

---

## 5. Scope Boundaries — What Is NOT Included

| Out of Scope | Rationale |
|---|---|
| IV smile/skew modeling | Flat vol assumption is standard for this type of simulation |
| Saving/loading simulation presets | Future enhancement if users request it |
| Historical IV data | Only current IV is used; no historical IV lookup |
| Risk-free rate adjustment | Use hardcoded 5% as in current implementation |
| Multiple theoretical lines | Only one theoretical line at a time; not "what-if" comparison overlays |
| Dividend yield adjustment | Not part of this request |
| Adding legs in analysis mode | TastyTrade allows adding new legs for analysis; out of scope for now |

---

## 6. Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|--------------|
| AC-1 | The T+0 line is renamed to "P/L Theoretical" in the legend and tooltip | Visual inspection |
| AC-2 | A date selection control is visible in the Analysis tab with calendar picker and arrow buttons | Visual inspection |
| AC-3 | The date control allows selection from Today to the latest expiration date | Interact with control, verify range |
| AC-4 | Selecting a future date updates the theoretical P&L line to show P&L at that date | Select date, verify line shape changes (time decay visible) |
| AC-5 | An IV adjustment control is visible with increment/decrement buttons | Visual inspection |
| AC-6 | The IV control shows the current expiration IV as default | Verify default value matches weighted average of legs' IVs |
| AC-7 | Adjusting the IV updates the theoretical P&L line | Adjust IV, verify line shape changes (vega impact visible) |
| AC-8 | "Use per-contract IVs" checkbox enables per-leg IV adjustment | Enable checkbox, verify individual leg IV controls appear |
| AC-9 | A spot price simulation control is visible with increment/decrement buttons | Visual inspection |
| AC-10 | Adjusting the spot price shows P&L at the theoretical price | Adjust spot price, verify P&L updates |
| AC-11 | Line visibility toggles work for both P/L at Expiration and P/L Theoretical | Toggle each line, verify visibility changes |
| AC-12 | All controls can be adjusted simultaneously with correct combined effect | Adjust all, verify P&L reflects all changes |
| AC-13 | A Reset button restores all controls to defaults | Click Reset, verify controls return to defaults |
| AC-14 | Controls reset to defaults when the user changes position/legs | Select different legs, verify controls reset |
| AC-15 | When IV data is unavailable, the volatility control is disabled | Test with symbol lacking IV data |
| AC-16 | All existing chart interactions (zoom, pan, crosshair) continue to work | Interact with chart, verify no regressions |
| AC-17 | Chart updates smoothly without flickering when controls are adjusted | Click buttons, verify smooth visual updates |
| AC-18 | For 0 DTE positions, the theoretical line matches the expiration line | Select 0 DTE position, verify lines overlap |
| AC-19 | For mixed-expiration positions, the date picker maximum is the latest expiration | Select calendar spread, verify date range |
| AC-20 | All existing frontend tests continue to pass | Run `npm test` in `trade-app/` |

---

## 7. Technical Context (for implementation reference)

### 7.1 Key Files Involved

| File | Role |
|---|---|
| `trade-app/src/utils/blackScholes.js` | Black-Scholes pricing; extend to support custom date, IV, and spot price |
| `trade-app/src/utils/chartUtils.js` | Chart configuration; rename T+0 dataset to P/L Theoretical |
| `trade-app/src/components/PayoffChart.vue` | Chart component; may need to accept control state as props |
| `trade-app/src/components/RightPanel.vue` | Analysis section; add simulation controls |
| `trade-app/src/views/OptionsTrading.vue` | Data flow; may need to pass control state through chartData |
| `trade-app/src/services/smartMarketDataStore.js` | Greeks data; already provides IV per option symbol |

### 7.2 Current Data Flow

```
OptionsTrading.vue
  → updateChartData()
    → generateMultiLegPayoff() → expiration payoffs
    → computeT0ForPositions() → T+0 payoffs (uses current IV, current date)
  → chartData = { prices, payoffs, t0Payoffs, ... }
  → RightPanel.vue (passes chartData as prop)
    → PayoffChart.vue (renders chart)
```

### 7.3 Proposed Data Flow with Controls

```
RightPanel.vue (new state)
  → selectedDate = ref(today)
  → selectedIV = ref(currentCombinedIV)
  → selectedSpotPrice = ref(currentSpotPrice)
  → usePerContractIV = ref(false)
  → perContractIVs = ref({})
  → showExpirationLine = ref(true)
  → showTheoreticalLine = ref(true)
  → emit control changes to parent

OptionsTrading.vue
  → receives control state from RightPanel (or manages it locally)
  → updateChartData(positions, simulationState)
    → generateMultiLegPayoff() → expiration payoffs (unchanged)
    → computeTheoreticalPayoffs(positions, selectedDate, selectedIV, selectedSpotPrice) → theoretical payoffs
  → chartData = { prices, payoffs, t0Payoffs: theoreticalPayoffs, ... }
  → RightPanel.vue
    → PayoffChart.vue
```

### 7.4 IV Calculation

**Current combined IV (weighted average):**
```javascript
const legs = optionPositions.map(pos => ({
  iv: getOptionGreeks(pos.symbol)?.value?.implied_volatility,
  weight: Math.abs(pos.qty) * pos.strike_price // or other weighting
}));

const combinedIV = legs.reduce((sum, leg) => sum + leg.iv * leg.weight, 0) 
                 / legs.reduce((sum, leg) => sum + leg.weight, 0);
```

### 7.5 Date-to-Time Calculation

```javascript
const now = new Date();
const selectedDate = new Date(userSelectedDate);
const timeToSelectedDate = (selectedDate - now) / (365.25 * 24 * 60 * 60 * 1000); // years

// For each leg:
const legExpiry = new Date(pos.expiry_date + "T16:00:00");
const timeFromSelectedToExpiry = (legExpiry - selectedDate) / (365.25 * 24 * 60 * 60 * 1000);
const T = Math.max(timeFromSelectedToExpiry, 0); // 0 if expired at selected date
```

---

## 8. UI/UX Design Notes

### 8.1 Control Placement
The controls should be placed in the **right panel** within the Analysis section, above the payoff chart. Following TastyTrade's layout:

```
┌─────────────────────────────────────────────────────┐
│ Analysis Mode                                        │
├─────────────────────────────────────────────────────┤
│ Position: NFLX Vertical Call Spread                 │
│ P/L Open: -$23.00                                   │
├─────────────────────────────────────────────────────┤
│ ┌─────────────────────────────────────────────────┐ │
│ │ Evaluate at Date                                │ │
│ │  [◀]  [  Jan 30, 2025  ]  [📅]                 │ │
│ └─────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────┐ │
│ │ Implied Volatility                              │ │
│ │  [−]  [   37.7%   ]  [+]                        │ │
│ │  Current: 32.7% (+5.0%)                         │ │
│ │  ☐ Use per-contract IVs                         │ │
│ └─────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────┐ │
│ │ Theo Spot Price                                 │ │
│ │  [−]  [   $580.00   ]  [+]                      │ │
│ │  Current: $565.50                               │ │
│ └─────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────┐ │
│ │ Lines                                           │ │
│ │  ☑ P/L at Expiration   ☑ P/L Theoretical       │ │
│ └─────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────┐ │
│ │ Greek Overlay: [No Greek ▼]                    │ │
│ └─────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────┐ │
│ │            [Reset to Defaults]                  │ │
│ └─────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────┐ │
│ │                                                 │ │
│ │            [Payoff Chart Canvas]                │ │
│ │                                                 │ │
│ └─────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

### 8.2 Control Styling
- Use PrimeVue InputNumber component for numeric inputs with increment/decrement buttons
- Use PrimeVue Calendar component for date picker
- Use PrimeVue Checkbox for toggles
- Use PrimeVue Dropdown for Greek selector
- Match existing dark theme colors from `theme.css`

### 8.3 Button Behavior
- **Date arrows:** Increment/decrement by 1 day
- **IV buttons:** Increment/decrement by 1% absolute IV
- **Spot price buttons:** Increment/decrement by appropriate amount (e.g., $1 for stocks, $0.01 for some underlyings)

---

## 9. Implementation Phases

### Phase 1: Core Controls (MVP)
- FR-1: Rename T+0 to P/L Theoretical
- FR-2: Date selection control
- FR-3.1-3.2: Expiration IV mode only
- FR-5: Line visibility toggles
- FR-8: Reset button

### Phase 2: Enhanced Features
- FR-3.3: Per-contract IV mode
- FR-4: Spot price simulation
- FR-6: Greek overlay (optional)

### Phase 3: Polish
- Performance optimization
- Mobile responsiveness
- Additional edge case handling

---

## 10. References

- **Previous implementation:** `docs/issue-14-enhance-payoff-chart/` — T+0 line requirements and architecture
- **TastyTrade Desktop Platform:** Analysis Mode with P/L Theo, IV adjustment, and date simulation
- **YouTube - "Analyzing Theoretical Scenarios on Open Positions":** https://www.youtube.com/watch?v=nnshba7E-Bk
- **YouTube - "Analysis Mode Overview (tastytrade Desktop Platform)":** https://www.youtube.com/watch?v=AF3uy8cHPs0
