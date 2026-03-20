# Test Plan: Remove Inner Group-Level Collapse

**Parent Issue:** #29 — Allow multiple combinations of indicators to enter a trade
**Refinement:** Remove inner group-level collapse from dashboard cards
**Component:** `trade-app/src/components/automation/AutomationDashboard.vue`
**Test File:** `trade-app/tests/qa-remove-inner-collapse.test.js` (new)

## Scope

This plan validates the removal of the inner group-level collapse mechanism. The change is:
- **Before:** Expanding a section showed collapsed groups; each group required a second click to see its data.
- **After:** Expanding a section shows ALL groups with ALL data immediately. Group headers are static (non-clickable, no chevron).

Section-level collapse (outer) is unchanged. Single-group cards are unchanged.

## Existing Test Coverage

| File | Tests | Status |
|------|-------|--------|
| `AutomationDashboard.test.js` | ~45 | Already updated for static group headers |
| `qa-AutomationDashboard.test.js` | ~20 | Already updated (expands section, not groups) |
| `qa-dashboard-compact.test.js` | ~75 | Already updated for no group-level collapse |

These existing tests cover: section toggle mechanics, summary text accuracy, chevron icons, multi-card independence, rapid toggles, state persistence, WebSocket handler edge cases, adversarial data shapes, large configs, action interactions, evaluation dialog, and mixed card types.

**This plan focuses exclusively on NEW tests** that verify the removal itself and the specific "all data visible on expand" behavior that was not previously testable (because inner collapse existed).

## Acceptance Criteria Mapping

| AC | Description | Covered By Steps |
|----|-------------|-----------------|
| AC-1 | Entry Criteria expand shows ALL groups + indicator chips immediately | 3 |
| AC-2 | Last Evaluation expand shows ALL groups + result chips immediately | 4 |
| AC-3 | Group headers are static (not clickable, no chevron icons) | 5 |
| AC-4 | No references to expandedGroups/toggleGroup/isGroupExpanded/group-collapse-header/collapse-chevron-sm | 2 |
| AC-5 | Section-level collapse still works exactly as before | 6 |
| AC-6 | Single-group cards completely unchanged | 7 |
| AC-7 | All existing tests pass (no regressions) | 1, 8 |

---

## Step 1 — Baseline Regression Run (AC-7)

**Goal:** Confirm all existing tests pass before adding new tests.

**Action:** Run all three test files and record pass/fail counts.

```sh
cd trade-app && npx vitest run tests/AutomationDashboard.test.js tests/qa-AutomationDashboard.test.js tests/qa-dashboard-compact.test.js
```

**Expected:** All tests pass. Any pre-existing failures (e.g., CollapsibleOptionsChain) are noted but excluded.

**No new tests in this step.**

---

## Step 2 — Code Removal Verification (AC-4)

**Goal:** Confirm the inner group-level collapse code is fully removed from the component source.

These are static code-analysis tests that grep the component source text. They ensure no remnants of the old group collapse mechanism remain.

| # | Test | Assertion |
|---|------|-----------|
| 2.1 | Component source contains no `expandedGroups` | `readFileSync()` of `.vue` file does not contain `expandedGroups` |
| 2.2 | Component source contains no `toggleGroup` | Does not contain `toggleGroup` |
| 2.3 | Component source contains no `isGroupExpanded` | Does not contain `isGroupExpanded` |
| 2.4 | Component source contains no `group-collapse-header` class | Does not contain `group-collapse-header` |
| 2.5 | Component source contains no `collapse-chevron-sm` class | Does not contain `collapse-chevron-sm` |
| 2.6 | Component source contains `group-header-static` class (replacement) | Contains `group-header-static` |

**6 tests**

---

## Step 3 — Entry Criteria: All Groups Visible on Expand (AC-1, AC-3)

**Goal:** When the Entry Criteria section is expanded, ALL groups and ALL indicator chips are immediately visible — no second interaction needed.

Fixtures: 2-group config, 3-group config, 5-group config (variety of group sizes).

| # | Test | Assertion |
|---|------|-----------|
| 3.1 | 2-group: expand entry → both groups' indicator chips rendered in DOM immediately | `.indicators-grid` count = 2; total `.indicator-chip` count = sum of all enabled indicators across both groups |
| 3.2 | 3-group: expand entry → all 3 groups' indicator chips rendered, correct chip count per group | For each `.indicator-group-dashboard`, count `.indicator-chip` matches that group's enabled indicator count |
| 3.3 | 5-group (mixed sizes: 1, 3, 0, 2, 4 indicators): expand entry → 5 groups rendered, indicator chip counts match per group, empty group has 0 chips | Per-group chip count: [1, 3, 0, 2, 4]; total chips = 10 |
| 3.4 | 3-group: expand entry → chip text content matches formatted indicator types and conditions | For each indicator chip: `.indicator-name` text matches `formatIndicatorType(type)`, `.indicator-value` text matches `formatIndicatorCondition(indicator)` |
| 3.5 | 3-group: expand entry → OR dividers rendered between groups (count = groups - 1) | `.or-divider-compact` count = 2; `.or-divider-compact-label` text = "OR" |

**5 tests**

---

## Step 4 — Last Evaluation: All Groups Visible on Expand (AC-2, AC-3)

**Goal:** When the Last Evaluation section is expanded, ALL groups' result chips (with pass/fail coloring, values, stale icons) are immediately visible.

Fixtures: 2-group and 3-group configs with running statuses, varied pass/fail/stale indicator results.

