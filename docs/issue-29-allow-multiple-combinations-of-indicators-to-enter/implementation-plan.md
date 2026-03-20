# Implementation Plan: Indicator Groups for Trade Automation

**Issue:** [#29 — Allow multiple combinations of indicators to enter a trade](https://github.com/schardosin/juicytrade/issues/29)
**Prerequisites:** [requirements.md](./requirements.md) · [architecture.md](./architecture.md) · [ux-design.md](./ux-design.md)

---

## Overview

This plan breaks the work into **8 self-contained steps**. Each step produces a compilable/runnable commit with its own tests. Steps 1–5 are backend, steps 6–7 are frontend, and step 8 is a final integration verification pass.

Dependencies flow top-down: each step builds on the previous, but each is independently testable.

---

## Step 1: Data Model — Add `IndicatorGroup`, `GroupResult`, and Update Structs

**Files modified:**
- `trade-backend-go/internal/automation/types/types.go`
- `trade-backend-go/internal/automation/models.go`

**Changes:**

1. **Add `IndicatorGroup` struct** to `types.go` (after `IndicatorConfig`, ~line 108):
   ```go
   type IndicatorGroup struct {
       ID         string            `json:"id"`
       Name       string            `json:"name"`
       Indicators []IndicatorConfig `json:"indicators"`
   }
   ```

2. **Add `GroupResult` struct** to `types.go` (after `IndicatorResult`, ~line 125):
   ```go
   type GroupResult struct {
       GroupID          string            `json:"group_id"`
       GroupName        string            `json:"group_name"`
       Pass             bool              `json:"pass"`
       IndicatorResults []IndicatorResult `json:"indicator_results"`
   }
   ```

3. **Add `GenerateGroupID` function** to `types.go` (next to `GenerateIndicatorID`, ~line 288):
   ```go
   func GenerateGroupID() string {
       return fmt.Sprintf("grp_%d_%s", time.Now().UnixNano(), randomString(4))
   }
   ```

4. **Add `IndicatorGroups` field** to `AutomationConfig` (~line 168):
   ```go
   IndicatorGroups []IndicatorGroup `json:"indicator_groups,omitempty"`
   ```
   Place it immediately after the `Indicators` field. Leave `Indicators` as-is for backward compatibility.

5. **Add `GetEffectiveIndicatorGroups` method** on `AutomationConfig`:
   ```go
   func (c *AutomationConfig) GetEffectiveIndicatorGroups() []IndicatorGroup {
       if len(c.IndicatorGroups) > 0 {
           return c.IndicatorGroups
       }
       if len(c.Indicators) > 0 {
           return []IndicatorGroup{
               {ID: "default", Name: "Default", Indicators: c.Indicators},
           }
       }
       return []IndicatorGroup{}
   }
   ```

6. **Add `GroupResults` field** to `ActiveAutomation` (~line 223):
   ```go
   GroupResults []GroupResult `json:"group_results,omitempty"`
   ```
   Place it immediately after `IndicatorResults`.

7. **Update `NewAutomationConfig`** to initialize `IndicatorGroups` to an empty slice:
   ```go
   IndicatorGroups: []IndicatorGroup{}
   ```

8. **Update `models.go`** — add type aliases:
   ```go
   IndicatorGroup = types.IndicatorGroup
   GroupResult    = types.GroupResult
   ```
   And re-export `GenerateGroupID`:
   ```go
   GenerateGroupID = types.GenerateGroupID
   ```

**Tests — create `trade-backend-go/internal/automation/types/types_test.go`:**

| Test | Description |
|------|-------------|
| `TestGetEffectiveIndicatorGroups_HasGroups` | Config with `IndicatorGroups` populated → returns them directly |
| `TestGetEffectiveIndicatorGroups_LegacyOnly` | Config with only `Indicators` → wraps in single "Default" group |
| `TestGetEffectiveIndicatorGroups_BothEmpty` | Both fields empty → returns empty slice |
| `TestGetEffectiveIndicatorGroups_GroupsTakePriority` | Both `IndicatorGroups` and `Indicators` populated → `IndicatorGroups` wins |
| `TestGenerateGroupID_Format` | Generated ID starts with `grp_` and has expected length |

**Verification:** `cd trade-backend-go && go build ./... && go test ./internal/automation/types/...`

---

## Step 2: Evaluation Logic — Add `EvaluateIndicatorGroups` to Indicator Service

**Files modified:**
- `trade-backend-go/internal/automation/indicators/service.go`

**Changes:**

1. **Add `EvaluateIndicatorGroups` method** to `Service` (after `AllIndicatorsPass`, ~line 543):
   ```go
   // EvaluateIndicatorGroups evaluates all indicator groups and returns per-group results.
   // All groups are always evaluated (no short-circuit) so the dashboard shows the full picture.
   // Returns: (groupResults, flatResults, anyGroupPasses)
   func (s *Service) EvaluateIndicatorGroups(
       ctx context.Context,
       configID string,
       groups []types.IndicatorGroup,
   ) ([]types.GroupResult, []types.IndicatorResult, bool) {
       groupResults := make([]types.GroupResult, 0, len(groups))
       allFlatResults := make([]types.IndicatorResult, 0)
       anyGroupPasses := false

       for _, group := range groups {
           indicatorResults := s.EvaluateAllIndicators(ctx, configID, group.Indicators)
           groupPass := s.AllIndicatorsPass(indicatorResults)

           groupResults = append(groupResults, types.GroupResult{
               GroupID:          group.ID,
               GroupName:        group.Name,
               Pass:             groupPass,
               IndicatorResults: indicatorResults,
           })

           allFlatResults = append(allFlatResults, indicatorResults...)

           if groupPass {
               anyGroupPasses = true
           }
       }

       return groupResults, allFlatResults, anyGroupPasses
   }
   ```

**Key design decisions (per architecture.md):**
- No short-circuit — all groups always evaluated (FR-2.6)
- Reuses existing `EvaluateAllIndicators` and `AllIndicatorsPass` per group
- Stale data in one group does not block other groups (FR-2.4)

**Tests — add to `trade-backend-go/internal/automation/indicators/service_test.go` (new file):**

These tests can use in-memory mock results since `EvaluateIndicatorGroups` delegates to `EvaluateAllIndicators` which calls `EvaluateIndicator`. For unit testing the group logic, we can construct `IndicatorResult` values directly and test `AllIndicatorsPass` behavior through the group function. For the `EvaluateIndicatorGroups` itself, since it calls out to provider APIs, we test the group-level OR/AND logic via a testable wrapper:

| Test | Description |
|------|-------------|
| `TestAllIndicatorsPass_AllPass` | 2 passing results → true |
| `TestAllIndicatorsPass_OneFails` | 1 passing + 1 failing → false |
| `TestAllIndicatorsPass_StaleBlocks` | 1 passing + 1 stale → false |
| `TestAllIndicatorsPass_DisabledSkipped` | 1 disabled + 1 passing → true |
| `TestAllIndicatorsPass_Empty` | No results → true (vacuously) |
| `TestGroupResultLogic_SingleGroupPass` | Construct 1 GroupResult with pass=true, verify OR logic |
| `TestGroupResultLogic_TwoGroupsFirstPasses` | Group A pass, Group B fail → anyGroupPasses=true |
| `TestGroupResultLogic_TwoGroupsBothFail` | Both fail → anyGroupPasses=false |
| `TestGroupResultLogic_EmptyGroups` | No groups → anyGroupPasses=false |
| `TestGroupResultLogic_FlatResultsAggregation` | Verify flat results concatenate all indicator results in order |

Note: We test the pure logic of `AllIndicatorsPass` (already existing, no mocks needed) and create a lightweight test helper that constructs `GroupResult` slices to verify the OR/AND composition without needing provider mocks.

**Verification:** `cd trade-backend-go && go test ./internal/automation/indicators/...`

---

## Step 3: Storage Migration — Migrate Flat Indicators to Groups on Load

**Files modified:**
- `trade-backend-go/internal/automation/storage.go`

**Changes:**

1. **Add `migrateIndicatorGroups` method** (after `migrateIndicatorIDs`, ~line 109):
   ```go
   func (s *Storage) migrateIndicatorGroups() bool {
       migrated := false
       for _, config := range s.configs {
           if len(config.IndicatorGroups) > 0 {
               continue
           }
           if len(config.Indicators) == 0 {
               continue
           }
           config.IndicatorGroups = []types.IndicatorGroup{
               {
                   ID:         types.GenerateGroupID(),
                   Name:       "Default",
                   Indicators: config.Indicators,
               },
           }
           config.Indicators = []types.IndicatorConfig{}
           slog.Info("Migrated indicators to group",
               "automation", config.Name,
               "groupName", "Default",
               "indicatorCount", len(config.IndicatorGroups[0].Indicators))
           migrated = true
       }
       return migrated
   }
   ```

2. **Update `migrateIndicatorIDs`** (~line 92) to also iterate through `IndicatorGroups`:
   ```go
   // After existing loop over config.Indicators:
   for g := range config.IndicatorGroups {
       for i := range config.IndicatorGroups[g].Indicators {
           if config.IndicatorGroups[g].Indicators[i].ID == "" {
               config.IndicatorGroups[g].Indicators[i].ID = types.GenerateIndicatorID()
               migrated = true
           }
       }
   }
   ```

3. **Update `load()`** (~line 60) to chain both migrations with a shared `needsSave` flag:
   ```go
   needsSave := false
   if s.migrateIndicatorIDs() {
       needsSave = true
   }
   if s.migrateIndicatorGroups() {
       needsSave = true
   }
   if needsSave {
       if err := s.saveWithoutLock(); err != nil {
           slog.Warn("Failed to save migrated configs", "error", err)
       }
   }
   ```
   Replace the current direct `migrateIndicatorIDs()` call and its conditional save.

4. **Bump storage version** in `saveWithoutLock` (~line 119): change `"1.0"` to `"1.1"`.

**Tests — create `trade-backend-go/internal/automation/storage_test.go`:**

These tests can be done with in-memory Storage structs (populate `s.configs` directly, call migration, check results):

| Test | Description |
|------|-------------|
| `TestMigrateIndicatorGroups_FlatToGroup` | Config with `Indicators` and no `IndicatorGroups` → creates "Default" group, clears `Indicators` |
| `TestMigrateIndicatorGroups_AlreadyMigrated` | Config with `IndicatorGroups` → returns false, no changes |
| `TestMigrateIndicatorGroups_EmptyIndicators` | Config with empty `Indicators` and no groups → returns false, no changes |
| `TestMigrateIndicatorGroups_MultipleConfigs` | Mix of migrated and unmigrated configs → only unmigrated ones get migrated |
| `TestMigrateIndicatorIDs_AlsoCoversGroups` | Config with groups containing indicators with empty IDs → IDs get backfilled |
| `TestMigrationOrdering` | Verify `migrateIndicatorIDs` runs before `migrateIndicatorGroups` — indicators get IDs before grouping |

**Verification:** `cd trade-backend-go && go test ./internal/automation/...`

---

## Step 4: Engine — Update `handleWaitingState` and `EvaluateIndicators` to Use Groups

**Files modified:**
- `trade-backend-go/internal/automation/engine.go`

**Changes:**

1. **Update `handleWaitingState`** (~line 419-425). Replace the flat evaluation block:

   **Before:**
   ```go
   results := e.indicatorService.EvaluateAllIndicators(ctx, id, active.Config.Indicators)
   e.mu.Lock()
   active.IndicatorResults = results
   active.AllIndicatorsPass = e.indicatorService.AllIndicatorsPass(results)
   active.LastEvaluation = time.Now()
   e.mu.Unlock()
   ```

   **After:**
   ```go
   groups := active.Config.GetEffectiveIndicatorGroups()
   groupResults, flatResults, anyGroupPasses := e.indicatorService.EvaluateIndicatorGroups(ctx, id, groups)
   e.mu.Lock()
   active.GroupResults = groupResults
   active.IndicatorResults = flatResults
   active.AllIndicatorsPass = anyGroupPasses
   active.LastEvaluation = time.Now()
   e.mu.Unlock()
   ```

2. **Update stale indicator check** (~line 429-434). The existing stale check iterates `results`. Update it to iterate `flatResults` (same variable name change). The logic is identical — it's still checking all results across all groups for stale data to emit the correct log message. No behavioral change.

3. **Update `EvaluateIndicators` method** (~line 307-309). This is the public method called by handlers:

   **Before:**
   ```go
   func (e *Engine) EvaluateIndicators(ctx context.Context, config *types.AutomationConfig) []types.IndicatorResult {
       return e.indicatorService.EvaluateAllIndicators(ctx, config.ID, config.Indicators)
   }
   ```

   **After:**
   ```go
   func (e *Engine) EvaluateIndicators(ctx context.Context, config *types.AutomationConfig) ([]types.GroupResult, []types.IndicatorResult, bool) {
       groups := config.GetEffectiveIndicatorGroups()
       return e.indicatorService.EvaluateIndicatorGroups(ctx, config.ID, groups)
   }
   ```

**Tests:** The engine's `handleWaitingState` is integration-heavy (requires provider mocks). The key logic change is that it now calls `EvaluateIndicatorGroups` instead of `EvaluateAllIndicators`, which was tested in Step 2. Existing tests (if any) that exercise the engine loop should still pass since `GetEffectiveIndicatorGroups` falls back to wrapping legacy `Indicators` in a group.

**Verification:** `cd trade-backend-go && go build ./... && go test ./internal/automation/...`

---

## Step 5: API & WebSocket — Update Handlers and Broadcast to Include Group Results

**Files modified:**
- `trade-backend-go/internal/api/handlers/automation.go`
- `trade-backend-go/cmd/server/main.go`

### 5a: Update `handlers/automation.go`

1. **Update `EvaluateIndicators` handler** (~line 332-357). Change to use the new 3-return-value `Engine.EvaluateIndicators`:

   **Before:**
   ```go
   results := h.engine.EvaluateIndicators(c.Request.Context(), config)
   allPass := h.engine.GetIndicatorService().AllIndicatorsPass(results)
   ```

   **After:**
   ```go
   groupResults, flatResults, anyGroupPasses := h.engine.EvaluateIndicators(c.Request.Context(), config)
   ```

   Update the response JSON:
   ```go
   c.JSON(http.StatusOK, gin.H{
       "success": true,
       "data": map[string]interface{}{
           "indicators":    flatResults,
           "group_results": groupResults,
           "all_pass":      anyGroupPasses,
           "symbol":        config.Symbol,
       },
       "message": "Indicators evaluated successfully",
   })
   ```

2. **Update `EvaluateIndicatorsPreview` handler** (~line 359-395). Change the request struct to also accept `indicator_groups`:

   ```go
   var request struct {
       Indicators      []types.IndicatorConfig `json:"indicators"`
       IndicatorGroups []types.IndicatorGroup  `json:"indicator_groups"`
   }
   ```

   After parsing, determine which groups to evaluate:
   ```go
   var groups []types.IndicatorGroup
   if len(request.IndicatorGroups) > 0 {
       groups = request.IndicatorGroups
   } else if len(request.Indicators) > 0 {
       groups = []types.IndicatorGroup{
           {ID: "preview_default", Name: "Default", Indicators: request.Indicators},
       }
   } else {
       // Existing default indicator fallback
       groups = []types.IndicatorGroup{
           {ID: "preview_default", Name: "Default", Indicators: []types.IndicatorConfig{
               types.NewIndicatorConfig(types.IndicatorVIX),
               types.NewIndicatorConfig(types.IndicatorGap),
               types.NewIndicatorConfig(types.IndicatorRange),
               types.NewIndicatorConfig(types.IndicatorTrend),
               types.NewIndicatorConfig(types.IndicatorCalendar),
           }},
       }
   }

   groupResults, flatResults, anyGroupPasses := h.engine.GetIndicatorService().EvaluateIndicatorGroups(
       c.Request.Context(), "", groups)

   c.JSON(http.StatusOK, gin.H{
       "success": true,
       "data": map[string]interface{}{
           "indicators":    flatResults,
           "group_results": groupResults,
           "all_pass":      anyGroupPasses,
       },
       "message": "Indicators evaluated successfully",
   })
   ```

3. **Update `CreateConfig` handler** (~line 82-137). Change the default indicators fallback (~line 102-110). Currently it populates `config.Indicators` with 5 defaults when the indicators array is empty. Update to also check `IndicatorGroups` — only apply the default if **both** `Indicators` and `IndicatorGroups` are empty:

   ```go
   if len(config.Indicators) == 0 && len(config.IndicatorGroups) == 0 {
       config.Indicators = []types.IndicatorConfig{
           types.NewIndicatorConfig(types.IndicatorVIX),
           // ... existing defaults ...
       }
   }
   ```

### 5b: Update `cmd/server/main.go`

1. **Add `group_results` to the WebSocket broadcast** (~line 1565-1576). Add one line to the data map:

   ```go
   "group_results": automation.GroupResults,
   ```

**Tests:** The handler changes are tested via the frontend integration tests in steps 6–7. The WebSocket change is purely additive (new field in the map). Existing backend tests should continue to pass since the response format is backward-compatible (new `group_results` field is added alongside existing fields).

**Verification:** `cd trade-backend-go && go build ./... && go test ./...`

---

## Step 6: Frontend Config Form — Restructure Indicators Section for Groups

**Files modified:**
- `trade-app/src/components/automation/AutomationConfigForm.vue`

This is the largest single-file change. The approach is methodical: update the data model first, then the template, then the methods.

### 6a: Data Model Changes (script section)

1. **Update `config` ref initialization** (~line 814-841). Replace `indicators: []` with:
   ```js
   indicators: [], // Legacy — kept empty for backward compat
   indicator_groups: [
     { id: generateGroupId(), name: 'Default', indicators: [] }
   ],
   ```

2. **Add `generateGroupId` function** (near `generateIndicatorId`, ~line 1031):
   ```js
   const generateGroupId = () => {
     return `grp_${Date.now()}_${Math.random().toString(36).substring(2, 6)}`
   }
   ```

3. **Add `groupTestResults` ref** for per-group pass/fail after Test All:
   ```js
   const groupTestResults = ref({}) // groupId → { pass: bool }
   ```

4. **Update `addIndicator`** (~line 1036). It currently pushes to `config.value.indicators`. Change it to accept a `groupIndex` parameter and push to the correct group:
   ```js
   const addIndicatorToGroup = (type, groupIndex) => {
     // ... same indicator construction logic ...
     config.value.indicator_groups[groupIndex].indicators.push(newIndicator)
     expandedIndicators.value[newIndicator.id] = true
     showAddIndicatorDialog.value = false
   }
   ```

5. **Add group management methods:**
   ```js
   // Track which group the "Add Indicator" dialog is targeting
   const addIndicatorTargetGroup = ref(0)

   const addGroup = () => {
     const groupNum = config.value.indicator_groups.length + 1
     config.value.indicator_groups.push({
       id: generateGroupId(),
       name: `Group ${groupNum}`,
       indicators: []
     })
   }

   const removeGroup = (groupIndex) => {
     const group = config.value.indicator_groups[groupIndex]
     if (group.indicators.length > 0) {
       if (!confirm(`Delete group "${group.name}" and its ${group.indicators.length} indicators?`)) {
         return
       }
     }
     // Clean up test results for indicators in this group
     group.indicators.forEach(ind => {
       delete indicatorResults.value[ind.id]
       delete expandedIndicators.value[ind.id]
     })
     delete groupTestResults.value[group.id]
     config.value.indicator_groups.splice(groupIndex, 1)
   }

   const removeIndicatorFromGroup = (groupIndex, indicatorId) => {
     const group = config.value.indicator_groups[groupIndex]
     group.indicators = group.indicators.filter(ind => ind.id !== indicatorId)
     delete indicatorResults.value[indicatorId]
     delete expandedIndicators.value[indicatorId]
   }
   ```

6. **Update `loadConfig`** (~line 1072-1097). After merging configData, handle legacy configs that might only have `indicators`:
   ```js
   // Ensure indicator_groups is populated
   if (!config.value.indicator_groups || config.value.indicator_groups.length === 0) {
     if (config.value.indicators && config.value.indicators.length > 0) {
       config.value.indicator_groups = [{
         id: generateGroupId(),
         name: 'Default',
         indicators: config.value.indicators
       }]
       config.value.indicators = []
     } else {
       config.value.indicator_groups = [{ id: generateGroupId(), name: 'Default', indicators: [] }]
     }
   }
   ```

7. **Update `testAllIndicators`** (~line 1179-1229). Change to send `indicator_groups` and process `group_results`:
   ```js
   const testAllIndicators = async () => {
     testingAll.value = true
     allIndicatorsResult.value = null
     indicatorResults.value = {}
     groupTestResults.value = {}

     try {
       const groupsPayload = config.value.indicator_groups.map(g => ({
         ...g,
         indicators: g.indicators.filter(ind => ind.enabled)
       }))

       const response = await api.previewAutomationIndicators({
         indicator_groups: groupsPayload
       })

       const groupResultsData = response.data?.group_results || []
       const anyGroupPasses = response.data?.all_pass ?? false

       // Map group results
       groupResultsData.forEach(gr => {
         groupTestResults.value[gr.group_id] = { pass: gr.pass }

         // Map per-indicator results by ID within each group
         const matchingGroup = config.value.indicator_groups.find(g => g.id === gr.group_id)
         if (matchingGroup && gr.indicator_results) {
           const enabledInGroup = matchingGroup.indicators.filter(ind => ind.enabled)
           const typeCounters = {}
           gr.indicator_results.forEach(result => {
             const typeList = enabledInGroup.filter(ind => ind.type === result.type)
             const idx = typeCounters[result.type] || 0
             typeCounters[result.type] = idx + 1
             const match = typeList[idx]
             if (match) {
               indicatorResults.value[match.id] = {
                 value: result.value,
                 passed: result.pass,
                 operator: result.operator,
                 threshold: result.threshold,
                 symbol: result.symbol,
                 details: result.details,
                 stale: result.stale || false,
                 error: result.error || ''
               }
             }
           })
         }
       })

       allIndicatorsResult.value = anyGroupPasses
     } catch (err) {
       console.error('Failed to test indicators:', err)
       allIndicatorsResult.value = false
     } finally {
       testingAll.value = false
     }
   }
   ```

8. **Update `saveConfig`** (~line 1117-1143). Change the cleanup to operate on `indicator_groups` instead of `indicators`, and clear the legacy field:
   ```js
   const cleanedConfig = {
     ...config.value,
     indicators: [], // Legacy field — always empty
     indicator_groups: config.value.indicator_groups.map(group => ({
       ...group,
       indicators: group.indicators.map(ind => ({
         ...ind,
         symbol: ind.symbol?.trim() || undefined
       }))
     }))
   }
   ```

### 6b: Template Changes

1. **Replace the indicators section** (~line 107-254). Replace the flat `v-for="indicator in config.indicators"` with a grouped structure:
   - Update the section description text based on group count (1 group → AND-only text; 2+ groups → AND+OR explanation per ux-design.md section 2.1)
   - Loop over `config.indicator_groups` with group containers
   - Single group → `.indicator-group.lightweight` class (no visible border/bg)
   - Multiple groups → `.indicator-group` with visible container + OR dividers between groups
   - Each group has: editable name input, per-group pass/fail badge, delete button (hidden when 1 group), indicator cards, "Add Indicator" button
   - OR divider rendered between groups (not before first, not after last)
   - "Add Group" button placement changes: inline in action bar for 1 group, standalone below groups for 2+
   - "Test All" button moved outside group containers, overall result message updated for group-aware wording

2. **Update Add Indicator dialog** (~line 257-294). The `@click="addIndicator(type.value)"` changes to `@click="addIndicatorToGroup(type.value, addIndicatorTargetGroup)"`. Dialog header can optionally show group name.

### 6c: Scoped CSS Changes

Add new CSS classes per ux-design.md section 5:
- `.indicator-group`, `.indicator-group.lightweight`
- `.group-header`, `.group-name-input`, `.group-header-actions`
- `.group-test-badge`
- `.group-body`, `.group-actions`
- `.or-divider`, `.or-divider-label`
- `.section-actions`

### 6d: Expose in Return Object

Add all new refs and methods to the `return` object in `setup()`:
- `groupTestResults`, `addIndicatorTargetGroup`
- `generateGroupId`, `addGroup`, `removeGroup`, `removeIndicatorFromGroup`, `addIndicatorToGroup`

**Tests — create `trade-app/tests/AutomationConfigForm.test.js`:**

| Test | Description |
|------|-------------|
| `renders with a single default group` | Mount form → verify one group container exists with "Default" name |
| `add group creates new empty group` | Click "Add Group" → verify `indicator_groups` length increases by 1 |
| `remove group with confirmation` | Add 2 groups, add indicator to group 2, click delete → confirm dialog shown |
| `remove group button hidden for last group` | Only 1 group → delete button not rendered |
| `add indicator to specific group` | Open dialog from group 1, select type → indicator added to group 1's array |
| `remove indicator from group` | Click remove on indicator → removed from correct group |
| `group name editing` | Change group name input → `indicator_groups[i].name` updated |
| `OR divider shown with 2+ groups` | Add 2 groups → `.or-divider` element exists |
| `no OR divider with 1 group` | Only 1 group → `.or-divider` element does not exist |
| `save sends indicator_groups, not indicators` | Click save → verify API call payload has `indicator_groups` and `indicators: []` |
| `loads legacy config with client-side migration` | Mock API returns config with `indicators` only → form wraps them in a group |

**Verification:** `cd trade-app && npx vitest run tests/AutomationConfigForm.test.js`

---

## Step 7: Frontend Dashboard — Group-Aware Indicator Display

**Files modified:**
- `trade-app/src/components/automation/AutomationDashboard.vue`

### 7a: Template Changes

1. **Update Entry Criteria section** (~line 96-110). Replace the flat `indicators-grid` with a grouped display when `group_results` is available:
   - Check `config.indicator_groups?.length > 1` to decide between grouped and flat display
   - For single group: render flat (identical to current) for lightweight feel
   - For multiple groups: render each group in a `.indicator-group-dashboard` container with group name label, indicator chips inside, and OR dividers between groups

2. **Update "Last Evaluation" section** (~line 162-181). When `group_results` is available from status:
   - Render grouped result chips with per-group pass/fail badges
   - Add overall status line below all groups ("Passing (Low VIX)" or "Not passing")
   - Fall back to flat `indicator_results` display when `group_results` is absent (backward compat)

3. **Update Evaluation Result Dialog** (~line 304-338). When the evaluate response includes `group_results`:
   - Group results display with per-group header showing pass/fail
   - OR dividers between groups
   - Summary text changes from "All indicators passed" to group-aware messaging

4. **Update `getEnabledIndicators`** (~line 548-550). Currently reads from `config.indicators`. Update to also check `config.indicator_groups`:
   ```js
   const getEnabledIndicators = (config) => {
     if (config.indicator_groups?.length > 0) {
       return config.indicator_groups.flatMap(g => g.indicators.filter(ind => ind.enabled))
     }
     return (config.indicators || []).filter(ind => ind.enabled)
   }
   ```

5. **Update `duplicateConfig`** (~line 659-689). Include `indicator_groups` in the copy alongside `indicators`.

### 7b: Script Changes

1. **Update `evaluateIndicators`** (~line 628-640). Process the group-aware response:
   ```js
   const evaluateIndicators = async (config) => {
     actionLoading.value = `eval_${config.id}`
     try {
       const response = await api.evaluateAutomationConfig(config.id)
       evalResult.value = {
         all_passed: response.data?.all_pass ?? false,
         results: response.data?.indicators || [],
         group_results: response.data?.group_results || [],
       }
       showEvalDialog.value = true
     } catch (err) {
       // ... existing error handling ...
     } finally {
       actionLoading.value = null
     }
   }
   ```

2. **Update `formatIndicatorType`** (~line 562-571). The existing map only knows 5 types. Extend or make it more generic. The config form already uses metadata — the dashboard should also consult `indicator_groups` metadata if available, but a quick fallback map is sufficient for now. Add the new indicator types to the map:
   ```js
   const formatIndicatorType = (type) => {
     const types = {
       vix: 'VIX', gap: 'Gap', range: 'Range', trend: 'Trend', calendar: 'FOMC',
       rsi: 'RSI', macd: 'MACD', momentum: 'Momentum', cmo: 'CMO',
       stoch: 'Stoch', stoch_rsi: 'StochRSI', adx: 'ADX', cci: 'CCI',
       sma: 'SMA', ema: 'EMA', atr: 'ATR', bb_percent: 'BB%B'
     }
     return types[type] || type
   }
   ```

### 7c: Scoped CSS Changes

Add new dashboard-specific CSS classes per ux-design.md section 5:
- `.indicator-group-dashboard`, `.group-label`, `.group-result-badge`
- `.or-divider-compact`
- `.overall-status`

**Tests — update `trade-app/tests/AutomationDashboard.test.js`:**

| Test | Description |
|------|-------------|
| `renders grouped indicators when config has indicator_groups` | Mock config with 2 groups → verify `.indicator-group-dashboard` containers rendered |
| `renders flat indicators for legacy config` | Mock config with only `indicators` → flat display (backward compat) |
| `OR divider shown between groups` | 2 groups → `.or-divider-compact` element exists |
| `no OR divider for single group` | 1 group → no `.or-divider-compact` |
| `group results from WebSocket update` | Simulate WebSocket message with `group_results` → verify per-group pass/fail displayed |
| `fallback to flat when no group_results` | WebSocket message without `group_results` → flat display |
| `getEnabledIndicators reads from indicator_groups` | Config with `indicator_groups` → returns flattened enabled indicators |
| `duplicate config includes indicator_groups` | Verify `duplicateConfig` copies `indicator_groups` |

**Verification:** `cd trade-app && npx vitest run tests/AutomationDashboard.test.js`

---

## Step 8: Integration Verification & Regression Check

**No new code changes.** This step is a full regression and integration verification.

**Actions:**

1. **Run full backend test suite:**
   ```sh
   cd trade-backend-go && go test ./...
   ```
   All existing tests must pass. The data model changes are backward-compatible (new optional fields). The engine changes use `GetEffectiveIndicatorGroups` which falls back to legacy `Indicators`.

2. **Run full frontend test suite:**
   ```sh
   cd trade-app && npx vitest run
   ```
   All existing tests must pass. The dashboard's `handleAutomationUpdate` merges all data generically and won't break from the new `group_results` field.

3. **Manual smoke test** (if running locally with `make dev`):
   - Load an existing automation config → verify it auto-migrated to a single "Default" group
   - Edit the config → see the single group in the form (lightweight mode)
   - Add a second group → see OR divider, group containers appear
   - Add indicators to each group → save → reload → groups persist
   - Test All → see per-group pass/fail badges and overall result
   - Start automation → dashboard shows grouped indicator results
   - Delete a group → verify transition back to lightweight mode

4. **Verify backward compatibility:**
   - Old `automations.json` with flat `indicators` → auto-migrates to v1.1 format
   - After migration, automation starts and evaluates correctly with grouped logic
   - WebSocket broadcasts include `group_results` alongside existing fields

---

## Summary Table

| Step | Scope | Files | Tests |
|------|-------|-------|-------|
| 1 | Data model: `IndicatorGroup`, `GroupResult`, struct updates | `types.go`, `models.go` | `types_test.go` |
| 2 | Evaluation logic: `EvaluateIndicatorGroups` | `indicators/service.go` | `indicators/service_test.go` |
| 3 | Storage migration: flat → groups | `storage.go` | `storage_test.go` |
| 4 | Engine: use group evaluation in `handleWaitingState` | `engine.go` | Build + existing tests |
| 5 | API handlers + WebSocket broadcast | `handlers/automation.go`, `main.go` | Build + existing tests |
| 6 | Frontend config form: grouped UI | `AutomationConfigForm.vue` | `AutomationConfigForm.test.js` |
| 7 | Frontend dashboard: group-aware display | `AutomationDashboard.vue` | `AutomationDashboard.test.js` |
| 8 | Integration verification & regression | None (test only) | Full `go test ./...` + `npx vitest run` |
