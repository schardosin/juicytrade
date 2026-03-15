# UX Design Spec — Compact Simulation Controls Layout

**Task:** Refinement 2 — Reduce vertical footprint of `SimulationControls.vue` by ~50%
**Author:** @ux
**Status:** Ready for implementation

---

## 1. Current State Analysis

The current layout stacks 4 groups vertically with generous spacing:

```
┌─────────────────────────────────────────────────────┐
│ EVALUATE AT DATE                          (label)   │  ~11px
│ ◀  [ Calendar date picker         📅 ]  ▶          │  ~36px
│ Expires: Jun 20, 2025                    (hint)     │  ~14px
│                                          (gap 16px) │
│ IMPLIED VOLATILITY                       (label)    │  ~11px
│ -  [ 32 %                            ]  +           │  ~36px
│ Current: 32.5% (+2.0%)                  (info)      │  ~14px
│                                          (gap 16px) │
│ LINES                                    (label)    │  ~11px
│ ☐ P/L at Expiration                                 │  ~20px
│ ☐ P/L Theoretical                       (gap 6px)  │  ~20px
│                                          (gap 16px) │
│ [ 🔄 Reset to Defaults               ] (full-width) │  ~36px
└─────────────────────────────────────────────────────┘
  Estimated total height: ~240px (content) + 12px padding top/bottom = ~264px
```

**Problems:** Each control group has a separate uppercase label row. The two checkboxes stack vertically. The reset button is full-width. The 16px gaps between groups add up. All of this produces a tall panel for just 4 interactive elements.

---

## 2. Compact Layout Design

### Design Principles
- **Inline labels** replace stacked labels — put labels on the same row as their controls
- **Two-column grid** for the date and IV rows — they're independent controls of similar width
- **Single row** for both checkboxes + reset button
- **Reduce gaps** from 16px to 8px between rows
- **Reduce padding** from 12px to 8px

### Target Layout (ASCII)

```
┌──────────────────────────────────────────────────────────────────┐
│  Date  ◀ [ Jun 14, 2025    📅 ] ▶   IV  - [ 32% ] +    [ ↺ ]  │
│         Exp: Jun 20, 2025             Current: 32.5% (+2.0%)    │
│  ☐ P/L at Expiration    ☐ P/L Theoretical                      │
└──────────────────────────────────────────────────────────────────┘
  Estimated total height: ~90px (content) + 8px padding top/bottom = ~106px
  Reduction: ~60%
```

### Row-by-Row Specification

#### Row 1: Date + IV + Reset (primary controls row)

This is the main row. Use **CSS Grid** with three zones:

```
grid-template-columns: 1fr 1fr auto;
gap: 8px;
align-items: center;
```

| Zone | Content | Details |
|------|---------|---------|
| **Date zone** | `Date` label + `◀` button + Calendar picker + `▶` button | Label is inline, not above. Flex row internally. |
| **IV zone** | `IV` label + `-` button + InputNumber + `+` button | Same pattern as Date zone. Flex row internally. |
| **Reset zone** | Icon-only reset button `↺` | `pi-refresh` icon, `p-button-sm p-button-text`, no label text. Tooltip: "Reset to Defaults". |

**Zone internals (flex rows):**

Each of the Date and IV zones is a flex row:
```
display: flex;
align-items: center;
gap: 4px;
```

- **Inline label:** The text "Date" / "IV" is rendered as a `<span>` (not a `<label>` above).
  - Font: `11px`, `font-weight: 500`, `color: var(--text-secondary, #cccccc)`
  - NOT uppercase (inline labels read better in sentence case at this size)
  - `white-space: nowrap; min-width: fit-content;`
