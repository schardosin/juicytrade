# Phase 3 — Bug Fixes Implementation Plan

## Overview

Three bugs reported from customer testing of the Simulation Controls. All changes are scoped to two files:
- `trade-app/src/components/SimulationControls.vue` (Bugs 1, 3)
- `trade-app/src/composables/useSimulationState.js` (Bug 2)

Tests will be added/updated in:
- `trade-app/tests/SimulationControls.test.js` (new)
- `trade-app/tests/useSimulationState.test.js` (new)

---

## Step 1: Bug 1 — Change checkbox color from orange to blue

**File:** `SimulationControls.vue` (lines 484-487)

**What to change:**
- In the `:deep(.p-checkbox-box.p-highlight)` rule, replace both `var(--color-brand, #ff6b35)` with `#007bff`
  - `background-color: #007bff !important;`
  - `border-color: #007bff !important;`
- This matches the `RightPanel.vue` pattern at lines 1388-1391 where `.custom-checkbox:checked + .checkbox-label` uses `#007bff`

**What NOT to change:**
- The unchecked state (`.p-checkbox-box`) must keep `border-color: var(--text-tertiary, #555555)` — this is the Refinement 5 fix
- Input focus borders (`.p-calendar-input:focus`, `.p-inputnumber-input:focus`) stay orange — they are not checkboxes

**Test (Step 1):**
- Write a test in `SimulationControls.test.js` that mounts the component with positions and verifies:
  - The two PrimeVue Checkbox components render
  - Checkboxes are toggleable (v-model binding works)
- Visual confirmation: checked checkboxes are blue, not orange (CSS-only — note in test comments)

---

## Step 2: Bug 2 — Fix date navigation boundary in incrementDate()

**File:** `useSimulationState.js` (lines 103-114)

**Root cause:** `incrementDate()` compares `next > max` using raw Date objects. `max` is constructed as `new Date(pos.expiry_date + 'T16:00:00')` (4pm), and `selectedDate` may carry a non-midnight time from `new Date()` initialization. When `selectedDate` is the day before expiry, adding 1 day produces a date with the same non-midnight time. The comparison `next > max` may pass, but the `isMaxDate` computed in `SimulationControls.vue` (which normalizes both to midnight) disables the button one day early because it sees the day-before as NOT max, yet incrementDate's comparison blocks the move.

The actual inconsistency: `isMaxDate` normalizes to midnight and checks `>=`, while `incrementDate` does NOT normalize and checks `>`. When `selectedDate` is the day before expiry at e.g. 10:32am, adding a day gives expiry-day at 10:32am. `max` is expiry-day at 4pm. So `next (10:32am) > max (4pm)` is false — the increment succeeds. But then `isMaxDate` normalizes both to midnight and `expiry-midnight >= expiry-midnight` is true — button correctly disables. So the bug manifests when **`selectedDate` time > 4pm** (the `T16:00:00` used in maxDate), causing `next > max` to be true and blocking the move even though we haven't reached expiry day yet.

**What to change:**
1. In `incrementDate()`, normalize both `next` and `max` to midnight before comparing:
   ```js
   const incrementDate = () => {
     const current = selectedDate.value;
     const next = new Date(current);
     next.setDate(next.getDate() + 1);
     next.setHours(0, 0, 0, 0);

     const max = maxDate.value;
     if (max) {
       const maxNormalized = new Date(max);
       maxNormalized.setHours(0, 0, 0, 0);
       if (next > maxNormalized) {
         return;
       }
     }

     selectedDate.value = next;
   };
   ```
2. In `decrementDate()`, apply the same normalization for consistency:
   ```js
   const decrementDate = () => {
     const current = selectedDate.value;
     const prev = new Date(current);
     prev.setDate(prev.getDate() - 1);
     prev.setHours(0, 0, 0, 0);

     const min = minDate.value;
     if (prev < min) {
       return;
     }

     selectedDate.value = prev;
   };
   ```
3. In `reset()`, normalize the new date to midnight:
   ```js
   const reset = () => {
     const today = new Date();
     today.setHours(0, 0, 0, 0);
     selectedDate.value = today;
     ...
   };
   ```
