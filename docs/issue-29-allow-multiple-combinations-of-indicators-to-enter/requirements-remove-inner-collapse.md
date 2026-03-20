# Requirements: Remove Inner Group Collapse from Dashboard Cards

**Parent Issue:** #29 — Allow multiple combinations of indicators to enter a trade
**Type:** UX Refinement (follow-up to Dashboard Compact Cards enhancement)
**Scope:** Frontend only — single Vue component

## Context & Motivation

The dashboard cards currently use two levels of progressive disclosure:

1. **Section-level collapse** (outer) — Entry Criteria and Last Evaluation sections show a compact summary line by default (e.g., "3 groups · 9 indicators"). Click to expand.
2. **Group-level collapse** (inner) — When a section is expanded, each indicator group shows only its name + badge. Click a group to see its indicator chips.

The customer has confirmed the feature works well but finds the **double collapsible is too much**. The inner group-level collapse adds unnecessary friction — when you expand a section, you want to see the data, not another set of expand buttons.

## What Changes

**Remove the inner group-level collapse.** When a section is expanded, all groups and their indicator data are shown directly — no per-group expand/collapse toggles.

### Behavior After Change

| State | What the user sees |
|-------|-------------------|
| **Collapsed** (default for multi-group) | Same as today — compact summary line with chevron (no change) |
| **Expanded** | All groups shown with their full indicator data visible immediately. Group name + badge shown as a header for each group, followed by all indicator chips. OR dividers between groups. No chevron or click-to-expand on individual groups. |

This applies to **both sections**:
- **Entry Criteria** (top of card) — config indicator groups
- **Last Evaluation** (bottom of card, when running) — group evaluation results

### What Does NOT Change

- Section-level collapse/expand behavior (the outer level) — unchanged
- Summary line text and formatting — unchanged
- Single-group cards — unchanged (no collapse controls, flat display)
- Config form — unchanged
- Evaluation dialog (test indicators popup) — unchanged
- Backend/API — unchanged
- Mobile card-level collapse — unchanged

## Functional Requirements

### FR-1: Entry Criteria — Expanded State Shows All Data

When the Entry Criteria section is expanded for a multi-group card:

1. Each group is displayed with its **group name** as a non-clickable header label (no chevron icon, no cursor:pointer)
2. Each group's **indicator chips** are shown directly below the group name — always visible, no expand needed
3. **OR dividers** between groups are shown (as today)
4. The group name still shows the indicator count (e.g., "Low VIX — 3 indicators")

### FR-2: Last Evaluation — Expanded State Shows All Data

When the Last Evaluation section is expanded for a multi-group running card:

1. Each group is displayed with its **group name + pass/fail badge** as a non-clickable header (no chevron, no cursor:pointer)
2. Each group's **indicator result chips** (with pass/fail coloring, values, stale icons) are shown directly below — always visible
3. **OR dividers** between groups are shown (as today)
4. The **overall status line** at the bottom is shown (as today)

### FR-3: Remove Group-Level Collapse State

1. Remove the `expandedGroups` reactive state map (no longer needed)
2. Remove `toggleGroup()` and `isGroupExpanded()` methods
3. Remove `group-collapse-header` clickable elements from the template
4. Remove `collapse-chevron-sm` icons from group headers
5. Group headers become static display elements (`.group-header-static` or similar)

### FR-4: Preserve All Other Behavior

1. Section-level collapse (`expandedEntrySections`, `expandedEvalSections`, `toggleEntrySection`, `toggleEvalSection`) — unchanged
2. `isMultiGroup()`, `getEntrySummary()`, `getEvalSummary()` helpers — unchanged
3. State persistence across WebSocket updates — unchanged (section-level state still keyed by config ID)
4. All card actions (start, stop, edit, delete, etc.) — unchanged

## Acceptance Criteria

| AC | Description |
|----|-------------|
| AC-1 | Multi-group Entry Criteria section is collapsed by default showing summary line (unchanged) |
| AC-2 | Clicking the Entry Criteria summary expands to show **all groups with all indicator chips visible** — no inner collapse toggles |
| AC-3 | Multi-group Last Evaluation section is collapsed by default showing pass/fail summary (unchanged) |
| AC-4 | Clicking the Last Evaluation summary expands to show **all groups with all indicator result chips visible** — no inner collapse toggles |
| AC-5 | Group headers (name + count or name + badge) are displayed but are NOT clickable — no chevron icons, no cursor:pointer |
| AC-6 | OR dividers between groups are displayed (unchanged) |
| AC-7 | Single-group cards are completely unchanged — no collapse controls |
| AC-8 | Section-level collapse state persists across WebSocket data updates (unchanged) |
| AC-9 | All existing tests pass (no regressions) — pre-existing failures in CollapsibleOptionsChain excluded |

## Scope Boundaries (NOT included)

- No changes to the collapsed state appearance (summary lines)
- No backend/API changes
- No config form changes
- No evaluation dialog changes
- No mobile-specific changes beyond what flows naturally from the template change

## Files to Modify

| File | Change |
|------|--------|
| `trade-app/src/components/automation/AutomationDashboard.vue` | Remove inner group collapse: template, setup() state, CSS |
| `trade-app/tests/AutomationDashboard.test.js` | Update tests that assert on group-level collapse behavior |
| `trade-app/tests/qa-AutomationDashboard.test.js` | Update QA tests that expand groups before asserting |
| `trade-app/tests/qa-dashboard-compact.test.js` | Update/remove QA tests for group-level collapse |
