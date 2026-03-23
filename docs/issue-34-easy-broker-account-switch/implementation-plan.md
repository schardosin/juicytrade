# Implementation Plan: Easy Broker Account Switch (Issue #34)

**Architecture:** [architecture.md](./architecture.md)
**Requirements:** [requirements.md](./requirements.md)

---

## File Modified

| File | Action |
|------|--------|
| `trade-app/src/components/TopBar.vue` | Modify (script, template, styles) |
| `trade-app/tests/TopBar.test.js` | Create (new test file) |

---

## Step 1: Script Changes

**Goal:** Add all new reactive state, computed properties, the config watcher, and methods to `TopBar.vue`'s `setup()` function. No template or style changes yet — the new code will be inert until the template is wired up in Step 2.

### 1a. Destructure additional methods from `useMarketData()`

The existing destructure (line 261-269) already pulls `getAvailableProviders`, `getProviderConfig`, `isLoading`, and `getError`. Add `updateProviderConfig`, `refreshBalance`, and `refreshProviderData` to the same call:

```js
const {
  lookupSymbols,
  getBalance,
  getAccountInfo,
  getAvailableProviders,
  getProviderConfig,
  updateProviderConfig,   // NEW
  refreshBalance,         // NEW
  isLoading,
  getError
} = useMarketData();
```

### 1b. Add new reactive refs

After the existing `showProviderTooltip` ref (line 291), add:

```js
// === Trade Account Switching State ===
const switchingAccount = ref(false);
const switchError = ref(null);
const dropdownOpen = ref(false);
const selectedTradeAccount = ref(null);
```

### 1c. Add new computed properties

After the existing `tradeAccountTypeClass` computed (line 479-492), add:

```js
const tradeAccountOptions = computed(() => {
  const providers = reactiveAvailableProviders.value;
  if (!providers) return [];

  const options = [];
  for (const [instanceId, providerData] of Object.entries(providers)) {
    const restCapabilities = providerData.capabilities?.rest || [];
    if (restCapabilities.includes('trade_account')) {
      options.push({
        label: `${providerData.display_name || instanceId} (${providerData.paper ? 'Paper' : 'Live'})`,
        value: instanceId,
        paper: providerData.paper,
        displayName: providerData.display_name || instanceId
      });
    }
  }
  return options;
});

const tradeDropdownDisabled = computed(() => {
  return switchingAccount.value || tradeAccountOptions.value.length <= 1;
});
```

### 1d. Add watcher to sync `selectedTradeAccount` with config

After the existing route watcher (line 966-975), add:

```js
watch(
  () => reactiveProviderConfig.value?.trade_account,
  (newVal) => {
    if (newVal && !switchingAccount.value) {
      selectedTradeAccount.value = newVal;
    }
  },
  { immediate: true }
);
```

### 1e. Add `handleTradeAccountSwitch` method

After the existing `formatProviderName` method (line 728-743), add:

```js
const handleTradeAccountSwitch = async (event) => {
  const newInstanceId = event.value;
  const currentConfig = reactiveProviderConfig.value;

  if (!newInstanceId || newInstanceId === currentConfig?.trade_account) {
    return;
  }

  const previousAccount = currentConfig?.trade_account;
  switchingAccount.value = true;
  switchError.value = null;

  try {
    const updatedConfig = { ...currentConfig, trade_account: newInstanceId };
    await updateProviderConfig(updatedConfig);
    await refreshBalance();

    setTimeout(() => {
      showProviderTooltip.value = false;
      dropdownOpen.value = false;
    }, 300);
  } catch (error) {
    console.error('Failed to switch trade account:', error);
    selectedTradeAccount.value = previousAccount;
    switchError.value = 'Failed to switch account. Please try again.';

    setTimeout(() => {
      switchError.value = null;
    }, 4000);
  } finally {
    switchingAccount.value = false;
  }
};
```

### 1f. Add `onTooltipAreaLeave` method

```js
const onTooltipAreaLeave = () => {
  if (dropdownOpen.value) {
    return;
  }
  showProviderTooltip.value = false;
};
```

### 1g. Update the `return` block

Add these to the return statement (line 992-1051):

```js
// Trade account switching
selectedTradeAccount,
tradeAccountOptions,
tradeDropdownDisabled,
switchingAccount,
switchError,
dropdownOpen,
handleTradeAccountSwitch,
onTooltipAreaLeave,
```

### Verification

```sh
cd trade-app && npx vitest run
```

All existing tests should pass since no template changes have been made. The new code is inert.

---

## Step 2: Template Changes

