# Test Results: Easy Broker Account Switch (Issue #34)

**Test Plan:** [test-plan.md](./test-plan.md)
**Requirements:** [requirements.md](./requirements.md)
**Architecture:** [architecture.md](./architecture.md)
**Test File:** `trade-app/tests/TopBar.test.js`
**Date:** 2026-03-23
**QA Verdict:** ✅ **APPROVED**

---

## 1. Summary

| Metric | Value |
|--------|-------|
| Total TopBar tests | **35** (26 existing + 9 QA additions) |
| TopBar tests passed | **35** |
| TopBar tests failed | **0** |
| Full suite total | **1147** |
| Full suite passed | **1145** |
| Full suite failed | **2** (pre-existing, unrelated) |
| Regressions introduced | **0** |
| Acceptance criteria covered | **11 / 11** |

---

## 2. Acceptance Criteria Verification

| AC | Criterion | Verdict | Tests Covering |
|----|-----------|---------|----------------|
| AC-1 | Trade Account row displays a PrimeVue Dropdown instead of a static label | ✅ Pass | `renders Dropdown in tooltip when showProviderTooltip is true` |
| AC-2 | Dropdown lists only providers with `trade_account` in `capabilities.rest` | ✅ Pass | `includes providers with trade_account capability`, `excludes providers without trade_account capability`, `returns empty array when providers are null`, `updates reactively when providers change after mount` |
| AC-3 | Each dropdown option shows display name + Paper/Live badge | ✅ Pass | `builds options with correct shape (label, value, paper, displayName)`, `uses instanceId as fallback when display_name is missing` |
| AC-4 | Switching calls `updateProviderConfig` with full config, only `trade_account` changed | ✅ Pass | `calls updateProviderConfig with full config (only trade_account changed) and refreshBalance on success`, `is a no-op when same account is selected`, `is a no-op when event value is null` |
| AC-5 | TopBar indicator updates after successful switch | ✅ Pass | `renders trade account indicator with correct name and type`, **`updates trade account indicator after successful switch`** (QA) |
| AC-6 | `refreshBalance()` called after successful switch; Net Liq/BP update | ✅ Pass | `calls ... refreshBalance on success`, **`updates Net Liq and Buying Power after successful switch`** (QA) |
| AC-7 | Tooltip stays open during dropdown interaction (`dropdownOpen` guard) | ✅ Pass | `keeps tooltip open when dropdownOpen is true`, `closes tooltip when dropdownOpen is false` |
| AC-8 | Loading state shown during switch (`switchingAccount` flag) | ✅ Pass | `sets switchingAccount during the operation`, **`passes correct props to Dropdown component`** (QA — verifies loading prop) |
| AC-9 | On failure: selection reverts, error shown, auto-clears after 4s | ✅ Pass | `reverts selectedTradeAccount and sets switchError on failure`, `auto-clears switchError after 4 seconds`, `renders error message when switchError is set`, `does not render error message when switchError is null` |
| AC-10 | Other service rows remain read-only labels | ✅ Pass | `renders market data service rows as static labels (not dropdowns)` |
| AC-11 | Dropdown disabled when only one trade-capable provider exists | ✅ Pass | `is disabled when only 1 trade-capable provider exists`, `is disabled when zero trade-capable providers exist`, `is NOT disabled when multiple options and not switching`, **`Dropdown shows disabled state when single trade-capable provider`** (QA) |

Tests marked with **(QA)** were added during this QA pass.

---

## 3. QA-Added Tests (9 tests)

### Gap 1 — AC-5: Reactive indicator update after switch
- **`updates trade account indicator after successful switch`** — Triggers switch, simulates config update, asserts `tradeAccountName`, `tradeAccountType`, and `tradeAccountTypeClass` reactively update to reflect the new provider.

### Gap 2 — AC-6: Balance display values update after switch
- **`updates Net Liq and Buying Power after successful switch`** — Triggers switch, simulates new balance data, asserts `netLiquidation` and `buyingPower` reactively update via the watcher.

### Gap 3 — Tooltip visibility guards
- **`does not render tooltip when providersLoading is true`** — Verifies the `v-if` guard hides the tooltip during loading state.
- **`does not render tooltip when providersError is set`** — Verifies the `v-if` guard hides the tooltip when an error exists.

### Gap 4 — Dropdown prop verification
- **`passes correct props to Dropdown component`** — Verifies the Dropdown stub receives correct `options`, `disabled`, `loading`, and `modelValue` props.
- **`Dropdown shows disabled state when single trade-capable provider`** — Verifies the Dropdown's `disabled` prop is `true` when only one trade-capable provider exists.

### Gap 5 — Concurrent switch behavior (documented)
- **`allows concurrent calls since no re-entry guard exists (documents behavior)`** — Documents that the handler lacks a `switchingAccount`-based re-entry guard. The Dropdown's `disabled` prop prevents this in practice, but direct programmatic calls would result in duplicate API calls. This is acceptable given the UI guard, but noted for awareness.

### Gap 6 — Null config edge case
- **`handles gracefully when provider config is null`** — Verifies the handler works correctly when `reactiveProviderConfig.value` is `null`, producing `{ trade_account: 'tastytrade_live' }` via the spread operator.

### Gap 7 — Reactive options update
- **`updates reactively when providers change after mount`** — Verifies `tradeAccountOptions` computed property reacts to changes in `mockAvailableProviders` after initial mount (e.g., a new provider being added).

---

## 4. Regression Check

### Full Frontend Test Suite

```
Test Files  1 failed | 34 passed (35)
     Tests  2 failed | 1145 passed (1147)
  Duration  37.88s
```

| Metric | Baseline (before QA) | After QA | Delta |
|--------|---------------------|----------|-------|
| Total tests | 1138 | 1147 | **+9** |
| Passed | 1136 | 1145 | **+9** |
| Failed | 2 | 2 | **0** |

### Pre-existing Failures (not related to Issue #34)

Both failures are in `tests/CollapsibleOptionsChain.test.js`:
1. `displays strike count filter with correct options` — Expected 6 `<option>` elements, found 10.
2. `provides all expected strike count options` — Expected `['50','60','70','80','90','100']`, got `['50','60','70','80','90','100','150','200','250','300']`.

These reflect a prior component update that added strike count options without updating the tests. They are **not regressions** from this feature.

---

## 5. Code Quality Observations

### Implementation Review
The implementation in `TopBar.vue` is clean, well-structured, and faithful to the architecture document:
- ✅ Correct use of PrimeVue Dropdown with `append-to="self"` for tooltip interaction
- ✅ Proper `mouseenter`/`mouseleave` handling on `.trade-account-section` with `dropdownOpen` guard
- ✅ Full config spread pattern (`{ ...currentConfig, trade_account: newInstanceId }`) matches existing conventions
- ✅ Error rollback with auto-clear timer
- ✅ Watcher sync with `switchingAccount` guard to prevent overwrite during switch
- ✅ All new state returned from `setup()` for template access

### Minor Observation (non-blocking)
- **No re-entry guard in handler:** The `handleTradeAccountSwitch` method doesn't check `switchingAccount.value` before proceeding. In practice, the Dropdown is disabled during switching (via `tradeDropdownDisabled` computed), so the UI prevents concurrent calls. However, a direct programmatic call could trigger duplicate API requests. This is a minor robustness improvement that could be added in a future iteration but is **not a blocker** for this feature.

---

## 6. Conclusion

All 11 acceptance criteria are verified and covered by automated tests. The implementation is correct, handles edge cases appropriately, and introduces no regressions. **QA approved.**
