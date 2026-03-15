# Phase 3 — Bug Fixes from Customer Testing

## Context
Customer tested the Phase 2 implementation and reported 3 issues with the Simulation Controls component. All are defects in our recently-delivered code.

---

## Bug 1: Checkbox Color — Use Blue Instead of Orange

**Symptom:** The simulation checkboxes use an orange highlight (`#ff6b35`) which the customer says is "too dark inside." They want the blue checkbox style used in the position detail section.

**Root Cause:** In `SimulationControls.vue`, the PrimeVue Checkbox `:deep()` overrides use `--color-brand` (#ff6b35) for the checked state. The position detail checkboxes in `RightPanel.vue` use native HTML checkboxes with `#007bff` blue.

**Fix:**
- Change the PrimeVue Checkbox checked state color from `#ff6b35` to `#007bff` (matching position detail checkboxes)
- Update both `background-color` and `border-color` in the `.p-checkbox-box.p-highlight` override

**File:** `trade-app/src/components/SimulationControls.vue`

**Acceptance Criteria:**
- Checked checkboxes display with blue (`#007bff`) background and border
- Unchecked checkboxes remain visible with `#555555` border (Refinement 5 fix preserved)
- Visual match with position detail checkboxes in RightPanel

---

## Bug 2: Date Navigation Button Doesn't Reach Expiration Day

**Symptom:** The forward (chevron-right) navigation button allows moving only up to the day *before* expiration. The Calendar date picker correctly allows selecting the expiration day, but the button stops one day short.

**Root Cause:** In `useSimulationState.js`, `incrementDate()` creates `next` by adding 1 day to `selectedDate`. The comparison `next > max` uses raw Date objects where `max` is set to `expiryDate + 'T16:00:00'` (4pm). However, the `isMaxDate` computed property in `SimulationControls.vue` normalizes both dates to midnight for the `>=` check. The issue is likely that:
1. `selectedDate` is initialized with `new Date()` which includes current time (not midnight-normalized)
2. When `incrementDate()` adds a day, it preserves the time component
3. The boundary check in `incrementDate()` (`next > max`) and the disabled check in `isMaxDate` may produce inconsistent results due to different time normalization

**Fix:**
- In `useSimulationState.js`, normalize dates to midnight in `incrementDate()` and `decrementDate()` before comparison
- Ensure the boundary comparison in `incrementDate()` uses the same midnight-normalization as `isMaxDate` in `SimulationControls.vue`
- Both the Calendar date picker and the navigation buttons must allow reaching the expiration day

**Files:** `trade-app/src/composables/useSimulationState.js`, possibly `trade-app/src/components/SimulationControls.vue`

**Acceptance Criteria:**
- Forward button navigates all the way TO the expiration date (not just the day before)
- Forward button is disabled only WHEN already on the expiration date
- Backward button is disabled only WHEN already on today's date
- Calendar picker and navigation buttons have identical date range behavior
- Existing unit tests updated to cover this boundary case

---

## Bug 3: Layout Issues — Oversized Components, Misaligned Labels, Overlapping IV Controls

**Symptom:** The customer describes the simulation layout as "ugly" with three specific sub-issues:
1. Components appear oversized
2. Labels are not aligned with their associated components
3. The +/- navigation buttons for IV percentage overlap with the percentage InputNumber

**Root Cause:** The compact 3-column grid layout was designed in the UX spec but the PrimeVue component default sizing wasn't constrained enough. The IV InputNumber (`min-width: 50px`) doesn't account for the suffix "%" and the adjacent +/- buttons, causing overlap in the available space.

**Fix — Component Sizing:**
- Constrain PrimeVue Calendar input height to ~28-30px (matching the nav button heights)
- Constrain PrimeVue InputNumber input height to ~28-30px
- Reduce PrimeVue InputNumber input width explicitly (e.g., `max-width: 60px` or `width: 60px`) so the "%" suffix and value fit but don't expand to fill available space
- Ensure the +/- buttons for IV don't overlap the input — add explicit sizing or adjust gap

**Fix — Label Alignment:**
- Ensure zone labels ("Date", "IV") are vertically centered with their controls using `align-items: center` (already set on `.control-zone`, but may need explicit height/line-height on labels)
- Consider adding `align-self: center` on labels if flex alignment isn't working

**Fix — Overall Compactness:**
- Reduce font size on PrimeVue inputs via `:deep()` overrides (e.g., 11px or 12px)
- Ensure navigation buttons (chevron-left/right, +/-) are compact (28x28px or smaller)
- Verify the overall row height doesn't exceed ~32-34px

**File:** `trade-app/src/components/SimulationControls.vue`

**Acceptance Criteria:**
- All controls in the main row appear proportionally sized (not oversized)
- "Date" and "IV" labels are visually aligned with the center of their respective controls
- IV +/- buttons do NOT overlap with the IV percentage InputNumber
- IV InputNumber is narrow enough to show the value + "%" without wasted space
- Overall simulation controls section looks clean and compact
- No regressions in functionality — all controls still work correctly

---

## Scope
- Only `SimulationControls.vue` and `useSimulationState.js` are affected
- No changes to chart rendering, theoretical payoff calculations, or other components
- All existing tests must continue to pass