**Goal:** Wire up the new script code to the template — move hover events to the parent, replace the Trade Account static label with a PrimeVue Dropdown, add the error message div.

### 2a. Move hover events from `.trade-account-indicator` to `.trade-account-section`

**Current** (lines 146-152 of TopBar.vue):
```html
<div class="trade-account-section">
  <div
    class="trade-account-indicator"
    ...
    @mouseenter="showProviderTooltip = true"
    @mouseleave="showProviderTooltip = false"
  >
```

**Change to:**
```html
<div class="trade-account-section"
     @mouseenter="showProviderTooltip = true"
     @mouseleave="onTooltipAreaLeave">
  <div
    class="trade-account-indicator"
    ...
    <!-- REMOVE @mouseenter and @mouseleave from here -->
  >
```

### 2b. Replace Trade Account static label with Dropdown

**Current** (lines 173-179):
```html
<div class="provider-category">
  <h5>Trading Services</h5>
  <div class="provider-item">
    <span class="service-name">Trade Account</span>
    <span class="provider-name">{{ formatProviderName('trade_account') }}</span>
  </div>
</div>
```

**Replace with:**
```html
<div class="provider-category">
  <h5>Trading Services</h5>
  <div class="provider-item trade-account-row">
    <span class="service-name">Trade Account</span>
    <div class="trade-account-dropdown-wrapper">
      <Dropdown
        v-model="selectedTradeAccount"
        :options="tradeAccountOptions"
        option-label="label"
        option-value="value"
        :loading="switchingAccount"
        :disabled="tradeDropdownDisabled"
        @change="handleTradeAccountSwitch"
        @show="dropdownOpen = true"
        @hide="dropdownOpen = false"
        class="trade-account-dropdown"
        append-to="self"
        placeholder="Select account"
      />
      <div v-if="switchError" class="switch-error">
        <i class="pi pi-exclamation-triangle"></i>
        {{ switchError }}
      </div>
    </div>
  </div>
</div>
```

### Verification

```sh
cd trade-app && npx vitest run
```

Existing tests should still pass (TopBar is not yet unit-tested, so this is a regression check on other components). Manual verification in browser: hover over the trade account indicator, confirm the dropdown appears in the tooltip, confirm tooltip stays open when interacting with the dropdown.

---

## Step 3: Style Changes

**Goal:** Add CSS for the new dropdown, error message, and overflow handling. All styles go in the `<style scoped>` section of `TopBar.vue`.

### 3a. Dropdown wrapper and dropdown sizing

Add after the existing `.provider-name` styles (around line 1678):

```css
/* Trade Account Dropdown in Tooltip */
.trade-account-row {
  flex-wrap: wrap;
  gap: var(--spacing-xs);
}

.trade-account-row .service-name {
  flex-shrink: 0;
}

.trade-account-dropdown-wrapper {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  position: relative;
  overflow: visible;
}

.trade-account-dropdown {
  width: 180px;
  font-size: var(--font-size-base);
}
```

### 3b. PrimeVue `:deep()` overrides for compact dropdown

```css
:deep(.trade-account-dropdown .p-dropdown) {
  background: var(--bg-secondary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  min-height: unset;
  height: 28px;
}

:deep(.trade-account-dropdown .p-dropdown-label) {
  padding: 2px 8px;
  font-size: var(--font-size-base);
  color: var(--text-primary);
}

:deep(.trade-account-dropdown .p-dropdown-trigger) {
  width: 24px;
}

:deep(.trade-account-dropdown .p-dropdown-panel) {
  background: var(--bg-tertiary);
  border: 1px solid var(--border-secondary);
  border-radius: var(--radius-sm);
  box-shadow: var(--shadow-md);
}

:deep(.trade-account-dropdown .p-dropdown-item) {
  padding: var(--spacing-xs) var(--spacing-sm);
  font-size: var(--font-size-base);
  color: var(--text-primary);
}

:deep(.trade-account-dropdown .p-dropdown-item:hover) {
  background: var(--bg-quaternary);
}

:deep(.trade-account-dropdown .p-dropdown-item.p-highlight) {
  background: var(--color-info);
  color: white;
}
```

### 3c. Error message styling

```css
.switch-error {
  width: 100%;
  display: flex;
  align-items: center;
  gap: var(--spacing-xs);
  color: var(--color-danger);
  font-size: var(--font-size-xs);
  padding-top: var(--spacing-xs);
}

.switch-error i {
  font-size: var(--font-size-xs);
}
```

### 3d. Overflow fixes for `append-to="self"`

Modify existing `.provider-tooltip` and `.tooltip-content` rules to prevent the dropdown overlay from being clipped:

