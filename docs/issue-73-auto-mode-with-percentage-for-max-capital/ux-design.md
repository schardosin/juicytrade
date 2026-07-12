# UX Design — Percentage-based Max Capital (Issue #73)

**Issue:** [#73 — Auto Mode with percentage for Max Capital](https://github.com/schardosin/juicytrade/issues/73)
**Requirements:** [requirements.md](./requirements.md) · **Architecture:** [architecture.md](./architecture.md)
**Author:** @ux
**Status:** Draft — awaiting PO/customer review
**Scope:** `trade-app` (Vue 3). Frontend only. Consumes the backend contract in architecture §4 and §10.

---

## 0. Summary of decisions

1. **Config form** — Add a **Fixed `$` / Percent `%` segmented toggle** directly inside the existing
   *Max Capital* `.form-field` in the **Trade Configuration** section of `AutomationConfigForm.vue`.
   The single `InputNumber` swaps its formatting (currency vs. percentage) based on the active mode.
   The user's fixed dollar value is **never cleared** when toggling (FR-2).
2. **Dashboard** — Do **both** surfaces (per architect's recommendation):
   - Keep the existing status **Message** line (backend already embeds the resolved dollars).
   - Add a dedicated **"Capital Used"** `.status-row` in the `status-details` block of
     `AutomationDashboard.vue`, rendered from the structured `effective_capital*` fields.
   - Also make the static **"Max Capital"** summary line mode-aware (show `60%` vs `$5,000`).
3. **No layout redesign.** New elements reuse existing `.form-field`, `.field-hint`, `.status-row`,
   and `.summary-item` patterns. No new sections, no restructuring.

All field names match the architecture contract exactly:
`max_capital`, `max_capital_mode` (`"fixed"` | `"percent"`), `max_capital_percent` (1–100),
and status fields `effective_capital`, `effective_capital_net_liq`, `effective_capital_percent`,
`effective_capital_at`.

---

## 1. Config form — Max Capital mode toggle

**File:** `trade-app/src/components/automation/AutomationConfigForm.vue`
**Location:** the existing *Max Capital* `.form-field` inside the **Trade Configuration** `.form-grid`
(currently a single `InputNumber` with `mode="currency"` and `:min="100"`).

### 1.1 Current state (do NOT change surrounding fields)

```html
<div class="form-field">
  <label for="maxCapital">Max Capital</label>
  <InputNumber id="maxCapital" v-model="config.trade_config.max_capital"
    mode="currency" currency="USD" :min="100" placeholder="e.g., 5000" />
  <small class="field-hint">Maximum capital to risk on this trade</small>
</div>
```

### 1.2 Target layout

Add the toggle **above** the input, inside the same `.form-field`, then conditionally render the
currency input (fixed) or percentage input (percent). Structure:

```
┌─ .form-field (Max Capital) ────────────────────────────────┐
│  label: "Max Capital"                                       │
│  ┌ .capital-mode-toggle (pill) ─────────┐                   │
│  │ [ $ Fixed ]  [ % of Net Liq ]        │  ← segmented btns │
│  └──────────────────────────────────────┘                  │
│                                                             │
│  ── Fixed mode ──►  InputNumber (currency USD, min 100)     │
│  ── Percent mode ►  InputNumber (suffix "%", min 1 max 100) │
│                                                             │
│  small.field-hint:                                          │
│    fixed  → "Maximum capital to risk on this trade"         │
│    percent→ "% of account Net Liq. ≈ $6,000 of $10,000"     │
│             (resolved hint shown only when Net Liq known)   │
└─────────────────────────────────────────────────────────────┘
```

### 1.3 Markup (developer reference — adapt to project style)

```html
<div class="form-field">
  <label for="maxCapital">Max Capital</label>

  <!-- Mode toggle: matches DataImportDialog.vue "selection-mode-toggle" pattern -->
  <div class="capital-mode-toggle">
    <button type="button" class="mode-btn"
            :class="{ active: config.trade_config.max_capital_mode !== 'percent' }"
            @click="setCapitalMode('fixed')">$ Fixed</button>
    <button type="button" class="mode-btn"
            :class="{ active: config.trade_config.max_capital_mode === 'percent' }"
            @click="setCapitalMode('percent')">% of Net Liq.</button>
  </div>

  <!-- Fixed mode: existing currency input, unchanged behavior -->
  <InputNumber v-if="config.trade_config.max_capital_mode !== 'percent'"
    id="maxCapital" v-model="config.trade_config.max_capital"
    mode="currency" currency="USD" :min="100" placeholder="e.g., 5000" />

  <!-- Percent mode: percentage input 1..100 with % suffix -->
  <InputNumber v-else
    id="maxCapitalPercent" v-model="config.trade_config.max_capital_percent"
    suffix="%" :min="1" :max="100" :maxFractionDigits="0" placeholder="e.g., 60" />

  <small class="field-hint">{{ maxCapitalHint }}</small>
</div>
```

### 1.4 Behavior & script guidance

- **Default mode:** `fixed`. Treat missing/empty `max_capital_mode` as `fixed` (backward compat, FR-9).
  Add `max_capital_mode: 'fixed'` and `max_capital_percent: 60` to the `config` ref default in
  `setup()` (alongside `max_capital: 5000`) so new configs are unambiguous and the percent input has a
  sensible starting value.
- **Preserve values across toggles (FR-2):** `setCapitalMode(mode)` ONLY sets
  `config.trade_config.max_capital_mode = mode`. It must **NOT** touch `max_capital` or
  `max_capital_percent`. This guarantees the user's last fixed dollar value survives a round-trip
  Fixed → Percent → Fixed, and vice-versa.
- **Resolved-dollars hint (FR-2, FR-5 preview intent):** In percent mode, if the account Net Liq is
  known client-side, show the resolved figure in the hint:
  - Compute a `netLiq` value from the account (see §1.5). Then:
    ```js
    const maxCapitalHint = computed(() => {
      if (config.value.trade_config.max_capital_mode !== 'percent')
        return 'Maximum capital to risk on this trade'
      const pct = config.value.trade_config.max_capital_percent || 0
      if (netLiq.value > 0) {
        const resolved = Math.round(netLiq.value * pct / 100)   // mirror backend math.Round
        return `${pct}% of account Net Liq. ≈ $${resolved.toLocaleString()} of $${netLiq.value.toLocaleString()}`
      }
      return `${pct}% of account Net Liquidating Value`          // graceful fallback, no dollars
    })
    ```
  - This is a **client-side convenience preview only**. It is NOT authoritative — the backend
    resolves the real figure at monitoring-start and trade-time. Never block save on it.

### 1.5 Net Liq source for the preview hint (frontend)

- Preferred: if an account endpoint/store is already available in the app (look for an existing
  account/portfolio API client — e.g. `api.getAccount()` or a portfolio store), fetch it once on
  mount and read the portfolio/net-liq value (matches backend `Account.portfolio_value`, arch §5.3).
- If **no** account value is readily available client-side, **omit the dollar estimate** and show the
  plain fallback hint (`"60% of account Net Liquidating Value"`). Do **not** add new backend
  endpoints for this — a missing estimate is acceptable; the confirmed figure appears on the dashboard
  once the automation starts (FR-5). @dev: confirm with @po via me if unsure whether an account value
  is cheaply available.

### 1.6 Validation (client-side)

Current `validateConfig()` only checks name/symbol/entry_time. Extend it minimally:

- **Percent mode:** require `1 <= max_capital_percent <= 100`. On failure set
  `errors.max_capital_percent` and render `<small class="p-error">` under the input
  (same pattern as `errors.name`). The `InputNumber` `:min/:max` already constrain typing, so this is
  a safety net for empty/out-of-range values.
- **Fixed mode:** existing `:min="100"` is sufficient; no new required-field error unless the field is
  empty.
- Backend also validates (arch §10.1) and returns 400 — surface that message via the existing
  save-error handling.

### 1.7 Persistence

No plumbing changes needed. `saveConfig()` already spreads `config.value` into the payload, so
`max_capital_mode` and `max_capital_percent` on `trade_config` are sent automatically to
`createAutomationConfig` / `updateAutomationConfig`.

---

## 2. Dashboard — surfacing effective capital

**File:** `trade-app/src/components/automation/AutomationDashboard.vue`

Status objects reach the card via `getAutomationStatus(config.id)`, which merges:
- REST poll (`loadStatuses`) — spreads `item.details` (so `effective_capital*` land here), and
- WebSocket (`handleAutomationUpdate`) — spreads `message.data`.
So the new `effective_capital*` fields (arch §4.2) are available with **no store changes**.

### 2.1 Dedicated "Capital Used" line (primary UX)

Add a new `.status-row` in the existing `status-details` block, placed **directly after the
"State:" row and before the "Message:" row**, so the concrete figure sits near the top of the live
status. Only render when the backend has captured a value (`effective_capital` present).

```html
<!-- inside <div class="status-details"> , after the State row -->
<div v-if="getAutomationStatus(config.id)?.effective_capital" class="status-row">
  <span class="status-label">Capital Used:</span>
  <span class="status-value">{{ formatCapitalUsed(getAutomationStatus(config.id)) }}</span>
</div>
```

Helper (add to `setup()` return, reuse existing `formatNumber`):

```js
const formatCapitalUsed = (status) => {
  const eff = status?.effective_capital
  if (eff == null) return 'N/A'
  const pct = status?.effective_capital_percent
  const netLiq = status?.effective_capital_net_liq
  // percent mode: backend populates pct + net_liq (>0)
  if (pct > 0 && netLiq > 0) {
    return `$${formatNumber(Math.round(eff))} (${pct}% of $${formatNumber(Math.round(netLiq))})`
  }
  // fixed mode: just the dollar figure
  return `$${formatNumber(Math.round(eff))}`
}
```

Rendered examples:
- Percent: **`Capital Used: $6,000 (60% of $10,000)`**
- Fixed:   **`Capital Used: $5,000`**

### 2.2 Keep the status Message line (audit surface)

Leave the existing `Message:` `.status-row` exactly as-is. The backend already embeds the resolved
dollars in the message string (arch §6.4, e.g. *"Trading — capital $6,000 (60% of $10,000)"*), which
provides the phase-aware (monitoring-start vs trade-time) audit narrative alongside the structured
"Capital Used" line. No change required.

### 2.3 Make the static "Max Capital" summary mode-aware

In the `trade-summary` block there is a static config readout:

```html
<div class="summary-item">
  <span class="summary-label">Max Capital</span>
  <span class="summary-value">${{ formatNumber(config.trade_config?.max_capital) }}</span>
</div>
```

Update the value to reflect the configured mode (this shows intent even before the automation runs):

```html
<span class="summary-value">
  <template v-if="config.trade_config?.max_capital_mode === 'percent'">
    {{ config.trade_config?.max_capital_percent }}% of Net Liq.
  </template>
  <template v-else>
    ${{ formatNumber(config.trade_config?.max_capital) }}
  </template>
</span>
```

This is the *configured* value (static). The `status-details` "Capital Used" line (§2.1) is the
*resolved* value (live) — they are complementary: one shows the rule, the other shows the outcome.

### 2.4 Logs dialog

No change needed. The existing logs dialog already renders `log.message` + `log.details`, so the
backend's percent-mode audit entries (arch §9) appear automatically.

---

## 3. Component & styling guidance

### 3.1 Toggle component — reuse existing idiom (no PrimeVue SelectButton)

The codebase has **no** `SelectButton`/`ToggleButton`; segmented selectors are custom `<button>` pills
(e.g. `DataImportDialog.vue` `.selection-mode-toggle` / `.mode-btn`). Match that idiom for consistency.
Add scoped CSS in `AutomationConfigForm.vue`:

```css
.capital-mode-toggle {
  display: flex;
  background: var(--bg-tertiary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-md);
  padding: 2px;
  width: fit-content;
  margin-bottom: var(--spacing-xs);   /* aligns with .form-field gap rhythm */
}
.capital-mode-toggle .mode-btn {
  background: none;
  border: none;
  padding: var(--spacing-xs) var(--spacing-md);
  border-radius: var(--radius-sm);
  cursor: pointer;
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  transition: var(--transition-fast);
}
.capital-mode-toggle .mode-btn.active {
  background: var(--color-brand);
  color: var(--text-primary);
}
```

- Use `type="button"` so the buttons never submit the form.
- Keep the toggle compact (`width: fit-content`) — it should not stretch the full grid cell.

### 3.2 Inputs

- **Percent input:** PrimeVue `InputNumber` with `suffix="%"`, `:min="1"`, `:max="100"`,
  `:maxFractionDigits="0"` (whole-percent; backend rounds dollars anyway, arch §5.4).
- **Fixed input:** unchanged (`mode="currency" currency="USD" :min="100"`).
- Both live in the same grid cell, so the two-column `.form-grid` layout is preserved.

### 3.3 Hint / error text

- Reuse `<small class="field-hint">` (`--font-size-xs`, `--text-tertiary`) for the resolved-dollars
  hint — the automation form does not italicize hints, so don't add italics.
- Reuse `<small class="p-error">` for the percent validation error, mirroring `errors.name`.

### 3.4 "Capital Used" row — no new CSS

Reuse the existing `.status-row` / `.status-label` / `.status-value` classes. No new styles required.
(`--text-tertiary` label, `--text-primary` value — consistent with State/Message rows.)

### 3.5 Visual hierarchy & ordering

- Config form: `label → toggle → input → hint`. Toggle sits between label and input so the mode is
  chosen before the value is entered — natural reading order, no added vertical noise.
- Dashboard `status-details` order: `State → Capital Used → [Today's Trade] → Message → evaluation`.
  Placing "Capital Used" high gives the risk figure prominence without competing with the status badge.

---

## 4. States & interaction notes

| State | Config form | Dashboard |
|---|---|---|
| **Fixed (default)** | Currency input, `$` prefix, min $100. Hint: "Maximum capital to risk on this trade". | "Capital Used: $5,000" (once captured). Summary: "$5,000". |
| **Percent** | `%` suffix input 1–100. Hint shows "≈ $X of $Y" when Net Liq known, else plain "% of Net Liq." | "Capital Used: $6,000 (60% of $10,000)". Summary: "60% of Net Liq." |
| **Toggle switch** | Values preserved on both sides (FR-2); active pill highlighted in brand color. | — |
| **Net Liq unknown (form)** | Percent hint falls back to no-dollar text; save still allowed. | — |
| **Not yet captured** | — | "Capital Used" row hidden (`v-if` on `effective_capital`); Message still shows status. |
| **Net Liq unavailable at trade time (FR-8)** | — | Backend sets Message (e.g. "Net Liq unavailable — will retry"); card status turns waiting/failed via existing `getRunningStatusClass`. No special UX; the existing failed/waiting styling covers it. |
| **Validation error (percent)** | `<small class="p-error">` under input; save blocked client-side. | — |

---

## 5. What NOT to change (explicit)

- Do **not** restructure the Trade Configuration section, the `.form-grid`, or any other field
  (Strategy, Target Delta, Spread Width, Order Settings, Price Ladder, Iron Condor sides, Expiration).
- Do **not** replace the existing currency `InputNumber` behavior in fixed mode — it must stay
  byte-for-byte for regression parity (AC-4).
- Do **not** introduce a new UI library component or a PrimeVue `SelectButton`; use the existing
  custom pill-toggle idiom.
- Do **not** add or restructure dashboard sections; only add one `status-row` and make two existing
  values mode-aware.
- Do **not** add new backend endpoints for the form's Net-Liq preview; degrade gracefully if the value
  isn't cheaply available client-side.
- Do **not** clear `max_capital` or `max_capital_percent` when toggling modes (FR-2).

---

## 6. Acceptance-criteria coverage (UI portion)

- **AC-1/AC-2:** §1 toggle + percent input (1–100) persisted via existing save (§1.7).
- **AC-4:** §1.4/§5 keep fixed-mode input & behavior unchanged.
- **AC-5/AC-6:** §2.1 "Capital Used" line renders monitoring-start & trade-time snapshots from
  `effective_capital*`; §2.2 keeps the phase-aware Message; §2.4 logs unchanged.
- **AC-7:** §1.4 treats empty `max_capital_mode` as fixed; §2.3 summary defaults to `$` when no mode.
- **AC-8:** §4 — Net-Liq-unavailable surfaces via existing Message + failed/waiting card styling.

---

*Prepared by @ux. Awaiting @po review, then @dev implementation.*
