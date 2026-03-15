# Phase 4 Implementation Plan — SimulationControls Visual Bug Fixes

**Target file:** `trade-app/src/components/SimulationControls.vue`

---

## Step 1: Replace PrimeVue Checkbox with Native HTML Checkbox (Bug 4)

**Goal:** Eliminate dark/invisible PrimeVue checkboxes by using the exact native HTML pattern proven in RightPanel.vue.

### Template changes (lines 91-110)

Replace both `<Checkbox>` usages in `.controls-toggles-row` with the RightPanel.vue pattern:

**Before:**
```html
<div class="toggle-item">
  <Checkbox
    v-model="showExpirationLineValue"
    inputId="exp-line"
    :binary="true"
    :disabled="!hasPositions"
  />
  <label for="exp-line" class="toggle-label">P/L at Expiration</label>
</div>
```

**After:**
```html
<div class="toggle-item">
  <div class="position-checkbox">
    <input
      type="checkbox"
      id="exp-line"
      :checked="showExpirationLineValue"
      @change="showExpirationLineValue = $event.target.checked"
      :disabled="!hasPositions"
      class="custom-checkbox"
    />
    <label for="exp-line" class="checkbox-label"></label>
  </div>
  <label for="exp-line" class="toggle-label">P/L at Expiration</label>
</div>
```

Apply the same pattern for the `theo-line` checkbox.

### Script changes (lines 116-128)

- Remove `import Checkbox from 'primevue/checkbox';` (line 119)
- Remove `Checkbox` from the `components: { ... }` registration (line 128)

### CSS changes

- Remove the PrimeVue checkbox overrides (lines 500-508):
  ```css
  :deep(.p-checkbox-box) { ... }
  :deep(.p-checkbox-box.p-highlight) { ... }
  ```

- Add the native checkbox CSS copied **exactly** from `RightPanel.vue` (lines 1361-1401):
  ```css
  .position-checkbox {
    position: relative;
    width: 20px;
    height: 20px;
  }

  .custom-checkbox {
    opacity: 0;
    position: absolute;
    width: 100%;
    height: 100%;
    cursor: pointer;
  }

  .checkbox-label {
    position: absolute;
    top: 0;
    left: 0;
    width: 16px;
    height: 16px;
    border: 2px solid var(--border-secondary, #2a2d33);
    border-radius: 3px;
    background-color: var(--bg-primary, #0b0d10);
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .custom-checkbox:checked + .checkbox-label {
    background-color: #007bff;
    border-color: #007bff;
  }

  .custom-checkbox:checked + .checkbox-label::after {
    content: "✓";
    position: absolute;
    top: -2px;
    left: 2px;
    color: white;
    font-size: 12px;
    font-weight: bold;
  }
  ```

### Verification
- Unchecked: dark background (`#0b0d10`) with visible border (`#2a2d33`)
- Checked: blue background (`#007bff`), white checkmark (✓)
- Clicking label toggles the checkbox

---

## Step 2: Replace PrimeVue Buttons with Styled Plain Buttons (Bug 2)

**Goal:** Make all nav/IV/reset buttons visible with solid dark backgrounds, matching BottomTradingPanel.vue's `.price-btn` pattern.

### Template changes

**Date nav buttons (lines 7-13 and 24-30)** — Replace PrimeVue `<Button>` with plain `<button>`:

**Before:**
```html
<Button
  icon="pi pi-chevron-left"
  class="p-button-sm p-button-text p-button-plain"
  @click="handleDecrementDate"
  :disabled="isMinDate || !hasPositions"
  v-tooltip="'Previous day'"
/>
```

**After:**
```html
<button
  class="sim-btn"
  @click="handleDecrementDate"
  :disabled="isMinDate || !hasPositions"
  title="Previous day"
>
  <i class="pi pi-chevron-left"></i>
</button>
```

Apply the same pattern for:
- Date forward button: `pi pi-chevron-right` icon, `handleIncrementDate` handler, `isMaxDate` disabled check
- IV minus button (lines 34-40): text content `-` (no icon), `handleDecrementIV` handler, `isMinIV || !ivAvailable || !hasPositions` disabled check, `ivTooltip` as `title`
- IV plus button (lines 51-57): text content `+` (no icon), `handleIncrementIV` handler, `isMaxIV || !ivAvailable || !hasPositions` disabled check, `ivTooltip` as `title`
- Reset button (lines 59-65): `pi pi-refresh` icon, `handleReset` handler, `isDefaultState || !hasPositions` disabled check, title "Reset to Defaults"

**Note:** Replace `v-tooltip` with native `title` attribute since we are removing PrimeVue Button. (Or keep `v-tooltip` if it's globally registered — check if it's a directive from PrimeVue Tooltip plugin. If globally available, keep `v-tooltip`; otherwise use `title`.)

### Script changes (lines 116-128)

- Remove `import Button from 'primevue/button';` (line 116)
- Remove `Button` from the `components: { ... }` registration (line 125)

### CSS changes

- Remove all PrimeVue button overrides (lines 493-517):
  ```css
  .control-zone :deep(.p-button.p-button-sm) { ... }
  :deep(.p-button.p-button-text) { ... }
  :deep(.p-button.p-button-text:hover) { ... }
  ```

- Remove the `.reset-btn` PrimeVue-specific rule (lines 384-389) and replace it.

