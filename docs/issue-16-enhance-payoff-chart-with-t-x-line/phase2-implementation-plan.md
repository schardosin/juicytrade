# Phase 2 Implementation Plan: Payoff Chart Refinements

**Issue:** #16 - Enhance Payoff Chart with T+X line (Phase 2)
**Scope:** Refinements 1, 3, 4, and 5 only (Refinement 2 is pending UX review)
**Date:** 2026-03-15

---

## Overview

This plan addresses four Phase 2 refinements in priority order:

1. **Step 1 (Refinement 4):** Fix `calculateTimesToExpiry` to return T=0 on expiration day
2. **Step 2 (Refinement 3):** Verify time decay accuracy, add explanatory comments
3. **Step 3 (Refinement 1):** Reorder Analysis section so PayoffChart is above SimulationControls
4. **Step 4 (Refinement 5):** Improve checkbox visibility in dark theme

---

## Step 1: Fix `calculateTimesToExpiry` — T=0 on Expiration Day

**Priority:** High (correctness fix)

### Problem

In `trade-app/src/utils/theoreticalPayoff.js:37-39`, the expiry datetime is constructed as `new Date('YYYY-MM-DDT16:00:00')` (4 PM market close), while `selectedDate` is normalized to midnight via `setHours(0,0,0,0)` in `SimulationControls.vue:166`. On the expiration day itself, this produces T ≈ 16 hours ≈ 0.00183 years — a small but non-zero value that causes Black-Scholes to add residual time premium instead of returning pure intrinsic value.

### Changes

**File: `trade-app/src/utils/theoreticalPayoff.js`**

1. In `calculateTimesToExpiry()`, after computing `expiryDate` and before computing `msToExpiry`, add a calendar-day comparison:
   - Extract year, month, day from both `selectedDate` and the expiry date string.
   - If they match (same calendar day), return `T = 0` for that leg immediately.
   - Otherwise, proceed with the existing millisecond-based calculation.

2. Implementation detail — compare using the `YYYY-MM-DD` string directly against `selectedDate`'s local date components to avoid timezone drift:
   ```js
   // Inside the positions.map() callback, after parsing pos.expiry_date:
   const [expiryYear, expiryMonth, expiryDay] = pos.expiry_date.split('-').map(Number);
   const selYear = selectedDate.getFullYear();
   const selMonth = selectedDate.getMonth() + 1;
   const selDay = selectedDate.getDate();

   if (expiryYear === selYear && expiryMonth === selMonth && expiryDay === selDay) {
     return 0; // Expiration day: T=0 so Black-Scholes returns intrinsic value
   }
   ```

3. Add a JSDoc comment explaining why this check exists (theoretical line must overlay the expiration line on expiry day).

**File: `trade-app/tests/theoreticalPayoff.test.js`**

4. Add new test case in the `calculateTimesToExpiry` describe block:
   - `"returns T=0 when selected date matches expiry calendar day"` — set `selectedDate` to midnight on the expiry day, verify result is exactly `0`.

5. Add a second test case:
   - `"returns T=0 when selected date is later in the expiry day (e.g., 9 AM)"` — set `selectedDate` to 9 AM on expiry day, verify result is still `0` (the calendar-day check overrides the sub-day time difference).

6. Update the existing test `"uses 4 PM market close time for expiry"` (line 58) — this test uses a `selectedDate` of `2026-06-19T09:00:00` which is the same calendar day as the expiry `2026-06-19`. With the fix, this should now return `0` instead of a small positive number. Update the assertion accordingly.

7. Add an integration-level test in `calculateTheoreticalPayoffs`:
   - `"theoretical P/L equals intrinsic value when selected date is expiration day"` — for a simple long call at strike 500, verify that the theoretical payoff at price 520 equals `(520 - 500) * qty * 100 - entry_cost` (pure intrinsic), matching what the expiration line would show.

### Verification

```bash
cd trade-app && npm test -- --run tests/theoreticalPayoff.test.js
```

All existing tests must pass (with the one updated assertion). New tests must pass.

---

## Step 2: Verify Time Decay Accuracy & Add Explanatory Comments

**Priority:** High (related to Step 1, should be done immediately after)

### Problem

The customer observes that the P/L theoretical line shows very little change for most days, then a "big jump" on the last day. This is mathematically correct Black-Scholes behavior (theta acceleration), but we should verify the calculation and document it.

### Changes

**File: `trade-app/src/utils/theoreticalPayoff.js`**

1. Verify the `MS_PER_YEAR` constant (line 12): `365.25 * 24 * 60 * 60 * 1000` = 31,557,600,000 ms. This is correct for calendar-day-based time. Add a comment confirming this choice:
   ```js
   // Calendar-day-based time-to-expiry (365.25 days/year).
   // Industry standard for options pricing uses trading days (252/year), but calendar
   // days are acceptable for T+X simulation purposes. The difference is cosmetic:
   // calendar days spread theta more evenly across weekends, while trading days
   // concentrate it on trading days only. Both produce the same expiration-day result.
   const MS_PER_YEAR = 365.25 * 24 * 60 * 60 * 1000;
   ```

2. Add a block comment above or inside `calculateTimesToExpiry()` explaining the theta acceleration behavior:
   ```js
   // NOTE: Black-Scholes theta (time decay) is proportional to 1/sqrt(T).
   // This means time decay accelerates exponentially as expiration approaches:
   //   - At 30 DTE, daily theta is modest
   //   - At 7 DTE, daily theta roughly doubles
   //   - At 1 DTE, daily theta is very large
   // The resulting "hockey stick" shape in the P/L curve near expiration is
   // mathematically correct and expected. The theoretical line will show minimal
   // movement for most of the date range, then converge rapidly to the
   // expiration line in the final days.
   ```

