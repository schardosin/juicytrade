# Implementation Plan: Dashboard Card Compact Mode (Progressive Disclosure)

**Requirements:** `requirements-dashboard-compact.md`  
**Target Component:** `trade-app/src/components/automation/AutomationDashboard.vue`  
**Existing Tests:** `trade-app/tests/AutomationDashboard.test.js`

---

## Step 1: Add collapse state tracking + helper methods in `setup()`

**Goal:** Wire up all reactive state and logic for progressive disclosure — no visual changes yet.

**Files to modify:**
- `trade-app/src/components/automation/AutomationDashboard.vue` (script section only)
- `trade-app/tests/AutomationDashboard.test.js` (add new test block)

### 1a. Add reactive state maps (after `expandedCards` on line 513)

Add three reactive `ref({})` maps for per-card, per-section collapse tracking:

```js
// Compact mode collapse state (per-card)
const expandedEntrySections = ref({})  // { [configId]: boolean } — Entry Criteria section expanded?
const expandedEvalSections = ref({})   // { [configId]: boolean } — Last Evaluation section expanded?
const expandedGroups = ref({})         // { [configId__section__groupIdx]: boolean } — individual group expanded?
```

Default values: all absent keys = `false` (collapsed). This satisfies FR-5.1/FR-5.2 — collapsed by default for multi-group, per-card tracking, not persisted.

### 1b. Add toggle methods

```js
const toggleEntrySection = (configId) => {
  expandedEntrySections.value = {
    ...expandedEntrySections.value,
    [configId]: !expandedEntrySections.value[configId]
  }
}

const toggleEvalSection = (configId) => {
  expandedEvalSections.value = {
    ...expandedEvalSections.value,
    [configId]: !expandedEvalSections.value[configId]
  }
}

const toggleGroup = (configId, section, groupIdx) => {
  // section: 'entry' | 'eval'
  const key = `${configId}__${section}__${groupIdx}`
  expandedGroups.value = {
    ...expandedGroups.value,
    [key]: !expandedGroups.value[key]
  }
}

const isGroupExpanded = (configId, section, groupIdx) => {
  return !!expandedGroups.value[`${configId}__${section}__${groupIdx}`]
}
```

The spread-and-reassign pattern (not mutating in place) ensures Vue reactivity triggers, consistent with the existing `handleAutomationUpdate` pattern on line 537-545.

### 1c. Add helper methods for summaries

```js
const isMultiGroup = (config) => {
  return (config.indicator_groups?.length || 0) > 1
}

const getTotalGroupCount = (config) => {
  return config.indicator_groups?.length || 0
}

const getTotalIndicatorCount = (config) => {
  if (!config.indicator_groups?.length) return 0
  return config.indicator_groups.reduce((sum, g) => {
    return sum + (g.indicators || []).filter(ind => ind.enabled).length
  }, 0)
}

const getEntrySummary = (config) => {
  // Returns: "3 groups · 9 indicators" (FR-1.2)
  const groups = getTotalGroupCount(config)
  const indicators = getTotalIndicatorCount(config)
  return `${groups} groups · ${indicators} indicators`
}

const getEvalSummary = (configId) => {
  // Returns: { passing: boolean, passingGroupName: string|null, summary: string } (FR-2.2)
  const status = statuses.value[configId]
  if (!status?.group_results?.length) return null

  const groupResults = status.group_results
  const passingGroups = groupResults.filter(g => g.pass)
  const passing = passingGroups.length > 0

  return {
    passing,
    passingGroupName: passingGroups[0]?.group_name || null,
    summary: `${passingGroups.length} of ${groupResults.length} groups passing`
  }
}
```

### 1d. Register in return statement

Add all new state and methods to the return object (after the `// Mobile` section, around line 981):

```js
// Compact mode (progressive disclosure)
expandedEntrySections,
expandedEvalSections,
expandedGroups,
toggleEntrySection,
toggleEvalSection,
toggleGroup,
isGroupExpanded,
isMultiGroup,
getTotalGroupCount,
getTotalIndicatorCount,
getEntrySummary,
getEvalSummary,
```

### 1e. Tests for Step 1

Add a new `describe('Compact mode helpers')` block in `AutomationDashboard.test.js`:

**`isMultiGroup`:**
- Returns `true` for config with 2+ `indicator_groups`
- Returns `false` for config with 1 group
- Returns `false` for config with no groups (legacy)
- Returns `false` for config with empty `indicator_groups` array