- **Nav buttons (◀ ▶ and - +):** Keep existing `p-button-sm p-button-text p-button-plain` styling. Use `padding: 0 4px` to make them narrower.
- **Calendar picker:** `flex: 1; min-width: 120px;`. Keep existing `:deep` styling.
- **InputNumber:** `flex: 1; min-width: 60px;`. Keep existing `:deep` styling. Suffix `%` stays.
- **Reset button:** Icon-only (`pi-refresh`), class `p-button-sm p-button-text p-button-rounded`. Uses `width: 28px; height: 28px;`. Disabled state when `isDefaultState` — same logic as current. Add `v-tooltip="'Reset to Defaults'"`.

#### Row 2: Hint/Info text row

Below the controls, show the contextual hint texts in a two-column layout matching Row 1's grid:

```
display: grid;
grid-template-columns: 1fr 1fr auto;
gap: 8px;
```

| Zone | Content | Details |
|------|---------|---------|
| **Date hint** | "Exp: Jun 20, 2025" | Shortened from "Expires:" to "Exp:" to save space. Font: `10px`, `color: var(--text-tertiary, #888888)`. `padding-left` should align roughly with the Calendar input (after the label + prev button). |
| **IV info** | "Current: 32.5% (+2.0%)" or "IV data unavailable" | Same font/color as current. Keep the green/red delta coloring. |
| **Reset spacer** | Empty | Keeps grid alignment with Row 1. |

**Conditional rendering:** Only show Row 2 if `hasPositions` is true AND there's something to display (expiry date exists or IV info exists). If neither hint has content, omit the entire row to save space.

#### Row 3: Line toggles

Both checkboxes on a single horizontal row:

```
display: flex;
align-items: center;
gap: 16px;    /* space between the two checkbox groups */
```

Each checkbox group is:
```
display: flex;
align-items: center;
gap: 6px;     /* space between checkbox and its label */
```

- Checkbox: Keep existing PrimeVue `<Checkbox>` with `:binary="true"`. The improved `#555555` border from Refinement 5 is already applied.
- Label: `12px`, `color: var(--text-primary, #ffffff)`, `cursor: pointer`. Clickable via `for` attribute.
- The 16px gap between the two checkbox groups provides clear visual separation without needing a section label.

**No "Lines" label.** The checkboxes are self-explanatory with their text labels. Removing the section header saves one row.

---

## 3. Container Styling Changes

Update the `.simulation-controls` root class:

```css
.simulation-controls {
  display: flex;
  flex-direction: column;
  gap: 6px;                            /* was 16px */
  padding: 8px 12px;                   /* was 12px all sides */
  background-color: var(--bg-primary, #0b0d10);
  border-radius: var(--radius-md, 6px);
}
```

**Key change:** Gap reduced from 16px → 6px. Padding top/bottom from 12px → 8px. Horizontal padding stays 12px for alignment with the RightPanelSection header content.

---

## 4. Responsive Behavior

The panel content area varies between ~340px (at 400px panel width minus 60px icon menu) and ~640px.

| Width | Behavior |
|-------|----------|
| **≥ 500px** | Full 3-column grid layout as described above. All controls on one row. |
| **< 500px** | The date and IV zones may feel cramped. Add a breakpoint: stack the date and IV zones vertically (each takes full width), keep reset button at the end of the IV row. Checkboxes stay horizontal. |

**Implementation:** Use a CSS media query or, more practically, a container-width approach. Since the panel width is controlled by the parent, a simple approach is:

```css
@media (max-width: 900px) {
  /* At 900px viewport, RightPanel is 400px, content is ~340px */
  .controls-main-row {
    grid-template-columns: 1fr auto;  /* IV drops to second row */
  }
}
```

At the smallest breakpoint, the layout becomes:
```
┌─────────────────────────────────────┐
│ Date ◀ [ Jun 14, 2025  📅 ] ▶ [ ↺ ]│
│ IV   - [ 32% ] +                    │
│ Exp: Jun 20      Current: 32.5%     │
│ ☐ P/L at Expiration  ☐ P/L Theo... │
└─────────────────────────────────────┘
```

This is still compact (~120px) — acceptable for narrow panels.

---

## 5. Component Structure (Template Guidance)

