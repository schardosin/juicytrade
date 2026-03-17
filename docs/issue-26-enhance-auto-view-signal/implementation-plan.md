# Implementation Plan: Enhance Auto View Signal Colors (Issue #26)

## Summary

All changes are in a single file: `trade-app/src/components/automation/AutomationDashboard.vue`.
A new test file is created at `trade-app/tests/AutomationDashboard.test.js`.
No backend changes required — `all_indicators_pass` is already available in both WebSocket updates and REST polling responses.

---

## Step 1: Update `getStatusClass()` to return 4 states instead of 3

**File:** `AutomationDashboard.vue` — JS, lines 489-493

**Current logic:**
```js
const getStatusClass = (config) => {
  if (!config.enabled) return 'disabled'
  if (isConfigRunning(config.id)) return 'running'
  return 'idle'
}
```

**New logic:**
```js
const getStatusClass = (config) => {
  if (!config.enabled) return 'disabled'
  if (isConfigRunning(config.id)) {
    const status = getAutomationStatus(config.id)
    return status?.all_indicators_pass === true ? 'running-ready' : 'running-not-ready'
  }
  return 'idle'
}
```

**Behavior by case:**
| Condition | Return value |
|-----------|-------------|
| `config.enabled === false` | `'disabled'` |
| Enabled, not running | `'idle'` |
| Running, `all_indicators_pass === true` | `'running-ready'` |
| Running, `all_indicators_pass === false` | `'running-not-ready'` |
| Running, `all_indicators_pass` is `undefined`/`null` (not yet evaluated) | `'running-not-ready'` (FR-7: default to yellow until confirmed) |
| Running, no indicators (vacuously true from backend) | `'running-ready'` (FR-8) |

**Why strict `=== true`:** The falsy default covers `undefined` (first poll before evaluation), `null`, and `false` — all map to yellow. Only an explicit `true` from the backend earns green.

**Tests for this step:**
1. Disabled config returns `'disabled'`
2. Enabled, not running returns `'idle'`
3. Running with `all_indicators_pass: true` returns `'running-ready'`
4. Running with `all_indicators_pass: false` returns `'running-not-ready'`
5. Running with `all_indicators_pass: undefined` returns `'running-not-ready'` (FR-7)
6. Running with `all_indicators_pass: null` returns `'running-not-ready'` (FR-7)
7. Running with no status entry at all returns `'running-not-ready'`

---

## Step 2: Update `getRunningStatusClass()` for badge consistency

**File:** `AutomationDashboard.vue` — JS, lines 495-513

**Current logic:**
```js
const getRunningStatusClass = (config) => {
  if (!config.enabled) return 'status-disabled'
  const status = getAutomationStatus(config.id)
  if (status) {
    switch (status.state) {
      case 'waiting':
      case 'evaluating':
        return 'status-waiting'       // always yellow
      case 'trading':
      case 'monitoring':
        return 'status-running'       // always green
      case 'completed':
        return 'status-completed'
      case 'failed':
        return 'status-failed'
    }
  }
  return 'status-idle'
}
```

**New logic:**
```js
const getRunningStatusClass = (config) => {
  if (!config.enabled) return 'status-disabled'
  const status = getAutomationStatus(config.id)
  if (status) {
    switch (status.state) {
      case 'waiting':
      case 'evaluating':
        // Badge reflects indicator readiness (FR-5)
        return status.all_indicators_pass === true ? 'status-running' : 'status-waiting'
      case 'trading':
      case 'monitoring':
        return 'status-running'
      case 'completed':
        return 'status-completed'
      case 'failed':
        return 'status-failed'
    }
  }
  return 'status-idle'
}
```

**Change scope:** Only the `waiting`/`evaluating` cases change. When `all_indicators_pass` is true during these phases, the badge switches from yellow (`status-waiting`) to green (`status-running`). The `trading`/`monitoring`, `completed`, and `failed` cases are unchanged.

No new CSS classes are needed — `status-waiting` (yellow) and `status-running` (green) already exist at lines 1053-1061.

**Tests for this step:**
1. Disabled config returns `'status-disabled'`
2. No status returns `'status-idle'`
3. `state: 'waiting'`, `all_indicators_pass: false` returns `'status-waiting'`
4. `state: 'waiting'`, `all_indicators_pass: true` returns `'status-running'`
5. `state: 'evaluating'`, `all_indicators_pass: false` returns `'status-waiting'`
6. `state: 'evaluating'`, `all_indicators_pass: true` returns `'status-running'`
7. `state: 'trading'` always returns `'status-running'` (regardless of `all_indicators_pass`)
8. `state: 'monitoring'` always returns `'status-running'`
9. `state: 'completed'` returns `'status-completed'`
10. `state: 'failed'` returns `'status-failed'`

---

## Step 3: Update CSS — rename `.running` to `.running-ready`, add `.running-not-ready`

**File:** `AutomationDashboard.vue` — `<style scoped>`, lines 971-973

**Current CSS:**
```css
.config-card.running {
  border-left: 4px solid var(--color-success);
}
```

**Replace with:**
```css
.config-card.running-ready {
  border-left: 4px solid var(--color-success);
}

.config-card.running-not-ready {
  border-left: 4px solid var(--color-warning);
}
```