4. Normalize the initial `selectedDate` ref:
   ```js
   const now = new Date();
   now.setHours(0, 0, 0, 0);
   const selectedDate = ref(now);
   ```

**Test (Step 2):**
- Write tests in `useSimulationState.test.js`:
  - `incrementDate` from day-before-expiry reaches expiry day
  - `incrementDate` from expiry day does NOT advance past expiry
  - `decrementDate` from today does NOT go before today
  - `decrementDate` from day-after-today reaches today
  - `reset` sets date to today (midnight-normalized)
  - Initial `selectedDate` is midnight-normalized
- Run: `cd trade-app && npx vitest run tests/useSimulationState.test.js`

---

## Step 3: Bug 3a — Constrain component heights (oversized controls)

**File:** `SimulationControls.vue` (CSS section)

**What to change:**
- Add `:deep(.p-calendar-input)` height constraint: `height: 28px; padding: 4px 8px; font-size: 11px;`
- Add `:deep(.p-inputnumber-input)` height constraint: `height: 28px; padding: 4px 6px; font-size: 11px;`
- Add `:deep(.p-datepicker-trigger)` (calendar icon button) height constraint: `height: 28px; width: 28px;`
- Ensure nav buttons already have `height: 28px` via existing `.p-button-sm` — add explicit override if needed: `:deep(.p-button.p-button-sm) { height: 28px; width: 28px; min-width: 28px; }`

**Test (Step 3):**
- Visual test only (CSS changes); verify no regressions by running the full existing test suite: `cd trade-app && npx vitest run`

---

## Step 4: Bug 3b — Fix label alignment

**File:** `SimulationControls.vue` (CSS section)

**What to change:**
- Add explicit `line-height` and `align-self` to `.zone-label`:
  ```css
  .zone-label {
    font-size: 11px;
    font-weight: 500;
    color: var(--text-secondary, #cccccc);
    white-space: nowrap;
    line-height: 28px;
    align-self: center;
  }
  ```
- The parent `.control-zone` already has `align-items: center`, but the label's intrinsic height may not match the 28px control height. Setting `line-height: 28px` ensures vertical centering relative to the controls.

**Test (Step 4):**
- Visual test only (CSS changes); run full test suite to confirm no regressions: `cd trade-app && npx vitest run`

---

## Step 5: Bug 3c — Fix IV +/- button overlap with InputNumber

**File:** `SimulationControls.vue` (CSS section)

**What to change:**
- Constrain the IV InputNumber width so it doesn't expand into button space:
  ```css
  .iv-input {
    flex: 0 1 auto;
    min-width: 50px;
    max-width: 60px;
  }
  ```
  Change `flex: 1` to `flex: 0 1 auto` so the input doesn't grow to fill available space.
- Add explicit sizing to the IV +/- buttons to prevent them from being squeezed:
  ```css
  .iv-zone :deep(.p-button.p-button-sm) {
    min-width: 28px;
    flex-shrink: 0;
  }
  ```
- Increase `gap` in `.iv-zone` from inherited `4px` to `4px` (keep) — the overlap is caused by the input expanding, not insufficient gap.

**Test (Step 5):**
- Visual test only (CSS changes); run full test suite: `cd trade-app && npx vitest run`

---

## Step 6: Final verification

**What to do:**
1. Run the full frontend test suite: `cd trade-app && npx vitest run`
2. Confirm all existing tests pass (no regressions)
3. Confirm new tests for Steps 1 and 2 pass
4. Review all changes against the acceptance criteria in `phase3-bugfixes-requirements.md`

**Checklist:**
- [ ] Checkboxes are blue `#007bff` when checked
- [ ] Unchecked checkbox borders remain `#555555`
- [ ] Forward button reaches expiration day
- [ ] Forward button disabled ON expiration day
- [ ] Backward button disabled ON today
- [ ] Calendar picker and nav buttons have identical range
- [ ] Controls are compact (~28px height)
- [ ] Labels vertically aligned with controls
- [ ] IV +/- buttons don't overlap InputNumber
- [ ] All existing tests pass
