# QA Test Plan: Indicator Groups for Trade Automation (Issue #29)

**Issue:** [#29 — Allow multiple combinations of indicators to enter a trade](https://github.com/schardosin/juicytrade/issues/29)
**Requirements:** [requirements.md](./requirements.md)
**Architecture:** [architecture.md](./architecture.md)

---

## Overview

This plan covers seven sequential test steps. Steps 1 and 7 are regression gates (full suite runs). Steps 2–6 are QA-specific test suites targeting edge cases and acceptance criteria not covered by the existing implementation tests. QA test files use the `qa_` prefix for Go and `qa-` prefix for JavaScript, per project conventions.

**Acceptance Criteria Cross-Reference:**

| AC | Description | Covered In |
|----|-------------|------------|
| AC-1 | Multiple indicator groups persist across save/reload | Steps 4, 5 |
| AC-2 | OR logic — any group passing triggers trade | Steps 2, 3 |
| AC-3 | AND logic — all enabled indicators in group must pass | Steps 2, 3 |
| AC-4 | Legacy flat configs auto-migrate to single "Default" group | Steps 2, 4 |
| AC-5 | Config form shows groups as distinct containers with "OR" labels | Step 5 |
| AC-6 | Per-group pass/fail after "Test All" | Step 5 |
| AC-7 | Dashboard displays grouped results with per-group pass/fail | Step 6 |
| AC-8 | Group CRUD (add, name, rename, delete) | Step 5 |
| AC-9 | Single-group UX feels lightweight | Steps 5, 6 |
| AC-10 | Stale data in one group doesn't block other groups | Step 3 |
| AC-11 | All existing tests pass (no regressions) | Steps 1, 7 |

---

## Step 1: Baseline Regression Run

**Goal:** Confirm all existing tests pass before adding QA tests.

**Commands:**

```sh
# Backend
cd trade-backend-go && go test ./...

# Frontend
cd trade-app && npx vitest run
```

**Pass Criteria:**
- 0 test failures across both suites.
- This establishes the green baseline. If any test fails here, it must be investigated as a pre-existing issue before proceeding.

---

## Step 2: QA Tests for `types/types.go`

**File:** `trade-backend-go/internal/automation/types/qa_types_test.go`
**Run:** `cd trade-backend-go && go test ./internal/automation/types/ -run QA -v`

### 2.1 `GetEffectiveIndicatorGroups` — Edge Cases

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 2.1.1 | `QA_GetEffectiveIndicatorGroups_EmptyGroupsNonEmptyIndicators` | `IndicatorGroups` is `[]IndicatorGroup{}` (empty slice, not nil) but `Indicators` has entries. Verify legacy fallback wraps into "Default" group. The distinction between nil vs empty slice matters for JSON deserialization. | AC-4 |
| 2.1.2 | `QA_GetEffectiveIndicatorGroups_NilGroupsNilIndicators` | Both fields are nil (not initialized). Verify returns empty slice `[]IndicatorGroup{}` without panic. | AC-4 |
| 2.1.3 | `QA_GetEffectiveIndicatorGroups_GroupsWithEmptyIndicators` | `IndicatorGroups` has one group with zero indicators. Verify it's returned as-is (the group exists but is empty — vacuously true behavior is the evaluator's concern, not the helper's). | AC-10 |
| 2.1.4 | `QA_GetEffectiveIndicatorGroups_MultipleGroupsPreserveOrder` | `IndicatorGroups` has 3 groups. Verify the returned slice preserves insertion order (group IDs and names match in sequence). | AC-1 |
| 2.1.5 | `QA_GetEffectiveIndicatorGroups_LegacyFallbackIDAndName` | When falling back to legacy, verify the synthetic group has exactly `ID: "default"` and `Name: "Default"`. These values are depended upon by the frontend for display. | AC-4 |

### 2.2 JSON Serialization Round-Trip

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 2.2.1 | `QA_AutomationConfig_JSONRoundTrip_WithGroups` | Create an `AutomationConfig` with 2 indicator groups (each with 2 indicators including params). Marshal to JSON, unmarshal back. Verify all fields survive: group IDs, group names, indicator IDs, types, operators, thresholds, params, enabled state, and symbols. | AC-1 |
| 2.2.2 | `QA_AutomationConfig_JSONRoundTrip_EmptyGroups` | Config with `IndicatorGroups: []` (empty). Marshal/unmarshal. Verify `indicator_groups` deserializes to empty slice (not nil) when `omitempty` tag is present — check that a JSON payload with `"indicator_groups": []` round-trips correctly vs one with the field omitted. | AC-4 |
| 2.2.3 | `QA_AutomationConfig_JSONRoundTrip_LegacyFormat` | Simulate a v1.0 JSON blob (has `indicators` array, no `indicator_groups` key). Unmarshal into `AutomationConfig`. Verify `IndicatorGroups` is nil/empty and `Indicators` is populated. Then call `GetEffectiveIndicatorGroups()` and verify fallback works. | AC-4 |
| 2.2.4 | `QA_GroupResult_JSONRoundTrip` | Create a `GroupResult` with nested `IndicatorResult` entries (including one with `Stale: true`, one with `Error` set, one with `LastGoodValue`). Marshal/unmarshal. Verify all nested fields survive, especially optional pointer fields. | AC-7 |
| 2.2.5 | `QA_ActiveAutomation_JSONRoundTrip_WithGroupResults` | Create `ActiveAutomation` with both `IndicatorResults` (flat) and `GroupResults` populated. Marshal/unmarshal. Verify both are present. Verify `AllIndicatorsPass` survives. This validates the WebSocket broadcast payload shape. | AC-7 |

### 2.3 `GroupResult` Struct Validation

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 2.3.1 | `QA_GroupResult_PassFieldReflectsIndicators` | Construct `GroupResult` manually with `Pass: true` but `IndicatorResults` containing a failing indicator. Verify the struct doesn't self-validate (it's a data container — `Pass` is set by the evaluator). This documents the contract. | — |
| 2.3.2 | `QA_GroupResult_EmptyIndicatorResults` | Construct `GroupResult` with empty `IndicatorResults`. Verify JSON serialization produces `"indicator_results": []` (not null). The frontend depends on iterating this array. | AC-7 |