Replace the current 4 `control-group` divs with this structure:

```html
<div class="simulation-controls">

  <!-- Row 1: Main controls -->
  <div class="controls-main-row">
    <!-- Date zone -->
    <div class="control-zone date-zone">
      <span class="zone-label">Date</span>
      <Button icon="pi pi-chevron-left" ... />
      <Calendar ... class="date-calendar" />
      <Button icon="pi pi-chevron-right" ... />
    </div>
    <!-- IV zone -->
    <div class="control-zone iv-zone">
      <span class="zone-label">IV</span>
      <Button label="-" ... />
      <InputNumber ... class="iv-input" />
      <Button label="+" ... />
    </div>
    <!-- Reset button -->
    <Button icon="pi pi-refresh"
            class="p-button-sm p-button-text p-button-rounded reset-btn"
            v-tooltip="'Reset to Defaults'"
            ... />
  </div>

  <!-- Row 2: Hint/info text (conditional) -->
  <div v-if="hasPositions && (maxDateValue || ivAvailable)" class="controls-info-row">
    <div class="date-hint">
      <span v-if="maxDateValue">Exp: {{ formatDate(maxDateValue) }}</span>
    </div>
    <div class="iv-info">
      <span v-if="ivAvailable && defaultIVValue !== null">
        Current: {{ formatPercent(defaultIVValue) }}
        <span v-if="ivDeltaValue !== 0" :class="...">
          ({{ ivDeltaValue > 0 ? '+' : '' }}{{ ivDeltaValue.toFixed(1) }}%)
        </span>
      </span>
      <span v-else class="iv-unavailable">IV data unavailable</span>
    </div>
    <div></div> <!-- grid spacer for reset column -->
  </div>

  <!-- Row 3: Line toggles -->
  <div class="controls-toggles-row">
    <div class="toggle-item">
      <Checkbox v-model="showExpirationLineValue" inputId="exp-line" :binary="true" ... />
      <label for="exp-line" class="toggle-label">P/L at Expiration</label>
    </div>
    <div class="toggle-item">
      <Checkbox v-model="showTheoreticalLineValue" inputId="theo-line" :binary="true" ... />
      <label for="theo-line" class="toggle-label">P/L Theoretical</label>
    </div>
  </div>

</div>
```

---

## 6. CSS Specification

```css
/* Container */
.simulation-controls {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 12px;
  background-color: var(--bg-primary, #0b0d10);
  border-radius: var(--radius-md, 6px);
}

/* Row 1: Main controls - 3-column grid */
.controls-main-row {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 8px;
  align-items: center;
}

/* Each control zone: inline flex row */
.control-zone {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;  /* allow flex shrinking */
}

/* Inline zone labels */
.zone-label {
  font-size: 11px;
  font-weight: 500;
  color: var(--text-secondary, #cccccc);
  white-space: nowrap;
}

/* Calendar and IV input sizing */
.date-calendar {
  flex: 1;
  min-width: 110px;
}

.iv-input {
  flex: 1;
  min-width: 50px;
}

/* Reset button - icon only, compact */
.reset-btn {
  width: 28px;
  height: 28px;
  padding: 0;
  flex-shrink: 0;
}

/* Row 2: Info/hint text - matches grid alignment */
.controls-info-row {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 8px;
  padding-left: 0;
}

.date-hint {
  font-size: 10px;
  color: var(--text-tertiary, #888888);
  padding-left: 32px;  /* align with calendar input (past label + prev button) */
}

.iv-info {
  font-size: 10px;
  color: var(--text-tertiary, #888888);
  padding-left: 20px;  /* align with IV input (past label + minus button) */
}

/* Row 3: Toggle checkboxes - horizontal */
.controls-toggles-row {
  display: flex;
  align-items: center;
  gap: 16px;
  padding-top: 2px;
}

.toggle-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.toggle-label {
  font-size: 12px;
  color: var(--text-primary, #ffffff);
  cursor: pointer;
  white-space: nowrap;
}

/* Responsive: narrow panels */
@media (max-width: 900px) {
  .controls-main-row {
    grid-template-columns: 1fr auto;
  }
  .iv-zone {
    grid-column: 1;
  }
  .reset-btn {
    grid-row: 1;
    grid-column: 2;
  }
  .controls-info-row {
    grid-template-columns: 1fr 1fr;
  }
}
```

