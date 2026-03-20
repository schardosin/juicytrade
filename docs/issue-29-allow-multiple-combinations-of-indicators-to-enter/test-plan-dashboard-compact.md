# QA Test Plan: Dashboard Card Compact Mode (Progressive Disclosure)

**Issue:** Follow-up to Issue #29 — Indicator Groups
**Component:** `AutomationDashboard.vue`
**Date:** 2025-01-28
**Dev tests:** 29 tests in `AutomationDashboard.test.js` (happy-path coverage)
**Existing QA tests:** 16 tests in `qa-AutomationDashboard.test.js` (group rendering, OR dividers, backward compat)

---

## Scope

This plan covers edge cases, boundary conditions, adversarial data shapes, state management subtleties, multi-card interactions, rapid toggling, and data-update-during-interaction scenarios that the 29 dev tests do not exercise. Each step maps to one or more acceptance criteria (AC-1 through AC-10).

---

## Step 1 — Baseline Regression Run (AC-10)

Run the full existing test suite before writing any new QA tests to confirm a green baseline.

| # | Action | Expected |
|---|--------|----------|
| 1.1 | `cd trade-app && npx vitest run` | All 29 dev tests + 16 existing QA tests pass |
| 1.2 | Visually inspect terminal output for deprecation warnings or console errors during test run | No unexpected warnings |

---

## Step 2 — Boundary: Exactly 2 Groups (Minimum Multi-Group Threshold) (AC-1, AC-3, AC-6)

The dev tests use 2- and 3-group configs. This step focuses on the boundary at exactly 2 groups (the threshold where compact mode activates) and verifies single-group stays flat.

| # | Test | AC |
|---|------|----|
| 2.1 | Config with exactly 2 groups (1 indicator each): Entry Criteria section renders collapsed summary header, not flat chips | AC-1 |
| 2.2 | Config with exactly 2 group_results (1 indicator each): Last Evaluation section renders collapsed summary, not flat result chips | AC-3 |
| 2.3 | Config transitions from 1 group to 2 groups (simulated by replacing `configs` ref): confirm collapse header appears after update | AC-1 |
| 2.4 | Config with exactly 1 group still renders flat — no `.section-collapse-header` in indicators section | AC-6 |
| 2.5 | Config with 0 groups and legacy `indicators` array still renders flat | AC-6 |

---

## Step 3 — Adversarial / Unusual Data Shapes (AC-1, AC-3, AC-6)

The dev tests use well-formed data. These tests probe malformed, empty, and edge-case data payloads.

| # | Test | AC |
|---|------|----|
| 3.1 | Multi-group config where every group has an empty `indicators: []` array — summary should read `"2 groups · 0 indicators"` | AC-1 |
| 3.2 | Multi-group config where `indicators` property is missing from a group (`undefined`) — `getEntrySummary` should not throw, should count 0 for that group | AC-1 |
| 3.3 | Multi-group config where one group has `indicators: null` — guard against `.filter()` on null | AC-1 |
| 3.4 | Status with `group_results: []` (empty array) — `getEvalSummary` returns null, template uses flat path, no crash | AC-3 |
| 3.5 | Status with `group_results` containing a group with `indicator_results: []` (no indicators evaluated) — expand section + expand group = no result chips, no error | AC-5 |
| 3.6 | Status with `group_results` containing a group with `indicator_results: undefined` — expand should not throw | AC-5 |
| 3.7 | Config where `indicator_groups` is `null` (not `undefined`, not `[]`) — `isMultiGroup` returns false, flat display, no crash | AC-6 |
| 3.8 | `getEvalSummary` called with a config ID whose status has `group_results` but every group has `pass: undefined` (truthy/falsy edge) — verify `passing` resolves to `false` | AC-3 |
| 3.9 | Group with `group_name: null` — group-collapse-header should fall back to `"Group N"` text | AC-5 |
| 3.10 | Group with `group_name: ''` (empty string) — group-collapse-header should fall back to `"Group N"` (template uses `group.name \|\| 'Group ' + (gIdx + 1)`) | AC-5 |

---

## Step 4 — Large / Extreme Configs (AC-1, AC-3, AC-9)

Stress test the summary counts and DOM rendering with unusually large configs.

