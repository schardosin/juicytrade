# Requirements — Auto Mode: Percentage-based Max Capital (Issue #73)

**Issue:** [#73 — Auto Mode with percentage for Max Capital](https://github.com/schardosin/juicytrade/issues/73)
**Author:** @schardosin
**Status:** Draft — awaiting customer approval

---

## 1. Summary

Today, the **Max Capital** field in an automation's Trade Configuration is a
fixed dollar amount (e.g. `$5,000`). It is used to determine how many spread
units to trade: `units = MaxCapital / (spreadWidth × 100)`.

The customer wants the option to set Max Capital **dynamically as a percentage
of the account's Net Liquidating Value (Net Liq.)** instead of a fixed dollar
amount. Example: with a Net Liq. of `$10,000` and Max Capital set to `60%`,
the automation would risk `$6,000` on the trade.

## 2. Context & Motivation

- Position sizing based on a static dollar amount does not scale with the
  account. As the account grows or shrinks, the user must manually update the
  fixed Max Capital on every config.
- A percentage-of-account model lets the user express risk as a proportion of
  their capital, so sizing automatically tracks the account value.
- This is opt-in: users who prefer a fixed dollar cap keep the current
  behavior unchanged.

## 3. Functional Requirements

### FR-1 — Max Capital mode selector (toggle)
- The Trade Configuration form gains a **mode toggle** for the Max Capital
  field with two options:
  - **Fixed Amount** (`fixed`) — current behavior; a dollar value.
  - **Percentage of Net Liq.** (`percent`) — a percentage of the account's
    current Net Liquidating Value.
- Default mode is **Fixed Amount**, preserving the current behavior for all
  existing configurations.

### FR-2 — Input behavior per mode
- **Fixed mode:** the input is a currency value (as today), minimum `$100`.
- **Percentage mode:** the input is a percentage value between `1%` and
  `100%` (inclusive). The UI displays a `%` suffix and appropriate hint text.
- Switching modes preserves each mode's own value where practical, and clearly
  indicates which mode is active.

### FR-3 — Effective capital resolution (backend)
- When the automation reaches the point of sizing a position, it must compute
  an **effective Max Capital in dollars**:
  - **Fixed mode:** effective capital = the configured dollar amount (unchanged).
  - **Percentage mode:** effective capital = `NetLiq × (percentage / 100)`,
    where `NetLiq` is fetched from the account at that time.
- The number of units is then computed from the effective capital using the
  existing formula: `units = floor(effectiveCapital / (spreadWidth × 100))`
  (Iron Condor uses the wider side's width, as it does today).

### FR-4 — Source of Net Liquidating Value
- Net Liq. is obtained from the configured trading provider via the existing
  account lookup (`ProviderManager.GetAccount`).
- The value used is the account's net liquidating value / portfolio value
  (the `portfolio_value` / `equity` field on the `Account` model, whichever
  the provider populates as Net Liq.). The exact field mapping is a technical
  design decision for @architect.

### FR-5 — Capital capture display at monitoring start
- When the automation **starts monitoring** (i.e. when it becomes active and
  begins evaluating), it must capture and display the **effective capital it
  would use** based on the current Net Liq. and the configured percentage.
  - This confirms to the user that the percentage feature is working and shows
    the concrete dollar figure derived at that moment.
  - For fixed mode, this simply shows the fixed dollar amount.

### FR-6 — Capital capture display at trade time
- At the **moment of the trade** (when the automation sizes the position and
  places the order), it must capture and display the **effective capital
  actually used** for that trade, based on the Net Liq. read at that moment.
  - This may differ from the value shown at monitoring start if the account
    value changed in the interim.
  - Both the percentage and the resolved dollar amount should be visible.

### FR-7 — Logging & audit
- The automation log should record, in percentage mode:
  - The Net Liq. value read, the percentage, and the resolved effective
    capital, at both capture points (monitoring start and trade time).
- This ensures the user can audit how the traded size was derived.

### FR-8 — Failure handling when Net Liq. is unavailable
- If, in percentage mode, the account Net Liq. cannot be retrieved at trade
  time (provider error, missing field, or zero value), the automation must
  **not** place a trade with an incorrect size. It should:
  - Log a clear error explaining Net Liq. could not be determined, and
  - Follow the existing error path (increment error count / fail / retry per
    recurrence mode), the same way it does today when strike-finding fails.
- The precise retry/fail behavior mirrors the existing "insufficient capital /
  failed to find strikes" handling; final behavior to be confirmed in the
  architecture design.

### FR-9 — Backward compatibility & persistence
- Existing saved configs (which have only a fixed `max_capital` number and no
  mode field) must continue to work unchanged, defaulting to **Fixed** mode.
- The new mode and percentage value are persisted with the trade config and
  survive save/load and server restart (runtime state restoration).

## 4. Acceptance Criteria

1. **AC-1:** In the automation config form, the user can toggle the Max Capital
   field between **Fixed Amount** and **Percentage of Net Liq.**; Fixed is the
   default.
2. **AC-2:** In percentage mode, the user can enter a value from 1–100 and it
   is persisted with the config.
3. **AC-3:** With Net Liq. = `$10,000` and Max Capital = `60%`, the effective
   capital used for sizing is `$6,000` (matching the issue example).
4. **AC-4:** In fixed mode, sizing behavior is byte-for-byte identical to the
   current behavior (no regression).
5. **AC-5:** When monitoring starts, the UI/status shows the effective capital
   (dollar amount) that would be used based on the current Net Liq.
6. **AC-6:** At the moment the trade is placed, the effective capital used is
   computed from the Net Liq. read at that time and is displayed to the user
   and recorded in the automation log.
7. **AC-7:** Existing saved configs load and run without error and behave as
   fixed-amount configs.
8. **AC-8:** In percentage mode, if Net Liq. cannot be obtained at trade time,
   the automation does not place an incorrectly-sized order and logs a clear
   error following the existing failure path.
9. **AC-9:** All existing backend and frontend tests pass; new tests cover
   percentage resolution, the `$10,000 × 60% = $6,000` example, mode defaulting,
   and the Net-Liq-unavailable failure path.

## 5. Scope Boundaries (Explicitly NOT included)

- **Not** changing how spread width, delta, or units math works beyond
  substituting effective capital for the fixed amount.
- **Not** adding new position-sizing models beyond fixed-dollar and
  percent-of-Net-Liq (e.g. no per-trade Kelly sizing, no buying-power %).
- **Not** changing which account/provider is used; it uses the already
  configured trading provider.
- **Not** introducing real-time continuous recomputation of size while an order
  is live — capital is captured at the two defined points (monitoring start and
  trade time) only.
- **Not** altering the Iron Condor width-selection logic; percentage mode feeds
  the same existing units formula.

## 6. Open Questions for the Customer

1. **Q1 — Display location:** For the "captured capital" display (FR-5 / FR-6),
   is showing it in the automation **status message + log** sufficient, or would
   you like a dedicated field in the automation dashboard card (e.g. a
   "Capital Used: $6,000 (60% of $10,000)" line)? @ux can design either.
2. **Q2 — Rounding:** Should the effective dollar amount be rounded (e.g. to the
   nearest dollar) before the units calculation, or used as-is? (The units
   result is floored regardless, so this is a minor cosmetic/audit detail.)

---

*Prepared by @po. Awaiting customer approval before design (@architect / @ux)
and implementation (@dev).*