---

## 7. Interaction & State Notes

| Aspect | Specification |
|--------|--------------|
| **Disabled state** | All controls disabled when `!hasPositions`. IV controls also disabled when `!ivAvailable`. Same logic as current — no changes. |
| **Reset button** | Disabled when `isDefaultState`. Tooltip always shows "Reset to Defaults". Becomes visually muted (inherits `p-button-text` disabled style). |
| **Tooltips** | Keep existing tooltips on nav buttons ("Previous day", "Next day") and IV buttons (ivTooltip). Add tooltip to reset button. |
| **Calendar popup** | The Calendar popup opens downward by default. Since the controls are now below the chart, the popup has room. No positioning changes needed. |
| **Keyboard navigation** | No changes to keyboard behavior. Tab order follows DOM order: Date prev → Calendar → Date next → IV minus → IV input → IV plus → Reset → Checkbox 1 → Checkbox 2. |

---

## 8. What NOT to Change

- **No changes to the RightPanelSection wrapper** — the collapsible "Simulation" header stays as-is
- **No changes to any JavaScript logic** — all computed properties, watchers, emit behavior remain identical
- **No changes to the PrimeVue deep styles** for Calendar input, InputNumber input, Checkbox box, and Button variants — keep the existing `:deep()` overrides (including the Refinement 5 checkbox fix)
- **No changes to props, emits, or the composable integration** — this is purely a template + CSS restructuring
- **No changes to PayoffChart, RightPanel, or any other component**

---

## 9. Height Budget Comparison

| Element | Current Height | Compact Height |
|---------|---------------|----------------|
| Container padding (top) | 12px | 8px |
| "EVALUATE AT DATE" label | 11px | 0px (inline) |
| Gap after label | 8px | 0px |
| Date control row | 36px | 32px (shared row) |
| Date hint text | 14px | 14px |
| Gap between groups | 16px | 6px |
| "IMPLIED VOLATILITY" label | 11px | 0px (inline) |
| Gap after label | 8px | 0px |
| IV control row | 36px | 0px (shared row with date) |
| IV info text | 14px | 0px (shared row with date hint) |
| Gap between groups | 16px | 6px |
| "LINES" label | 11px | 0px (removed) |
| Gap after label | 6px | 0px |
| Checkbox row 1 | 20px | 20px (shared row) |
| Checkbox row 2 | 20px | 0px (shared row) |
| Gap before reset | 16px + 8px | 0px (removed) |
| Reset button row | 36px | 0px (inline with Row 1) |
| Container padding (bottom) | 12px | 8px |
| **Total** | **~302px** | **~94px** |

**Reduction: ~69%** — exceeds the 50% target.

---

## 10. Visual Reference (Proportioned ASCII)

**At full width (~640px content area):**
```
┌────────────────────────────────────────────────────────────────┐
│ Date ◀ Jun 14, 2025    📅  ▶ │ IV  - [ 32% ] +  │  ↺         │
│       Exp: Jun 20, 2025       │ Current: 32.5% (+2.0%)        │
│ ☐ P/L at Expiration           ☐ P/L Theoretical               │
└────────────────────────────────────────────────────────────────┘
```

**At narrow width (~340px content area):**
```
┌──────────────────────────────────┐
│ Date ◀ Jun 14, 2025 📅 ▶   [ ↺ ]│
│ IV   - [ 32% ] +                 │
│ Exp: Jun 20     Current: 32.5%   │
│ ☐ P/L at Exp.   ☐ P/L Theo.     │
└──────────────────────────────────┘
```
