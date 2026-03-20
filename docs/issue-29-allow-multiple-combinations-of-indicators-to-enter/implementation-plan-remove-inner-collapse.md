# Implementation Plan: Remove Inner Group-Level Collapse

**Parent Issue:** #29 — Allow multiple combinations of indicators to enter a trade
**Requirement Doc:** `requirements-remove-inner-collapse.md`
**Strategy:** Each step is a single focused commit. Tests are updated in the same step as the production code they cover, so every commit leaves the test suite green.

---

## Step 1: Remove group-level collapse state and methods from `setup()`

**File:** `AutomationDashboard.vue` (lines 565, 974–985, 1108–1112)

### Changes

1. **Delete** the `expandedGroups` reactive ref declaration (line 565):
   ```js
   const expandedGroups = ref({})  // DELETE
   ```

2. **Delete** the `toggleGroup` method (lines 974–981):
   ```js
   const toggleGroup = (configId, section, groupIdx) => { ... }  // DELETE
   ```

3. **Delete** the `isGroupExpanded` method (lines 983–985):
   ```js
   const isGroupExpanded = (configId, section, groupIdx) => { ... }  // DELETE
   ```

4. **Remove** `expandedGroups`, `toggleGroup`, and `isGroupExpanded` from the `return` block (lines 1108–1112):
   ```js
   expandedGroups,     // DELETE
   toggleGroup,        // DELETE
   isGroupExpanded,    // DELETE
   ```

### Test updates

**File:** `AutomationDashboard.test.js`

- **Delete** the `'Toggle behavior'` sub-describe tests for `toggleGroup` and `isGroupExpanded` (lines 905–916):
  - `'toggleGroup expands group 0 without affecting group 1'`
  - `'isGroupExpanded returns false by default, true after toggle'`

- **Delete** the `'State independence'` sub-describe tests that reference `isGroupExpanded` (lines 920–936):
  - `'toggling card A does not affect card B'`
  — Or rewrite to only assert on `expandedEntrySections`/`expandedEvalSections` (remove `isGroupExpanded`/`toggleGroup` assertions).

### Why this step is safe

Removing state + methods from `setup()` first is safe because the template still references them — Vue will silently return `undefined` for missing properties, so the UI won't crash. But we'll fix the template in the very next step. The tests that assert on the removed methods are deleted in the same commit, so the test suite stays green.

---

## Step 2: Replace inner group collapse in the Entry Criteria template

**File:** `AutomationDashboard.vue` (lines 119–143)

### Changes

Replace the current clickable group header + conditional body in the **Entry Criteria** section:

**Before (lines 120–141):**
```html
<div class="indicator-group-dashboard">
  <!-- Group-level collapse header (FR-3.1, FR-3.2, FR-3.5) -->
  <div
    class="group-collapse-header"
    @click="toggleGroup(config.id, 'entry', gIdx)"
  >
    <i :class="isGroupExpanded(...) ? 'pi pi-chevron-down' : 'pi pi-chevron-right'" class="collapse-chevron collapse-chevron-sm"></i>
    <span class="group-label">{{ group.name || 'Group ' + (gIdx + 1) }}</span>
    <span class="group-indicator-count">...</span>
  </div>
  <!-- Group body: indicator chips (FR-3.3) -->
  <div v-if="isGroupExpanded(config.id, 'entry', gIdx)" class="indicators-grid">
    ...
  </div>
</div>
```

**After:**
```html
<div class="indicator-group-dashboard">
  <!-- Static group header (always visible, not clickable) -->
  <div class="group-header-static">
    <span class="group-label">{{ group.name || 'Group ' + (gIdx + 1) }}</span>
    <span class="group-indicator-count">{{ (group.indicators || []).filter(ind => ind.enabled).length }} indicators</span>
  </div>
  <!-- Indicator chips (always visible when section is expanded) -->
  <div class="indicators-grid">
    <div
      v-for="indicator in (group.indicators || []).filter(ind => ind.enabled)"
      :key="indicator.id || indicator.type"
      class="indicator-chip"
      :class="getIndicatorStatusClass(config, indicator)"
    >
      <span class="indicator-name">{{ formatIndicatorType(indicator.type) }}</span>
      <span class="indicator-value">{{ formatIndicatorCondition(indicator) }}</span>
    </div>
  </div>
</div>
```

Key differences:
- `.group-collapse-header` → `.group-header-static` (no `@click`, no `cursor:pointer`)
- Removed `<i>` chevron icon entirely
- Removed `v-if="isGroupExpanded(...)"` from `.indicators-grid` — chips are always shown
- Kept `.group-indicator-count` for the "N indicators" label

### Test updates

**File:** `AutomationDashboard.test.js`

- **Update** `'groups are collapsed by default within expanded section'` (lines 424–441):
  - **Before:** Asserts groups are collapsed (`.indicators-grid` count = 0) and checks `.group-collapse-header`.
  - **After:** Assert `.group-header-static` exists (×3), all `.indicators-grid` are visible (×3), no `.group-collapse-header` exists.

