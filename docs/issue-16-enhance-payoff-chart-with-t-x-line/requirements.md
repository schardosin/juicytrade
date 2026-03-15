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

This feature is inspired by TastyTrade's "Expiration Implied Volatility" control, which shows the current combined IV but allows users to modify it for scenario analysis.

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

#### FR-2.2: UI Control
A date selection control shall be added to the Payoff Chart section in the Analysis tab. Options:
- **Option A (Preferred):** A slider control with date labels
  - Slider range: 0 to days-to-expiration
  - Labels: "Today", "Expiration", and intermediate dates
  - Visual feedback: Show selected date and days remaining
- **Option B:** A date picker input
  - Min/max constraints enforced
  - Shows days remaining for selected date

#### FR-2.3: Calculation
When the user selects a date:
1. Calculate time-to-expiration for each leg as: `(selectedDate - today) + (expiryDate - selectedDate)` = time remaining from selected date to each leg's expiration
2. For legs already expired at the selected date, use intrinsic value only
3. Use Black-Scholes pricing with the adjusted time-to-expiration
4. Recalculate the theoretical P&L array

#### FR-2.4: Mixed Expirations
For positions with legs expiring on different dates (calendar spreads, diagonals):
- Each leg uses its own time-to-expiration from the selected date
- If a leg expires before the selected date, it contributes $0 to the position (closed/expired)
- The date slider maximum is the **latest** expiration date among all legs

### FR-3: Volatility Adjustment Control

The user shall be able to adjust the implied volatility used in the theoretical P&L calculation.

