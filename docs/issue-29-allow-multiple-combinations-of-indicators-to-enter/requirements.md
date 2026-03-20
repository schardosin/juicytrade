# Requirements: Indicator Groups for Trade Automation

**Issue:** [#29 — Allow multiple combinations of indicators to enter a trade](https://github.com/schardosin/juicytrade/issues/29)
**Status:** Draft — Awaiting Customer Approval

---

## 1. Overview

### Problem Statement

Currently, each automation configuration has a flat list of indicators that are **all evaluated with AND logic** — every enabled indicator must pass for the trade to execute. This means that if a user wants to enter the same trade under different market conditions (e.g., "low VIX + gap < 0%" OR "higher VIX + gap < 1%"), they must create **duplicate automation configs** — one for each combination of conditions. The trade configuration (strategy, delta, width, capital, etc.) is identical across all of them; only the indicator thresholds differ.

This leads to:
- **Config sprawl**: 4+ duplicate automations for what is logically a single trading strategy
- **Maintenance burden**: Changing the trade parameters requires updating every duplicate
- **Cluttered dashboard**: Multiple cards showing essentially the same trade

### Solution

Introduce **Indicator Groups** — a way to organize indicators into named groups within a single automation config. Groups are evaluated with **OR logic** between them (any group passing is sufficient), while indicators **within** a group retain the existing **AND logic** (all must pass). This allows the user to consolidate multiple entry scenarios into one automation.

### Conceptual Model

```
Automation Config
├── Indicator Group 1: "Low VIX"        ← All indicators in this group must pass (AND)
│   ├── VIX < 15
│   └── Gap < 0%
├── Indicator Group 2: "Medium VIX"     ← OR this group passes (AND within)
│   ├── VIX >= 15 AND VIX < 20
│   └── Gap < 0.5%
├── Indicator Group 3: "High VIX"       ← OR this group passes
│   ├── VIX >= 20
│   └── Gap < 1%
└── Trade Configuration (shared)
    ├── Strategy: Put Spread
    ├── Width: 20, Delta: 0.05
    └── ...
```

**If ANY group evaluates to true → green light to enter the trade.**

---

## 2. Functional Requirements

### FR-1: Indicator Group Data Model

**FR-1.1** — A new `IndicatorGroup` structure shall be introduced with the following properties:
- `id` (string): Unique identifier for the group
- `name` (string): User-defined display name (e.g., "Low VIX", "High Volatility")
- `indicators` (array of `IndicatorConfig`): The indicators belonging to this group

**FR-1.2** — The `AutomationConfig` type shall gain a new field `indicator_groups` (array of `IndicatorGroup`) alongside the existing `indicators` field.

**FR-1.3** — **Backward compatibility**: Existing configs that have a flat `indicators` array (no groups) shall continue to work unchanged. When loaded:
- If `indicator_groups` is present and non-empty, use the grouped evaluation logic.
- If only `indicators` is present (legacy format), treat it as a single unnamed default group. The system shall transparently migrate the flat list into a single group on the first save/update.

**FR-1.4** — Each automation must have **at least one group** with **at least one enabled indicator** to be valid for execution.

### FR-2: Group Evaluation Logic (Backend)

**FR-2.1** — **Within a group**: All enabled indicators must pass (AND logic) — same as current behavior. Disabled indicators within a group are skipped (always pass).

**FR-2.2** — **Across groups**: If **any** group passes, the overall indicator check passes (OR logic). The automation proceeds to trade entry.

**FR-2.3** — A group with **no enabled indicators** shall be treated as "always passes" (vacuously true), consistent with current behavior for disabled indicators.

**FR-2.4** — **Stale data handling**: The existing stale-data blocking behavior shall apply per-group. A group with any stale enabled indicator is treated as failing (does not pass). Other groups with fresh data can still pass.

**FR-2.5** — The `ActiveAutomation` response shall be extended to communicate group-level results:
- `indicator_results` shall remain as a flat list of all indicator results (for backward compat and simple display).
- A new `group_results` field shall be added: an array of objects, each containing:
  - `group_id` (string)
  - `group_name` (string)
  - `pass` (bool): Whether all enabled indicators in this group passed
  - `indicator_results` (array of `IndicatorResult`): Results for indicators in this group
- `all_indicators_pass` shall now reflect the OR-of-groups logic (true if any group passes).

**FR-2.6** — The engine's evaluation cycle shall evaluate **all groups** (not short-circuit), so the dashboard can display the status of every group/indicator.

### FR-3: Configuration UI — Indicator Groups in AutomationConfigForm

**FR-3.1** — The "Entry Indicators" section shall be restructured to display indicators organized by group.

**FR-3.2** — **Group container**: Each group shall be visually represented as a distinct, labeled card/container with:
- A group name (editable inline or via a text field) displayed as the header
- The list of indicators belonging to that group (using the existing indicator card UI)
- A button to add an indicator to that specific group
- A button to remove/delete the group (with confirmation if it contains indicators)
- Visual separation between groups with an "OR" label/divider between them to make the logic clear

**FR-3.3** — **Add Group**: A prominent "Add Group" button shall allow creating a new empty indicator group. The user provides a name (with a sensible default like "Group 1", "Group 2", etc.).

**FR-3.4** — **Single-group experience**: When there is only one group, the UI should feel lightweight — similar to the current flat list but with the group name visible and the ability to add more groups. The intent is that a single-group config should feel no more complex than today's UI.

**FR-3.5** — **Add Indicator**: The existing "Add Indicator" dialog shall work within the context of a specific group. Clicking "Add Indicator" on a group opens the indicator picker and adds the selected indicator to that group.

**FR-3.6** — **Test All**: The "Test All" button shall test all indicators across all groups and display the per-group pass/fail result. The overall result should indicate whether at least one group passes.

**FR-3.7** — **Per-group test results**: After testing, each group header shall show a pass/fail badge. The overall result label should say something like "Group 'Low VIX' passed" or "No groups passing" rather than just "All Passed" / "Some Failed".

**FR-3.8** — **Drag/reorder indicators within a group** is NOT required in this iteration.

### FR-4: Dashboard UI — Group-Aware Status Display

**FR-4.1** — The automation card on the dashboard shall display indicator results grouped by their group, with each group's name and pass/fail status visible.

**FR-4.2** — The "OR" relationship between groups shall be visually communicated on the dashboard (e.g., "OR" divider between groups, or a summary like "Group 'Low VIX' ✓ | Group 'Medium VIX' ✗").

**FR-4.3** — The overall indicator status (pass/fail) on the automation card shall reflect the OR-of-groups logic.

### FR-5: API Changes

**FR-5.1** — `POST /api/automation/configs` and `PUT /api/automation/configs/:id` shall accept the new `indicator_groups` field in the request body.

**FR-5.2** — `GET /api/automation/configs` and `GET /api/automation/configs/:id` shall return `indicator_groups` in the response.

**FR-5.3** — The WebSocket automation update broadcast shall include the new `group_results` field alongside existing fields.

**FR-5.4** — The `POST /api/automation/configs/:id/test-indicators` endpoint (or equivalent) shall return group-level results in addition to individual indicator results.

### FR-6: Storage Migration

**FR-6.1** — When loading `automations.json`, if a config has a flat `indicators` array but no `indicator_groups`, the system shall auto-migrate by wrapping the existing indicators into a single group named "Default".

**FR-6.2** — The migrated config shall be saved back to disk automatically (similar to the existing `migrateIndicatorIDs` pattern).

**FR-6.3** — After migration, the legacy `indicators` field may be kept empty or removed from the config. The `indicator_groups` field becomes the source of truth.

---

## 3. Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| AC-1 | A user can create an automation with multiple indicator groups, each containing different indicator configurations | Create a config with 2+ groups, save, reload — groups persist correctly |
| AC-2 | When any one group's indicators all pass, the automation proceeds to trade (OR logic) | Start automation with 2 groups. Make only one group's conditions true. Verify trade entry proceeds. |
| AC-3 | Within a group, all enabled indicators must pass for the group to pass (AND logic) | In a group with 2 indicators, make one fail. Verify the group does not pass. |
| AC-4 | Existing single-group (legacy flat list) configs load and work without any user action | Load an existing `automations.json` with flat `indicators`. Verify it auto-migrates to a single "Default" group and functions correctly. |
| AC-5 | The config form UI shows groups as visually distinct containers with "OR" labels between them | Visual inspection of the form with 2+ groups |
| AC-6 | Each group shows its own pass/fail status after "Test All" | Click "Test All" with 2+ groups configured, verify per-group badges |
| AC-7 | The dashboard displays indicator results grouped by their group with pass/fail per group | Start an automation with multiple groups, verify dashboard display |
| AC-8 | A group can be added, named, renamed, and deleted from the config form | CRUD operations on groups in the UI |
| AC-9 | An automation with a single group feels no more complex than the current UI | Visual/UX review — single group should be lightweight |
| AC-10 | Stale indicator data in one group does not block other groups from passing | One group has stale data (fails), another group has fresh passing data → automation proceeds |
| AC-11 | All existing automation tests continue to pass (no regressions) | Run `go test ./...` and `npx vitest run` |

---

## 4. Scope Boundaries

### In Scope
- New `IndicatorGroup` data model and OR-of-groups evaluation logic
- UI for creating, editing, and deleting indicator groups within an automation
- Dashboard display of group-level results
- Backward-compatible migration of existing flat indicator configs
- WebSocket updates with group-level results
- Unit tests for the new evaluation logic

### Out of Scope
- Drag-and-drop reordering of indicators between groups
- Drag-and-drop reordering of groups themselves
- Nesting groups within groups (only one level of grouping)
- Duplicating/cloning a group
- Copying indicators between groups
- Any changes to the trade configuration section (shared across all groups, unchanged)
- Any changes to indicator types or their evaluation logic (individual indicators work the same)

---

## 5. Non-Functional Requirements

- **Performance**: Group evaluation should not add meaningful latency. All indicators across all groups are evaluated in the same cycle (no sequential group evaluation).
- **Storage format**: `automations.json` schema versioning should be bumped to reflect the new structure.
- **Backward compatibility**: Existing configs must auto-migrate seamlessly. No data loss.
- **UI responsiveness**: The grouped UI must work well on the existing responsive breakpoints (the app already handles mobile via `isMobile` detection).
