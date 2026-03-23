# Requirements: Easy Broker Account Switch

**Issue:** [#34 - Easy Broker Account Switch](https://github.com/schardosin/juicytrade/issues/34)
**Status:** Draft — Awaiting Customer Approval

---

## 1. Overview

Currently, the TopBar displays the active trade account (provider name + Paper/Live badge) inside a static indicator element. Hovering over it reveals a read-only tooltip showing the full provider configuration (Trade Account, Stock Quotes, Options Chain, etc.).

The customer wants to convert the **Trade Account** row inside this tooltip into an interactive combobox/dropdown, allowing a quick switch between configured broker accounts without navigating to the Settings dialog.

## 2. Motivation & Business Value

- **Speed:** Traders who manage multiple broker accounts (e.g., a paper account for testing and a live account for real trades, or accounts across different brokers) need to switch quickly without opening settings.
- **Reduced friction:** The current flow requires opening Settings → Providers tab → changing the trade_account routing → saving. This feature reduces that to a single click from the TopBar.
- **Visibility:** The TopBar already surfaces the active account prominently — making it interactive is a natural UX extension.

## 3. Functional Requirements

### FR-1: Convert Trade Account Display to Combobox

**Current behavior:** In the provider configuration tooltip (shown on hover over the trade account indicator in the TopBar), the "Trade Account" row displays a static label showing the provider name and account type (e.g., "Alpaca (Paper)").

**New behavior:** The "Trade Account" row in the tooltip shall display a combobox/dropdown instead of a static label. The combobox shall:

- Show the currently selected trade account provider as its default value.
- When clicked/opened, display a list of all available provider instances that support the `trade_account` capability (filtered from the `providers.available` data — only providers whose `capabilities.rest` array includes `"trade_account"`).
- Each option in the dropdown shall display:
  - The provider display name (e.g., "Alpaca", "TastyTrade", "Schwab").
  - The account type badge — "Paper" or "Live" — to clearly distinguish account types.
- The currently active trade account shall be visually indicated (e.g., highlighted, checkmark, or selected state).

### FR-2: Switching the Trade Account

When the user selects a different account from the combobox:

1. The system shall call the existing `PUT /api/providers/config` endpoint with an updated configuration where `trade_account` is set to the newly selected provider instance ID.
2. All other provider routing keys (stock_quotes, options_chain, historical_data, etc.) shall remain unchanged — only `trade_account` is updated.
3. The backend will handle the full config update flow (stop old account stream, update config, restart streaming, restart account stream with new provider) — this already exists.
4. After successful update, the frontend shall:
   - Refresh the provider config data (`providers.config`) to reflect the new state.
   - Refresh balance/account data so the TopBar displays the new account's Net Liq and Buying Power.
   - The trade account indicator in the TopBar (outside the tooltip) shall update to show the new account name and Paper/Live badge.

### FR-3: Tooltip Behavior Adjustments

- The tooltip currently appears on `mouseenter` and disappears on `mouseleave`. Since the tooltip will now contain an interactive combobox, the tooltip must remain open while the user is interacting with the dropdown (i.e., it must not close when the mouse moves to the dropdown options).
- The tooltip shall close when:
  - The user moves the mouse away from both the indicator and the tooltip.
  - The user selects an account (after the switch completes or starts).
  - The user clicks outside the tooltip area.

### FR-4: Loading & Feedback States

- While the account switch is in progress (API call + backend restart), show a brief loading indicator on the combobox or the trade account indicator to signal the switch is happening.
- If the switch fails (API error, network issue), display an inline error message or toast notification so the user knows the switch did not succeed. The combobox shall revert to the previous selection.
- If there is only one provider instance that supports `trade_account`, the combobox shall still render but be visually indicated as having no alternatives (e.g., a single item, or disabled state). The user should understand there are no other accounts to switch to.

### FR-5: Market Data Services Remain Read-Only

- Only the "Trade Account" row in the tooltip shall become a combobox. All other rows (Stock Quotes, Options Chain, Historical Data, Symbol Lookup, Market Calendar, Streaming Quotes, Greeks, Streaming Greeks) shall remain as read-only labels, unchanged from current behavior.

## 4. Acceptance Criteria

| # | Criterion | Verification |
|---|-----------|-------------|
| AC-1 | The Trade Account row in the provider tooltip displays a combobox instead of a static label. | Visual inspection |
| AC-2 | The combobox lists all active provider instances that have `trade_account` in their REST capabilities. | Inspect dropdown options against `/api/providers/available` response |
| AC-3 | Each dropdown option shows the provider display name and a Paper/Live indicator. | Visual inspection |
| AC-4 | Selecting a different account triggers a `PUT /api/providers/config` with only `trade_account` changed. | Network inspection / backend logs |
| AC-5 | After successful switch, the TopBar indicator updates to reflect the new account name and type. | Visual inspection |
| AC-6 | After successful switch, Net Liq and Buying Power update to the new account's values. | Visual inspection |
| AC-7 | The tooltip does not close while the user is interacting with the combobox dropdown. | Manual interaction test |
| AC-8 | A loading state is shown during the switch operation. | Visual inspection |
| AC-9 | If the switch fails, an error is displayed and the selection reverts to the previous account. | Simulate API failure |
| AC-10 | Market data service rows in the tooltip remain read-only labels. | Visual inspection |
| AC-11 | When only one trade-capable provider exists, the combobox renders but indicates no alternatives. | Configure single provider and inspect |

## 5. Scope Boundaries

### In Scope
- Converting the Trade Account label to a combobox in the existing provider tooltip.
- Calling the existing backend `PUT /api/providers/config` endpoint to update the trade account.
- Frontend state refresh after switch (provider config, balance, account info).
- Loading and error states for the switch operation.

### Out of Scope
- **No backend API changes.** The existing `PUT /api/providers/config` and `GET /api/providers/available` endpoints provide everything needed.
- **No changes to other service routing.** This feature only switches the `trade_account` routing. Changing other services (stock_quotes, streaming, etc.) remains in the Settings dialog.
- **No changes to the Settings dialog or Setup Wizard.** Those flows remain as-is.
- **No mobile-specific implementation.** The trade account indicator and tooltip are desktop-only (hidden on mobile per existing TopBar behavior). Mobile support is not required for this feature.

## 6. Technical Notes

- **Data source for dropdown options:** The `providers.available` reactive data (from `SmartMarketDataStore`) already contains all active provider instances with their capabilities. Filter for instances where `capabilities.rest` includes `"trade_account"`.
- **Existing update flow:** `SmartMarketDataStore.updateProviderConfig(newConfig)` already handles: API call → update local state → clear symbol data cache → refresh available providers. This should be reused.
- **PrimeVue Dropdown:** The project uses PrimeVue 3.46 globally. A `Dropdown` (or `Select`) component should be used for the combobox to match the existing design system.
- **Config merge behavior:** When calling `PUT /api/providers/config`, the backend merges the sent keys with existing config for keys not provided. So sending just `{ trade_account: "new_instance_id" }` will preserve all other routing. However, the frontend's `updateProviderConfig` sends the full config object — the implementation should follow the same pattern to be safe.

## 7. Non-Functional Requirements

- **Performance:** The account switch should feel responsive. The HTTP response is sent before account stream restart (which happens asynchronously in the backend), so the UI should update quickly.
- **Consistency:** The dropdown styling must match the existing PrimeVue dark theme (`aura-dark-noir`) and the TopBar's visual language.
- **No regressions:** The hover tooltip for read-only services must continue working exactly as before. The TopBar's account balance display, connection status, and search functionality must not be affected.