| # | Test | AC |
|---|------|----|
| 4.1 | Config with 20 groups, each containing 10 enabled indicators — entry summary reads `"20 groups · 200 indicators"` | AC-1 |
| 4.2 | Same config, expand section → 20 group-collapse-headers render, 19 OR dividers render | AC-2, AC-5 |
| 4.3 | Status with 20 group_results — eval summary line correctly shows `"X of 20 groups passing"` | AC-3 |
| 4.4 | Config with 2 groups but one group has 50 enabled indicators — verify indicator count in group-collapse-header shows `"50 indicators"` | AC-1 |

---

## Step 5 — Multi-Card Independence (AC-7, AC-1, AC-3)

The dev tests check card-A vs card-B state isolation at the `vm` level. These tests verify it through rendered DOM with multiple cards.

| # | Test | AC |
|---|------|----|
| 5.1 | Mount 3 cards: card A (multi-group), card B (multi-group), card C (single-group). Expand card A entry section → card B entry section stays collapsed, card C has no collapse header | AC-1, AC-6 |
| 5.2 | Expand card A eval section + expand group 0. Expand card B eval section → card B group 0 is collapsed (not influenced by card A) | AC-5, AC-7 |
| 5.3 | Collapse card A entry section back. Verify card A's `expandedEntrySections` is false while card B remains as set | AC-7 |
| 5.4 | Two multi-group cards: expand entry on both, expand group 1 on card A, expand group 2 on card B. Verify cross-card group state is independent via `isGroupExpanded` | AC-7 |

---

## Step 6 — Rapid Toggle Sequences (AC-2, AC-4, AC-5, AC-7)

Test rapid repeated clicks to ensure toggle logic is idempotent and does not produce stale or mismatched state.

| # | Test | AC |
|---|------|----|
| 6.1 | Click entry section header 10 times rapidly (alternating expand/collapse) — final state matches expected parity (odd = expanded, even = collapsed) | AC-2 |
| 6.2 | Click eval section header 10 times rapidly — same parity check | AC-4 |
| 6.3 | Expand entry section, then click group 0 header 10 times rapidly — final state matches expected parity | AC-5 |
| 6.4 | Expand entry section, expand group 0, then immediately collapse the section. Re-expand section → group 0 should still be expanded (group state survives section collapse/re-expand) | AC-5, AC-7 |
| 6.5 | Expand eval section, expand group 0, collapse eval section. Simulate WebSocket status update. Re-expand eval section → group 0 should still be expanded | AC-7 |

---

## Step 7 — State Persistence Across Data Updates (AC-7)

The dev tests check one data update scenario. These tests cover more update patterns.

| # | Test | AC |
|---|------|----|
| 7.1 | Expand entry section + group 0. Update `configs` ref with new indicator data for the same config ID (e.g., threshold changes). Verify entry section and group 0 remain expanded | AC-7 |
| 7.2 | Expand eval section. Simulate `handleAutomationUpdate` WebSocket message with new group_results (different values, same structure). Verify eval section stays expanded | AC-7 |
| 7.3 | Expand eval section + group 1. Simulate WebSocket update that changes the number of groups from 3 to 2 (group removed). Verify: eval section stays expanded, group 1 key still in `expandedGroups` but group 2 is no longer rendered (no crash) | AC-7 |
| 7.4 | Expand eval section + group 0. Simulate WebSocket update that changes the number of groups from 2 to 3 (group added). Verify: eval section stays expanded, group 0 still expanded, new group 2 defaults to collapsed | AC-7 |
| 7.5 | Expand entry section. Replace `configs` entirely with a new array containing the same config ID but different group count. Verify entry section remains expanded | AC-7 |
| 7.6 | Status transitions from no `group_results` (flat path) to having `group_results` with 2+ entries (grouped path) while card is visible. Verify: DOM switches to collapsed grouped view without crash | AC-7 |
| 7.7 | Status transitions from `group_results` with 3 entries to `group_results` with 1 entry. Verify: DOM switches back to flat result chip rendering | AC-7 |

---

## Step 8 — Entry Criteria Summary Text Accuracy (AC-1)

Validate the `getEntrySummary` return value for various combinations of enabled/disabled indicators.

