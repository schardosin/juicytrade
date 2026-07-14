# Requirements — Auto Mode with Multiple Trades (Lot Size)

**Issue:** [#81 — Auto Mode with multiple trades](https://github.com/schardosin/juicytrade/issues/81)
**Repository:** schardosin/juicytrade
**Branch:** `fleet/issue-81-auto-mode-with-multiple-trades`
**Status:** Draft — pending customer approval

---

## 1. Summary

Add a **lot size** capability to Auto Trade mode so that a single automation
trigger can split its capital-derived quantity into **multiple sequential
orders** of a configured size, instead of placing one large order.

Each order enters the **same strategy legs** and is priced **fresh to the live
market** at the moment that order begins (its own price ladder). A new
**delta-drift-per-lot flag** controls whether subsequent orders re-select
strikes or lock to the first order's legs.

## 2. Context & Motivation

Today, when an automation fires, the engine computes a single `units` quantity
from capital (`CalculateUnitsWithCapital = MaxCapital ÷ (width × 100)`), finds
strikes for the target delta, and places **one** order for the full quantity. It
then monitors that single order to fill (walking the price ladder, optionally
replacing strikes on delta drift).

Placing one large order can suffer from poor fills and market impact. Traders
often prefer to "leg into" a position with several smaller clips, each priced to
the current market. This feature lets the user define a **lot size** so the same
total quantity is executed across multiple smaller, sequentially-placed orders.

**Example (from the issue):** Trading NDX iron condor, 50-wide each side risks
$5,000/unit. With Max Capital $30,000, the engine sizes 6 units. With **lot
size = 2**, the automation places **3 orders of 2** — one after another — rather
than one order of 6.

## 3. Functional Requirements

### FR-1 — Lot Size configuration
- Add a new configuration field **`lot_size`** (integer) to the trade
  configuration (`TradeConfiguration`).
- `lot_size` represents the **number of units (spread quantity) per order**.
- Valid range: integer `≥ 1`.
- **Default / backward compatibility:** when `lot_size` is unset, `0`, or `1`,
  behavior is identical to today (a single order for the full quantity). No
  existing automation changes behavior unless the user opts in with `lot_size ≥ 2`.

### FR-2 — Total quantity is computed FIRST, then split
- The engine computes the **total quantity** exactly as today via
  `CalculateUnitsWithCapital(resolvedCap)` — this remains the hard cap.
- The total is then split into orders of `lot_size`:
  - `fullLots = totalUnits ÷ lot_size` (integer division)
  - `remainder = totalUnits mod lot_size`
- The resulting order plan is: `fullLots` orders of `lot_size` units, **plus**,
  if `remainder > 0`, **one final order of `remainder` units** (customer
  answer #1 — Option (a): the remainder IS traded).
  - Example: total 7, lot size 2 → orders of `[2, 2, 2, 1]` (4 orders).
  - Example: total 6, lot size 2 → orders of `[2, 2, 2]` (3 orders).
  - Example: total 5, lot size 5 → orders of `[5]` (1 order).
  - Example: total 3, lot size 5 → orders of `[3]` (1 order — lot size larger
    than total yields a single order for the total).

### FR-3 — Capital is never exceeded
- The sum of all order quantities MUST equal the capital-derived `totalUnits`
  and MUST NOT exceed it (customer answer #4). Lot size is purely a mechanism to
  **slice the same total** into smaller orders — it never increases exposure.
- If `totalUnits == 0` (insufficient capital), behavior is unchanged: the
  automation fails with "Insufficient capital for minimum position size" and no
  orders are placed.

### FR-4 — Sequential execution ("one after the other")
- Orders are placed **strictly sequentially**. Order **N+1 does not begin until
  order N is completely filled** (customer answer #2 — Option (a)).
- The existing monitoring / price-ladder / fill lifecycle is reused **per
  order**: each order is placed, monitored, and walked down the price ladder
  until it fills, then the next order begins.

### FR-5 — Per-order dynamic pricing
- Each order's limit price is **recalculated fresh** at the moment that order
  starts, using the existing pricing rules (starting offset, price ladder step,
  min credit, attempt interval, max attempts) against the **current market
  price** at that time (customer answer #3).
- Orders do **not** reuse the previous order's fill price — each order runs its
  own independent price ladder.

### FR-6 — Legs & the delta-drift-per-lot flag
- Add a new boolean configuration flag controlling strike selection across
  orders (customer answer #6). Proposed name: **`legs_drift`** (final name to be
  set by @architect):
  - **`legs_drift = false` (default, and the "same legs" behavior):**
    - The **first order** selects strikes for the target delta.
    - **All subsequent orders reuse the exact same strikes** as the first order.
    - **No delta-drift replacement occurs at all** — not at the start of any
      order and **not during** any order's monitoring / price-ladder phase. The
      existing mid-order delta-drift replacement (`DeltaDriftLimit`) is
      effectively disabled for the entire automation run when `legs_drift = false`.
  - **`legs_drift = true`:**
    - **Before starting each new order**, the engine re-selects strikes for the
      target delta based on the then-current market. These strikes may be the
      same as, or different from, the first order's strikes.
    - The existing per-order delta-drift behavior during monitoring continues to
      apply as it does today (governed by `DeltaDriftLimit`).
- **Backward compatibility:** the default (`legs_drift = false`) means existing
  single-order automations behave as before, since with `lot_size ≤ 1` there is
  only one order and the current delta-drift behavior during that order's
  monitoring is preserved **only when `legs_drift = true`**. See Open Decision
  OD-1 below regarding interaction with existing single-order delta drift.

### FR-7 — Recurrence interaction (`once` and `daily`)
- Lot size applies **per trigger**, for **both** `once` and `daily` recurrence
  (customer answer #5).
- **"Traded today" (daily) means ALL orders for that trigger have been submitted
  and completely filled** (customer answer #5). The automation must not mark
  `TradedToday = true` / reset for the next day until the full order plan has
  filled.
- For `once` recurrence, the automation reaches the terminal `completed` state
  only after **all** orders in the plan have filled.

### FR-8 — State, tracking & observability
- Each order in the plan is recorded as its own `PlacedOrder` (appended to
  `PlacedOrders`) and produces its own `AutomationPosition` on fill, exactly as a
  single order does today.
- Progress must be visible to the user during execution, e.g. logs / status
  message indicating "Order 2 of 3 filled (2 units)…". Exact UX is delegated to
  @ux, but the requirement is that the user can see how many orders are planned
  and how many have filled.

### FR-9 — Validation
- `lot_size` must be an integer `≥ 1` (or unset). Reject negative or non-integer
  values at the API / config validation layer with a clear error.
- `legs_drift` is a boolean, default `false`.

## 4. Error & Edge-Case Handling

- **E-1 — Order fails to fill:** Per customer answer #2, order N+1 cannot open
  until N fills. If an order exhausts its price ladder / max attempts **without
  filling**, the engine follows the **existing** single-order
  failure/exhaustion behavior for that order and the automation run stops
  placing further orders (the remaining lots are not placed). The specific
  end-state (fail vs. wait-for-next-day for `daily`) reuses today's logic. This
  is captured as **Open Decision OD-2** for confirmation.
- **E-2 — `totalUnits < lot_size`:** Place a single order for `totalUnits`
  (FR-2). Not an error.
- **E-3 — Insufficient capital (`totalUnits == 0`):** No orders placed; existing
  failure path (FR-3).
- **E-4 — Strike-finding failure on a later order (`legs_drift = true`):** Reuse
  the existing strike-finding error/retry behavior. Captured under OD-2.
- **E-5 — Backward compatibility:** Automations without `lot_size` /
  `legs_drift` in stored config must load and run unchanged (fields default to
  `0/false`).

## 5. Non-Functional Requirements

- **NFR-1 — Backward compatibility:** Existing persisted automations
  (`provider`/automation storage JSON) must deserialize without error and behave
  exactly as today.
- **NFR-2 — No new external dependencies.** Reuse existing pricing, monitoring,
  strike-finding, and tracking code paths.
- **NFR-3 — Concurrency safety:** Sequencing state must be managed with the
  engine's existing `sync.RWMutex` discipline; no data races on the multi-order
  run state.
- **NFR-4 — Conventions:** Follow project Go conventions — `snake_case` JSON
  tags, structured `log/slog` logging, `<success,data,message>` API responses.

## 6. Acceptance Criteria

1. **AC-1:** A `lot_size` field exists on the trade configuration, persists to
   storage, and is surfaced in the automation setup UI. Values `< 1` (except
   unset) are rejected with a clear validation message.
2. **AC-2:** With total quantity 6 and `lot_size = 2`, the automation places
   exactly **3 orders of 2 units each**, sequentially, and only after all 3 fill
   is the automation considered done for that trigger.
3. **AC-3:** With total quantity 7 and `lot_size = 2`, the automation places
   **4 orders** with quantities `[2, 2, 2, 1]` (remainder is traded — Option a).
4. **AC-4:** The sum of all order quantities equals the capital-derived total and
   never exceeds Max Capital.
5. **AC-5:** Order N+1 is not placed until order N is completely filled.
6. **AC-6:** Each order's limit price is computed fresh from the live market when
   that order begins (independent price ladder per order).
7. **AC-7 (`legs_drift = false`):** All orders use the **exact same strikes** as
   the first order, and **no delta-drift strike replacement occurs at any point**
   (neither at order start nor mid-order).
8. **AC-8 (`legs_drift = true`):** Each new order re-selects strikes for the
   target delta before it begins; existing mid-order delta-drift behavior applies.
9. **AC-9:** Lot size works for both `once` and `daily` recurrence. For `daily`,
   `TradedToday` becomes true only after **all** orders have filled.
10. **AC-10:** An existing automation saved without `lot_size` / `legs_drift`
    loads and runs identically to before (single order).
11. **AC-11:** The user can observe multi-order progress (planned count and
    filled count) in logs/status during execution.

## 7. Scope

**In scope:**
- New `lot_size` and `legs_drift` (or architect-chosen name) config fields.
- Splitting the capital-derived total into sequential orders.
- Per-order fresh pricing reusing existing price-ladder logic.
- Per-order strike behavior governed by the drift flag.
- Recurrence (`once`/`daily`) correctness with multi-order plans.
- Backend automation engine, config/model, validation, storage.
- Frontend automation setup UI additions (lot size input, drift toggle) and
  progress display.

**Out of scope (explicitly NOT included):**
- Parallel / concurrent order submission (orders are strictly sequential).
- Distributing orders across time windows / TWAP-style scheduling beyond
  "next order after previous fills".
- Varying leg selection strategy beyond the two drift modes described.
- Changing the capital-sizing formula itself.
- Any change to non-automation manual trading flows.

## 8. Open Decisions for @architect / follow-up

- **OD-1 (delta drift + single order):** When `lot_size ≤ 1` (single order),
  should `legs_drift = false` also disable the existing mid-order delta-drift
  replacement, or should single-order behavior remain exactly as today
  regardless of the new flag? *Recommendation:* preserve today's single-order
  behavior when `lot_size ≤ 1` to guarantee zero regression, and only let
  `legs_drift` govern multi-order runs. **To confirm with customer.**
- **OD-2 (partial-plan failure end-state):** When an order in the middle of the
  plan fails to fill after exhausting attempts, confirm the desired end-state:
  stop and mark failed (leaving earlier filled orders as open positions), vs.
  `daily` → wait for next trading day. *Recommendation:* reuse existing
  single-order exhaustion behavior and stop placing further lots. **To confirm.**

---

*Answers incorporated from customer clarifications (#1 Option a; #2 after fill /
Option a; #3 fresh price per order; #4 total computed first, never exceed
capital; #5 applies to once & daily, "traded today" = all orders filled; #6 new
legs-drift flag.)*