### 2.4 ID Generation

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 2.4.1 | `QA_GenerateGroupID_Uniqueness` | Generate 100 group IDs in a loop. Verify all are unique and all start with `grp_`. | — |
| 2.4.2 | `QA_GenerateIndicatorID_Uniqueness` | Generate 100 indicator IDs in a loop. Verify all are unique and all start with `ind_`. | — |

---

## Step 3: QA Tests for `indicators/service.go`

**File:** `trade-backend-go/internal/automation/indicators/qa_service_test.go`
**Run:** `cd trade-backend-go && go test ./internal/automation/indicators/ -run QA -v`

These tests use the `computeGroupORLogic` helper (already defined in `service_test.go`) to test evaluation logic without provider dependencies.

### 3.1 `EvaluateIndicatorGroups` — Additional Edge Cases

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 3.1.1 | `QA_GroupEval_AllDisabledIndicatorsInGroup` | One group where all indicators have `Enabled: false`. `AllIndicatorsPass` returns true for that group (vacuously true per FR-2.3). Verify `anyGroupPasses = true`. | AC-10 |
| 3.1.2 | `QA_GroupEval_MixedDisabledAndEnabled` | Group A: one enabled (passes) + one disabled. Group B: one enabled (fails). Verify Group A passes (disabled is skipped), Group B fails, `anyGroupPasses = true`. | AC-3 |
| 3.1.3 | `QA_GroupEval_EmptyGroupAutoPassesVacuouslyTrue` | One group with `Indicators: []`. `AllIndicatorsPass([])` returns true. Verify `anyGroupPasses = true`, `groupResults[0].Pass = true`. | AC-10 |
| 3.1.4 | `QA_GroupEval_EmptyGroupPlusFailingGroup` | Group A: empty (passes vacuously). Group B: one indicator fails. Verify `anyGroupPasses = true` (Group A passes), both group results are populated. | AC-2, AC-10 |
| 3.1.5 | `QA_GroupEval_MixedStaleAndFreshAcrossGroups` | Group A: indicator is stale (fails per stale-blocking rule). Group B: indicator is fresh and passes. Verify `anyGroupPasses = true` — stale data in one group does not block another group's fresh passing data. | AC-10 |
| 3.1.6 | `QA_GroupEval_AllGroupsStale` | Group A: stale indicator. Group B: stale indicator. Verify `anyGroupPasses = false`. | AC-10 |
| 3.1.7 | `QA_GroupEval_SingleGroupBehavesLikeFlatList` | One group with 3 indicators (all pass). Verify behavior is identical to calling `AllIndicatorsPass` directly on the same results — `anyGroupPasses = true`, flat results match group results, group has `Pass = true`. This validates AC-4's requirement that single-group behaves like the old flat list. | AC-4 |
| 3.1.8 | `QA_GroupEval_SingleGroupSingleIndicatorFails` | One group with 1 indicator that fails. Verify `anyGroupPasses = false`, `groupResults[0].Pass = false`. | AC-3 |
| 3.1.9 | `QA_GroupEval_NoShortCircuit` | Group A passes. Group B fails. Verify both `groupResults` entries are populated with correct data — proves all groups are evaluated (FR-2.6, no short-circuit). | AC-2 |
| 3.1.10 | `QA_GroupEval_ThreeGroupsSecondPasses` | Group A: fails. Group B: passes. Group C: fails. Verify `anyGroupPasses = true` and only `groupResults[1].Pass = true`. | AC-2 |
| 3.1.11 | `QA_GroupEval_FlatResultsOrder` | Three groups with 2, 1, and 3 indicators respectively. Verify `flatResults` has 6 entries in group-then-indicator order (Group A's indicators first, then Group B's, then Group C's). | — |
| 3.1.12 | `QA_GroupEval_GroupMetadataPreserved` | Verify `GroupResult.GroupID` and `GroupResult.GroupName` match the input `IndicatorGroup.ID` and `IndicatorGroup.Name` for each group. | AC-7 |

### 3.2 `AllIndicatorsPass` — Additional Edge Cases

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 3.2.1 | `QA_AllIndicatorsPass_AllDisabled` | All indicators in the list are disabled. Verify returns `true` (vacuously true — no enabled indicator to fail). | AC-10 |
| 3.2.2 | `QA_AllIndicatorsPass_StaleButPassingValue` | One indicator is stale (`Stale: true`) but its value still passes the threshold (`Pass: true`). Verify returns `false` — stale always blocks regardless of the pass value. | AC-10 |
| 3.2.3 | `QA_AllIndicatorsPass_MixOfDisabledStaleFresh` | Three indicators: (1) disabled, (2) enabled + stale, (3) enabled + fresh + passing. Verify returns `false` because indicator (2) is stale. | AC-10 |

---

## Step 4: QA Tests for `storage.go`

**File:** `trade-backend-go/internal/automation/qa_storage_test.go`
**Run:** `cd trade-backend-go && go test ./internal/automation/ -run QA -v`

### 4.1 Migration Idempotency

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 4.1.1 | `QA_MigrateIndicatorGroups_IdempotentOnSecondRun` | Run `migrateIndicatorGroups()` on a config with flat indicators. Verify migration occurs (returns `true`). Then run it again on the same config. Verify it returns `false` (no-op) and the group structure is unchanged (same group ID, same indicator count). | AC-4, AC-11 |
| 4.1.2 | `QA_MigrateIndicatorIDs_IdempotentOnSecondRun` | Run `migrateIndicatorIDs()` on a config with empty indicator IDs. Verify migration occurs. Record the generated IDs. Run it again. Verify returns `false` and IDs are unchanged. | AC-11 |
| 4.1.3 | `QA_MigrateIndicatorGroups_IdempotentWithEmptyConfig` | Config has both `Indicators: []` and `IndicatorGroups: []`. Run migration twice. Both return `false`. Nothing changes. | AC-4 |

### 4.2 Migration Ordering

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 4.2.1 | `QA_MigrationOrder_IDsBeforeGroups` | Config with flat `Indicators` where one indicator has `ID: ""`. Run `migrateIndicatorIDs()` first, then `migrateIndicatorGroups()`. Verify the indicator inside the resulting group has a non-empty ID (IDs were backfilled before grouping). | AC-4 |
| 4.2.2 | `QA_MigrationOrder_IDsInGroupIndicators` | Config already has `IndicatorGroups` with one indicator missing an ID. Run `migrateIndicatorIDs()`. Verify it backfills the ID inside the group (not just in the legacy `Indicators` array). | AC-11 |
| 4.2.3 | `QA_MigrationOrder_FullChain` | Config with flat `Indicators`, two have empty IDs. Simulate the full `load()` migration chain: `migrateIndicatorIDs()` then `migrateIndicatorGroups()`. Verify: (1) both indicators now have IDs, (2) they are in a single "Default" group, (3) `config.Indicators` is empty, (4) `needsSave` is true. | AC-4 |

### 4.3 JSON Round-Trip Persistence

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 4.3.1 | `QA_Storage_JSONPersistence_GroupsSurviveWriteRead` | Create a `Storage` with a temp file. Insert a config with 2 indicator groups (each with 2 indicators). Call `save()`. Create a new `Storage` pointing to the same file. Call `load()`. Verify the loaded config has identical groups, indicators, IDs, names, thresholds, operators, params. | AC-1 |
| 4.3.2 | `QA_Storage_JSONPersistence_VersionBump` | After `saveWithoutLock()`, read the raw JSON file. Verify `"version": "1.1"` is present. | AC-4 |
| 4.3.3 | `QA_Storage_JSONPersistence_LegacyFileAutoMigrates` | Write a v1.0-style JSON file (flat `indicators`, no `indicator_groups`). Create a `Storage` that loads this file. Verify: (1) config now has `IndicatorGroups` with a "Default" group, (2) `Indicators` is empty, (3) the file on disk is updated with v1.1 format. | AC-4 |
| 4.3.4 | `QA_Storage_JSONPersistence_EmptyIndicatorsField` | After migration, verify the JSON file contains `"indicators": []` (empty array, not omitted and not null) for backward compatibility with older frontends. | AC-4 |

---

## Step 5: QA Tests for `AutomationConfigForm.vue`

**File:** `trade-app/tests/qa-AutomationConfigForm.test.js`
**Run:** `cd trade-app && npx vitest run tests/qa-AutomationConfigForm.test.js`

### 5.1 Group CRUD (AC-5, AC-8)

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 5.1.1 | `addGroup increments group count and assigns sequential names` | Start with 1 group ("Default"). Call `addGroup()` three times. Verify groups are named "Group 2", "Group 3", "Group 4" with unique `grp_` IDs. | AC-8 |
| 5.1.2 | `addGroup assigns unique IDs even when called rapidly` | Call `addGroup()` twice in rapid succession (no await between). Verify both new groups have distinct IDs. | AC-8 |
| 5.1.3 | `removeGroup with empty group skips confirmation` | Add a group with 0 indicators. Call `removeGroup()` on it. Verify `window.confirm` is NOT called and the group is removed. | AC-8 |
| 5.1.4 | `removeGroup with indicators prompts confirmation and respects accept` | Add indicator to group. Mock `confirm` to return `true`. Call `removeGroup()`. Verify group is removed and indicator's test result is cleaned up from `indicatorResults`. | AC-8 |
| 5.1.5 | `removeGroup cleans up groupTestResults` | Set `groupTestResults[groupId] = { pass: true }`. Remove the group. Verify `groupTestResults[groupId]` is deleted. | AC-8 |
| 5.1.6 | `renaming group updates group.name` | Set `config.indicator_groups[0].name = 'Custom Name'`. Verify the value is stored. (Tests the v-model binding path.) | AC-8 |
| 5.1.7 | `cannot delete last remaining group` | With only 1 group, verify the delete button is not rendered (the template shows it only when `indicator_groups.length > 1`). | AC-9 |

### 5.2 OR Divider (AC-5, AC-6)

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 5.2.1 | `no OR divider with exactly 1 group` | Default state (1 group). Verify `.or-divider` element does not exist. | AC-5, AC-9 |
| 5.2.2 | `one OR divider with exactly 2 groups` | Add one group. Verify exactly 1 `.or-divider` exists. | AC-5 |
| 5.2.3 | `two OR dividers with 3 groups` | Add two groups (total 3). Verify exactly 2 `.or-divider` elements. | AC-5 |
| 5.2.4 | `OR divider disappears when group removed back to 1` | Start with 2 groups. Remove one. Verify `.or-divider` no longer exists. | AC-5, AC-9 |

### 5.3 Test All — Per-Group Results (AC-6, AC-8)

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 5.3.1 | `testAllIndicators sends indicator_groups in API payload` | Set up 2 groups with indicators. Mock `previewAutomationIndicators`. Call `testAllIndicators()`. Verify the API call payload contains `indicator_groups` (not flat `indicators`). | AC-6 |
| 5.3.2 | `testAllIndicators maps group_results to groupTestResults by group_id` | Mock API to return `group_results: [{ group_id: 'grp_1', pass: true }, { group_id: 'grp_2', pass: false }]`. Call `testAllIndicators()`. Verify `groupTestResults['grp_1'].pass === true` and `groupTestResults['grp_2'].pass === false`. | AC-6 |
| 5.3.3 | `testAllIndicators maps individual indicator results by indicator id` | Mock API to return `group_results` with nested `indicator_results`. Verify `indicatorResults[indicatorId]` is populated with correct value, pass, stale fields. | AC-6 |
| 5.3.4 | `overall result shows "X of Y groups passing" for multi-group` | 2 groups, API returns 1 passing. Verify `allIndicatorsResult` is `true` and the template renders "1 of 2 groups passing". | AC-6 |
| 5.3.5 | `overall result shows "No groups passing" when all fail` | 2 groups, API returns 0 passing. Verify `allIndicatorsResult` is `false` and text contains "No groups passing". | AC-6 |
| 5.3.6 | `overall result shows "All Passed" / "Some Failed" for single group` | 1 group. Verify the template uses the simple "All Passed" / "Some Failed" language (not the group-counting language). | AC-9 |

### 5.4 Single-Group Lightweight UX (AC-9)

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 5.4.1 | `single group has "lightweight" CSS class on container` | 1 group. Verify `.indicator-group.lightweight` exists. | AC-9 |
| 5.4.2 | `single group name input has "lightweight" CSS class` | 1 group. Verify `.group-name-input.lightweight` exists. | AC-9 |
| 5.4.3 | `"Add Group" button visible with single group` | 1 group. Verify the "Add Group" button is rendered (allows user to create groups from the simple view). | AC-9 |
| 5.4.4 | `multi-group removes "lightweight" class` | Add a group (now 2). Verify `.indicator-group.lightweight` no longer exists. | AC-9 |

### 5.5 Save Payload Structure (AC-1)

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 5.5.1 | `save sends indicator_groups with correct nested structure` | Set up 2 groups with 1 indicator each. Call `saveConfig()`. Inspect the payload sent to `createAutomationConfig`. Verify: (a) `indicators` is `[]`, (b) `indicator_groups` has 2 entries, (c) each entry has `id`, `name`, `indicators` array with full indicator objects (id, type, enabled, operator, threshold). | AC-1 |
| 5.5.2 | `save preserves indicator params in groups` | Add a technical indicator (e.g., RSI with period=21). Save. Verify the payload's indicator within the group has `params: { period: 21 }`. | AC-1 |
| 5.5.3 | `save preserves indicator symbol in groups` | Add a VIX indicator with custom symbol "UVXY". Save. Verify the payload's indicator has `symbol: "UVXY"`. | AC-1 |

---

## Step 6: QA Tests for `AutomationDashboard.vue`

**File:** `trade-app/tests/qa-AutomationDashboard.test.js`
**Run:** `cd trade-app && npx vitest run tests/qa-AutomationDashboard.test.js`

### 6.1 Group Results Rendering (AC-7)

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 6.1.1 | `renders grouped indicator results when group_results has 2+ entries` | Set `statuses[configId].group_results` with 2 group results. Set config with 2 `indicator_groups`. Verify: (a) `.indicator-group-dashboard` containers appear in the status-details area, (b) `.group-result-badge` elements show "Pass" / "Fail". | AC-7 |
| 6.1.2 | `renders per-group indicator chips inside each group` | Set group_results with indicator_results per group. Verify `.result-chip` elements appear within each group container, showing correct type labels and values. | AC-7 |
| 6.1.3 | `renders overall status line with passing group name` | Set `all_indicators_pass: true` and one group passing. Verify `.overall-status.passing` contains the passing group's name. | AC-7 |
| 6.1.4 | `renders overall "Not passing" status when no groups pass` | Set `all_indicators_pass: false`. Verify `.overall-status.failing` contains "Not passing". | AC-7 |
| 6.1.5 | `stale indicator in group_results shows stale chip styling` | Set a group result with one indicator having `stale: true`. Verify the `.result-chip.stale` class is applied and the stale icon is rendered. | AC-7, AC-10 |

### 6.2 OR Divider Between Groups (AC-7)

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 6.2.1 | `OR divider rendered between group results in status details` | 2 group results. Verify exactly 1 `.or-divider-compact` in the status-details section. | AC-7 |
| 6.2.2 | `OR divider rendered between groups in config indicators section` | Config with 2 `indicator_groups`. Verify `.or-divider-compact` in the indicators-section. | AC-5 |
| 6.2.3 | `no OR divider with 1 group result` | 1 group result. Verify no `.or-divider-compact` in status-details. | AC-9 |

### 6.3 Single-Group Flat Display (AC-9)

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 6.3.1 | `single group config renders flat indicator chips (no group container)` | Config with 1 `indicator_group`. Verify `.indicator-group-dashboard` does NOT appear in the indicators-section. Instead, flat `.indicator-chip` elements are rendered. | AC-9 |
| 6.3.2 | `single group status renders flat results (no group header)` | Status has `group_results` with 1 entry. Verify the dashboard uses the flat `indicator_results` rendering path (the template checks `group_results?.length > 1`). | AC-9 |
| 6.3.3 | `single group evaluation dialog renders flat results` | Trigger `evaluateIndicators` with a config that returns 1 group. Verify the eval dialog uses the flat rendering template. | AC-9 |

### 6.4 Backward Compatibility with Legacy Data (AC-4)

| # | Test Case | Description | AC |
|---|-----------|-------------|-----|
| 6.4.1 | `config with only legacy indicators renders flat chips` | Config has `indicators: [{...}, {...}]` and no `indicator_groups`. Verify `getEnabledIndicators` returns the correct indicators and the dashboard renders flat `.indicator-chip` elements. | AC-4 |
| 6.4.2 | `status update without group_results renders flat indicator_results` | WebSocket update has `indicator_results` but no `group_results`. Verify the dashboard renders the flat results view (backward compat path). | AC-4 |
| 6.4.3 | `handleAutomationUpdate merges group_results into statuses` | Simulate a WebSocket message with `{ automation_id: 'cfg-1', data: { group_results: [...], indicator_results: [...], all_indicators_pass: true } }`. Call `handleAutomationUpdate`. Verify `statuses['cfg-1'].group_results` is populated. | AC-7 |
| 6.4.4 | `evaluation dialog handles multi-group response` | Mock `evaluateAutomationConfig` to return `group_results` with 2 groups. Call `evaluateIndicators`. Verify `evalResult.group_results` has 2 entries and the dialog renders the grouped template. | AC-7 |

---

## Step 7: Final Regression Run

**Goal:** Confirm all existing tests plus all new QA tests pass together.

**Commands:**

```sh
# Backend — run ALL tests (existing + qa_)
cd trade-backend-go && go test ./...

# Frontend — run ALL tests (existing + qa-)
cd trade-app && npx vitest run
```

**Pass Criteria:**
- 0 test failures across both suites.
- All new QA tests pass.
- All pre-existing tests still pass (AC-11).
- No regressions from the new indicator group functionality.

---

## Summary: Test File Inventory

| File | Test Count | Covers |
|------|-----------|--------|
| `trade-backend-go/internal/automation/types/qa_types_test.go` | 12 tests | Step 2: GetEffectiveIndicatorGroups edge cases, JSON round-trip, GroupResult validation, ID generation |
| `trade-backend-go/internal/automation/indicators/qa_service_test.go` | 15 tests | Step 3: EvaluateIndicatorGroups edge cases (vacuously true, stale/fresh, single-group, no short-circuit) |
| `trade-backend-go/internal/automation/qa_storage_test.go` | 10 tests | Step 4: Migration idempotency, ordering, JSON persistence round-trip |
| `trade-app/tests/qa-AutomationConfigForm.test.js` | 20 tests | Step 5: Group CRUD, OR divider, Test All per-group, single-group UX, save payload |
| `trade-app/tests/qa-AutomationDashboard.test.js` | 15 tests | Step 6: group_results rendering, OR divider, single-group flat display, backward compat |
| **Total** | **72 QA tests** | |

---

## Execution Notes

1. **Sequential execution:** Steps must be run in order. Steps 1 and 7 are gates; do not proceed past Step 1 with failures. Steps 2–6 can be developed in parallel but should be run sequentially before Step 7.

2. **No provider/API mocking needed for Go tests:** Steps 2–4 test pure logic using the `computeGroupORLogic` helper and direct struct manipulation. No `ProviderManager` or HTTP calls are required.

3. **Frontend mock pattern:** Steps 5–6 follow the existing mock pattern in `AutomationConfigForm.test.js` and `AutomationDashboard.test.js` — mock `vue-router`, `api.js`, `webSocketClient.js`, and `useMobileDetection`.

4. **Naming convention:** All Go QA tests use function names starting with `QA_` and are placed in files with `qa_` prefix. All JavaScript QA tests are placed in files with `qa-` prefix. This allows selective running via `go test -run QA` and `npx vitest run tests/qa-*`.