- Add `.sim-btn` class matching BottomTradingPanel's `.price-btn` (lines 1357-1375):
  ```css
  .sim-btn {
    background-color: #333;
    border: 1px solid #444;
    color: #ccc;
    width: 28px;
    height: 28px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s ease;
    flex-shrink: 0;
    padding: 0;
  }

  .sim-btn:hover:not(:disabled) {
    background-color: #444;
    color: #fff;
  }

  .sim-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  ```

### Verification
- All buttons have solid dark background (#333), visible border (#444), readable text/icons (#ccc)
- Hover changes background to #444, text to #fff
- Disabled state shows reduced opacity
- The `+` button appears clearly after the IV input, not overlapping it

---

## Step 3: Fix Calendar Button Height Mismatch (Bug 1)

**Goal:** Make the calendar icon trigger button and the date input field exactly the same height.

### CSS changes

Update the existing `:deep(.p-calendar-input)` rule (lines 461-468) to add `min-height` and `box-sizing`:

```css
:deep(.p-calendar-input) {
  background-color: var(--bg-tertiary, #1a1d23) !important;
  border-color: var(--border-secondary, #2a2d33) !important;
  color: var(--text-primary, #ffffff) !important;
  height: 28px;
  min-height: 28px;
  padding: 4px 8px;
  font-size: 11px;
  box-sizing: border-box;
}
```

Update the existing `:deep(.p-datepicker-trigger)` rule (lines 488-491) to add `min-height` and `box-sizing`:

```css
:deep(.p-datepicker-trigger) {
  height: 28px !important;
  min-height: 28px !important;
  width: 28px !important;
  box-sizing: border-box !important;
}
```

### Verification
- Calendar icon button and date input field are visually the same height (28px)
- No visual gap or misalignment between the two elements

---

## Step 4: Fix Label Alignment (Bug 3)

**Goal:** Align "Exp:" and "Current:" labels with their respective input fields above them.

### CSS changes

Adjust `.date-hint` padding-left (line 401). The date zone has: zone-label ("Date") + gap(4px) + date-nav-button(28px) + gap(4px). The input starts after the zone-label and first nav button. Estimate the zone-label width (~24px for "Date") + gap(4px) + button(28px) + gap(4px) = ~60px. Fine-tune:

```css
.date-hint {
  font-size: 10px;
  color: var(--text-tertiary, #888888);
  padding-left: 60px; /* Align with calendar input start: "Date" label + nav btn + gaps */
}
```

Adjust `.iv-info` padding-left (line 407). The IV zone has: zone-label ("IV") + gap(4px) + minus-btn(28px) + gap(4px). Estimate ~18px for "IV" + 4px + 28px + 4px = ~54px:

```css
.iv-info {
  font-size: 10px;
  color: var(--text-tertiary, #888888);
  padding-left: 46px; /* Align with IV input start: "IV" label + minus btn + gaps */
}
```

**Note:** These values may need visual fine-tuning after the button changes in Step 2 since PrimeVue buttons and plain buttons may have slightly different rendered widths. Inspect in browser and adjust if needed.

### Verification
- "Exp:" label left-aligns visually with the left edge of the date Calendar input above it
- "Current:" label left-aligns visually with the left edge of the IV InputNumber above it

---

## Step 5: Run Tests, Commit, and Push

### Commands

```bash
# Run frontend tests
cd trade-app && npm test

# If tests pass, stage and commit
git add trade-app/src/components/SimulationControls.vue
git commit -m "fix(SimulationControls): replace PrimeVue Checkbox/Button with native HTML, fix calendar height and label alignment (Phase 4 bugs 1-4)"

# Push
git push
```

### Verification
- All existing tests pass (especially any SimulationControls tests)
- No console errors in browser
- Visual inspection confirms all 4 bugs are resolved

---

## Summary of All Changes to `SimulationControls.vue`

| Section | Changes |
|---|---|
| **Template** | Replace 2x `<Checkbox>` with native `<input type="checkbox">` + `<label>` in `.position-checkbox` wrapper. Replace 5x `<Button>` with plain `<button class="sim-btn">`. |
| **Script imports** | Remove `Button` and `Checkbox` imports. Remove from `components` registration. Keep `Calendar` and `InputNumber`. |
| **CSS (add)** | `.sim-btn`, `.sim-btn:hover:not(:disabled)`, `.sim-btn:disabled`, `.position-checkbox`, `.custom-checkbox`, `.checkbox-label`, `.custom-checkbox:checked + .checkbox-label`, `.custom-checkbox:checked + .checkbox-label::after` |
| **CSS (modify)** | `.p-calendar-input` add `min-height: 28px; box-sizing: border-box`. `.p-datepicker-trigger` add `min-height: 28px !important; box-sizing: border-box !important`. Adjust `.date-hint` padding-left to ~60px. Adjust `.iv-info` padding-left to ~46px. |
| **CSS (remove)** | `:deep(.p-checkbox-box)`, `:deep(.p-checkbox-box.p-highlight)`, `.control-zone :deep(.p-button.p-button-sm)`, `:deep(.p-button.p-button-text)`, `:deep(.p-button.p-button-text:hover)`, old `.reset-btn` rule |

### Files Modified
- `trade-app/src/components/SimulationControls.vue` (only file)

### Reference Files (read-only)
- `trade-app/src/components/BottomTradingPanel.vue` — `.price-btn` CSS pattern (lines 1357-1375)
- `trade-app/src/components/RightPanel.vue` — `.custom-checkbox` / `.checkbox-label` CSS (lines 1361-1401) and HTML template (lines 166-217)