#### FR-3.1: Default Volatility
- **Default:** Current combined IV of the position (weighted average of all legs' IVs)
- **Label:** "Expiration Implied Volatility" (matching TastyTrade terminology)

#### FR-3.2: UI Control
A volatility adjustment control shall be added to the Payoff Chart section:
- **Control type:** Slider with percentage display
- **Range:** 10% to 200% of the default IV (or absolute range: 5% to 150% IV)
- **Step size:** 1% increments
- **Display:** Show current IV value as percentage (e.g., "IV: 28.5%")

#### FR-3.3: Calculation
When the user adjusts volatility:
1. Apply the user-selected IV to **all legs** uniformly (flat vol assumption)
2. Recalculate Black-Scholes prices with the adjusted IV
3. Recalculate the theoretical P&L array

#### FR-3.4: Per-Leg IV Override (Future Consideration)
For this iteration, a single IV adjustment applies to all legs uniformly. Per-leg IV control is out of scope but may be considered for future enhancement.

### FR-4: Combined Simulation (Date + Volatility)

The user shall be able to adjust **both** date and volatility simultaneously. The theoretical P&L line shall update to reflect:
- Time decay from the selected date
- Volatility impact from the selected IV

This combined simulation enables powerful scenario analysis:
- *"In 20 days, if IV drops to 20%, what's my P&L?"*

### FR-5: Reset to Defaults

A **"Reset"** button shall be provided to restore both controls to their default values:
- Date: Today (T+0)
- Volatility: Current combined IV

### FR-6: Visual Feedback

The controls shall provide clear visual feedback:
- **Date control:** Show selected date and days remaining (e.g., "Jan 15 (20 days)")
- **Volatility control:** Show current IV percentage (e.g., "IV: 28.5%")
- **Difference indicator:** Optionally show how far the selected values deviate from defaults (e.g., "IV: 28.5% (-5.2% from current)")

### FR-7: Data Flow & State Management

#### FR-7.1: Control State
The date and volatility selections shall be:
- **Persistent during session:** Selections persist while the user views the same position
- **Reset on position change:** When the user selects different legs or changes the underlying symbol, controls reset to defaults
- **Not persisted across sessions:** No localStorage or backend persistence required

#### FR-7.2: Reactive Updates
The theoretical P&L line shall update:
- **Immediately** when the user adjusts either control (no debounce for slider interactions)
- **Smoothly** without flickering or full chart re-render
- **Efficiently** using the existing minor update path where possible

### FR-8: Graceful Degradation

#### FR-8.1: Missing IV Data
If IV data is not available for any leg:
- The volatility control shall be **disabled** (grayed out)
- A tooltip shall explain: "IV data unavailable"
- The theoretical P&L line shall not be rendered (current behavior)

#### FR-8.2: Single Expiration Date
For positions with all legs expiring on the same date:
- The date slider maximum is that expiration date
- Normal behavior

#### FR-8.3: 0 DTE Position
For positions expiring today:
- Date slider range is 0 to 0 (effectively disabled)
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
- Black-Scholes recalculation for slider interactions must complete in **<50ms** for typical 4-leg strategies
- Slider dragging must feel responsive (no perceptible lag)
- Consider using a **debounce (100-200ms)** for slider updates to balance responsiveness and performance

### NFR-3: Code Organization
- The date/volatility calculation logic shall be implemented as **pure utility functions** in `blackScholes.js`
- The UI controls shall be implemented in a new component or as part of `RightPanel.vue`'s Analysis section
- The state management shall use Vue reactivity (refs, computed) without introducing new external state management

### NFR-4: Mobile Consideration
- The controls must be usable on mobile devices (touch-friendly slider, appropriate sizing)
- The mobile overlay chart (if applicable) should also support these controls

---

## 5. Scope Boundaries — What Is NOT Included

| Out of Scope | Rationale |
|---|---|
| Per-leg IV adjustment | Single IV control is sufficient for this iteration; per-leg control adds significant UI complexity |
| IV smile/skew modeling | Flat vol assumption is standard for this type of simulation |
- | Saving/loading simulation presets | Future enhancement if users request it |
| Historical IV data | Only current IV is used; no historical IV lookup |
| Greeks display on chart | Showing delta, gamma, theta, vega values is not part of this request |
| Risk-free rate adjustment | Use hardcoded 5% as in current implementation |
| Multiple theoretical lines | Only one theoretical line at a time; not "what-if" comparison overlays |

---

## 6. Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|--------------|
| AC-1 | The T+0 line is renamed to "P/L Theoretical" in the legend and tooltip | Visual inspection |
| AC-2 | A date selection control is visible in the Analysis tab's Payoff Chart section | Visual inspection |
| AC-3 | The date control allows selection from Today to the latest expiration date | Interact with control, verify range |
| AC-4 | Selecting a future date updates the theoretical P&L line to show P&L at that date | Select date, verify line shape changes (time decay visible) |
| AC-5 | A volatility adjustment control is visible in the Analysis tab's Payoff Chart section | Visual inspection |
| AC-6 | The volatility control shows the current combined IV as default | Verify default value matches weighted average of legs' IVs |
| AC-7 | Adjusting the volatility updates the theoretical P&L line | Adjust IV slider, verify line shape changes (vega impact visible) |
| AC-8 | Both controls can be adjusted simultaneously with correct combined effect | Adjust both, verify P&L reflects both time and vol changes |
| AC-9 | A Reset button restores both controls to defaults | Click Reset, verify controls return to T+0 and current IV |
| AC-10 | Controls reset to defaults when the user changes position/legs | Select different legs, verify controls reset |
| AC-11 | When IV data is unavailable, the volatility control is disabled and theoretical line is not shown | Test with symbol lacking IV data |
| AC-12 | All existing chart interactions (zoom, pan, crosshair) continue to work | Interact with chart, verify no regressions |
| AC-13 | Chart updates smoothly without flickering when controls are adjusted | Drag sliders, verify smooth visual updates |
| AC-14 | For 0 DTE positions, the theoretical line matches the expiration line | Select 0 DTE position, verify lines overlap |
| AC-15 | For mixed-expiration positions, the date slider maximum is the latest expiration | Select calendar spread, verify date range |
| AC-16 | All existing frontend tests continue to pass | Run `npm test` in `trade-app/` |

---

## 7. Technical Context (for implementation reference)

### 7.1 Key Files Involved

| File | Role |
|---|---|
| `trade-app/src/utils/blackScholes.js` | Black-Scholes pricing; extend to support custom date and IV |
| `trade-app/src/utils/chartUtils.js` | Chart configuration; rename T+0 dataset to P/L Theoretical |
| `trade-app/src/components/PayoffChart.vue` | Chart component; may need to accept control state as props |
| `trade-app/src/components/RightPanel.vue` | Analysis section; add date and volatility controls |
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
  → emit control changes to parent

OptionsTrading.vue
  → receives control state from RightPanel (or manages it locally)
  → updateChartData(positions, selectedDate, selectedIV)
    → generateMultiLegPayoff() → expiration payoffs (unchanged)
    → computeTheoreticalPayoffs(positions, selectedDate, selectedIV) → theoretical payoffs
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
The controls should be placed **above the payoff chart** in the Analysis tab, within the "Payoff Chart" section. Suggested layout:

```
┌─────────────────────────────────────────────────────┐
│ Payoff Chart                                    [−] │
├─────────────────────────────────────────────────────┤
│ ┌─────────────────────────────────────────────────┐ │
│ │ Date: [====●=========] Jan 28 (Today)           │ │
│ │        Today                    Expiration      │ │
│ └─────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────┐ │
│ │ IV:   [========●====] 28.5%                    │ │
│ │        10%                      150%            │ │
│ │                              [Reset]            │ │
│ └─────────────────────────────────────────────────┘ │
│ ┌─────────────────────────────────────────────────┐ │
│ │                                                 │ │
│ │            [Payoff Chart Canvas]                │ │
│ │                                                 │ │
│ └─────────────────────────────────────────────────┘ │
│ 💡 Interactive Chart: Use +/- buttons to zoom...   │
└─────────────────────────────────────────────────────┘
```

### 8.2 Slider Behavior
- **Date slider:** 
  - Linear scale from 0 to max days
  - Snap to integer days
  - Label shows formatted date and days remaining
  
- **IV slider:**
  - Linear or logarithmic scale (TBD based on typical IV ranges)
  - Snap to 0.5% or 1% increments
  - Label shows current IV percentage

### 8.3 Styling
- Use existing PrimeVue slider component if available, or custom styled range input
- Match existing dark theme colors
- Controls should be compact but touch-friendly

---

## 9. Questions for Customer

Before proceeding with implementation, please confirm:

1. **Date control format:** Would you prefer a slider (as shown above) or a date picker input? Slider provides quicker interaction; date picker allows precise date selection.

2. **IV range:** What should be the minimum and maximum IV values? Suggested: 5% to 150% absolute IV. Is this appropriate for your use cases?

3. **Control placement:** Should the controls be inside the Payoff Chart section (as proposed) or in a separate "Simulation Controls" section?

4. **Default IV display:** Should the default IV be the weighted average of all legs' IVs, or would you prefer a different calculation (e.g., ATM IV, IVx of the underlying)?

5. **Per-leg IV control:** Is the single IV control sufficient for now, or do you need per-leg IV adjustment in this iteration?

---

## 10. References

- **Previous implementation:** `docs/issue-14-enhance-payoff-chart/` — T+0 line requirements and architecture
- **TastyTrade reference:** Their "Expiration Implied Volatility" control shows combined IV and allows user adjustment
- **ThinkorSwim reference:** Their "Theoretical P&L" tool allows date and volatility simulation
