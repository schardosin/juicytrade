# Phase 6 Implementation Plan — SimulationControls Layout Bugfixes

## File to Modify

- `trade-app/src/components/SimulationControls.vue` (template + CSS only; no script changes)

## Steps

### Step 1: Restructure the template into two self-contained vertical blocks (Bug 1)

In the template section of `SimulationControls.vue`:

1. **Wrap existing controls** inside each `.control-zone` in a new `.zone-controls` div so they become the top row within each block.
2. **Move the "Exp:" label** from the `.controls-info-row` into the `.date-zone` div, directly after the new `.zone-controls` wrapper. Use class `zone-info date-hint` and the existing `v-if="hasPositions && maxDateValue"` guard.
3. **Move the "Current:" / "IV unavailable" labels** from the `.controls-info-row` into the `.iv-zone` div, directly after its `.zone-controls` wrapper. Use class `zone-info iv-info` with the existing conditional guards (`ivAvailable && defaultIVValue !== null` and the `v-else-if` for unavailable).
4. **Delete the entire Row 2 block** — the `<div class="controls-info-row">` and all its children (lines 63–83).

### Step 2: Update CSS for vertical block layout (Bug 1)

1. **Change `.control-zone`** from `display: flex; align-items: center;` to `display: flex; flex-direction: column; gap: 2px; min-width: 0;` — this stacks controls on top and label below.
2. **Add `.zone-controls`** rule: `display: flex; align-items: center; gap: 4px;` — this keeps the controls in a horizontal row within the block.
3. **Add `.zone-info`** rule: `font-size: 10px; color: var(--text-tertiary, #888888); text-align: left;` — left-aligned within parent, no padding-left.
4. **Remove `.controls-info-row`** rule entirely (lines 414–418).
5. **Remove `padding-left: 60px`** from `.date-hint` (line 423).
6. **Remove `padding-left: 50px`** from `.iv-info` (line 429).
7. **Update the `@media (max-width: 900px)` block**: remove the `.controls-info-row` override (line 474–476) since that element no longer exists.
8. **Adjust `.controls-main-row` `align-items`** from `center` to `start` so that the date and IV blocks top-align when one has a label line and the other doesn't.

### Step 3: Make IV input expand to fill available space (Bug 2)

In the `.iv-input` CSS rule (lines 378–383):

1. **Change** `flex: 0 0 auto;` to `flex: 1;`.
2. **Change** `width: 60px;` to `min-width: 60px;`.
3. **Remove** `max-width: 60px;`.
4. **Keep** `overflow: hidden;`.

Result:
```css
.iv-input {
  flex: 1;
  min-width: 60px;
  overflow: hidden;
}
```

### Step 4: Verify no regressions

1. Run the full frontend test suite: `cd trade-app && npm test` — all 905+ tests must pass.
2. Confirm that `RightPanel.test.js` and `RightPanel.raceCondition.test.js` still pass (they mock `SimulationControls` as a stub, so template changes should not affect them).
3. There is no dedicated `SimulationControls.test.js` file, so no unit tests need updating.

## Acceptance Criteria

1. Date block is self-contained: controls on top, "Exp:" label below, left-aligned naturally within the block.
2. IV block is self-contained: controls on top, "Current:" label below, left-aligned naturally within the block.
3. No `padding-left` hacks used for label alignment.
4. IV input expands to fill available horizontal space (no fixed max-width).
5. Reset button remains at the far right edge, top-aligned with the control rows.
6. No regressions in existing test suite.
7. Responsive behavior preserved for narrow panels.
