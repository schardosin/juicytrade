# UX Design — Auto Mode with Multiple Trades (Lot Size)

**Issue:** [#81 — Auto Mode with multiple trades](https://github.com/schardosin/juicytrade/issues/81)
**Repository:** schardosin/juicytrade
**Branch:** `fleet/issue-81-auto-mode-with-multiple-trades`
**Author:** @ux
**Status:** Draft — pending PO review
**Grounded in:** architecture §10, `AutomationConfigForm.vue`, `AutomationDashboard.vue`
**References:**
- 📄 [requirements.md](https://github.com/schardosin/juicytrade/blob/fleet/issue-81-auto-mode-with-multiple-trades/docs/issue-81-auto-mode-with-multiple-trades/requirements.md)
- 📐 [architecture.md](https://github.com/schardosin/juicytrade/blob/fleet/issue-81-auto-mode-with-multiple-trades/docs/issue-81-auto-mode-with-multiple-trades/architecture.md) (§10)

---

## 1. Scope & Design Principles

This spec covers **two UI surfaces** and nothing more:

1. **Config form** — add a **Lot Size** input and a **Legs Drift** toggle to
   `AutomationConfigForm.vue`, integrated into the existing Trade Parameters grid.
2. **Running-automation progress** — add a **"Lot X of N"** indicator plus per-lot
   fill progress to the status area in `AutomationDashboard.vue`, collapsing to
   today's single-order status when there is no multi-lot plan.

**Principles (aligned with the architecture's zero-regression stance):**

- **Reuse existing patterns, not new ones.** Both surfaces are built entirely from
  components/classes already in these files (`InputNumber`, `InputSwitch`, `form-field`,
  `field-hint`, `status-row`, `result-chip`). No new component library, no new design
  tokens.
- **Additive, not disruptive.** Nothing existing moves or changes visually. The two new
  form fields append to the current grid; the progress block is inserted as one new
  `status-row` group. When `lot_size = 1`, the config form looks essentially unchanged in
  spirit and the dashboard renders **byte-for-byte** as today (UI mirror of OD-1).
- **Progressive disclosure.** The progress display only appears for genuine multi-lot
  runs (`order_plan.length > 1`). Single-order users never see new status clutter.

---

## 2. Surface 1 — Config Form Fields (`AutomationConfigForm.vue`)

### 2.1 Placement

The Trade Parameters `form-grid` ends at **line 630**, with `Delta Drift Limit`
(lines 617–629) as its last field. Append the two new fields **as the last two
`form-field` blocks in that same grid**, immediately after `Delta Drift Limit` and
before the closing `</div>` at line 630.

Rationale for this ordering:
- `Lot Size` and `Legs Drift` are *execution-slicing* parameters, conceptually adjacent
  to the pricing/attempt/drift controls that dominate the lower half of the grid. They
  belong with `Delta Drift Limit`, not up with capital/strike selection.
- `Legs Drift` directly modulates `Delta Drift Limit` behavior (when off, mid-order delta
  drift is disabled for a multi-lot run — see architecture §6.4). Placing them adjacent
  makes that relationship visually obvious.
- Ordering **Lot Size → Legs Drift** reflects the mental model: "how many lots" first,
  then "how do later lots pick strikes".

```
Trade Parameters grid (existing, unchanged order):
  [ Starting Offset ] [ Minimum Credit ] [ Price Ladder Step ]
  [ Max Attempts    ] [ Attempt Interval] [ Delta Drift Limit ]
  [ Lot Size (NEW)  ] [ Legs Drift (NEW)]        ← appended
```

The grid is already responsive (auto-flowing `form-grid`); two more cells flow naturally
into the next row on desktop and stack on mobile. **No grid/layout CSS changes needed.**

### 2.2 Lot Size field

Mirror the existing `InputNumber` pattern (e.g. `Max Attempts`, lines 596–604). Integer,
no fractional digits, no grouping separator.

```html
<div class="form-field">
  <label for="lotSize">Lot Size</label>
  <InputNumber
    id="lotSize"
    v-model="config.trade_config.lot_size"
    :min="1"
    :max="100"
    :useGrouping="false"
    placeholder="e.g., 1"
  />
  <small class="field-hint">Units per order. Leave at 1 for a single order; 2+ splits the total into sequential lots.</small>
</div>
```

- `:min="1"` — the form never lets the user enter `< 1`. This is the primary UX guard;
  the backend `validateLotConfig` (architecture §9.1) remains the authority and rejects
  `< 0`, but the input should never produce an invalid value in the happy path.
- Integer only (no `minFractionDigits`/`maxFractionDigits`), matching `Max Attempts` /
  `Attempt Interval`.
- `:max="100"` is a sane guard rail consistent with the other capped numerics in this
  grid; it is not a hard requirement — align with the developer if the backend prefers a
  different ceiling.

### 2.3 Legs Drift field

Use the same grid cell shape as the other fields so it aligns in the row. The control is
a PrimeVue **`InputSwitch`** (already globally registered — see `main.js:134`, and used
at `AutomationConfigForm.vue:179` and `:872`). Because `InputSwitch` is a small inline
control (not a full-width input like the others), lay it out with the label and switch on
one line, keeping the shared `field-hint` beneath.

```html
<div class="form-field">
  <label for="legsDrift">Legs Drift</label>
  <div class="inline-switch-field">
    <InputSwitch id="legsDrift" v-model="config.trade_config.legs_drift" />
    <span class="switch-state-label">
      {{ config.trade_config.legs_drift ? 'On — re-select strikes each lot' : 'Off — reuse first lot strikes' }}
    </span>
  </div>
  <small class="field-hint">When off, every lot reuses the first lot's strikes and mid-order delta drift is disabled. When on, strikes are re-selected before each lot and delta drift applies.</small>
</div>
```

**Scoped CSS to add** (small, consistent with existing token usage — `--spacing-sm`,
`--text-tertiary`, `--font-size-xs`):

```css
.inline-switch-field {
  display: flex;
  align-items: center;
  gap: var(--spacing-sm);
}
.switch-state-label {
  color: var(--text-tertiary);
  font-size: var(--font-size-sm);
}
```

The inline state label ("On …" / "Off …") mirrors the pattern already used for the
config `enable-toggle` (lines 869–879), where the switch is paired with a live text
description. This gives the boolean a plain-language meaning at a glance, which matters
because the two states have non-obvious consequences.

### 2.4 Default config object

Per architecture §10.2, add to the reactive default `trade_config` (currently ends at
line 966–967). Place these two keys **adjacent to `delta_drift_limit` (line 959)** to
keep the JS default ordering parallel to the visual field ordering:

```js
delta_drift_limit: 0.01,
lot_size: 1,
legs_drift: false,
// ... starting_offset, min_credit, etc.
```

`lot_size: 1` and `legs_drift: false` are the legacy-equivalent defaults, so a freshly
created config behaves exactly as today (AC-10).

### 2.5 Interaction notes (config form)

- **No new validation UI.** These fields reuse the form's existing submit/validation
  flow. `InputNumber :min="1"` prevents invalid lot sizes at the source. Do **not** add
  bespoke inline error text — if the backend rejects a value, it surfaces through the
  form's existing error path, consistent with all other fields.
- **No conditional hiding.** Both fields are always visible (like every other trade
  parameter). Do not hide `Legs Drift` when `lot_size = 1`; keeping it visible is simpler
  and consistent, and its hint already explains it applies to multi-lot runs. (Optional
  enhancement, only if the developer finds it trivial: dim/disable `Legs Drift` when
  `lot_size < 2` since it has no effect then. Not required for this issue.)
- **Iron Condor:** these are run-level parameters, not per-side, so they live once in the
  main grid — **not** duplicated into the IC put/call side configs. Correct as specified.

---

## 3. Surface 2 — Running-Automation Progress (`AutomationDashboard.vue`)

### 3.1 Placement

The running-status block is the `.status-details` container (lines 190–297). It renders a
sequence of `.status-row` entries (State, Today's Trade, Message) followed by the
evaluation results. Insert the lot-progress block as a **new `.status-row` group placed
immediately after the `State:` row (line 194) and before the daily `Today's Trade:` row
(line 196).**

Rationale: lot progress is *execution* status — it belongs right next to `State:` (which
shows `trading`/`monitoring`), above the indicator-evaluation details. It reads top-down
as: *what state → how far through the plan → indicator context*.

```
.status-details
  ├─ State: monitoring                          (existing, line 191–194)
  ├─ Lot 2 of 4   [▓▓░░]  (NEW — see §3.2)       ← inserted here
  ├─ Today's Trade: Pending   (daily only)       (existing, line 196–202)
  ├─ Message: ...                                (existing, line 203–206)
  └─ Last Evaluation: ...                        (existing, line 207+)
```

### 3.2 Progress markup

Reuse the existing `.status-row` / `.status-label` / `.status-value` pattern for the
textual indicator, and reuse the existing `.result-chip` styling vocabulary for the
per-lot dots. Wrap the whole thing in a single `v-if` guard so it is completely absent for
single-order/legacy runs.

```html
<!-- Multi-lot progress (only for genuine multi-lot plans) -->
<template v-if="getLotProgress(config.id)">
  <div class="status-row lot-progress-row">
    <span class="status-label">Lot Progress:</span>
    <span class="status-value">
      Lot {{ getLotProgress(config.id).currentDisplay }} of {{ getLotProgress(config.id).total }}
    </span>
  </div>
  <div class="lot-dots" :title="getLotProgress(config.id).tooltip">
    <span
      v-for="(lot, i) in getLotProgress(config.id).lots"
      :key="i"
      class="lot-dot"
      :class="lot.state"
    >{{ lot.qty }}</span>
  </div>
</template>
```

Each `lot-dot` shows the lot's unit quantity (e.g. `2`, `2`, `2`, `1` for a `[2,2,2,1]`
plan) and is colored by state:
- **filled** — a lot with a completed fill (green, reusing `--color-success`)
- **active** — the lot currently being placed/monitored (info/brand accent)
- **pending** — not yet started (neutral/tertiary)

This gives an at-a-glance "fuel gauge" of the plan (`▓▓░░`) plus the exact "Lot 2 of 4"
count, satisfying FR-8 / AC-11.

### 3.3 Derived-state helper (`getLotProgress`)

Add a computed helper next to `getAutomationStatus` (line 645). It is the **single guard
point** for the legacy collapse rule — the template stays clean and there is exactly one
place that decides "is this a multi-lot run?".

```js
// Returns null for single-order / legacy runs so the progress block is hidden entirely.
const getLotProgress = (configId) => {
  const s = getAutomationStatus(configId)
  const plan = s?.order_plan
  // Collapse to today's single-order status when no plan or a trivial 1-lot plan.
  if (!plan || plan.length <= 1) return null

  const total = plan.length
  const currentIdx = s.current_lot_index ?? 0            // 0-based
  const filledCount = Array.isArray(s.placed_orders)
    ? s.placed_orders.filter(o => o.status === 'filled').length
    : currentIdx                                         // fallback: lots before current are filled

  const lots = plan.map((qty, i) => ({
    qty,
    state: i < filledCount ? 'filled'
         : i === currentIdx ? 'active'
         : 'pending',
  }))

  return {
    total,
    currentDisplay: Math.min(currentIdx + 1, total),     // 1-based for display
    lots,
    tooltip: `${filledCount} of ${total} lots filled · plan [${plan.join(', ')}]`,
  }
}
```

**Notes for the developer:**
- `current_lot_index` is **0-based** on the payload (architecture §4.3). Display is
  **1-based** (`Lot 2 of 4`), hence `currentIdx + 1`.
- The `filledCount` derivation prefers `placed_orders` (counting `status === 'filled'`).
  Confirm the exact per-order status field name with the backend payload; if
  `placed_orders` entries don't carry a reliable fill flag, the `currentIdx` fallback
  (lots before the current index are filled) is correct given the strictly-sequential
  model (architecture §6.2 — lot N+1 only starts after N fills).
- On the **last lot filled** (terminal), `current_lot_index` points at the last lot and
  all dots show filled — the block naturally shows "Lot N of N" all-green just before the
  run goes to `completed` / daily reset. Once the run leaves active states the status
  block for a `once` run stops rendering as today; no special handling needed.

### 3.4 Scoped CSS to add

Reuse existing tokens and mirror `.result-chip` sizing so the dots visually belong to the
same family as the indicator chips already in this card.

```css
.lot-progress-row .status-value {
  font-variant-numeric: tabular-nums;
}
.lot-dots {
  display: flex;
  flex-wrap: wrap;
  gap: var(--spacing-xs);
  margin: 2px 0 var(--spacing-xs) 0;
}
.lot-dot {
  min-width: 20px;
  padding: 2px 6px;
  border-radius: var(--radius-sm);
  font-size: var(--font-size-xs);
  text-align: center;
  font-weight: var(--font-weight-medium);
}
.lot-dot.filled {
  background: rgba(34, 197, 94, 0.1);   /* matches .result-chip.passed */
  color: var(--color-success);
}
.lot-dot.active {
  background: rgba(59, 130, 246, 0.12);
  color: var(--color-info);
  outline: 1px solid var(--color-info);
}
.lot-dot.pending {
  background: var(--bg-quaternary, var(--border-primary));
  color: var(--text-tertiary);
}
```

(The `rgba(34, 197, 94, 0.1)` filled style is copied verbatim from the existing
`.result-chip.passed` rule at line ~1486 so the two chip families match exactly.)

### 3.5 States the developer must handle

| State | Rendering |
|-------|-----------|
| **Single-order / legacy** (`order_plan` empty or length ≤ 1) | `getLotProgress` returns `null` → block absent. Status card identical to today (AC-10, OD-1 UI mirror). |
| **Multi-lot, in progress** | "Lot X of N" + dots: filled lots green, current lot outlined, pending lots neutral. |
| **Multi-lot, last lot filled** | "Lot N of N", all dots green, momentarily before terminal transition. |
| **Mid-plan exhaustion / halt (OD-2)** | No special progress-specific UI. The dots freeze at the last known state (some filled, current stalled); the existing `Message:` row and `status-badge` already communicate the failure/wait state. Do not invent a new error visual — reuse existing status/message channels. |
| **Mobile** | `.lot-dots` uses `flex-wrap`, so dots wrap cleanly in the narrow card. The `status-row` label/value already collapse gracefully on mobile today. No extra work. |

---

## 4. What NOT to Change

- **Do not** reorder or restyle any existing Trade Parameters field. The two new fields
  are appended only.
- **Do not** change the `.form-grid`, `.status-details`, `.status-row`, or `.result-chip`
  layout/CSS. Only *add* the small scoped rules in §2.3 and §3.4.
- **Do not** touch the indicator-evaluation section (`group_results`, `indicator_results`,
  lines 207–296) — the lot-progress block sits above it and is independent.
- **Do not** add lot fields to the Iron Condor per-side configs — lot size / legs drift
  are run-level.
- **Do not** build a custom validation/error component for `lot_size` — the `:min="1"`
  input guard plus the existing form error flow are sufficient.
- **Do not** render the progress block for single-order runs — the `getLotProgress` guard
  must return `null` so the dashboard is unchanged for the legacy path.

---

## 5. Traceability

| Requirement | Covered by |
|-------------|-----------|
| AC-1 (Lot Size field exists, in UI, `<1` guarded) | §2.2 (`:min="1"`) |
| FR-6 / Legs Drift field | §2.3 |
| AC-10 / OD-1 (legacy config & status unchanged) | §2.4 defaults, §3.3 null-guard, §3.5, §4 |
| FR-8 / AC-11 (observe multi-lot progress) | §3.2 "Lot X of N" + per-lot dots from `order_plan`/`current_lot_index`/`placed_orders` |
| OD-2 (mid-plan halt) UI behavior | §3.5 (reuse existing message/badge, dots freeze) |

---

*Design authored by @ux. Built entirely on existing PrimeVue components and the card's
existing CSS vocabulary; grounded in `AutomationConfigForm.vue` (trade-params grid + default
config) and `AutomationDashboard.vue` (`.status-details` block, `getAutomationStatus`).*