- **Update** `'clicking a group header expands that group'` (lines 444–463):
  - **Before:** Clicks `.group-collapse-header` and asserts one grid appears.
  - **After:** Remove this test entirely — there is no group-level click interaction. (Or rename to assert all grids are visible without clicking.)

**File:** `qa-dashboard-compact.test.js`

- **Update** Step 2 boundary tests (lines 80–200):
  - Test `2.4` (line 162): Remove assertion on `.group-collapse-header` — replace with `.group-header-static`.

- **Update** Step 3 adversarial tests that expand groups before asserting:
  - Test `3.2` (line 222): Remove group expand click, assert `.indicators-grid` is immediately visible.
  - Test `3.3` (line 248): Remove group expand click, assert chips are visible directly.

---

## Step 3: Replace inner group collapse in the Last Evaluation template

**File:** `AutomationDashboard.vue` (lines 240–269)

### Changes

Replace the clickable group header + conditional body in the **Last Evaluation** section:

**Before (lines 240–269):**
```html
<div class="indicator-group-dashboard">
  <div
    class="group-collapse-header"
    @click="toggleGroup(config.id, 'eval', grIdx)"
  >
    <i :class="isGroupExpanded(...)" class="collapse-chevron collapse-chevron-sm"></i>
    <span class="group-label">...</span>
    <span class="group-result-badge" :class="gr.pass ? 'passed' : 'failed'">...</span>
  </div>
  <div v-if="isGroupExpanded(config.id, 'eval', grIdx)" class="results-grid">
    ...
  </div>
</div>
```

**After:**
```html
<div class="indicator-group-dashboard">
  <!-- Static group header with pass/fail badge (always visible) -->
  <div class="group-header-static">
    <span class="group-label">{{ gr.group_name || 'Group ' + (grIdx + 1) }}</span>
    <span class="group-result-badge" :class="gr.pass ? 'passed' : 'failed'">
      {{ gr.pass ? '&#10003; Pass' : '&#10007; Fail' }}
    </span>
  </div>
  <!-- Result chips (always visible when section is expanded) -->
  <div class="results-grid">
    <div
      v-for="(result, idx) in gr.indicator_results"
      :key="idx"
      class="result-chip"
      :class="{
        'passed': result.pass && !result.stale,
        'failed': !result.pass && !result.stale,
        'stale': result.stale
      }"
      :title="result.stale ? `Stale data: ${result.error || 'Fetch failed'}` : result.details"
    >
      <span>{{ formatIndicatorType(result.type) }}</span>
      <span v-if="result.stale" class="stale-icon">&#9888;</span>
      <span>{{ formatIndicatorValue(result) }}</span>
    </div>
  </div>
</div>
```

Key differences (same pattern as Step 2):
- `.group-collapse-header` → `.group-header-static`
- Removed chevron `<i>` and `@click`
- Removed `v-if="isGroupExpanded(...)"` from `.results-grid`

### Test updates

**File:** `AutomationDashboard.test.js`

- **Update** `'groups show name and badge but no result chips when collapsed'` (lines 617–637):
  - **Before:** Asserts `.group-collapse-header` exists, `.results-grid` count = 0.
  - **After:** Assert `.group-header-static` exists (×3), `.results-grid` visible (×3), `.group-collapse-header` does not exist.

- **Update** `'clicking a group header expands that group to show result chips'` (lines 639–659):
  - **Before:** Clicks `.group-collapse-header`, asserts 1 grid.
  - **After:** Remove this test — no click needed. (Or rename to assert all grids visible.)

- **Update** `'state persists across data updates'` (lines 692–733):
  - **Before:** Expands group via click, asserts `.results-grid` count = 1 after update.
  - **After:** Remove group-expand click, assert all grids visible after section expand.

**File:** `qa-AutomationDashboard.test.js`

- **Update** `'renders per-group indicator chips inside each group'` (lines 115–151):
  - Remove the two `.group-collapse-header` click lines (140–142). Assert `.result-chip` count without needing to expand groups.

- **Update** `'stale indicator in group_results shows stale chip styling'` (lines 213–253):
  - Remove the `.group-collapse-header` expand clicks (lines 242–244). Stale chips visible immediately.

**File:** `qa-dashboard-compact.test.js`

- **Update** Step 3 tests that expand eval groups before asserting:
  - Tests `3.5` (line 310), `3.6` (line 348): Remove group expand clicks. Assert result chips/grids are visible immediately.

- **Update** Step 5 multi-card independence:
  - Test `5.2` (line 652): Remove group expand clicks. Assert result chips visible directly after section expand.
  - Test `5.4` (line 728): Replace `.group-collapse-header` click assertions with `.group-header-static` assertions. Verify indicator chips are visible without clicking groups.

---

## Step 4: Update CSS — remove group-collapse styles, add group-header-static

**File:** `AutomationDashboard.vue` (`<style scoped>` section)

### Changes

1. **Delete** the `.collapse-chevron-sm` rule (lines 1851–1853):
   ```css
   .collapse-chevron-sm {
     font-size: 10px;
   }
   ```

