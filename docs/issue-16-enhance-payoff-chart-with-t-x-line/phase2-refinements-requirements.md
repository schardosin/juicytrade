# Phase 2 Refinements — Payoff Chart Enhancement (Issue #16)

## Context & Motivation

After the Phase 1 delivery of the Payoff Chart T+X line feature, the customer tested the implementation and identified 5 visual and behavioral issues that need to be addressed before the feature is considered complete.

These refinements focus on UI layout optimization, calculation accuracy at expiration boundaries, and visual polish for the dark theme.

---

## Refinement 1: Move Simulation Controls Below the Payoff Chart

### Problem
The Simulation controls section currently appears **above** the Payoff Chart in the Analysis tab of the right panel. The customer expects it below the chart.

### Requirement
In `RightPanel.vue`, reorder the Analysis section so that:
1. **Payoff Chart** appears first (top)
2. **Simulation Controls** appears second (below the chart)
3. **Position Detail** appears third (bottom)

### Acceptance Criteria
- [ ] When the Analysis tab is open, the Payoff Chart is the first visible section
- [ ] Simulation Controls appear immediately below the Payoff Chart
- [ ] Position Detail remains at the bottom
- [ ] No functional changes — all interactions continue to work as before

---

## Refinement 2: Compact Simulation Controls Layout

### Problem
The Simulation Controls panel uses too much vertical space for just a few fields (date selector, IV input, two checkboxes, and a reset button). The vertical stack layout with large gaps wastes screen real estate.

### Requirement
Redesign the `SimulationControls.vue` layout to be more compact:
- Use a **horizontal/grid layout** where possible to reduce vertical height
- Reduce padding and gaps between elements
- Consider placing the date selector and IV adjustment side-by-side on the same row
- Place the two line toggle checkboxes on a single row (side by side)
- Make the reset button smaller and/or inline with other controls
- Remove or reduce section labels if they add unnecessary height

### Acceptance Criteria
- [ ] The Simulation Controls section takes up significantly less vertical space than before (target: ~50% reduction)
- [ ] All controls remain usable and accessible (not too cramped)
- [ ] Date selector, IV adjustment, line toggles, and reset button all remain functional
- [ ] The component remains responsive — does not overflow or clip at typical panel widths (500-700px)

### Note
This refinement should be reviewed by @ux for optimal layout decisions.

---

## Refinement 3: Verify Time Decay Accuracy Across Date Range

### Problem
The customer observes that changing the date shows very little P/L change for most days, then a "big jump" only on the last day. While this is mathematically expected behavior of Black-Scholes (theta accelerates exponentially near expiration), we should verify the implementation is correct.

### Requirement
Double-check the time-to-expiry calculation in `theoreticalPayoff.js`:
1. Verify that `calculateTimesToExpiry()` produces correct fractional-year values for each day
2. Verify the Black-Scholes implementation handles the non-linear time decay correctly
3. Add a comment in the code explaining that the "hockey stick" behavior near expiration is expected and mathematically correct (theta acceleration)
4. Consider whether using **trading days** (252/year) instead of **calendar days** (365.25/year) would produce more intuitive results — trading-day-based time calculation is the industry standard for options pricing

### Acceptance Criteria
- [ ] Time-to-expiry calculation is verified as mathematically correct
- [ ] If trading days are more appropriate, update `MS_PER_YEAR` or the calculation approach accordingly
- [ ] Code includes a comment explaining the expected theta acceleration behavior near expiration
- [ ] Existing unit tests continue to pass (update as needed if calculation method changes)

---

## Refinement 4: Theoretical Line Must Match Expiration Line on Expiry Date

### Problem
When the user selects the expiration date as the evaluation date, the P/L Theoretical line does **not** perfectly overlay the P/L at Expiration line. It should, because at expiration the theoretical value equals the intrinsic value.

### Root Cause
In `theoreticalPayoff.js`, `calculateTimesToExpiry()` creates the expiry datetime as `new Date('YYYY-MM-DDT16:00:00')` (4 PM market close), while the selected date is normalized to midnight (`setHours(0,0,0,0)`). On the expiration date itself, this leaves T ≈ 16 hours ≈ 0.00183 years — a small but non-zero time value that causes Black-Scholes to add residual time premium, preventing the theoretical line from matching the intrinsic-value expiration line.

### Requirement
When the selected date matches a leg's expiration date (same calendar day), the time-to-expiry for that leg MUST be set to exactly **0**, so Black-Scholes returns pure intrinsic value. This ensures the theoretical line converges to the expiration line on the last day.

**Implementation approach:**
In `calculateTimesToExpiry()`, after computing `msToExpiry`, add a check: if the selected date's calendar date (year, month, day) matches the expiry date's calendar date, set T = 0 for that leg.

### Acceptance Criteria
- [ ] When the evaluation date equals the expiration date, the P/L Theoretical line perfectly overlays the P/L at Expiration line
- [ ] For dates before expiration, the theoretical line continues to show time value as before
- [ ] Unit tests are added/updated to verify T=0 when selected date matches expiry date
- [ ] The behavior is visually confirmed: stepping through dates day by day, the theoretical line smoothly converges to the expiration line on the final day

---

## Refinement 5: Improve Checkbox Visibility in Dark Theme

### Problem
The line toggle checkboxes (P/L at Expiration, P/L Theoretical) are nearly invisible in the dark UI. The unchecked checkbox uses `background-color: var(--bg-tertiary, #1a1d23)` and `border-color: var(--border-secondary, #2a2d33)` — both are very dark, making the checkbox border almost invisible against the dark background.

### Requirement
Update the checkbox styling in `SimulationControls.vue` to be clearly visible in the dark theme:
- Use a **lighter border color** for unchecked state (e.g., `#555` or `var(--text-tertiary)`) so the checkbox outline is clearly visible
- Ensure the checked state remains clearly distinguishable (the current brand-orange highlight is fine)
- Apply consistent styling to any other checkboxes used in the simulation controls

### Acceptance Criteria
- [ ] Unchecked checkboxes are clearly visible against the dark background
- [ ] Checked checkboxes remain visually distinct (brand color highlight)
- [ ] The checkbox styling is consistent with the overall dark theme aesthetic
- [ ] No visual regressions on other checkbox elements in the application

---

## Scope Boundaries

- These refinements are limited to the Phase 1 features already delivered
- No new features are being added — this is polish and accuracy work
- Phase 2 features (multiple T+X lines, P/L probability cone) remain out of scope

## Implementation Order

1. **Refinement 4** (calculation fix) — highest priority, affects correctness
2. **Refinement 3** (verification) — related to #4, should be done together
3. **Refinement 1** (layout reorder) — simple change, quick win
4. **Refinement 5** (checkbox visibility) — simple CSS fix
5. **Refinement 2** (compact layout) — needs @ux review for optimal design