| # | Test | AC |
|---|------|----|
| 8.1 | 3 groups with [3 enabled, 0 disabled], [2 enabled, 1 disabled], [1 enabled, 2 disabled] — summary = `"3 groups · 6 indicators"` | AC-1 |
| 8.2 | 2 groups with all indicators `enabled: false` in both — summary = `"2 groups · 0 indicators"` | AC-1 |
| 8.3 | 4 groups with 1 indicator each, all enabled — summary = `"4 groups · 4 indicators"` | AC-1 |
| 8.4 | Rendered summary text in `.collapse-summary-text` matches `getEntrySummary()` return value (DOM vs computed agreement) | AC-1 |

---

## Step 9 — Last Evaluation Summary Text Accuracy (AC-3)

Validate the `getEvalSummary` return value for edge-case group_results data.

| # | Test | AC |
|---|------|----|
| 9.1 | All groups pass — `{ passing: true, passingGroupName: <first group name>, summary: "3 of 3 groups passing" }` | AC-3 |
| 9.2 | No groups pass — `{ passing: false, passingGroupName: null, summary: "0 of 3 groups passing" }` | AC-3 |
| 9.3 | Multiple groups pass — `passingGroupName` is the **first** passing group (array order) | AC-3 |
| 9.4 | Rendered collapsed summary DOM includes checkmark entity for passing, cross entity for not-passing | AC-3 |
| 9.5 | Summary with exactly 1 group passing out of 10 — `"1 of 10 groups passing"` | AC-3 |

---

## Step 10 — Chevron Icon Correctness (AC-2, AC-4, AC-5)

Verify the chevron icon classes toggle correctly at both section and group levels.

| # | Test | AC |
|---|------|----|
| 10.1 | Collapsed entry section → chevron has class `pi-chevron-right` | AC-2 |
| 10.2 | Expanded entry section → chevron has class `pi-chevron-down` | AC-2 |
| 10.3 | Collapsed eval section → chevron has class `pi-chevron-right` | AC-4 |
| 10.4 | Expanded eval section → chevron has class `pi-chevron-down` | AC-4 |
| 10.5 | Collapsed group within expanded section → chevron has class `pi-chevron-right` | AC-5 |
| 10.6 | Expanded group → chevron has class `pi-chevron-down` | AC-5 |
| 10.7 | Collapse a section and re-expand → chevron returns to `pi-chevron-down` | AC-2 |

---

## Step 11 — Interaction with Existing Actions (AC-8)

Verify that the card action buttons continue to work correctly when sections are in various expand/collapse states.

| # | Test | AC |
|---|------|----|
| 11.1 | With entry section expanded: click Start Automation → `api.startAutomation` called with correct config ID | AC-8 |
| 11.2 | With eval section expanded + group expanded: click Stop Automation → `api.stopAutomation` called with correct config ID | AC-8 |
| 11.3 | With entry section expanded: click Test Indicators → evaluation dialog opens with correct results, card collapse state unchanged after dialog closes | AC-8 |
| 11.4 | With eval section expanded: click Edit → router navigates to edit route | AC-8 |
| 11.5 | With entry section expanded: click Duplicate → `api.createAutomationConfig` called, new config appears, original card's collapse state unchanged | AC-8 |
| 11.6 | With entry section expanded: click Delete → confirmation dialog appears, confirming deletes config, remaining cards' collapse states unchanged | AC-8 |
| 11.7 | With eval section expanded: click Toggle (enable/disable) → config toggled, collapse state of other cards unchanged | AC-8 |
| 11.8 | Click View Logs with eval section expanded → logs dialog opens, eval section state unchanged when dialog closes | AC-8 |

---

## Step 12 — Evaluation Dialog Unaffected (AC-8)

Verify the evaluation result dialog is not affected by compact mode.

| # | Test | AC |
|---|------|----|
| 12.1 | Multi-group config: trigger evaluateIndicators → dialog shows full multi-group results (no collapse controls in dialog), all indicator details visible | AC-8 |
| 12.2 | Single-group config: trigger evaluateIndicators → dialog shows flat results, no group headers | AC-8 |
| 12.3 | Close evaluation dialog → card's collapse state is unchanged | AC-8 |

---

## Step 13 — Mixed Card Types in Grid (AC-6, AC-9)

Test a realistic dashboard with a mix of single-group, multi-group, legacy, and idle/running configs.