```css
/* Update existing .provider-tooltip rule */
.provider-tooltip {
  /* ...existing properties... */
  overflow: visible;   /* ADD: prevent clipping dropdown overlay */
}

/* Update existing .tooltip-content rule */
.tooltip-content {
  /* ...existing properties... */
  overflow: visible;   /* CHANGE from overflow-y: auto */
}
```

### Verification

```sh
cd trade-app && npx vitest run
```

Manual visual check: confirm dropdown fits within the tooltip, no clipping, error message appears correctly when simulating a failure.

---

## Step 4: Unit Tests

**Goal:** Create `trade-app/tests/TopBar.test.js` following the project's established test patterns (see `RightPanel.test.js`, `setup.js`).

### File: `trade-app/tests/TopBar.test.js`

The test file should follow the project conventions:
- Import `{ describe, it, expect, beforeEach, vi }` from `vitest`
- Import `{ mount }` from `@vue/test-utils`
- Import `{ ref, nextTick }` from `vue`
- Mock all dependencies with `vi.mock()` at the top level
- Use controllable `ref()` values for mock data so tests can manipulate state
- Mock child components (`SettingsDialog`, `MobileSearchOverlay`) as stubs
- Mock `vue-router` (`useRouter`, `useRoute`)
- Mock `webSocketClient`

### Test Cases

The test file should cover these specific scenarios:

**1. `tradeAccountOptions` computed property**
- Given `reactiveAvailableProviders` contains 2 providers, both with `trade_account` in `capabilities.rest` — expect 2 options
- Given one provider lacks `trade_account` capability — expect only the capable one in options
- Given `reactiveAvailableProviders` is `null` — expect empty array
- Verify each option has `label`, `value`, `paper`, and `displayName` fields

**2. `tradeDropdownDisabled` computed property**
- When `switchingAccount` is `true` — expect disabled
- When `tradeAccountOptions` has `<= 1` item — expect disabled
- When `switchingAccount` is `false` and `>1` options — expect not disabled

**3. `handleTradeAccountSwitch` method**
- Call with a new instance ID — verify `updateProviderConfig` called with full config where only `trade_account` changed
- Call with the same instance ID as current — verify `updateProviderConfig` NOT called (no-op guard)
- On success — verify `refreshBalance` called, `switchingAccount` becomes `false`
- On failure — verify `selectedTradeAccount` reverts to previous, `switchError` is set, `switchingAccount` becomes `false`

**4. `onTooltipAreaLeave` method**
- When `dropdownOpen` is `true` — verify `showProviderTooltip` stays `true`
- When `dropdownOpen` is `false` — verify `showProviderTooltip` becomes `false`

**5. Watcher: `selectedTradeAccount` sync**
- When `reactiveProviderConfig.trade_account` changes externally — verify `selectedTradeAccount` updates
- When `switchingAccount` is `true` — verify `selectedTradeAccount` does NOT update (prevents overwrite during switch)

**6. Template rendering**
- Verify the Dropdown component renders inside the tooltip when `showProviderTooltip` is `true`
- Verify the error message div renders when `switchError` is non-null
- Verify market data service rows remain as static labels (not dropdowns)

### Verification

```sh
cd trade-app && npx vitest run tests/TopBar.test.js
```

All new tests should pass.

---

## Step 5: Final Verification

**Goal:** Run the full test suite and confirm no regressions across the entire frontend.

### 5a. Run all frontend tests

```sh
cd trade-app && npx vitest run
```

All tests must pass, including the new `TopBar.test.js` and all existing tests.

### 5b. Regression checklist (manual)

| # | Check | Expected |
|---|-------|----------|
| 1 | Hover over trade account indicator | Tooltip opens with Dropdown in Trade Account row |
| 2 | Other service rows (Stock Quotes, etc.) | Remain read-only labels, unchanged |
| 3 | Click Dropdown, hover over options | Tooltip stays open, no flicker |
| 4 | Select a different account | Loading state shown, then TopBar updates (name, badge, Net Liq, BP) |
| 5 | Simulate API failure | Error shown inline, selection reverts |
| 6 | Single trade-capable provider | Dropdown renders but is disabled |
| 7 | Mouse leave with dropdown closed | Tooltip closes normally |
| 8 | Search bar functionality | Unaffected — search, results, keyboard nav all work |
| 9 | Connection status indicator | Unaffected — shows correct state |
| 10 | Settings dialog | Can still open, changes reflected in TopBar tooltip |

### 5c. Build check

```sh
cd trade-app && npm run build
```

Production build must succeed with no errors.
