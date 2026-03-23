# Test Plan: Easy Broker Account Switch (Issue #34)

**Requirements:** [requirements.md](./requirements.md)
**Architecture:** [architecture.md](./architecture.md)
**Test File:** `trade-app/tests/TopBar.test.js`

---

## 1. Acceptance Criteria Coverage Matrix

The existing test file contains **26 tests** organized into 6 `describe` blocks. The table below maps each acceptance criterion to the tests that cover it, along with a coverage assessment.

| AC | Criterion | Existing Tests | Coverage |
|----|-----------|---------------|----------|
| AC-1 | Trade Account row displays a combobox (Dropdown) instead of a static label | `Template rendering > renders Dropdown in tooltip when showProviderTooltip is true` | **Covered** — Verifies `.p-dropdown-mock` renders inside `.trade-account-dropdown-wrapper` when tooltip is visible |
| AC-2 | Combobox lists all active provider instances with `trade_account` in REST capabilities | `tradeAccountOptions computed > includes providers with trade_account capability`, `...excludes providers without trade_account capability`, `...returns empty array when providers are null` | **Covered** — Verifies filtering logic by capability, exclusion, and null handling |
| AC-3 | Each dropdown option shows provider display name and Paper/Live indicator | `tradeAccountOptions computed > builds options with correct shape (label, value, paper, displayName)` | **Covered** — Asserts `label: 'Alpaca (Paper)'` and `label: 'TastyTrade (Live)'` format |
| AC-4 | Selecting a different account triggers `PUT /api/providers/config` with only `trade_account` changed | `handleTradeAccountSwitch > calls updateProviderConfig with full config (only trade_account changed) and refreshBalance on success` | **Covered** — Asserts `sentConfig.trade_account === 'tastytrade_live'` while other keys remain unchanged |
| AC-5 | After successful switch, TopBar indicator updates to new account name and type | `Template rendering > renders trade account indicator with correct name and type` | **Partial** — Tests initial render, but does NOT test reactive update after a switch completes |
| AC-6 | After successful switch, Net Liq and Buying Power update to new account's values | `handleTradeAccountSwitch > calls ... refreshBalance on success` | **Partial** — Verifies `refreshBalance()` is called, but does NOT verify that `netLiquidation`/`buyingPower` reactively update from new balance data |
| AC-7 | Tooltip does not close while user interacts with combobox dropdown | `onTooltipAreaLeave > keeps tooltip open when dropdownOpen is true`, `...closes tooltip when dropdownOpen is false` | **Covered** — Tests the conditional close logic |
| AC-8 | Loading state shown during switch operation | `handleTradeAccountSwitch > sets switchingAccount during the operation` | **Covered** — Asserts `switchingAccount` is `true` during async op and `false` after |
| AC-9 | On failure, error displayed and selection reverts to previous account | `handleTradeAccountSwitch > reverts selectedTradeAccount and sets switchError on failure`, `...auto-clears switchError after 4 seconds` | **Covered** — Tests rollback, error message, and auto-clear |
| AC-10 | Market data service rows remain read-only labels | `Template rendering > renders market data service rows as static labels (not dropdowns)` | **Covered** — Asserts `.provider-name` spans exist and no `.trade-account-dropdown-wrapper` in market data category |
| AC-11 | Single trade-capable provider: combobox renders but indicates no alternatives | `tradeDropdownDisabled > is disabled when only 1 trade-capable provider exists`, `...is disabled when zero trade-capable providers exist` | **Covered** — Verifies `tradeDropdownDisabled === true` for 0 and 1 providers |

---

## 2. Existing Test Inventory (26 tests)

### Block 1: `tradeAccountOptions computed` (5 tests)
1. includes providers with trade_account capability
2. excludes providers without trade_account capability
3. returns empty array when providers are null
4. builds options with correct shape (label, value, paper, displayName)
5. uses instanceId as fallback when display_name is missing

### Block 2: `tradeDropdownDisabled computed` (4 tests)
6. is disabled when switchingAccount is true
7. is disabled when only 1 trade-capable provider exists
8. is disabled when zero trade-capable providers exist
9. is NOT disabled when multiple options and not switching

### Block 3: `handleTradeAccountSwitch` (6 tests)
10. calls updateProviderConfig with full config (only trade_account changed) and refreshBalance on success
11. is a no-op when same account is selected
12. is a no-op when event value is null
13. sets switchingAccount during the operation
14. reverts selectedTradeAccount and sets switchError on failure
15. auto-clears switchError after 4 seconds

### Block 4: `onTooltipAreaLeave` (2 tests)
16. keeps tooltip open when dropdownOpen is true
17. closes tooltip when dropdownOpen is false

### Block 5: `selectedTradeAccount sync watcher` (3 tests)
18. syncs selectedTradeAccount from provider config on mount
19. updates selectedTradeAccount when config changes externally
20. does NOT update selectedTradeAccount while switching is in progress

### Block 6: `Template rendering` (6 tests)
21. renders Dropdown in tooltip when showProviderTooltip is true
22. renders error message when switchError is set
23. does not render error message when switchError is null
24. renders market data service rows as static labels (not dropdowns)
25. closes tooltip after 300ms on success
26. renders trade account indicator with correct name and type

---

## 3. Coverage Gaps & Planned Additional Tests

### Gap 1: AC-5 — Reactive indicator update after switch (PARTIAL)

**Current state:** Test #26 verifies the initial render shows `Alpaca` and `Paper`. No test verifies that the indicator reactively updates after `handleTradeAccountSwitch` completes and the provider config changes.

