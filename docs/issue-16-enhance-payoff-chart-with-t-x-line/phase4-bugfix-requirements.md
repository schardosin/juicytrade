# Phase 4 Bug Fix Requirements — SimulationControls Visual Issues

## Overview
Customer reports 4 remaining visual issues with the SimulationControls component after Phase 3 fixes. All are CSS/template-level bugs in `SimulationControls.vue`.

---

## Bug 1: Calendar Button Height Mismatch

**Problem:** The calendar icon button is smaller than the date input field, creating a visual mismatch.

**Root Cause:** PrimeVue's Calendar component renders the trigger button with its own internal sizing that doesn't respect our 28px height override consistently.

**Fix:** Ensure both the calendar input AND the datepicker trigger button have exactly matching heights. Add explicit height + min-height to both elements. Consider adding `box-sizing: border-box` to both.

**Acceptance Criteria:**
- The calendar icon button and date input field are the same height visually
- Both should be 28px (or whatever height looks consistent)

---

## Bug 2: IV +/- Buttons — Replace with Visible Styled Buttons

**Problem:** The IV `-` and `+` buttons use PrimeVue's `p-button-text p-button-plain` style, which renders them as near-invisible text. The `-` looks like a dash, and the `+` appears to be inside the input field rather than after it.

**Customer Reference:** "Replace the buttons by - and + buttons like what we have in the Bottom Trade Dialog, to change the price. These are visible buttons with the - and + inside it."

**Reference Implementation:** `BottomTradingPanel.vue` uses `.price-btn` styled buttons:
```css
.price-btn {
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
}
.price-btn:hover {
  background-color: #444;
  color: #fff;
}
```

**Fix:** 
1. Replace the IV `<Button label="-" class="p-button-sm p-button-text p-button-plain">` with plain `<button class="iv-btn">` elements (NOT PrimeVue Button)
2. Style `.iv-btn` to match the `.price-btn` pattern from BottomTradingPanel
3. Do the same for the date navigation `<` and `>` buttons — make them visible with solid backgrounds
4. Ensure the `+` button appears AFTER the input, not overlapping it

**Acceptance Criteria:**
- IV `-` and `+` buttons have solid dark background (#333), visible border (#444), clearly readable text
- `-` button is clearly a minus button, not a dash
- `+` button appears after the input field, not inside it
- Date navigation `<` and `>` buttons also have the same solid visible style
- Hover state changes background to #444

---

## Bug 3: Label Alignment (Exp: and Current:)

**Problem:** The "Exp:" and "Current:" labels in Row 2 are misaligned with the input fields above them.

**Fix:** Adjust the `padding-left` values in `.date-hint` and `.iv-info` to align precisely with the start of the input fields above them. The current padding values (32px and 20px) don't account for the actual position of inputs in the grid.

**Acceptance Criteria:**
- "Exp:" label aligns visually with the date input field above it
- "Current:" label aligns visually with the IV input field above it

---

## Bug 4: Checkboxes Still Dark — Replace PrimeVue Checkbox with Native HTML

**Problem:** Despite CSS overrides to PrimeVue's `.p-checkbox-box.p-highlight`, the checkboxes still appear dark/invisible. The customer explicitly requested "the blue checkbox, like the ones used in the position detail to choose the legs."

**Reference Implementation:** `RightPanel.vue` uses native HTML checkboxes with custom CSS:
```html
<div class="position-checkbox">
  <input type="checkbox" :id="id" :checked="value" @change="handler" class="custom-checkbox" />
  <label :for="id" class="checkbox-label"></label>
</div>
```
```css
.custom-checkbox {
  opacity: 0;
  position: absolute;
  width: 100%;
  height: 100%;
  cursor: pointer;
}
.checkbox-label {
  position: absolute;
  top: 0; left: 0;
  width: 16px; height: 16px;
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
  top: -2px; left: 2px;
  color: white;
  font-size: 12px;
  font-weight: bold;
}
```

**Fix:**
1. Remove PrimeVue `<Checkbox>` import and usage entirely from SimulationControls.vue
2. Replace with native `<input type="checkbox">` + `<label>` using the EXACT same class names and CSS from RightPanel.vue
3. Copy the `.custom-checkbox`, `.checkbox-label`, and `.custom-checkbox:checked + .checkbox-label` CSS rules from RightPanel.vue

**Acceptance Criteria:**
- Checkboxes render identically to the blue checkboxes in the position detail section
- When checked: blue background (#007bff), white checkmark (✓)
- When unchecked: dark background with visible border
- Clicking the label toggles the checkbox
