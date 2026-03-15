# Phase 5 Bugfix Requirements — SimulationControls Visual Fixes

## Context

Customer tested the Phase 4 fixes and reported 3 remaining visual issues in the SimulationControls component. Root cause analysis reveals CSS selector errors and layout measurement issues.

**What worked (confirmed by customer):**
- ✅ Checkboxes replaced correctly with blue native HTML checkboxes
- ✅ `-` and `+` buttons now use the same solid style as the bottom trading panel
- ✅ "Exp:" label is correctly aligned

**Remaining issues:** 3 bugs detailed below.

---

## Bug 1: Calendar Date Input Height Still Too Big

**Symptom:** The date input field is visibly taller than the 28px calendar icon button next to it. The CSS fix from Phase 4 had no effect.

**Root Cause:** The CSS selector `:deep(.p-calendar-input)` targets a class that **does not exist** in PrimeVue Calendar v3. PrimeVue Calendar renders its input element with the classes `p-inputtext p-component` — NOT `p-calendar-input`. The selector never matched anything, so the height/padding overrides were silently ignored.

Additionally, the global `theme.css` styles `.p-inputtext` with:
```css
padding: var(--spacing-sm) var(--spacing-md) !important;  /* 8px 12px */
```
This global rule makes ALL PrimeVue text inputs (including the Calendar's input) taller than 28px. Our scoped override must use `!important` to beat this global rule.

**Fix:** In `SimulationControls.vue`, replace the incorrect CSS selector:

```css
/* REMOVE this — .p-calendar-input does not exist in PrimeVue v3 */
:deep(.p-calendar-input) { ... }
:deep(.p-calendar-input:focus) { ... }

/* REPLACE with — target the actual .p-inputtext class inside .date-calendar */
.date-calendar :deep(.p-inputtext) {
  background-color: var(--bg-tertiary, #1a1d23) !important;
  border-color: var(--border-secondary, #2a2d33) !important;
  color: var(--text-primary, #ffffff) !important;
  height: 28px !important;
  min-height: 28px !important;
  max-height: 28px !important;
  padding: 4px 8px !important;
  font-size: 11px !important;
  box-sizing: border-box !important;
  line-height: 1 !important;
}

.date-calendar :deep(.p-inputtext:focus) {
  border-color: var(--color-brand, #ff6b35) !important;
}
```

The key changes:
1. Correct CSS selector: `.date-calendar :deep(.p-inputtext)` — scoped to only the Calendar component's input
2. Use `!important` on ALL sizing properties to override the global `theme.css` `.p-inputtext` rule
3. Add `max-height: 28px` to prevent any expansion
4. Add `line-height: 1` to prevent text from inflating the height

**Acceptance Criteria:**
- The date input field and the calendar icon button are the exact same height (28px)
- The date text is legible and properly centered vertically within the input

---

## Bug 2: `+` Button Appears Inside the IV Percentage Input

**Symptom:** The `+` button after the IV input appears to be visually inside/overlapping the input field rather than clearly separated after it.

**Root Cause:** The PrimeVue `InputNumber` component renders a wrapper `<span class="p-inputnumber">` that contains the inner `<input class="p-inputnumber-input p-inputtext">`. The `.iv-input` class sets `width: 60px; max-width: 60px`, but this constrains the outer `.iv-input` wrapper — the inner PrimeVue `p-inputnumber` span may still expand or the input may have padding/borders that push visual boundaries. The `+` button immediately following it can appear to be inside the input's visual boundary.

**Fix:** In `SimulationControls.vue`:

1. Constrain the InputNumber's internal input to fit properly within 60px:
```css
.iv-input :deep(.p-inputnumber) {
  width: 100%;
}

.iv-input :deep(.p-inputtext) {
  width: 100% !important;
  max-width: 100% !important;
  height: 28px !important;
  min-height: 28px !important;
  max-height: 28px !important;
  padding: 4px 2px !important;
  font-size: 11px !important;
  text-align: center !important;
  box-sizing: border-box !important;
  line-height: 1 !important;
}
```

2. Remove the old `:deep(.p-inputnumber-input)` selector (same problem as Bug 1 — may not be targeting correctly since the actual class is `p-inputnumber-input p-inputtext`).

3. Ensure the `.iv-zone` has proper `gap: 4px` and the `.iv-input` has `overflow: hidden` to prevent visual bleeding.

**Acceptance Criteria:**
- The `+` button is visually separated from and positioned clearly after (to the right of) the IV percentage input
- The IV input, `-` button, and `+` button are all the same height (28px)
- The IV percentage value is readable and centered in the input

---

## Bug 3: "Current:" Label Misaligned (Centered Under IV Input)

**Symptom:** The "Current:" label in the info row appears roughly centered under the IV percentage input, rather than left-aligned under the start of the IV input (matching how "Exp:" aligns under the date input).

**Root Cause:** The `padding-left: 46px` value on `.iv-info` is incorrect. The actual left offset to align with the IV input should account for: "IV" label width (~14px) + gap (4px) + minus button width (28px) + gap (4px) = ~50px. But this is fragile because it depends on exact label width rendering.

**Fix:** A more robust approach — instead of using padding-left with pixel guessing, make the info row items align properly with the controls above them using the same flex structure:

Option A (simple padding adjustment): Change `.iv-info` padding-left to match the actual computed offset. Measure: "IV" zone-label is ~14px wide at 11px font + 4px gap + 28px button + 4px gap = 50px. Try `padding-left: 50px`.

Option B (more robust): Give `.iv-info` the same flex structure as `.iv-zone` by adding invisible spacer elements, or use CSS `calc()` with the known widths.

**Recommended:** Use Option A first (simpler), and if the "IV" label width varies, use `padding-left: calc(14px + 4px + 28px + 4px)` = `padding-left: 50px` to make the calculation explicit.

**Acceptance Criteria:**
- The "Current:" text left-aligns with the left edge of the IV percentage input field above it
- The alignment is visually consistent with how "Exp:" aligns under the date input

---

## Files to Modify

| File | Changes |
|------|---------|
| `trade-app/src/components/SimulationControls.vue` | Fix CSS selectors and layout (all 3 bugs) |

## Testing

- Run `npm test` in `trade-app/` — all existing tests must pass
- Visual verification: Calendar input, IV input, buttons should all be 28px height and properly separated
