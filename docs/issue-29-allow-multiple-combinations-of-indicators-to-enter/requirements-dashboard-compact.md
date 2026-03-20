# Requirements: Dashboard Card Compact Mode (Progressive Disclosure)

**Issue:** Follow-up to Issue #29 — Indicator Groups  
**Date:** 2025-01-28  
**Status:** Draft — Awaiting Customer Approval

---

## 1. Problem Statement

After implementing Indicator Groups (Issue #29), dashboard cards with multiple groups grow very tall. Each card displays:
- **Entry Criteria** section — all groups with their indicators + OR dividers
- **Trade Config Summary** — entry time, delta, width, max capital
- **Status Details** (when running) — state, message, and **Last Evaluation** with all group results + OR dividers + per-group indicator chips

For a config with 3-4 groups of 3-4 indicators each, this creates a very tall card that breaks the grid layout — cards become uneven heights, and the dashboard feels cluttered and hard to scan.

**Single-group configs are unaffected** — they're already compact. This enhancement targets multi-group configs specifically.

## 2. Solution: Option D — Progressive Disclosure with Two Levels of Collapse

Combine two layers of collapsibility to give users full control over information density:

**Layer 1 — Section-level collapse:** The "Entry Criteria" and "Last Evaluation" sections are **collapsed by default** for multi-group configs, showing only a compact one-line summary. Clicking the summary line expands that section.

**Layer 2 — Group-level collapse:** When a section is expanded, each indicator group shows only its **name + pass/fail badge** by default. Clicking a group expands it to reveal individual indicator details.

This means:
- **Default view (both collapsed):** Card is very compact — just the summary lines
- **Section expanded, groups collapsed:** You see all group names + badges at a glance
- **Specific group expanded:** You drill into that group's indicators

## 3. Functional Requirements

### FR-1: Entry Criteria Section — Compact Summary (Multi-Group Only)

**FR-1.1** When a config has **2+ indicator groups**, the "Entry Criteria" section displays a **compact summary line** by default instead of the full grouped indicator list.

**FR-1.2** The compact summary line shows:
- The section label "Entry Criteria"
- A brief text summary: e.g., `"3 groups · 9 indicators"` (total group count and total enabled indicator count across all groups)
- A chevron icon (▶/▼) indicating expand/collapse state

**FR-1.3** Clicking the summary line toggles the section between collapsed (summary only) and expanded (full grouped indicator display).

**FR-1.4** When expanded, the section shows the full grouped indicator display exactly as it does today (groups with OR dividers, indicator chips per group), **plus** each group is individually collapsible (see FR-3).

**FR-1.5** Single-group configs (1 group or flat indicators) are **unchanged** — they continue to show the flat indicator chips directly, with no collapse toggle. No summary line, no chevron.

### FR-2: Last Evaluation Section — Compact Summary (Multi-Group Only)

**FR-2.1** When a running automation has **2+ group results**, the "Last Evaluation" section within Status Details displays a **compact summary line** by default instead of the full group-by-group results.

**FR-2.2** The compact summary line shows:
- The label "Last Evaluation"
- Overall pass/fail status: e.g., `"✓ Passing (Low VIX)"` or `"✗ Not passing"`
- Group summary: e.g., `"1 of 3 groups passing"`
- A chevron icon indicating expand/collapse state

**FR-2.3** Clicking the summary line toggles the section between collapsed and expanded.

**FR-2.4** When expanded, the section shows the full group-by-group results with OR dividers, per-group pass/fail badges, and per-group indicator chips — exactly as today, **plus** each group is individually collapsible (see FR-3).

**FR-2.5** Single-group or flat indicator results are **unchanged** — they display as today with no collapse toggle.

### FR-3: Group-Level Collapse (Within Expanded Sections)

**FR-3.1** When either the Entry Criteria or Last Evaluation section is expanded for a multi-group config, each **individual group** is collapsible.

**FR-3.2** Collapsed group shows: group name + pass/fail badge (for Last Evaluation) or group name + indicator count (for Entry Criteria). One line per group.

**FR-3.3** Expanded group shows: the full indicator chips/results for that group (as today).

**FR-3.4** Default state when section is expanded: all groups are **collapsed** (showing name + badge only). This keeps the expanded section compact while showing the group overview.

**FR-3.5** Clicking a group header toggles that individual group between collapsed and expanded.

### FR-4: Visual Design

**FR-4.1** The expand/collapse chevron uses PrimeVue icon classes (`pi pi-chevron-right` for collapsed, `pi pi-chevron-down` for expanded).

**FR-4.2** The summary line is styled as a clickable row with `cursor: pointer` and a subtle hover effect.

**FR-4.3** Transitions: Use a simple CSS transition or no transition for expand/collapse. Keep it snappy — no slow animations.

**FR-4.4** All styling uses existing CSS custom properties (design tokens). No new theme tokens needed.

**FR-4.5** The OR dividers between groups remain visible in the expanded state, unchanged from the current implementation.

### FR-5: State Management

**FR-5.1** Expand/collapse state is **per-card, per-section** — tracked locally within the component. Not persisted across page reloads.

**FR-5.2** Each card independently tracks:
- Whether its Entry Criteria section is expanded (default: collapsed for multi-group)
- Whether its Last Evaluation section is expanded (default: collapsed for multi-group)
- Which individual groups within each section are expanded (default: all collapsed)

**FR-5.3** When new data arrives via WebSocket updates, the collapse state is preserved — the card does not re-collapse or re-expand on data refresh.

### FR-6: Evaluation Dialog

**FR-6.1** The Evaluation Result Dialog (shown when clicking "Test Indicators") is **not affected** by this change. It continues to show the full detailed results since it's a dedicated modal for reviewing results.

## 4. Scope Boundaries — What is NOT Included

- **No changes to single-group configs** — they remain exactly as today
- **No changes to the AutomationConfigForm** — the config editing form is unaffected
- **No changes to the Evaluation Dialog** — the modal test results popup stays as-is
- **No changes to backend/API** — this is purely a frontend display enhancement
- **No persistence** of expand/collapse state — it resets on page reload
- **No max-height / scrollable areas** — we use collapse instead of scroll
- **No changes to mobile behavior** — the existing mobile collapse (full card collapse) remains; this adds section-level collapse within expanded cards

## 5. Acceptance Criteria

| ID | Criterion |
|----|-----------|
| AC-1 | Dashboard card with 2+ indicator groups shows a compact summary line for "Entry Criteria" section by default (collapsed), not the full group list |
| AC-2 | Clicking the Entry Criteria summary line expands it to show all groups |
| AC-3 | Dashboard card with 2+ group results shows a compact summary line for "Last Evaluation" section by default (collapsed) |
| AC-4 | Clicking the Last Evaluation summary line expands it to show all group results |
| AC-5 | When a section is expanded, each group shows only its name + badge (collapsed); clicking a group header expands it to show individual indicators |
| AC-6 | Single-group configs display exactly as they do today — no collapse toggles, no summary lines |
| AC-7 | Expand/collapse state persists across WebSocket data updates (card doesn't re-collapse when new status arrives) |
| AC-8 | All existing dashboard functionality (start/stop, edit, delete, logs, evaluate, duplicate, toggle) remains unaffected |
| AC-9 | Cards in the grid have more consistent heights when multiple multi-group configs are present |
| AC-10 | All existing tests continue to pass (no regressions) |
