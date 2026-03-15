# Phase 6 Bugfix Requirements — SimulationControls Layout Restructuring

## Customer Feedback

> "The alignment of the Current: is still not good, we can align it with the IV, as long as it is easier, we should have two blocks horizontally, one with the date, with the label Exp below that, and one for the IV, with the Current: below that, then the labels can be aligned left in each box."
>
> "Another thing is that the IV input percentage, should use all the size available, so it should expand when we have more space, currently after the + button, we have a blank space, and then in the end aligned to right, we have the reset button"

## Bug 1: Label Alignment — Restructure into Two Self-Contained Blocks

### Problem
The info row (Row 2) uses fragile `padding-left` values (60px for "Exp:", 50px for "Current:") to try to visually align labels under inputs. This is a pixel-guessing approach that doesn't align properly and breaks at different widths.

### Root Cause
The controls row (Row 1) and info row (Row 2) are separate CSS grid containers. The info row tries to use `padding-left` to fake alignment with inputs in the controls row above it, but the offsets don't match because they depend on the exact width of the "Date"/"IV" labels + buttons.

### Fix — Two Self-Contained Vertical Blocks

Restructure the layout so that each control zone (Date and IV) is a **self-contained vertical block** containing both its controls AND its label underneath. This eliminates the need for any `padding-left` alignment hacks.

**New structure:**

```
┌─────────────────────────────────────────────────────────────────────┐
│ [Date block]                    [IV block]                  [Reset] │
│ ┌────────────────────────┐  ┌────────────────────────────┐         │
│ │ Date  ◄ [calendar] ►   │  │ IV  ─ [  input%  ] +       │  🔄    │
│ │ Exp: Jul 18, 2025       │  │ Current: 32.5% (+2.1%)     │         │
│ └────────────────────────┘  └────────────────────────────┘         │
└─────────────────────────────────────────────────────────────────────┘
```

**Implementation:**

1. **Remove the separate `.controls-info-row` div entirely** from the template.
2. **Move the "Exp:" label inside the `.date-zone`** div, below the controls.
3. **Move the "Current:" label inside the `.iv-zone`** div, below the controls.
4. **Change `.control-zone` to `flex-direction: column`** (or use a nested structure) so controls are on top and the label is below, left-aligned.
5. **Within each block**, the controls remain in a horizontal row (flex-row), and the label sits below, left-aligned with `text-align: left` — no padding-left hacking needed.
6. **Remove** all `padding-left` from `.date-hint` and `.iv-info` — they should simply be left-aligned within their parent block.
7. The third column (reset button) remains as `auto`-sized at the right edge, vertically centered.

**Specific template changes:**

Remove:
```html
<!-- Row 2: Hint/info text -->
<div v-if="hasPositions && (maxDateValue || ivAvailable)" class="controls-info-row">
  <div class="date-hint">...</div>
  <div class="iv-info">...</div>
  <div></div>
</div>
```

Move into `.date-zone`:
```html
<div class="control-zone date-zone">
  <div class="zone-controls">
    <span class="zone-label">Date</span>
    <button ...>◄</button>
    <Calendar ... />
    <button ...>►</button>
  </div>
  <div class="zone-info date-hint" v-if="hasPositions && maxDateValue">
    Exp: {{ formatDate(maxDateValue) }}
  </div>
</div>
```

Move into `.iv-zone`:
```html
<div class="control-zone iv-zone">
  <div class="zone-controls">
    <span class="zone-label">IV</span>
    <button ...>−</button>
    <InputNumber ... />
    <button ...>+</button>
  </div>
  <div class="zone-info iv-info" v-if="hasPositions && ivAvailable && defaultIVValue !== null">
    Current: {{ formatPercent(defaultIVValue) }} <span ...>(+2.1%)</span>
  </div>
  <div class="zone-info iv-info" v-else-if="hasPositions">
    <span class="iv-unavailable">IV data unavailable</span>
  </div>
</div>
```

**CSS changes:**

```css
.control-zone {
  display: flex;
  flex-direction: column;  /* Stack controls + label vertically */
  gap: 2px;
  min-width: 0;
}

.zone-controls {
  display: flex;
  align-items: center;
  gap: 4px;
}

.zone-info {
  font-size: 10px;
  color: var(--text-tertiary, #888888);
  text-align: left;
  /* NO padding-left — naturally left-aligns within the block */
}
```

Remove:
- `.controls-info-row` CSS entirely
- `.date-hint` `padding-left: 60px`
- `.iv-info` `padding-left: 50px`


## Bug 2: IV Input Should Expand to Fill Available Space

### Problem
After the IV `+` button, there is a blank space before the reset button (which sits at the far right). The IV percentage input is fixed at 60px width and doesn't expand to use the available horizontal space.

### Root Cause
The `.iv-input` CSS has `flex: 0 0 auto; width: 60px; max-width: 60px;` which prevents it from growing.

### Fix
Change `.iv-input` to use `flex: 1; min-width: 60px;` and **remove** the `max-width: 60px` constraint. This lets it grow to fill available space within the IV zone.

```css
.iv-input {
  flex: 1;
  min-width: 60px;
  /* REMOVE: max-width: 60px; */
  /* REMOVE: width: 60px; */
  /* KEEP: overflow: hidden; */
}
```

## File to Modify

- `trade-app/src/components/SimulationControls.vue` — template restructuring + CSS changes

## Acceptance Criteria

1. Date block is self-contained: controls on top, "Exp:" label below, left-aligned naturally
2. IV block is self-contained: controls on top, "Current:" label below, left-aligned naturally  
3. No `padding-left` hacks used for label alignment
4. IV input expands to fill available horizontal space (no fixed max-width)
5. Reset button remains at the far right edge
6. No regressions in existing test suite (905+ tests pass)
7. Responsive behavior preserved for narrow panels
