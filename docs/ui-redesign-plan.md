# Indicator Card Redesign Plan

Scope: `trade-app/src/components/automation/AutomationConfigForm.vue`

## Step 1 — Script: add state + helper methods

**State:**
- Add `expandedIndicators` reactive `Set` to track which indicator IDs are expanded.

**New helper methods (insert after `formatIndicatorTypeWithParams`, ~line 945):**
- `getOperatorLabel(op)` — map `'gt'`->`'>'`, `'lt'`->`'<'`, `'eq'`->`'='`, `'ne'`->`'!='`.
- `formatThreshold(indicator)` — return `indicator.threshold` formatted to 2 decimals (0 for calendar).
- `formatParamsSummary(indicator)` — join param labels + values into a short string like `"Period 14 / Factor 2.0"` (return `''` if no params).
- `toggleIndicatorExpanded(id)` — add/remove from `expandedIndicators`.
- `isIndicatorExpanded(id)` — return `expandedIndicators.has(id)`.

**Modify `addIndicator` (~line 960):**
- After pushing `newIndicator`, also call `expandedIndicators.add(newIndicator.id)` so newly added cards start expanded.

**Modify `removeIndicator` (~line 983):**
- Also call `expandedIndicators.delete(indicatorId)`.

**Update `return` block (~line 1216):**
- Expose: `expandedIndicators`, `getOperatorLabel`, `formatThreshold`, `formatParamsSummary`, `toggleIndicatorExpanded`, `isIndicatorExpanded`.

## Step 2 — Template: replace indicator-row with indicator-card

Replace lines 119-217 (the `indicators-list` + `indicator-row` v-for) with the new card structure:

```
indicators-list (keep class, add mobile-indicators logic as-is)
  indicator-card  (replaces indicator-row, add :class="{ disabled, expanded }")
    indicator-card-header  (@click="toggleIndicatorExpanded(indicator.id)")
      indicator-toggle  (InputSwitch — @click.stop to prevent toggle from toggling card)
      card-summary-name  (formatIndicatorTypeWithParams)
      card-summary-condition  ("getOperatorLabel(op) formatThreshold(ind)")
      card-summary-params  (formatParamsSummary — hidden on mobile)
      test-result-badge  (if indicatorResults[id] — reuse getIndicatorResultClass)
      card-header-actions
        test button  (pi-sync, @click.stop, desktop only)
        remove button  (pi-times, @click.stop)
      chevron  (pi-chevron-down / pi-chevron-up based on expanded state)
    indicator-card-body  (v-show="isIndicatorExpanded(indicator.id)")
      card-body-grid  (CSS grid, responsive: 2-col on desktop, 1-col mobile)
        form-field: label "Operator"  + Dropdown
        form-field: label "Threshold" + InputNumber
        form-field per dynamic param: label paramDef.label + InputNumber
        form-field: label "Symbol"    + InputText  (v-if getIndicatorNeedsSymbol)
```

Key differences from current layout:
- Header is a single clickable row showing a read-only summary.
- Body uses vertical label-above-input layout (like the rest of the form) instead of inline inputs.
- Mobile test result badge moves into the header (no separate mobile-only inline span).

## Step 3 — CSS: replace indicator-row styles with card styles

**Delete** (lines 1411-1532 and mobile overrides 1764-1863):
- `.indicators-list`, `.indicator-row`, `.indicator-row.disabled`
- `.indicator-toggle`, `.indicator-type`, `.type-label`, `.type-description`
- `.indicator-config`, `.indicator-config.dimmed`, `.operator-dropdown`, `:deep(.operator-dropdown)`, `.threshold-input`, `:deep(.threshold-input)`, `.symbol-input`, `:deep(.symbol-input)`
- `.indicator-test`, `.test-result`, `.test-result.passed/failed/stale`, `.stale-icon`
- `.indicator-remove`
- `.mobile-indicators *` rules, `.mobile-operator`, `.mobile-threshold`, `.mobile-symbol`, `.mobile-test-result*`

**Add new styles:**
- `.indicator-card` — border, border-radius, bg, margin-bottom, overflow hidden, transition.
- `.indicator-card.disabled` — opacity 0.5.
- `.indicator-card.expanded` — subtle highlight border or shadow.
- `.indicator-card-header` — flex row, align-items center, gap, padding, cursor pointer, hover bg.
  - `.card-summary-name` — font-weight medium, flex-shrink 0.
  - `.card-summary-condition` — monospace, muted color, flex-shrink 0.
  - `.card-summary-params` — font-size xs, text-tertiary, truncate with ellipsis.
  - `.test-result-badge` — reuse passed/failed/stale colors, small pill.
  - `.card-header-actions` — flex, gap, margin-left auto.
  - `.card-chevron` — transition transform 0.2s; rotated 180deg when expanded.
- `.indicator-card-body` — padding, border-top, transition (consider max-height or v-show).
  - `.card-body-grid` — `display: grid; grid-template-columns: repeat(2, 1fr); gap: var(--spacing-md);` reusing existing `.form-field` label-above-input pattern.
- **Mobile override** (`@media max-width 768px`):
  - `.card-body-grid` — single column.
  - `.card-summary-params` — display none.
  - `.indicator-card-header` — tighter padding/gap.