| # | Test | Assertion |
|---|------|-----------|
| 4.1 | 2-group: expand eval → both groups' result chips rendered immediately, correct chip count | `.results-grid` count = 2; total `.result-chip` count matches sum of indicator_results across groups |
| 4.2 | 3-group (mixed: 2 pass, 1 fail, includes stale): expand eval → all 3 groups rendered with correct pass/fail badge classes and stale chip styling | Group badges: `.group-result-badge.passed` count = 2, `.group-result-badge.failed` count = 1; `.result-chip.stale` count = 1 with `.stale-icon` present |
| 4.3 | 3-group: expand eval → result chip values match `formatIndicatorValue()` output | Each `.result-chip` text contains the formatted value (e.g., "15.50") |
| 4.4 | 3-group: expand eval → overall status line rendered at bottom with correct passing/failing class | `.overall-status.passing` exists when `all_indicators_pass: true`; text contains passing group name |
| 4.5 | 2-group: expand eval → OR dividers between groups (count = 1) | `.or-divider-compact` in `.section-collapse-body` count = 1 |

**5 tests**

---

## Step 5 — Static Group Headers: Not Clickable, No Chevrons (AC-3)

**Goal:** Group headers use `.group-header-static`, have no click handler, no `cursor: pointer`, and no chevron icons at the group level.

| # | Test | Assertion |
|---|------|-----------|
| 5.1 | Expanded entry section: `.group-header-static` elements exist, `.group-collapse-header` does not exist | `.group-header-static` count = group count; `.group-collapse-header` count = 0 |
| 5.2 | Expanded eval section: `.group-header-static` elements exist, `.group-collapse-header` does not exist | Same as 5.1 for eval section |
| 5.3 | Entry `.group-header-static` has no `collapse-chevron-sm` or `collapse-chevron` child elements | `groupHeaderStatic.find('.collapse-chevron-sm')` = false; `groupHeaderStatic.find('.collapse-chevron')` = false (for each static header in entry) |
| 5.4 | Eval `.group-header-static` has no `collapse-chevron-sm` or `collapse-chevron` child elements | Same check in eval section |
| 5.5 | Clicking a `.group-header-static` in entry section does not change DOM (no toggle behavior) | Click `.group-header-static`, `await nextTick()`, verify `.indicators-grid` count unchanged, no new elements appear or disappear |
| 5.6 | Clicking a `.group-header-static` in eval section does not change DOM (no toggle behavior) | Click `.group-header-static` in eval, `await nextTick()`, verify `.results-grid` count unchanged |

**6 tests**

---

## Step 6 — Section-Level Collapse Still Works (AC-5)

**Goal:** The section-level collapse/expand for Entry Criteria and Last Evaluation is unchanged.

These tests specifically verify that section-level mechanics still function correctly in the context of the removed inner collapse (i.e., the section-level collapse is the ONLY collapse mechanism now).

| # | Test | Assertion |
|---|------|-----------|
| 6.1 | Multi-group: Entry section collapsed by default → expand → all groups visible → collapse → summary line returns, no groups in DOM | After collapse: `.section-collapse-body` absent, `.indicator-group-dashboard` count = 0; summary text still correct |
| 6.2 | Multi-group: Eval section collapsed by default → expand → all results visible → collapse → summary returns, no result chips in DOM | After collapse: `.section-collapse-body` absent, `.results-grid` count = 0 within eval; collapsed summary text correct |
| 6.3 | Expand entry, then expand eval on same card → both sections expanded simultaneously, all groups visible in both | Entry `.section-collapse-body` exists with N indicator grids; Eval `.section-collapse-body` exists with N results grids |
| 6.4 | Collapse entry while eval stays expanded → entry groups gone, eval groups still visible | Entry `.section-collapse-body` absent; Eval `.section-collapse-body` present with all results-grids |

**4 tests**

---

## Step 7 — Single-Group Cards Unchanged (AC-6)

**Goal:** Single-group cards render identically to before — flat display, no collapse infrastructure, no group headers.

| # | Test | Assertion |
|---|------|-----------|
| 7.1 | Single-group config: no `.section-collapse-header`, no `.group-header-static`, no `.indicator-group-dashboard` in entry section | All three selectors return 0 matches |
| 7.2 | Single-group running config: flat `.result-chip` elements rendered, no `.group-header-static`, no `.section-collapse-header` in eval | `.result-chip` count >= 1; group infrastructure absent |
| 7.3 | Legacy config (no `indicator_groups`): renders flat indicator chips, no group infrastructure | `.indicator-chip` count matches enabled indicators; no `.group-header-static`, no `.section-collapse-header` |

**3 tests**

---

## Step 8 — Final Regression Run (AC-7)

**Goal:** After adding all new tests, run the full suite to confirm no regressions.

**Action:** Run all four test files (existing 3 + new qa-remove-inner-collapse.test.js).

```sh
cd trade-app && npx vitest run tests/AutomationDashboard.test.js tests/qa-AutomationDashboard.test.js tests/qa-dashboard-compact.test.js tests/qa-remove-inner-collapse.test.js
```

**Expected:** All tests pass.

**No new tests in this step.**

---

## Summary

| Step | Focus | New Tests |
|------|-------|-----------|
| 1 | Baseline regression run | 0 |
| 2 | Code removal verification (AC-4) | 6 |
| 3 | Entry Criteria all-visible (AC-1, AC-3) | 5 |
| 4 | Last Evaluation all-visible (AC-2, AC-3) | 5 |
| 5 | Static headers (AC-3) | 6 |
| 6 | Section-level collapse (AC-5) | 4 |
| 7 | Single-group unchanged (AC-6) | 3 |
| 8 | Final regression run (AC-7) | 0 |
| **Total** | | **29** |

## Test File Location

All 29 new tests go into a single file:

```
trade-app/tests/qa-remove-inner-collapse.test.js
```

This keeps them cleanly separated from the existing `qa-dashboard-compact.test.js` (which tests compact mode mechanics broadly) and focused specifically on the inner collapse removal.