2. **Delete** the `.group-collapse-header` rules (lines 1865–1881):
   ```css
   .group-collapse-header { ... }
   .group-collapse-header:hover { ... }
   .group-collapse-header .group-label { ... }
   ```

3. **Add** the `.group-header-static` rule (non-clickable group header):
   ```css
   .group-header-static {
     display: flex;
     align-items: center;
     gap: var(--spacing-xs);
     margin-bottom: var(--spacing-xs);
     user-select: none;
   }

   .group-header-static .group-label {
     margin-bottom: 0;
   }
   ```
   — No `cursor: pointer`, no `:hover` opacity change.

### Test updates

**File:** `qa-dashboard-compact.test.js`

- **Update** Step 10 chevron icon tests (lines 1516–1643):
  - **Delete** tests `10.5`, `10.6` (group-level chevrons, lines 1580–1614): These asserted `pi-chevron-right`/`pi-chevron-down` on `.group-collapse-header .collapse-chevron`, which no longer exist.
  - Section-level chevron tests (`10.1`–`10.4`, `10.7`) are **unchanged** — section collapse still uses chevrons.

---

## Step 5: Update remaining QA tests that reference removed group-collapse behavior

**File:** `qa-dashboard-compact.test.js`

### Changes

- **Update** Step 6 "Rapid Toggle Sequences" (lines 776–934):
  - Test `6.3` (line 830): Remove — rapid-clicks on `.group-collapse-header` are no longer applicable.
  - Test `6.4` (line 851): Remove the "expand group 0" part. Test only section-level collapse/expand persistence. Assert chips visible after re-expand without group toggle.
  - Test `6.5` (line 886): Same — remove group expand/collapse. Assert section expand persists across WebSocket updates.

- **Update** Step 7 "State Persistence Across Data Updates" (lines 940–1297):
  - Test `7.1` (line 942): Remove group-expand click and `isGroupExpanded` assertion. Assert indicators-grid visible immediately after section expand.
  - Test `7.3` (line 1030): Remove group-expand click. Assert result chips visible immediately. Remove `isGroupExpanded` assertions.
  - Test `7.4` (line 1093): Same pattern — remove group-expand, assert directly.

- **Update** Step 11 "Interaction with Existing Actions" (lines 1649–1878):
  - Test `11.2` (line 1689): Remove group expand click and `isGroupExpanded` assertion. Assert result chips visible directly.
  - Test `11.5` (line 1777): Remove group expand click and `isGroupExpanded` assertion.
  - Test `11.8` (line 1850): Remove group expand click and `isGroupExpanded` assertion.

- **Update** Step 12 "Evaluation Dialog Unaffected" (lines 1884–2043):
  - Test `12.3` (line 1977): Remove group expand clicks and `isGroupExpanded` assertions from both entry and eval sections.

- **Rewrite** Step 15 "Group Expand State Key Integrity" (lines 2383–2554):
  - **Delete all tests** in this section — they test `expandedGroups` key format, `toggleGroup`, and `isGroupExpanded`, which no longer exist. The entire describe block can be removed.

### Verification

After this step, no test file should reference:
- `expandedGroups`
- `toggleGroup`
- `isGroupExpanded`
- `.group-collapse-header`
- `.collapse-chevron-sm`

---

## Step 6: Final verification — run all tests

**Command:** `cd trade-app && npx vitest run`

### Expected results

- All tests in `AutomationDashboard.test.js` pass
- All tests in `qa-AutomationDashboard.test.js` pass
- All tests in `qa-dashboard-compact.test.js` pass
- No regressions in other test files (pre-existing `CollapsibleOptionsChain` failures excluded)

### Manual spot-check (optional)

If a dev environment is available:
1. Load the dashboard with a multi-group config
2. Click the Entry Criteria summary — all groups + chips visible immediately
3. Click the Last Evaluation summary — all groups + result chips visible immediately
4. Single-group cards still render flat (no collapse)
5. Section-level collapse/expand still works normally

---

## Summary of changes by file

| File | Adds | Removes | Modifies |
|------|------|---------|----------|
| `AutomationDashboard.vue` | `.group-header-static` CSS class | `expandedGroups`, `toggleGroup`, `isGroupExpanded` from setup(); `.group-collapse-header`, `.collapse-chevron-sm` from CSS | Entry Criteria template (lines 119–143); Last Evaluation template (lines 240–269) |
| `AutomationDashboard.test.js` | — | Tests for `toggleGroup`/`isGroupExpanded`; group-click expand tests | Tests that asserted groups collapsed by default → assert all visible |
| `qa-AutomationDashboard.test.js` | — | Group-expand clicks in tests | Tests that clicked groups before asserting → assert directly |
| `qa-dashboard-compact.test.js` | — | Step 15 (group key integrity); Tests `10.5`/`10.6` (group chevrons); Test `6.3` (rapid group toggle) | ~20 tests updated to remove group-expand clicks and `isGroupExpanded` assertions |

**Total steps:** 6 (5 code steps + 1 verification step)
**Estimated scope:** ~60 lines added, ~100 lines removed across all files