3. Consider trading days vs calendar days: Add a `// TODO:` comment noting that a future enhancement could offer a trading-days mode (252 days/year) for users who prefer industry-standard time calculation. Do NOT change the calculation in this step — it would change existing behavior and require updating all test expectations.

**File: `trade-app/tests/theoreticalPayoff.test.js`**

4. Add a verification test that confirms the non-linear theta behavior:
   - `"time decay accelerates near expiration (theta verification)"` — for a single ATM call:
     - Compute theoretical P/L at 30 DTE and 29 DTE; capture the daily change.
     - Compute theoretical P/L at 2 DTE and 1 DTE; capture the daily change.
     - Assert that the 2→1 DTE daily change is significantly larger (e.g., > 2x) than the 30→29 DTE daily change.
   - This test documents and validates the expected "hockey stick" behavior.

### Verification

```bash
cd trade-app && npm test -- --run tests/theoreticalPayoff.test.js
```

All tests pass. The new theta verification test confirms expected behavior.

---

## Step 3: Reorder Analysis Section — PayoffChart Above SimulationControls

**Priority:** Medium (layout fix)

### Problem

In `trade-app/src/components/RightPanel.vue:115-269`, the Analysis section renders `SimulationControls` first, then `PayoffChart`, then `Position Detail`. The customer expects the chart on top.

### Changes

**File: `trade-app/src/components/RightPanel.vue`**

1. In the Analysis section template (lines 115-270), swap the order of the two `<RightPanelSection>` blocks so that:
   - **First:** `<RightPanelSection title="Payoff Chart">` (currently at line 129)
   - **Second:** `<RightPanelSection title="Simulation">` (currently at line 116)
   - **Third:** `<RightPanelSection title="Position Detail">` (unchanged at line 154)

2. The swap is purely a template reorder. No props, emits, data flow, or logic changes are needed. The `SimulationControls` component's `@update:modelValue` handler and the `PayoffChart`'s props remain exactly the same.

3. Resulting order in the template:
   ```html
   <!-- Analysis Section -->
   <div v-else-if="activeSection === 'analysis'" class="section-content">
     <!-- 1. Payoff Chart (moved up) -->
     <RightPanelSection title="Payoff Chart" ...>
       <PayoffChart ... />
     </RightPanelSection>

     <!-- 2. Simulation Controls (moved down) -->
     <RightPanelSection title="Simulation" ...>
       <SimulationControls ... />
     </RightPanelSection>

     <!-- 3. Position Detail (unchanged) -->
     <RightPanelSection title="Position Detail" ...>
       ...
     </RightPanelSection>
   </div>
   ```

### Verification

- Visual: Open the Analysis tab in the right panel. Payoff Chart should appear first, Simulation Controls second, Position Detail third.
- Functional: All simulation controls (date picker, IV, line toggles, reset) must continue to update the chart. Line toggle checkboxes must still show/hide chart lines.

```bash
cd trade-app && npm test -- --run tests/RightPanel.test.js
```

Existing RightPanel tests must pass. If any test asserts DOM order of Analysis section children, update accordingly.

---

## Step 4: Improve Checkbox Visibility in Dark Theme

**Priority:** Low (CSS polish)

### Problem

In `trade-app/src/components/SimulationControls.vue:437-440`, the PrimeVue checkbox `<style>` override uses:
```css
:deep(.p-checkbox-box) {
  background-color: var(--bg-tertiary, #1a1d23) !important;
  border-color: var(--border-secondary, #2a2d33) !important;
}
```

Both `#1a1d23` (background) and `#2a2d33` (border) are nearly the same dark shade, making the unchecked checkbox almost invisible against the dark panel background.

### Changes

**File: `trade-app/src/components/SimulationControls.vue`**

1. Update the unchecked checkbox border to a lighter, more visible color. Change line 439 from:
   ```css
   border-color: var(--border-secondary, #2a2d33) !important;
   ```
   to:
   ```css
   border-color: var(--text-tertiary, #555555) !important;
   ```

   This uses `--text-tertiary` (with a `#555555` fallback) which provides enough contrast against the dark background to make the checkbox outline clearly visible, without being jarring.

2. Leave the checked/highlighted state unchanged — the existing brand-orange (`--color-brand, #ff6b35`) styling at lines 442-445 is already clearly visible and should remain as-is.

3. No other checkbox elements in the application are affected since this style is scoped (`:deep()` within `<style scoped>`).

### Verification

- Visual: Open the Simulation Controls in the Analysis tab. Unchecked "P/L at Expiration" and "P/L Theoretical" checkboxes should have a clearly visible border outline against the dark background.
- Toggle both checkboxes on and off. Checked state (orange) should remain distinct.
- No regressions on other UI elements.

```bash
cd trade-app && npm run build
```

Build must succeed with no errors.

---

## Summary

| Step | Refinement | Files Modified | Type |
|------|-----------|----------------|------|
| 1 | #4 — T=0 on expiry day | `theoreticalPayoff.js`, `theoreticalPayoff.test.js` | Bug fix + tests |
| 2 | #3 — Verify time decay | `theoreticalPayoff.js`, `theoreticalPayoff.test.js` | Comments + verification test |
| 3 | #1 — Reorder Analysis section | `RightPanel.vue` | Template reorder |
| 4 | #5 — Checkbox visibility | `SimulationControls.vue` | CSS fix |

## Dependencies

- Step 2 depends on Step 1 (they modify the same file and the T=0 fix affects time decay behavior near expiration).
- Steps 3 and 4 are independent of each other and of Steps 1-2.

## Out of Scope

- **Refinement 2** (compact SimulationControls layout) — pending UX review, not included in this plan.