**Planned test:**
```
handleTradeAccountSwitch > updates trade account indicator after successful switch
```
- Trigger `handleTradeAccountSwitch({ value: 'tastytrade_live' })`
- Update `mockProviderConfig.value` to reflect new trade account
- Assert `wrapper.vm.tradeAccountName` changes to `'TastyTrade'`
- Assert `wrapper.vm.tradeAccountType` changes to `'Live'`
- Assert `wrapper.vm.tradeAccountTypeClass` changes to `'type-live'`

### Gap 2: AC-6 — Balance display updates after switch (PARTIAL)

**Current state:** Test #10 verifies `refreshBalance()` is called. No test verifies the reactive data flow from balance update to `netLiquidation`/`buyingPower` display values.

**Planned test:**
```
handleTradeAccountSwitch > updates Net Liq and Buying Power after successful switch
```
- Trigger switch
- After `flushPromises()`, update `mockBalance.value` to `{ portfolio_value: 75000, buying_power: 40000 }`
- Assert `wrapper.vm.netLiquidation === 75000`
- Assert `wrapper.vm.buyingPower === 40000`

### Gap 3: Tooltip not shown during loading/error states

**Current state:** The template has `v-if="showProviderTooltip && !providersLoading && !providersError"` but no test verifies the loading/error gates.

**Planned tests:**
```
Template rendering > does not render tooltip when providersLoading is true
Template rendering > does not render tooltip when providersError is set
```

### Gap 4: Dropdown props verification

**Current state:** Tests verify the Dropdown renders, but don't verify that correct props are passed (options, disabled, loading, v-model value).

**Planned tests:**
```
Template rendering > passes correct props to Dropdown component
Template rendering > Dropdown receives loading=true during switch
Template rendering > Dropdown receives disabled=true when single provider
```

### Gap 5: Concurrent switch guard

**Current state:** No test verifies behavior if `handleTradeAccountSwitch` is called while another switch is already in progress (`switchingAccount === true` causes `tradeDropdownDisabled === true`, but the handler itself doesn't guard against re-entry).

**Planned test:**
```
handleTradeAccountSwitch > does not call updateProviderConfig if switchingAccount is already true
```
Note: The current implementation relies on the Dropdown being disabled, not a guard in the handler itself. This test documents the behavior and whether a direct call would cause issues.

### Gap 6: `handleTradeAccountSwitch` with stale/missing config

**Current state:** No test for the edge case where `reactiveProviderConfig.value` is `null` when switch is attempted.

**Planned test:**
```
handleTradeAccountSwitch > handles gracefully when provider config is null
```

### Gap 7: `tradeAccountOptions` reactivity when providers change

**Current state:** The computed is tested with initial data but not when `mockAvailableProviders` changes after mount.

**Planned test:**
```
tradeAccountOptions computed > updates reactively when providers change after mount
```

---

## 4. Test Execution Steps

### Step 1: Run existing TopBar tests (26 tests)

Verify all current tests pass before making any changes.

```sh
cd trade-app && npx vitest run tests/TopBar.test.js
```

**Expected:** 26 tests pass, 0 failures.

### Step 2: Run full frontend test suite (regression)

Verify no regressions across the entire frontend.

```sh
cd trade-app && npx vitest run
```

**Expected:** All tests pass. Note the total count for baseline (currently ~36 test files).

### Step 3: Add planned tests for coverage gaps

Add the tests identified in Section 3 to `trade-app/tests/TopBar.test.js`, following the existing patterns:

- Use the same `createWrapper()` helper
- Use `mockBalance`, `mockAvailableProviders`, `mockProviderConfig` refs for controlling data
- Use `vi.useFakeTimers()` / `vi.advanceTimersByTime()` for timer-dependent tests
- Use `await nextTick()` and `await flushPromises()` for reactive updates
- Organize new tests within the appropriate existing `describe` blocks

### Step 4: Run TopBar tests again after additions

```sh
cd trade-app && npx vitest run tests/TopBar.test.js
```

**Expected:** All tests pass (26 existing + new additions).

### Step 5: Run full frontend test suite again (regression)

```sh
cd trade-app && npx vitest run
```

**Expected:** No regressions. Same pass count as Step 2 plus new tests.

---

## 5. Test Conventions Reference

From `trade-app/tests/`:

- **Test runner:** Vitest + `@vue/test-utils` + happy-dom
- **File naming:** `*.test.js` (co-located in `tests/` directory, not alongside source)
- **QA tests:** `qa-` prefix for edge case / concurrency tests
- **Global setup:** `tests/setup.js` mocks `authService` and `global.fetch`
- **Mock pattern:** `vi.mock()` at module level for composables, services, child components
- **PrimeVue stubs:** Components like `Dropdown`, `InputText`, `Button`, `Menu` are stubbed in `global.stubs` with minimal templates
- **Reactive data control:** Tests create `ref()` values at module scope and mutate them to drive component behavior
- **Timer control:** `vi.useFakeTimers()` in `beforeEach`, `vi.useRealTimers()` in `afterEach`
- **Async patterns:** `await nextTick()` for Vue reactivity, `await flushPromises()` for Promise resolution

---

## 6. Summary

| Category | Count |
|----------|-------|
| Acceptance criteria fully covered | 8 of 11 (AC-1, AC-2, AC-3, AC-4, AC-7, AC-8, AC-9, AC-10, AC-11) |
| Acceptance criteria partially covered | 2 of 11 (AC-5, AC-6) |
| Existing tests | 26 |
| Planned additional tests | 8-10 |
| Coverage gaps identified | 7 |

The existing 26 tests provide strong coverage of the core switching logic, filtering, error handling, and tooltip interaction model. The primary gaps are in verifying the **end-to-end reactive data flow** after a switch (indicator name/type update, balance display update) and **edge case robustness** (null config, provider data changes after mount, tooltip visibility guards).