| # | Test | AC |
|---|------|----|
| 13.1 | Mount 5 cards: [legacy flat, single-group, 2-group, 3-group, disabled 2-group]. Verify: only 2-group and 3-group cards have `.section-collapse-header` in entry section | AC-6 |
| 13.2 | Cards with collapsed multi-group sections should not have `.indicators-grid` rendered inside entry section (verifying DOM compactness) | AC-9 |
| 13.3 | Running multi-group card has collapsed eval section by default while idle single-group card has no eval section at all — both render without errors | AC-9 |
| 13.4 | Disabled multi-group card still renders collapsed entry section (compact mode is independent of enabled/disabled state) | AC-6 |

---

## Step 14 — WebSocket Handler Edge Cases (AC-7)

Test the `handleAutomationUpdate` handler with unusual WebSocket payloads.

| # | Test | AC |
|---|------|----|
| 14.1 | WebSocket message with `automation_id` that doesn't match any current config — statuses updated without crash, no stale DOM | AC-7 |
| 14.2 | WebSocket message with `data: null` — handler does not crash (guard: `message.data` is falsy) | AC-7 |
| 14.3 | WebSocket message with `data: {}` (no status, no group_results) — statuses updated with empty fields, no crash | AC-7 |
| 14.4 | Two rapid WebSocket messages for the same `automation_id` with different `group_results` — final state reflects the last message | AC-7 |
| 14.5 | WebSocket message arrives while entry section is expanded and user is mid-interaction (group toggle in progress) — collapse state preserved | AC-7 |

---

## Step 15 — Group Expand State Key Integrity (AC-5, AC-7)

The implementation uses composite keys `${configId}__${section}__${groupIdx}`. Test key correctness.

| # | Test | AC |
|---|------|----|
| 15.1 | Expand entry group 0 and eval group 0 for the same config — both are independently expanded (different section in key) | AC-5 |
| 15.2 | Expand entry group 0 on card A and entry group 0 on card B — both are independently expanded (different configId in key) | AC-5, AC-7 |
| 15.3 | After expanding entry group 2, collapse entry section, then re-expand — `isGroupExpanded(configId, 'entry', 2)` still returns true | AC-7 |
| 15.4 | Config ID containing double underscores (e.g., `"cfg__special"`) — verify key `"cfg__special__entry__0"` still works correctly (no key collision) | AC-7 |

---

## Step 16 — Final Regression Run (AC-10)

Run the full test suite after all QA tests are implemented to confirm nothing was broken.

| # | Action | Expected |
|---|--------|----------|
| 16.1 | `cd trade-app && npx vitest run` | All dev tests (29) + existing QA tests (16) + new QA tests pass |
| 16.2 | Review test output for any new console warnings or errors logged during tests | No unexpected warnings |
| 16.3 | Verify no tests are skipped or marked `.todo` unintentionally | All tests executed |

---

## Summary

| Step | Focus Area | Test Count | Primary ACs |
|------|-----------|------------|-------------|
| 1 | Baseline regression | 2 | AC-10 |
| 2 | Boundary: exactly 2 groups | 5 | AC-1, AC-3, AC-6 |
| 3 | Adversarial data shapes | 10 | AC-1, AC-3, AC-5, AC-6 |
| 4 | Large / extreme configs | 4 | AC-1, AC-2, AC-3, AC-9 |
| 5 | Multi-card independence (DOM) | 4 | AC-1, AC-5, AC-6, AC-7 |
| 6 | Rapid toggle sequences | 5 | AC-2, AC-4, AC-5, AC-7 |
| 7 | State persistence across updates | 7 | AC-7 |
| 8 | Entry summary text accuracy | 4 | AC-1 |
| 9 | Eval summary text accuracy | 5 | AC-3 |
| 10 | Chevron icon correctness | 7 | AC-2, AC-4, AC-5 |
| 11 | Existing actions unaffected | 8 | AC-8 |
| 12 | Evaluation dialog unaffected | 3 | AC-8 |
| 13 | Mixed card types in grid | 4 | AC-6, AC-9 |
| 14 | WebSocket handler edge cases | 5 | AC-7 |
| 15 | Group expand key integrity | 4 | AC-5, AC-7 |
| 16 | Final regression | 3 | AC-10 |
| **Total** | | **80** | |
