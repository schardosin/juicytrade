# UX Design Spec: Indicator Groups for Trade Automation

**Issue:** [#29 — Allow multiple combinations of indicators to enter a trade](https://github.com/schardosin/juicytrade/issues/29)
**Requirements:** [requirements.md](https://github.com/schardosin/juicytrade/blob/fleet/issue-29-allow-multiple-combinations-of-indicators-to-enter/docs/issue-29-allow-multiple-combinations-of-indicators-to-enter/requirements.md)
**Architecture:** [architecture.md](https://github.com/schardosin/juicytrade/blob/fleet/issue-29-allow-multiple-combinations-of-indicators-to-enter/docs/issue-29-allow-multiple-combinations-of-indicators-to-enter/architecture.md)

---

## 1. Design Principles

1. **Single-group = today's experience.** A user with one group should see almost the same UI as today. No visual overhead.
2. **Progressive disclosure.** Group scaffolding appears only when the user adds a second group.
3. **OR logic must be unmistakable.** The relationship between groups must be visually obvious at a glance — no ambiguity.
4. **Reuse existing patterns.** Use the same card style, spacing tokens, status colors, and responsive breakpoint (768px) already established in the codebase.

---

## 2. AutomationConfigForm — Indicator Groups Section

### 2.1 Section Header Changes

The current section header reads:

> **Entry Indicators**
> All enabled indicators must pass for a trade to be executed.

**Change the description text** based on group count:

- **1 group:** "All enabled indicators must pass for a trade to be executed." *(unchanged from today)*
- **2+ groups:** "Indicators are organized into groups. All indicators within a group must pass (AND). If **any** group passes, the trade executes (OR)."

This keeps the single-group case identical to today and only introduces the OR concept when relevant.

### 2.2 Group Container Design

Each group is wrapped in a container. The design adapts based on how many groups exist.

#### Single Group (Lightweight Mode)

When there is exactly **one** group, the container should be **invisible** — no border, no background, no extra padding. The indicators render exactly as they do today (a flat `.indicators-list`). The only additions:

- A **subtle group name** shown as an inline-editable label just above the indicators list. Style it like `.section-description` (small, `var(--text-tertiary)`, `var(--font-size-sm)`). Placeholder: "Default". This feels like a tag, not a section header.
- An **"Add Group" link** placed in the `indicators-actions` bar (next to "Add Indicator" and "Test All"). Use `p-button-text` style with a `pi-plus-circle` icon and label "Add Group". It should feel secondary — not competing with "Add Indicator".

```
┌─────────────────────────────────────────────────┐
│ Entry Indicators                                │
│ All enabled indicators must pass for a trade... │
│                                                 │
│ Default  [edit icon]                            │  ← subtle group name
│                                                 │
│ ┌─ VIX < 15 ──────────── 12.50 ─ [x] ── ▼ ─┐  │  ← existing indicator card
│ └────────────────────────────────────────────┘  │
│ ┌─ Gap < 0% ──────────── -0.35 ─ [x] ── ▼ ─┐  │
│ └────────────────────────────────────────────┘  │
│                                                 │
│ [+ Add Indicator]  [▶ Test All]  [+ Add Group]  │
│                                                 │
└─────────────────────────────────────────────────┘
```

#### Multiple Groups (Full Group Mode)

When there are **2+ groups**, each group gets a visible container:

- **Background:** `var(--bg-tertiary)` — one step lighter than the section background (`var(--bg-secondary)`), creating a subtle inset card.
- **Border:** `1px solid var(--border-primary)`.
- **Border-radius:** `var(--radius-lg)` (8px).
- **Padding:** `var(--spacing-md)` inside.
- **Margin between groups:** `0` — the OR divider provides the visual separation.

**Group Header (inside the container):**

```
┌───────────────────────────────────────────────────┐
│  Low VIX  [✎]                        ✅ Pass  [🗑] │  ← group header row
│                                                   │
│  ┌─ VIX < 15 ──────────── 12.50 ─ [x] ── ▼ ─┐   │  ← indicator cards
│  └────────────────────────────────────────────┘   │
│  ┌─ Gap < 0% ──────────── -0.35 ─ [x] ── ▼ ─┐   │
│  └────────────────────────────────────────────┘   │
│                                                   │
│  [+ Add Indicator]                                │
└───────────────────────────────────────────────────┘
```

**Header row layout** (flexbox, space-between):
- **Left side:** Group name as an `InputText` (PrimeVue) — compact, borderless by default, showing the name as plain text. On hover/focus, the border appears for inline editing. Use `var(--font-size-md)`, `var(--font-weight-semibold)`, `var(--text-primary)`.
- **Right side:** A flex row containing:
  - **Pass/fail badge** (only visible after "Test All"): A small `<span>` styled like the existing `.test-result` class. Green `✓ Pass` or red `✗ Fail`. Hidden until test results are available.
  - **Delete group button:** `pi-trash` icon, `p-button-text p-button-danger p-button-sm`. Same pattern as the existing "remove indicator" button.

**"Add Indicator" per group:** Inside each group container, at the bottom. Use the existing `p-button-outlined` style with `pi-plus` icon and label "Add Indicator". This scopes the action to that group.

### 2.3 OR Divider Between Groups

Between each pair of group containers, render an OR divider. This is the most important visual element — it must clearly communicate the logical relationship.

**Design:**

```
──────────────────── OR ────────────────────
```

**Implementation:**
- Use a custom CSS class, not PrimeVue's `<Divider>` (the built-in Divider doesn't support centered text labels with the right styling easily).
- Horizontal rule with centered "OR" text label.
- **Line:** `1px solid var(--border-secondary)` — slightly more visible than `--border-primary`.
- **Label:** "OR" text in uppercase, `var(--font-size-xs)`, `var(--font-weight-semibold)`, `var(--color-brand)` (#ff6b35 — the orange brand color). The brand color makes it pop against the dark background without being jarring.
- **Label background:** `var(--bg-secondary)` (matches the section background so the line appears to pass behind the text).
- **Vertical spacing:** `var(--spacing-lg)` above and below the divider (16px each side).

**CSS pattern:**

```css
.or-divider {
  display: flex;
  align-items: center;
  gap: var(--spacing-md);
  margin: var(--spacing-lg) 0;
}

.or-divider::before,
.or-divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--border-secondary);
}

.or-divider-label {
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  color: var(--color-brand);
  text-transform: uppercase;
  letter-spacing: 1px;
  padding: var(--spacing-xs) var(--spacing-sm);
}
```

### 2.4 "Add Group" Button

**When 1 group exists:** Rendered inline in the bottom action bar alongside "Add Indicator" and "Test All". Style: `p-button-text` with `pi-plus-circle` icon and label "Add Group". Subdued — discoverable but not dominant.

**When 2+ groups exist:** Rendered as a full-width outlined button **below** the last group (and below its OR divider position). Style: `p-button-outlined` with `pi-plus` icon and label "Add Group". Centered. This makes it clear the new group will be added at the end of the OR chain.

```
┌── Low VIX ──────────────────────────────────┐
│  ... indicators ...                          │
│  [+ Add Indicator]                           │
└──────────────────────────────────────────────┘
                     ── OR ──
┌── High VIX ─────────────────────────────────┐
│  ... indicators ...                          │
│  [+ Add Indicator]                           │
└──────────────────────────────────────────────┘

              [+ Add Group]

[▶ Test All]                   No groups passing
```

### 2.5 "Remove Group" Behavior

- **Button:** `pi-trash` icon on the group header, `p-button-text p-button-danger p-button-sm`.
- **Confirmation:** If the group contains any indicators, show a `confirm()` dialog (or a PrimeVue `ConfirmDialog`) with the message: *"Delete group 'Low VIX' and its N indicators?"*. If the group is empty, delete immediately without confirmation.
- **Last group guard:** If only one group remains, the delete button should be **hidden** (not disabled). At least one group must always exist. The user cannot delete the last group.

### 2.6 Group Name Editing

- Use an `InputText` with a borderless/transparent style by default.
- On hover: show a subtle border (`var(--border-secondary)`).
- On focus: full border with blue focus ring (existing PrimeVue input focus style).
- Placeholder: "Group N" (auto-generated, e.g., "Group 1", "Group 2").
- **Max length:** 30 characters — keep it short since it appears in the dashboard too.
- No validation beyond non-empty — if the user clears the name and blurs, revert to the previous value or the default.

### 2.7 Test All Flow (Updated)

**Trigger:** The "Test All" button moves to the bottom of the section, outside all group containers (see layout in 2.4).

**Behavior:**
1. Calls the API with `{ indicator_groups: [...] }`.
2. On response, maps `group_results` by `group_id` to each group container.
3. **Per-group badges:** Each group header shows a pass/fail badge:
   - **Pass:** Green background `rgba(34, 197, 94, 0.1)`, green text `var(--color-success)`, text: "✓ Pass".
   - **Fail:** Red background `rgba(239, 68, 68, 0.1)`, red text `var(--color-danger)`, text: "✗ Fail".
   - Use the same badge styling as the existing `.test-result.passed` / `.test-result.failed` classes.
4. **Per-indicator badges:** Within each group, individual indicator results show on the indicator header row (existing behavior, unchanged).
5. **Overall result message** (below "Test All" button):
   - If any group passes: `"✓ Group 'Low VIX' passed"` — name the first passing group. Use green styling (`.all-result.passed` pattern).
   - If multiple groups pass: `"✓ 2 of 3 groups passing"`.
   - If no groups pass: `"✗ No groups passing"`. Red styling (`.all-result.failed` pattern).
   - For single group: Retain the existing labels: `"All Passed"` / `"Some Failed"` — no change in messaging.

### 2.8 Empty State

When a group has no indicators:

```
┌── Low VIX ─────────────────────── [🗑] ─────┐
│                                              │
│  ⚙  No indicators. Add one below.           │
│                                              │
│  [+ Add Indicator]                           │
└──────────────────────────────────────────────┘
```

Use the existing `.indicators-empty` pattern (centered icon + text), but scaled down (smaller padding, smaller icon) to fit within the group container. Use `pi-filter` icon as the existing empty state does.

### 2.9 "Add Indicator" Dialog — Group Context

When the user clicks "Add Indicator" within a group, the existing dialog opens identically (same search, same categorized list). The **only change** is that the selected indicator is pushed to the specific group's `indicators` array, not a global flat list.

No visual change to the dialog itself. The dialog header can optionally include the group name for context: *"Add Indicator to 'Low VIX'"* — but this is a nice-to-have, not required.

### 2.10 Full Multi-Group Layout (ASCII Reference)

```
┌─────────────────────────────────────────────────────────────┐
│ Entry Indicators                                            │
│ Indicators are organized into groups. All indicators within │
│ a group must pass (AND). If any group passes, the trade     │
│ executes (OR).                                              │
│                                                             │
│ ┌── Low VIX  [✎]                           ✅ Pass  [🗑] ──┐│
│ │                                                          ││
│ │ ┌─ VIX  < 15 ─────────── ✓ 12.50 ─ [⟳] [x] ── ▼ ──┐   ││
│ │ └────────────────────────────────────────────────────┘   ││
│ │ ┌─ Gap  < 0% ─────────── ✓ -0.35 ─ [⟳] [x] ── ▼ ──┐   ││
│ │ └────────────────────────────────────────────────────┘   ││
│ │                                                          ││
│ │ [+ Add Indicator]                                        ││
│ └──────────────────────────────────────────────────────────┘│
│                                                             │
│ ─────────────────────── OR ─────────────────────────        │
│                                                             │
│ ┌── High VIX  [✎]                          ❌ Fail  [🗑] ──┐│
│ │                                                          ││
│ │ ┌─ VIX  > 20 ─────────── ✗ 12.50 ─ [⟳] [x] ── ▼ ──┐   ││
│ │ └────────────────────────────────────────────────────┘   ││
│ │ ┌─ Gap  < 1% ─────────── ✓ -0.35 ─ [⟳] [x] ── ▼ ──┐   ││
│ │ └────────────────────────────────────────────────────┘   ││
│ │                                                          ││
│ │ [+ Add Indicator]                                        ││
│ └──────────────────────────────────────────────────────────┘│
│                                                             │
│                     [+ Add Group]                           │
│                                                             │
│ [▶ Test All]                        ✓ Group 'Low VIX' passed│
└─────────────────────────────────────────────────────────────┘
```

---

## 3. AutomationDashboard — Group-Aware Indicator Display

### 3.1 Existing "Entry Criteria" Section

Currently, the dashboard card has an "Entry Criteria" section with a flat `indicators-grid` showing indicator chips. The `status-details` section below it shows "Last Evaluation" with `result-chip` elements.

### 3.2 Single Group (Lightweight)

When only **one group** exists (or when `group_results` has length 1), the dashboard display is **identical to today**:
- The "Entry Criteria" section shows indicator chips as a flat list.
- The "Last Evaluation" results show as a flat row of result chips.
- No group name label, no OR divider. Completely unchanged.

### 3.3 Multiple Groups (Grouped Display)

When `group_results` has 2+ entries, restructure both the "Entry Criteria" and "Last Evaluation" sections.

#### Entry Criteria Section (Config-Based, Static)

Replace the flat `indicators-grid` with grouped display:

```
Entry Criteria
┌─ Low VIX ───────────────────────────────┐
│  VIX < 15    Gap < 0%                   │
└─────────────────────────────────────────┘
                    OR
┌─ High VIX ──────────────────────────────┐
│  VIX > 20    Gap < 1%                   │
└─────────────────────────────────────────┘
```

**Group container on dashboard:**
- **Background:** `var(--bg-tertiary)` (same as existing `.trade-summary` and `.status-details` background).
- **Border-radius:** `var(--radius-md)`.
- **Padding:** `var(--spacing-sm)`.
- **Group name:** Rendered as a small label inside the top of the container. Style: `var(--font-size-xs)`, `var(--font-weight-semibold)`, `var(--text-secondary)`.
- **Indicator chips inside:** Use the existing `.indicator-chip` styling, laid out as a flex-wrap row.

**OR divider on dashboard:**
Same concept as the config form but **more compact**:
- Line: `1px solid var(--border-primary)`.
- "OR" label: `var(--font-size-xs)`, `var(--color-brand)`, `var(--font-weight-semibold)`.
- Vertical spacing: `var(--spacing-sm)` above and below (8px — tighter than the form).

#### Last Evaluation Section (Live Results)

When the automation is running and `group_results` is available from the WebSocket update:

```
Last Evaluation
┌─ Low VIX ─────────────────────── ✅ Pass ─┐
│  VIX ✓ 12.50    Gap ✓ -0.35               │
└────────────────────────────────────────────┘
                     OR
┌─ High VIX ────────────────────── ❌ Fail ─┐
│  VIX ✗ 12.50    Gap ✓ -0.35               │
└────────────────────────────────────────────┘

Overall: ✅ Passing (Low VIX)
```

**Per-group pass/fail badge:**
- Placed at the end of the group name row, right-aligned.
- **Pass:** Small span, `var(--color-success)`, background `rgba(34, 197, 94, 0.1)`, text "✓ Pass", `var(--font-size-xs)`.
- **Fail:** Small span, `var(--color-danger)`, background `rgba(239, 68, 68, 0.1)`, text "✗ Fail", `var(--font-size-xs)`.
- Existing `.result-chip.passed` and `.result-chip.failed` styles can be reused.

**Individual indicator result chips inside each group:**
- Use the existing `.result-chip` styling — colored by pass/fail/stale.
- Wrapped in a flex row within the group container.

**Overall status line:**
- Placed below all groups.
- If any group passes: "✅ Passing (Low VIX)" — green text, naming the first passing group.
- If no groups pass: "❌ Not passing" — red text.
- Style: `var(--font-size-sm)`, `var(--font-weight-medium)`.

### 3.4 Backward Compatibility

If `group_results` is **absent** (data from before migration or old WebSocket message), fall back to the existing flat `indicator_results` display. No change to the current rendering path.

Check: `if (status.group_results && status.group_results.length > 0)` → grouped view. Else → flat view.

### 3.5 Evaluation Result Dialog

The existing "Indicator Evaluation Results" dialog (`showEvalDialog`) shows a summary and per-indicator results. Update it for groups:

**Single group:** No change — show existing flat list.

**Multiple groups:**
- Add a grouped structure: each group is a collapsible section with the group name and pass/fail badge as header.
- Keep the existing `.eval-item` styling for individual indicators within each group.
- Add the OR divider between groups (compact version).
- The summary at the top changes from "All indicators passed" to "Group 'Low VIX' passed" or "N of M groups passing".

---

## 4. Mobile Responsiveness

The app uses a single breakpoint: `@media (max-width: 768px)` and an `isMobile` composable.

### 4.1 Config Form on Mobile

- **Group containers:** Full width, no horizontal margin changes needed (they're already block-level).
- **Group header:** Stack the name and actions vertically if needed — but the header row should fit on one line since the group name is short and the actions are small icons. Keep as flex row.
- **OR divider:** Same design, no change needed — it's a simple horizontal line.
- **"Add Group" button:** Full width on mobile, centered text.
- **Indicator cards within groups:** Already responsive (they stack to single column on mobile per existing CSS).
- **Group name InputText:** Full width within the header flex row.

### 4.2 Dashboard on Mobile

- Dashboard cards already collapse on mobile with expand/collapse.
- **Grouped indicators:** When the card is expanded, the group containers stack naturally.
- **OR divider:** No change needed, stays horizontal.
- **When collapsed (mobile):** The card header already only shows the config name and status badge. The grouped indicator details are hidden. No change needed.

---

## 5. New CSS Classes Summary

All new classes should be added as **scoped CSS** within the respective component files, following the existing convention.

### AutomationConfigForm.vue — New Classes

| Class | Purpose |
|-------|---------|
| `.indicator-group` | Group container (visible border/bg in multi-group mode) |
| `.indicator-group.lightweight` | Single-group mode — no border, no background, no extra padding |
| `.group-header` | Flex row: group name + actions |
| `.group-name-input` | Inline-editable InputText for group name |
| `.group-header-actions` | Right-aligned flex row: test badge + delete button |
| `.group-test-badge` | Pass/fail badge on group header (reuse `.test-result` style) |
| `.group-body` | Container for the indicator cards within a group |
| `.group-actions` | Bottom row within group: "Add Indicator" button |
| `.or-divider` | Horizontal line with centered "OR" label |
| `.or-divider-label` | The "OR" text span |
| `.section-actions` | Bottom row outside groups: "Add Group", "Test All", overall result |

### AutomationDashboard.vue — New Classes

| Class | Purpose |
|-------|---------|
| `.indicator-group-dashboard` | Group container on dashboard cards |
| `.group-label` | Group name label within dashboard group container |
| `.group-result-badge` | Pass/fail badge next to group name on dashboard |
| `.or-divider-compact` | Compact OR divider for dashboard cards |
| `.overall-status` | Overall pass/fail line below all groups |

---

## 6. Interaction States

### 6.1 Adding a New Group

1. User clicks "Add Group".
2. New empty group container appears at the bottom of the group list with default name "Group N" (auto-incrementing).
3. The group name input is auto-focused for immediate renaming.
4. The group container shows the empty state ("No indicators. Add one below.").
5. An OR divider automatically appears between the previous last group and the new group.

### 6.2 Deleting a Group

1. User clicks the trash icon on a group header.
2. If the group has indicators → confirmation dialog: "Delete group 'Low VIX' and its 2 indicators?".
3. If the group is empty → immediate deletion, no confirmation.
4. On deletion:
   - The group container and its OR divider are removed.
   - If this reduces the group count to 1, the remaining group transitions to "lightweight" mode (no visible container border/bg).
   - Any test results for the deleted group are cleared.
5. The delete button is **hidden** (not shown) when only one group remains.

### 6.3 Transition Between Modes

When going from 1 group → 2 groups (user clicks "Add Group"):
- The existing single group **gains** a visible container (border, background).
- An OR divider appears between the two groups.
- The section description text updates to mention OR logic.
- The "Add Group" button moves from the inline action bar to the standalone position below all groups.

When going from 2 groups → 1 group (user deletes a group):
- The remaining group **loses** its visible container — returns to lightweight mode.
- The OR divider disappears.
- The section description reverts to the simple AND-only text.
- The "Add Group" button moves back to the inline action bar.

This transition should be **instant** (no animation needed) — it's just conditional CSS classes and v-if/v-else rendering.

---

## 7. What NOT to Change

- **Individual indicator card UI** — The type selector, operator dropdown, threshold input, enable/disable toggle, expand/collapse, test button — all stay exactly the same. They just live inside a group container instead of directly under the section.
- **Add Indicator dialog** — Same dialog, same search, same categories. Only the destination (which group's array) changes.
- **Trade Configuration section** — Completely untouched.
- **Basic Information section** — Untouched.
- **Entry Time & Schedule section** — Untouched.
- **Enable/Disable section** — Untouched.
- **Dashboard card layout** — The card header, trade summary, status details structure, and action buttons all stay the same. Only the indicator display within "Entry Criteria" and "Last Evaluation" is restructured.
- **Logs dialog** — Untouched.
- **Delete confirmation dialog** — Untouched (the existing one for config deletion).
- **Theme tokens** — No new CSS custom properties in `theme.css`. Use existing tokens only.

---

## 8. Design Token Reference

For developer convenience, here are the exact tokens to use for key elements:

| Element | Token |
|---------|-------|
| Group container background (multi-group) | `var(--bg-tertiary)` |
| Group container border | `1px solid var(--border-primary)` |
| Group container border-radius | `var(--radius-lg)` |
| Group container padding | `var(--spacing-md)` |
| Group name text | `var(--font-size-md)`, `var(--font-weight-semibold)`, `var(--text-primary)` |
| Group name (single-group, subtle) | `var(--font-size-sm)`, `var(--font-weight-medium)`, `var(--text-tertiary)` |
| OR divider line | `1px solid var(--border-secondary)` |
| OR divider label | `var(--font-size-xs)`, `var(--font-weight-semibold)`, `var(--color-brand)` |
| OR divider vertical spacing (form) | `var(--spacing-lg)` |
| OR divider vertical spacing (dashboard) | `var(--spacing-sm)` |
| Pass badge bg | `rgba(34, 197, 94, 0.1)` |
| Pass badge text | `var(--color-success)` |
| Fail badge bg | `rgba(239, 68, 68, 0.1)` |
| Fail badge text | `var(--color-danger)` |
| Dashboard group container bg | `var(--bg-tertiary)` |
| Dashboard group name | `var(--font-size-xs)`, `var(--font-weight-semibold)`, `var(--text-secondary)` |
