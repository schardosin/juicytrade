# UX Design — Auto Mode: Percentage-based Max Capital

**Issue:** [#75](https://github.com/schardosin/juicytrade/issues/75) — Auto Mode with percentage for Max Capital
**Author:** @ux
**Status:** Draft — pending PO review
**Requirements:** [requirements.md](./requirements.md)
**Architecture:** [architecture.md](./architecture.md)

---

## 0. Scope of this document

This spec covers **only** the two frontend changes defined in architecture §10:

1. `AutomationConfigForm.vue` — a capital-mode selector + conditional Max Capital input.
2. `AutomationDashboard.vue` — a mode-aware Max Capital summary line.

It is scoped to integrating these two elements into the **existing** UI without a
redesign. No other sections, layouts, or components change. The wire contract
(field names, status payload) is owned by @architect and is treated here as fixed.

---

## 1. Design summary (key decisions)

| # | Decision | Rationale |
|---|----------|-----------|
| D1 | Use a PrimeVue **`SelectButton`** (two-option segmented control) for the capital mode, **not** `InputSwitch`. | Two explicit, equally-weighted labeled choices ("Fixed Amount ($)" / "Percentage (%)"). A switch implies on/off, not a choice between two named modes. `SelectButton` is a standard PrimeVue 3.46 component and matches the "toggle / two-option selector" the customer asked for. |
| D2 | The mode selector and the capital input live **inside the existing "Max Capital" `form-field`**, replacing the single field with a compact stacked block. No new section. | Keeps Max Capital exactly where users already look for it (Trade Configuration → first `form-grid`, next to Strategy / Delta / Width). Zero disruption to surrounding fields or grid flow. |
| D3 | **Conditional single input** — only the input for the active mode renders (`v-if`). Each mode keeps its own reactive value (`max_capital` vs `max_capital_percent`), so toggling back and forth preserves both. | Matches FR-2. Avoids showing two inputs at once (visual clutter, ambiguity about which drives sizing). |
| D4 | **Omit** the client-side dollar preview inside the form (FR-4a is explicitly optional / "@ux decides"). | There is **no cheap client-side Net Liq source** in the frontend today (confirmed: no account store, composable, or field exposes `account.equity`). Fetching it would require a new API call the architecture deliberately avoids. The authoritative resolved figure is surfaced server-side and appears on the dashboard immediately after activation — which fully satisfies AC-4. Adding a speculative client preview would risk showing a number that diverges from the server's, undermining trust. |
| D5 | On the dashboard, show `pct%` always in percent mode, and append `(≈ $X,XXX)` only when `resolved_max_capital > 0` arrives via status. | Progressive disclosure: pre-activation the user sees intent (`60%`); post-activation/live they see the concrete risk (`60% (≈ $6,000)`). Uses the `≈` glyph to signal the value is derived/point-in-time. |
| D6 | Reuse existing CSS custom properties and the `field-hint` / `p-error` patterns already in both files. No new color tokens, no preprocessor. | Consistency with the established design system (`--spacing-*`, `--color-brand #ff6b35`, `--text-*`, `--radius-*`). |

---

## 2. `AutomationConfigForm.vue` — layout & interaction

### 2.1 Where it goes (unchanged surroundings)

The Trade Configuration section's first `form-grid` currently holds, in order:
**Strategy**, (Target Delta + Spread Width for non-IC), **Max Capital**. Only the
**Max Capital** `form-field` changes. Everything else in the grid, and all other
sections (Basic Info, Entry Time, Indicators, Order/Ladder/Expiration Settings,
Strike Preview, Enable toggle), stays **exactly as-is**.

### 2.2 New structure of the Max Capital field

Replace the current single-input field with a stacked block: a **label**, a
**mode selector**, then the **mode-appropriate input**, then a **hint / error**.

```
┌─ form-field (Max Capital) ─────────────────────────────┐
│  Max Capital                                            │  ← existing <label>
│  ┌───────────────────┬───────────────────┐             │
│  │  Fixed Amount ($) │  Percentage (%)   │  ← SelectButton (segmented)
│  └───────────────────┴───────────────────┘             │
│                                                         │
│  [ mode === 'fixed' ]                                   │
│    $ [  5,000        ]   ← existing currency InputNumber │
│    "Maximum capital to risk on this trade"  (hint)      │
│                                                         │
│  [ mode === 'percent' ]                                 │
│    [  60           ] %   ← percent InputNumber (suffix %)│
│    "Percent of account Net Liq (1–100)"  (hint)         │
│    "Enter a value between 1 and 100"  (p-error, if bad) │
└─────────────────────────────────────────────────────────┘
```

Full-width within its grid cell; the `SelectButton` sits directly under the label
with a small gap (`--spacing-sm`), the input directly below it (`--spacing-sm`).

### 2.3 Component markup guidance (for @dev)

Bind the selector to `config.trade_config.capital_mode`. Keep the existing
currency input untouched for fixed mode; add the percent input for percent mode.

```vue
<div class="form-field capital-field">
  <label for="maxCapital">Max Capital</label>

  <!-- Mode selector -->
  <SelectButton
    v-model="config.trade_config.capital_mode"
    :options="capitalModeOptions"
    optionLabel="label"
    optionValue="value"
    :allowEmpty="false"
    class="capital-mode-select"
    aria-label="Max Capital mode"
  />

  <!-- Fixed mode: existing currency input, unchanged -->
  <template v-if="config.trade_config.capital_mode === 'fixed'">
    <InputNumber
      id="maxCapital"
      v-model="config.trade_config.max_capital"
      mode="currency"
      currency="USD"
      :min="100"
      placeholder="e.g., 5000"
    />
    <small class="field-hint">Maximum capital to risk on this trade</small>
  </template>

  <!-- Percent mode: new percentage input -->
  <template v-else>
    <InputNumber
      id="maxCapitalPercent"
      v-model="config.trade_config.max_capital_percent"
      suffix=" %"
      :min="1"
      :max="100"
      :minFractionDigits="0"
      :maxFractionDigits="1"
      placeholder="e.g., 60"
      :class="{ 'p-invalid': errors.max_capital_percent }"
    />
    <small class="field-hint">Percent of account Net Liq (1–100)</small>
    <small v-if="errors.max_capital_percent" class="p-error">
      {{ errors.max_capital_percent }}
    </small>
  </template>
</div>
```

`capitalModeOptions` in `setup()`:

```js
const capitalModeOptions = [
  { label: 'Fixed Amount ($)', value: 'fixed' },
  { label: 'Percentage (%)', value: 'percent' },
]
```

**`SelectButton` must be registered.** If it is not already globally registered
(PrimeVue components are registered globally in this app), @dev should confirm and
register it in the same place the other PrimeVue components are registered.

### 2.4 Config default additions

In the reactive `config` default `trade_config` object, add (per architecture §10.1):

```js
capital_mode: 'fixed',
max_capital_percent: 60,   // sensible starting value; only used in percent mode
```

`max_capital: 5000` stays as the default fixed value. Because each field is
independent, switching modes in the UI naturally preserves each value (D3) — no
extra watchers needed.

### 2.5 Precision decision (resolves requirements open question)

- Allow **integer or up to 1 decimal place** (`maxFractionDigits: 1`). One decimal
  covers realistic risk sizing (e.g., `2.5%`) without inviting noise. `minFractionDigits: 0`
  so whole numbers display cleanly as `60 %`, not `60.0 %`.
- The `%` renders as an input **suffix** (`suffix=" %"`), mirroring how the fixed
  input uses `mode="currency"` for the `$` — consistent affordance, no separate label.

### 2.6 Validation & feedback (AC-2)

Match the **existing** validation pattern in this form (the `errors` ref +
`:class="{ 'p-invalid': ... }"` + `<small class="p-error">` used by `name`,
`symbol`, `entry_time`).

- The `InputNumber` `:min="1" :max="100"` clamps typed values, but validation must
  still guard against empty / cleared values on **save**.
- In the existing `validateForm()` / save path, add: when
  `capital_mode === 'percent'`, require `max_capital_percent` be a number in
  `[1, 100]`; otherwise set `errors.max_capital_percent = 'Enter a value between 1 and 100'`
  and block save (same as other field errors).
- Only validate the field for the **active** mode (don't block save on a stale
  `max_capital_percent` when mode is `fixed`, and vice-versa).
- The percent input carries `p-invalid` styling when its error is set, consistent
  with other invalid fields.

### 2.7 States

| State | Appearance |
|-------|------------|
| Fixed (default) | Segmented control, "Fixed Amount ($)" selected; currency input shown. Identical to today's field otherwise. |
| Percent | "Percentage (%)" selected; percent input with ` %` suffix shown; hint "Percent of account Net Liq (1–100)". |
| Percent, invalid | `p-invalid` border on the input + `p-error` message; Save blocked (existing pattern). |
| Edit mode, legacy config (no `capital_mode`) | Loads as `fixed` (backend defaults empty → fixed; the form default `'fixed'` also covers a missing field). User sees the unchanged currency input. |

### 2.8 Responsive / mobile

- The `SelectButton` two options fit comfortably on mobile at the field's full
  width; if space is tight, allow the buttons to stretch equally
  (`.capital-mode-select { display: flex; }` with each button `flex: 1`).
- No change to the form's existing responsive grid; the block simply occupies its
  single grid cell and stacks vertically (which it already does within a `form-field`).

---

## 3. `AutomationDashboard.vue` — mode-aware summary

### 3.1 Where it goes (unchanged surroundings)

Only the **Max Capital** `summary-item` in the `trade-summary` block changes
(currently the last item, rendering `${{ formatNumber(config.trade_config?.max_capital) }}`).
All other summary items, the status details, indicator results, actions, and
dialogs stay **exactly as-is**.

### 3.2 Display logic

Add a small helper (mirrors the existing `formatNumber` / `formatStrategy`
helpers) that returns the display string for the summary value:

```js
const formatMaxCapital = (config) => {
  const tc = config?.trade_config || {}
  const mode = tc.capital_mode || 'fixed'   // empty => fixed (backward compat)
  if (mode !== 'percent') {
    return `$${formatNumber(tc.max_capital)}`
  }
  // percent mode
  const pct = formatNumber(tc.max_capital_percent)
  const status = getAutomationStatus(config.id)
  const resolved = status?.resolved_max_capital
  if (resolved && resolved > 0) {
    return `${pct}% (≈ $${formatNumber(Math.round(resolved))})`
  }
  return `${pct}%`
}
```

Template — replace only the value span:

```vue
<div class="summary-item">
  <span class="summary-label">Max Capital</span>
  <span class="summary-value">{{ formatMaxCapital(config) }}</span>
</div>
```

### 3.3 Presentation rules

| Situation | Displayed value | Example |
|-----------|-----------------|---------|
| Fixed mode (or legacy) | `$<amount>` (unchanged) | `$5,000` |
| Percent mode, not yet activated (no resolved value) | `<pct>%` | `60%` |
| Percent mode, activated / live (status carries `resolved_max_capital > 0`) | `<pct>% (≈ $<resolved>)` | `60% (≈ $6,000)` |

- The resolved figure is read from the automation **status object**
  (`resolved_max_capital`), which already flows in via `loadStatuses()` polling and
  the WebSocket `handleAutomationUpdate` handler (both spread `item.details` /
  `message.data` into `statuses`). **No new API call, no store change.**
- Round the resolved dollar amount to the nearest dollar for a clean summary; the
  authoritative precise value lives in the logs and backend.
- Keep the existing `.summary-value` styling; no new CSS needed for the base case.
  Optionally, the ` (≈ $…)` portion may be wrapped in a muted `<span class="resolved-hint">`
  using `color: var(--text-secondary); font-size: var(--font-size-sm);` if the PO
  wants the percent to stay visually primary. This is optional polish, not required.

### 3.4 Live updates

Because the resolved value updates again at execution time (architecture §7), the
dashboard string will naturally refresh from `60% (≈ $6,000)` to the latest
resolved figure whenever a new status arrives — no extra handling required; it is
a pure function of reactive `statuses`.

### 3.5 Failure surfacing (no new UI)

Per architecture, an activation failure (Net Liq unavailable) returns HTTP 400 and
is surfaced by the **existing** `startAutomation` error handler
(`alert('Failed to start automation: ' + message)`). Execution-time failures land
in the automation **logs** (already viewable via the Logs dialog) and the status
`message` row (already rendered). **No new error UI is in scope** — the existing
alert + logs + status message already communicate the no-fallback failure clearly.

---

## 4. Styling notes

Use existing tokens only. Suggested minimal scoped CSS additions:

`AutomationConfigForm.vue`:
```css
.capital-field .capital-mode-select {
  margin-bottom: var(--spacing-sm);
}
.capital-field .capital-mode-select :deep(.p-button) {
  flex: 1;                 /* equal-width segments, good on mobile */
}
.capital-field .capital-mode-select {
  display: flex;
}
```

`AutomationDashboard.vue` (optional, only if D5 muted-hint polish is adopted):
```css
.summary-value .resolved-hint {
  color: var(--text-secondary);
  font-size: var(--font-size-sm);
  margin-left: 4px;
}
```

No changes to `theme.css`. No new design tokens.

---

## 5. Accessibility

- The `SelectButton` gets `aria-label="Max Capital mode"`; each option's label is
  its accessible name.
- Percent `InputNumber` keeps the `id`/`<label for>` association (`maxCapitalPercent`)
  so the label is programmatically linked.
- Validation error uses the existing `p-error` `<small>` so it is read in context;
  keep it adjacent to the input.
- `:allowEmpty="false"` on `SelectButton` guarantees a mode is always selected
  (no undefined state), preserving keyboard and screen-reader clarity.

---

## 6. What NOT to change (explicit)

- Do **not** modify any other field in Trade Configuration (Strategy, Target Delta,
  Width, Iron Condor per-side configs, Order Settings, Price Ladder, Expiration,
  Strike Preview).
- Do **not** alter the fixed-mode currency input behavior (`mode="currency"`,
  `:min="100"`), its hint text, or its `id="maxCapital"`.
- Do **not** add a client-side Net Liq fetch or dollar preview inside the config
  form (D4).
- Do **not** add new endpoints, stores, composables, or WebSocket handlers — the
  dashboard reads resolved values from the existing `statuses` map.
- Do **not** introduce new color tokens, a CSS preprocessor, TypeScript, or
  `<script setup>`. Stay on JS + Options API `setup()` + PrimeVue + scoped CSS.
- Do **not** change dashboard status details, logs dialog, eval dialog, actions,
  or the responsive/mobile collapse behavior.

---

## 7. Acceptance-criteria mapping

| AC | Covered by |
|----|------------|
| AC-1 (toggle fixed vs percent) | §2.2–2.3 `SelectButton` |
| AC-2 (1–100 validation + message) | §2.6 validation, §2.7 states |
| AC-3 (legacy configs unchanged) | §2.4 default `'fixed'`, §2.7 edit-mode row, §3.2 `mode || 'fixed'` |
| AC-4 (resolved $ visible at activation) | §3.2–3.3 dashboard `60% (≈ $6,000)` from status |
| AC-8 (dashboard reflects mode) | §3 entire section |

(AC-5/6/7/9 are backend/testing concerns owned by @architect / @dev.)

---

## 8. Handoff notes for @dev

- Register/confirm `SelectButton` global registration.
- Add `capital_mode` + `max_capital_percent` to the form's `trade_config` default
  and to the `return {}` exposure (`capitalModeOptions`).
- Extend the existing `validateForm()`/save path with the percent-mode check (§2.6);
  add `max_capital_percent` to the `errors` object handling.
- Add `formatMaxCapital(config)` to the dashboard `setup()` and expose it in the
  `return {}`; replace only the Max Capital `summary-value` binding.
- Frontend tests to add (per architecture §15): toggle swaps inputs; percent input
  rejects `<1`/`>100` with a message; dashboard renders `$` for fixed and `pct%`
  (+ resolved `$`) for percent.