**No other CSS changes:**
- `.config-card.idle` (line 975) — unchanged
- `.config-card.disabled` (line 979) — unchanged
- No new badge CSS — existing `.status-waiting` and `.status-running` already have correct colors
- No changes to transitions — `transition: var(--transition-normal)` on `.config-card` (line 963) already covers `border-left-color`
- No changes to mobile CSS — the left border renders identically in the single-column mobile layout

**Verification for this step:**
- Visual: disabled = gray border, idle = blue, running-not-ready = yellow, running-ready = green
- The `.config-card.running` selector no longer exists — confirm no other code references it (it was only used via `getStatusClass()`, which is updated in Step 1)

---

## Step 4: Write unit tests

**File:** `trade-app/tests/AutomationDashboard.test.js` (new file)

**Test infrastructure:**
- Framework: Vitest with `globals: true` (no explicit imports of `describe`/`it`/`expect`)
- Environment: `happy-dom` (configured in `vitest.config.js`)
- Setup file: `tests/setup.js` (mocks `authService` and `global.fetch`)
- Component mounting: `@vue/test-utils` `mount()` or `shallowMount()`
- Mocking: `vi.mock()` for `api.js`, `webSocketClient.js`, `useMobileDetection.js`, `vue-router`

**Test approach:** The functions under test (`getStatusClass`, `getRunningStatusClass`) are pure logic based on the `configs`, `statuses`, and helper function state. We can test them by:
1. Mounting the component with mocked dependencies
2. Setting `configs` and `statuses` reactive state via the component's exposed data
3. Calling the methods via the component VM

Alternatively, since these functions only depend on `statuses.value`, `config.enabled`, and `isConfigRunning()`, we can extract them as importable helpers for easier unit testing. However, to avoid refactoring, we will test via the component instance.

**Test cases (consolidated from Steps 1 and 2):**

### `getStatusClass()` tests
```
describe('getStatusClass', () => {
  it('returns "disabled" when config.enabled is false')
  it('returns "idle" when config is enabled but not running')
  it('returns "running-ready" when running and all_indicators_pass is true')
  it('returns "running-not-ready" when running and all_indicators_pass is false')
  it('returns "running-not-ready" when running and all_indicators_pass is undefined (not yet evaluated)')
  it('returns "running-not-ready" when running and all_indicators_pass is null')
  it('returns "running-not-ready" when running and no status entry exists')
})
```

### `getRunningStatusClass()` tests
```
describe('getRunningStatusClass', () => {
  it('returns "status-disabled" when config.enabled is false')
  it('returns "status-idle" when no status exists')
  it('returns "status-waiting" for state=waiting with all_indicators_pass=false')
  it('returns "status-running" for state=waiting with all_indicators_pass=true')
  it('returns "status-waiting" for state=evaluating with all_indicators_pass=false')
  it('returns "status-running" for state=evaluating with all_indicators_pass=true')
  it('returns "status-running" for state=trading regardless of all_indicators_pass')
  it('returns "status-running" for state=monitoring regardless of all_indicators_pass')
  it('returns "status-completed" for state=completed')
  it('returns "status-failed" for state=failed')
})
```

### CSS class application test (optional integration-level)
```
describe('card CSS classes', () => {
  it('applies running-ready class to card element when running and indicators pass')
  it('applies running-not-ready class to card element when running and indicators do not pass')
})
```

**Mocking strategy:**
```js
vi.mock('../../src/services/api.js', () => ({
  api: {
    getAutomationConfigs: vi.fn().mockResolvedValue({ data: { configs: [] } }),
    getAllAutomationStatuses: vi.fn().mockResolvedValue({ data: { automations: [] } }),
    // ... other methods as needed
  }
}))

vi.mock('../../src/services/webSocketClient.js', () => ({
  default: {
    addCallback: vi.fn(),
    removeCallback: vi.fn(),
  }
}))

vi.mock('../../src/composables/useMobileDetection.js', () => ({
  useMobileDetection: () => ({ isMobile: ref(false) })
}))
```

---

## Step 5: Run all tests and verify no regressions

**Commands:**
```bash
cd trade-app && npx vitest run --reporter=verbose
```

**Verification checklist:**
1. All 28 existing test files pass (no regressions)
2. New `AutomationDashboard.test.js` tests pass
3. No test references the old `'running'` CSS class string (only the new `'running-ready'`/`'running-not-ready'` values)
4. Build succeeds: `cd trade-app && npm run build` (confirms no template/CSS compilation errors)

---

## Execution Order

| Order | Step | Type | Estimated Scope |
|-------|------|------|-----------------|
| 1 | Step 1: Update `getStatusClass()` | JS logic | 6 lines changed |
| 2 | Step 3: Update CSS | CSS | 4 lines changed (rename + add) |
| 3 | Step 2: Update `getRunningStatusClass()` | JS logic | 2 lines changed |
| 4 | Step 4: Write tests | Test file | ~150-200 lines new |
| 5 | Step 5: Run tests + build | Verification | 0 lines changed |

Steps 1 and 3 are done together (the JS returns new class names, the CSS must match). Step 2 is independent. Step 4 depends on Steps 1-3 being complete. Step 5 is final verification.

**Total lines changed in `AutomationDashboard.vue`:** ~12 lines (JS + CSS).
**New file:** `trade-app/tests/AutomationDashboard.test.js` (~150-200 lines).
