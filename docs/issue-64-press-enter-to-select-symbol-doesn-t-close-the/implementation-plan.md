# Issue 64 Implementation Plan

## Context

- Branch: `fleet/issue-64-press-enter-to-select-symbol-doesn-t-close-the`
- Requested requirements file: `docs/issue-64-press-enter-to-select-symbol-doesn-t-close-the/requirements.md`
- Status: not present in the workspace at planning time
- Primary issue context available from code inspection:
  - `trade-app/src/components/TopBar.vue`
  - `trade-app/tests/TopBar.test.js`
  - `trade-app/src/components/MobileSearchOverlay.vue`
  - `trade-app/src/composables/useGlobalSymbol.js`

## Relevant Structure

- `trade-app/src/components/TopBar.vue`
  - Owns the desktop symbol search input, dropdown visibility, keyboard navigation, and symbol selection dispatch.
  - Currently binds both `@keydown="handleKeydown"` and `@keyup.enter="performSearch"` on the same input.
  - `handleKeydown()` calls `selectSymbol()` for highlighted results, but `performSearch()` manually dispatches `symbol-selected` and does not clear dropdown/search state the same way.
- `trade-app/tests/TopBar.test.js`
  - Already exists and uses direct `wrapper.vm` assertions plus mocked `lookupSymbols()`.
  - Is the right place for focused regression coverage for desktop search behavior.
- `trade-app/src/components/MobileSearchOverlay.vue`
  - Already uses `performSearch()` -> `selectSymbol()` to centralize selection cleanup.
  - Provides a useful reference for the desktop fix.
- `trade-app/src/composables/useGlobalSymbol.js`
  - Consumes the global `symbol-selected` event, so duplicate or inconsistent dispatches in `TopBar.vue` can cause downstream UI inconsistencies.

## Incremental Plan

1. Add failing regression tests for the desktop Enter-key search flow in `trade-app/tests/TopBar.test.js`.
   - Cover pressing Enter when a search result is highlighted.
   - Cover pressing Enter with no highlighted result but with search results available.
   - Assert that the symbol is selected, the dropdown closes, `highlightedIndex` resets, and the selection event is dispatched once.
   - Assert that pending search UI state is cleaned up the same way as mouse selection.
   - Tests:
     - `cd trade-app && npx vitest run tests/TopBar.test.js`

2. Unify desktop selection cleanup in `trade-app/src/components/TopBar.vue` so Enter-based selection follows the same path as clicking a result.
   - Refactor `performSearch()` to delegate to `selectSymbol()` when it resolves a symbol instead of manually dispatching the event and only updating `searchQuery`.
   - Keep the change minimal and local to `TopBar.vue`; do not introduce a new shared utility unless the refactor requires it.
   - Ensure the shared selection path clears `searchResults`, stops loading state, resets `highlightedIndex`, cancels any pending debounce timer, and hides the dropdown.
   - Tests:
     - Re-run `cd trade-app && npx vitest run tests/TopBar.test.js`
     - Confirm the new Enter-key regression tests pass.

3. Remove or tighten the duplicate Enter event path on the desktop input so selection happens once per keypress.
   - Evaluate the existing overlap between `@keydown="handleKeydown"` and `@keyup.enter="performSearch"`.
   - Prefer the smallest fix: either rely on `handleKeydown()` for Enter handling or add an explicit guard so `performSearch()` does not re-run immediately after `handleKeydown()` already selected a symbol.
   - Preserve current ArrowUp, ArrowDown, and Escape behavior.
   - Tests:
     - Extend `trade-app/tests/TopBar.test.js` if needed to prove Enter does not trigger duplicate lookup/dispatch behavior.
     - Run `cd trade-app && npx vitest run tests/TopBar.test.js`

4. Add focused regression coverage for adjacent desktop search interactions that could be affected by the fix.
   - Verify Escape still closes the dropdown without dispatching a symbol selection.
   - Verify mouse selection still closes the dropdown and dispatches the expected event.
   - Verify a no-results Enter path does not leave the dropdown stuck open in an inconsistent state.
   - Keep these tests in `trade-app/tests/TopBar.test.js` to stay aligned with the existing component-level test strategy.
   - Tests:
     - `cd trade-app && npx vitest run tests/TopBar.test.js`

5. Run a broader frontend regression pass for confidence that the TopBar search change does not break unrelated behavior already covered in the existing suite.
   - Run the focused TopBar file first, then the full frontend Vitest suite if the focused file passes.
   - If the full suite is too slow or flaky in the current environment, at minimum record the focused TopBar run result and any blocked broader verification.
   - Tests:
     - `cd trade-app && npx vitest run tests/TopBar.test.js`
     - `cd trade-app && npx vitest run`

## Expected Files To Change During Implementation

- `trade-app/src/components/TopBar.vue`
- `trade-app/tests/TopBar.test.js`

## Notes

- The mobile flow in `MobileSearchOverlay.vue` already centralizes selection through `selectSymbol()` and should remain unchanged unless testing exposes shared behavioral drift.
- The likely root cause is the mismatch between the desktop Enter path and the click-selection path, combined with overlapping key handlers on the same input.