**`getEntrySummary`:**
- Returns `"3 groups · 9 indicators"` for 3 groups with 3 enabled indicators each
- Counts only enabled indicators (skips `enabled: false`)
- Returns `"2 groups · 0 indicators"` when groups exist but all indicators are disabled

**`getEvalSummary`:**
- Returns `null` when no status exists for the config
- Returns `null` when status has no `group_results`
- Returns `{ passing: true, passingGroupName: 'Low VIX', summary: '1 of 3 groups passing' }` when 1 of 3 groups passes
- Returns `{ passing: false, passingGroupName: null, summary: '0 of 2 groups passing' }` when no groups pass

**Toggle behavior:**
- `toggleEntrySection(configId)` flips from collapsed (default) to expanded
- Calling `toggleEntrySection(configId)` twice returns to collapsed
- `toggleEvalSection` works independently from `toggleEntrySection` on the same card
- `toggleGroup(configId, 'entry', 0)` expands group 0 without affecting group 1
- `isGroupExpanded` returns `false` by default, `true` after toggle
- State is independent per config ID (toggling card A doesn't affect card B)

---

## Step 2: Template changes for Entry Criteria section + CSS

**Goal:** Multi-group Entry Criteria sections collapse to a summary line by default. Single-group configs unchanged.

**Files to modify:**
- `trade-app/src/components/automation/AutomationDashboard.vue` (template + style sections)
- `trade-app/tests/AutomationDashboard.test.js` (add rendering tests)

### 2a. Replace the multi-group Entry Criteria template block

Replace the existing multi-group `<template v-if="config.indicator_groups?.length > 1">` block (lines 100-123) with a two-level collapsible structure:

```html
<template v-if="isMultiGroup(config)">
  <!-- Collapsed summary (FR-1.1, FR-1.2) -->
  <div
    class="section-collapse-header"
    @click="toggleEntrySection(config.id)"
  >
    <i :class="expandedEntrySections[config.id] ? 'pi pi-chevron-down' : 'pi pi-chevron-right'" class="collapse-chevron"></i>
    <span class="collapse-summary-text">{{ getEntrySummary(config) }}</span>
  </div>

  <!-- Expanded: groups with individual collapse (FR-1.3, FR-1.4, FR-3) -->
  <div v-if="expandedEntrySections[config.id]" class="section-collapse-body">
    <div
      v-for="(group, gIdx) in config.indicator_groups"
      :key="group.id || gIdx"
    >
      <div v-if="gIdx > 0" class="or-divider-compact">
        <span class="or-divider-compact-label">OR</span>
      </div>
      <div class="indicator-group-dashboard">
        <!-- Group-level collapse header (FR-3.1, FR-3.2, FR-3.5) -->
        <div
          class="group-collapse-header"
          @click="toggleGroup(config.id, 'entry', gIdx)"
        >
          <i :class="isGroupExpanded(config.id, 'entry', gIdx) ? 'pi pi-chevron-down' : 'pi pi-chevron-right'" class="collapse-chevron collapse-chevron-sm"></i>
          <span class="group-label">{{ group.name || 'Group ' + (gIdx + 1) }}</span>
          <span class="group-indicator-count">{{ (group.indicators || []).filter(ind => ind.enabled).length }} indicators</span>
        </div>
        <!-- Group body: indicator chips (FR-3.3) -->
        <div v-if="isGroupExpanded(config.id, 'entry', gIdx)" class="indicators-grid">
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
    </div>
  </div>
</template>
```

The single-group / flat `<template v-else>` block (lines 124-137) stays **unchanged** (FR-1.5).

### 2b. Add CSS styles

Add the following styles to the `<style scoped>` section (before the `/* Responsive */` media query, around line 1696):

```css
/* Section Collapse Header (Entry Criteria / Last Evaluation) */
.section-collapse-header {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  padding: var(--spacing-xs) 0;
  cursor: pointer;
  border-radius: var(--radius-sm);
  user-select: none;
}

.section-collapse-header:hover {
  opacity: 0.8;
}

.collapse-chevron {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  transition: transform 0.15s ease;
  flex-shrink: 0;
}

.collapse-chevron-sm {
  font-size: 10px;
}

.collapse-summary-text {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
}

.section-collapse-body {
  margin-top: var(--spacing-xs);
}

/* Group Collapse Header (within expanded sections) */
.group-collapse-header {
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  cursor: pointer;
  margin-bottom: var(--spacing-xs);
  user-select: none;
}

.group-collapse-header:hover {
  opacity: 0.8;
}

.group-collapse-header .group-label {
  margin-bottom: 0;
}

.group-indicator-count {
  font-size: var(--font-size-xs);
  color: var(--text-tertiary);
  margin-left: auto;
}
```

Design notes:
- Uses existing CSS custom properties only (FR-4.4)
- PrimeVue `pi pi-chevron-right` / `pi pi-chevron-down` icons (FR-4.1)
- `cursor: pointer` + hover opacity for clickable feel (FR-4.2)
- Simple CSS transition on chevron, no slow animations (FR-4.3)
- OR dividers remain visible when expanded (FR-4.5)

### 2c. Tests for Step 2

Add a `describe('Compact mode — Entry Criteria')` block:

**Multi-group cards render collapsed by default:**
- Set up a config with 3 groups, mount the component
- Assert `.section-collapse-header` is rendered inside `.indicators-section`
- Assert `.collapse-summary-text` contains `"3 groups · 7 indicators"` (or whatever the test data yields)
- Assert `.section-collapse-body` is NOT rendered (collapsed by default)
- Assert `.indicator-group-dashboard` count is 0 (groups not visible when collapsed)

**Clicking expands the Entry Criteria section:**
- Click the `.section-collapse-header`
- Assert `.section-collapse-body` IS rendered
- Assert `.indicator-group-dashboard` containers appear (one per group)
- Assert OR dividers appear between groups
- Assert chevron icon class changed from `pi-chevron-right` to `pi-chevron-down`

**Groups are collapsed by default within expanded section:**
- After expanding the section, assert `.indicators-grid` inside each group is NOT rendered
- Assert `.group-collapse-header` is visible for each group
- Assert `.group-indicator-count` shows the indicator count for each group

**Clicking a group header expands that group:**
- Click the first `.group-collapse-header`
- Assert `.indicators-grid` appears inside that group with the correct indicator chips
- Assert the second group remains collapsed (its `.indicators-grid` is still not rendered)

**Single-group configs unchanged:**
- Set up a config with 1 group, mount the component
- Assert `.section-collapse-header` is NOT rendered
- Assert `.indicator-chip` elements render directly (flat display)
- Assert no `.group-collapse-header` elements exist

**Legacy configs (no groups) unchanged:**
- Set up a config with only `indicators` (no `indicator_groups`)
- Assert flat indicator chips render as before

---

## Step 3: Template changes for Last Evaluation section + tests

**Goal:** Multi-group Last Evaluation sections collapse to a summary line by default. Single-group/flat results unchanged.

**Files to modify:**
- `trade-app/src/components/automation/AutomationDashboard.vue` (template section only — CSS already added in Step 2)
- `trade-app/tests/AutomationDashboard.test.js` (add rendering tests)

### 3a. Replace the multi-group Last Evaluation template block

Replace the existing `<template v-if="getAutomationStatus(config.id)?.group_results?.length > 1">` block (lines 191-236) with a collapsible version:

```html
<template v-if="getAutomationStatus(config.id)?.group_results?.length > 1">
  <div class="indicator-results">
    <!-- Collapsed summary line (FR-2.1, FR-2.2) -->
    <div
      class="section-collapse-header"
      @click="toggleEvalSection(config.id)"
    >
      <i :class="expandedEvalSections[config.id] ? 'pi pi-chevron-down' : 'pi pi-chevron-right'" class="collapse-chevron"></i>
      <span class="results-label" style="margin-bottom: 0;">Last Evaluation:</span>
      <span v-if="getEvalSummary(config.id)" class="collapse-summary-text">
        <template v-if="getEvalSummary(config.id).passing">
          &#10003; Passing ({{ getEvalSummary(config.id).passingGroupName }}) · {{ getEvalSummary(config.id).summary }}
        </template>
        <template v-else>
          &#10007; Not passing · {{ getEvalSummary(config.id).summary }}
        </template>
      </span>
    </div>

    <!-- Expanded: group-by-group results (FR-2.3, FR-2.4, FR-3) -->
    <div v-if="expandedEvalSections[config.id]" class="section-collapse-body">
      <div
        v-for="(gr, grIdx) in getAutomationStatus(config.id).group_results"
        :key="gr.group_id || grIdx"
      >
        <div v-if="grIdx > 0" class="or-divider-compact">
          <span class="or-divider-compact-label">OR</span>
        </div>
        <div class="indicator-group-dashboard">
          <!-- Group-level collapse header (FR-3.1, FR-3.2, FR-3.5) -->
          <div
            class="group-collapse-header"
            @click="toggleGroup(config.id, 'eval', grIdx)"
          >
            <i :class="isGroupExpanded(config.id, 'eval', grIdx) ? 'pi pi-chevron-down' : 'pi pi-chevron-right'" class="collapse-chevron collapse-chevron-sm"></i>
            <span class="group-label">{{ gr.group_name || 'Group ' + (grIdx + 1) }}</span>
            <span class="group-result-badge" :class="gr.pass ? 'passed' : 'failed'">
              {{ gr.pass ? '&#10003; Pass' : '&#10007; Fail' }}
            </span>
          </div>
          <!-- Group body: indicator result chips (FR-3.3) -->
          <div v-if="isGroupExpanded(config.id, 'eval', grIdx)" class="results-grid">
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
      </div>
      <!-- Overall status line (unchanged from current) -->
      <div class="overall-status" :class="getAutomationStatus(config.id)?.all_indicators_pass ? 'passing' : 'failing'">
        <template v-if="getAutomationStatus(config.id)?.all_indicators_pass">
          &#10003; Passing ({{ getAutomationStatus(config.id).group_results.find(g => g.pass)?.group_name || 'Group' }})
        </template>
        <template v-else>
          &#10007; Not passing
        </template>
      </div>
    </div>
  </div>
</template>
```

The flat/single-group `<div v-else-if="...">` block (lines 238-257) stays **unchanged** (FR-2.5).

### 3b. Tests for Step 3

Add a `describe('Compact mode — Last Evaluation')` block:

**Multi-group eval renders collapsed by default:**
- Set up a running config with `statuses` containing `group_results` (3 groups, 1 passing)
- Assert `.section-collapse-header` is rendered inside `.indicator-results`
- Assert the summary text shows `"✓ Passing (Low VIX) · 1 of 3 groups passing"`
- Assert `.section-collapse-body` is NOT rendered
- Assert no `.indicator-group-dashboard` containers for eval (collapsed)

**Collapsed summary shows "Not passing" when no groups pass:**
- Set up statuses where all groups have `pass: false`
- Assert summary text shows `"✗ Not passing · 0 of 2 groups passing"`

**Clicking expands the Last Evaluation section:**
- Click `.section-collapse-header` in the eval section
- Assert `.section-collapse-body` IS rendered
- Assert `.indicator-group-dashboard` containers appear (one per group)
- Assert OR dividers appear between groups
- Assert overall-status line is visible at the bottom

**Group-level collapse within expanded eval section:**
- With section expanded, assert groups show name + pass/fail badge but no result chips
- Click a group header → assert `.results-grid` appears with result chips for that group
- Other groups remain collapsed

**Single-group running config unchanged:**
- Set up statuses with flat `indicator_results` (no `group_results`)
- Assert no `.section-collapse-header` in the eval section
- Assert result chips render flat as before

**State persists across data updates (FR-5.3):**
- Expand the eval section for a card
- Update `statuses` with new data (simulating WebSocket update)
- Assert the section is still expanded after `nextTick()`
- Expand a specific group, update data again, assert group is still expanded

---

## Step 4: Integration verification + regression check

**Goal:** Confirm all existing and new tests pass, no regressions.

**Files to verify (read-only):**
- `trade-app/tests/AutomationDashboard.test.js` — all tests green
- All existing frontend tests — full suite passes

### 4a. Run the full frontend test suite

```sh
cd trade-app && npx vitest run
```

Verify:
- All existing tests in `AutomationDashboard.test.js` still pass (AC-10)
- All new compact mode tests pass
- No other test files broken

### 4b. Run backend tests (sanity check)

```sh
cd trade-backend-go && go test ./...
```

Verify no backend test regressions (this is a frontend-only change, but confirm nothing was accidentally modified).

### 4c. Final commit

Single commit with all changes:

```
feat(automation): add progressive disclosure to dashboard cards for multi-group configs
```

Files in the commit:
- `trade-app/src/components/automation/AutomationDashboard.vue`
- `trade-app/tests/AutomationDashboard.test.js`

---

## Summary: Acceptance Criteria Coverage

| AC | Covered In |
|----|-----------|
| AC-1 | Step 2a — collapsed summary line for Entry Criteria |
| AC-2 | Step 2a — click toggles expansion |
| AC-3 | Step 3a — collapsed summary line for Last Evaluation |
| AC-4 | Step 3a — click toggles expansion |
| AC-5 | Steps 2a, 3a — group-level collapse within expanded sections |
| AC-6 | Steps 2a, 3a — single-group/flat templates unchanged |
| AC-7 | Step 1b — state stored in separate refs, not derived from data; tested in Step 3b |
| AC-8 | Step 4a — regression tests confirm existing functionality |
| AC-9 | Steps 2, 3 — compact cards by default reduce height variance |
| AC-10 | Step 4a — full test suite passes |
