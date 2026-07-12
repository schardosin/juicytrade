# Requirements — Auto Mode: Percentage-based Max Capital

**Issue:** [#75](https://github.com/schardosin/juicytrade/issues/75) — Auto Mode with percentage for Max Capital
**Author:** @schardosin
**Status:** Draft — pending customer approval

---

## 1. Summary

Today, an automation's **Max Capital** is a fixed dollar amount (`trade_config.max_capital`,
default `5000`). It caps the capital risked on a trade and drives position sizing via
`TradeConfiguration.CalculateUnits()` (`units = MaxCapital / (width * 100)`).

The customer wants the option to set Max Capital **dynamically as a percentage of the
account's Net Liquidating Value (Net Liq)** instead of a fixed dollar amount. For example,
with a Net Liq of $10,000 and Max Capital set to 60%, the automation would risk $6,000.

## 2. Context & Motivation

- A fixed dollar cap does not scale with account growth or drawdown. As the account balance
  changes, the trader must manually re-tune Max Capital on every automation.
- A percentage-of-Net-Liq cap lets risk sizing track the account automatically, keeping
  position size proportional to available capital.
- This is a common risk-management pattern ("risk X% of the account per trade").

## 3. Current Behavior (as-built, for reference)

- **Backend model:** `trade_config.max_capital float64` (`internal/automation/types/types.go`).
- **Sizing:** `CalculateUnits()` computes `int(MaxCapital / (width * 100))`.
- **Frontend:** `AutomationConfigForm.vue` renders a single currency `InputNumber` bound to
  `config.trade_config.max_capital` (min 100). `AutomationDashboard.vue` shows it as a
  `$` amount in the config summary.
- **Account data:** The backend already exposes account balances via
  `ProviderManager.GetAccount()`, returning `models.Account`. The **Net Liq** equivalent is
  the `Equity` field (Alpaca `account.Equity`, tastytrade `net-liquidating-value`), i.e.
  `account.equity` in the JSON contract. This is the value used for the percentage base.
- The account used is the currently active **trade account** provider.

## 4. Clarifications (confirmed with customer)

1. **UI control:** A **toggle** between "Fixed Amount ($)" and "Percentage (%)" modes.
2. **Percentage base:** **Net Liq** (account equity / net liquidating value) for now.
3. **When the percentage is resolved to a dollar amount — two moments:**
   - **At Auto Trade activation** — resolve and display the computed dollar figure so the
     user sees what will be risked when they start the automation.
   - **At the moment of the trade** — re-resolve using current/live account data so the
     actual sizing reflects the account state at execution time.
4. **Which account:** The **currently active trade account**.
5. **No fallback:** If Net Liq cannot be obtained (or is invalid/non-positive), the operation
   must **fail** — do not fall back to a fixed amount or a default. Fail activation and/or
   fail the trade with a clear error/log.
6. **Range:** Percentage is **1% to 100%**. Use existing account data for the calculation.
   No fallbacks.

## 5. Functional Requirements

### FR-1 — Capital mode toggle
- Add a **capital mode** to the trade configuration with two options:
  - `fixed` — the existing dollar-amount behavior (default; preserves backward compatibility).
  - `percent` — Max Capital is expressed as a percentage of account Net Liq.
- The UI presents this as a toggle (or two-option selector) in the Trade Configuration
  section, adjacent to the Max Capital input.

### FR-2 — Percentage input
- When mode is `percent`, the UI shows a percentage input constrained to **1–100** (integer or
  up to a small number of decimals — @ux to finalize; validation enforces 1 ≤ pct ≤ 100).
- When mode is `fixed`, the UI shows the existing currency input (min 100), unchanged.
- Switching modes swaps the visible input; each mode retains its own value.

### FR-3 — Backend model & persistence
- Extend `TradeConfiguration` to persist the capital mode and the percentage value, in addition
  to the existing `max_capital` dollar field. Exact field names/shape to be finalized by
  @architect (must use `snake_case` JSON tags per project convention and remain
  backward-compatible: existing configs with no mode default to `fixed`).

### FR-4 — Resolution at Auto Trade activation (preview)
- When the user activates/starts an automation whose capital mode is `percent`:
  - The backend fetches the active trade account's Net Liq (`account.equity`) via
    `ProviderManager.GetAccount()`.
  - It computes `resolved_max_capital = net_liq * (percent / 100)`.
  - The computed dollar figure is surfaced to the user (e.g., in the activation response /
    dashboard) so it is visible at start time.
- If Net Liq cannot be fetched or is non-positive, activation **fails** with a clear error
  message and an automation log entry. The automation must not start with an unresolved cap.

### FR-4a — Preview visibility in the config form
- Where the form already previews sizing/strikes, the percentage mode should reflect the
  resolved dollar figure when account data is available (best-effort display; exact placement
  is a @ux decision). This is a display aid; the authoritative resolution happens at
  activation (FR-4) and execution (FR-5).

### FR-5 — Resolution at trade execution (live)
- At the moment the automation places the trade (in the trading state, before
  `CalculateUnits()` is used for sizing), if capital mode is `percent`:
  - Re-fetch current Net Liq from the active trade account.
  - Recompute `resolved_max_capital = net_liq * (percent / 100)` using **current** data.
  - Use this resolved dollar value for position sizing (`CalculateUnits`).
- If Net Liq cannot be fetched or is non-positive at execution time, the trade **fails**
  (no fallback): the automation logs an error and transitions to the appropriate failed/error
  state rather than trading with a stale or default cap.

### FR-6 — Sizing calculation
- Position sizing continues to use the existing formula
  `units = int(resolved_max_capital / (width * 100))`. The only change is that
  `resolved_max_capital` may be derived from a percentage of Net Liq rather than a static
  dollar value. Iron Condor sizing (wider side) behavior is unchanged.

### FR-7 — Dashboard / summary display
- `AutomationDashboard.vue` config summary should indicate the capital mode:
  - `fixed`: show `$<amount>` (unchanged).
  - `percent`: show the percentage (e.g., `60%`) and, when a resolved dollar figure is
    available (post-activation/live), optionally show the resolved dollar amount alongside it.
  Exact presentation is a @ux decision.

### FR-8 — Logging
- Automation logs must record, at activation and at execution, when a percentage cap is used:
  the percentage, the Net Liq value read, and the resolved dollar cap — to make risk decisions
  auditable. On failure, log the reason (e.g., "Net Liq unavailable").

## 6. Acceptance Criteria

- **AC-1:** In the automation config form, a toggle lets the user choose between a fixed dollar
  Max Capital and a percentage-of-Net-Liq Max Capital.
- **AC-2:** In percentage mode, the input accepts values from **1 to 100** and rejects values
  outside that range with a validation message.
- **AC-3:** Existing automations (no capital mode stored) continue to behave exactly as before
  (`fixed` mode, using their stored `max_capital`). No migration breakage.
- **AC-4:** With capital mode = `percent`, on **Auto Trade activation** the system fetches Net
  Liq from the active trade account and displays the resolved dollar cap
  (e.g., Net Liq $10,000 @ 60% → $6,000 shown).
- **AC-5:** With capital mode = `percent`, at **trade execution** the system re-fetches current
  Net Liq, recomputes the dollar cap from live data, and sizes the position from that value.
- **AC-6:** If Net Liq is unavailable or non-positive, activation and/or trade execution
  **fails** with a clear error and log entry — **no fallback** to a default or fixed amount.
- **AC-7:** Position sizing in percentage mode equals
  `int((net_liq * pct/100) / (width * 100))`, matching the existing fixed-mode formula with the
  resolved cap.
- **AC-8:** The dashboard config summary correctly reflects the chosen mode (percentage vs.
  dollar amount).
- **AC-9:** Backend tests cover: percentage resolution math, the no-fallback failure path
  (Net Liq unavailable), and backward compatibility for `fixed` configs.

## 7. Scope Boundaries (explicitly NOT included)

- **No new percentage base other than Net Liq.** Buying power, cash, or portfolio value are
  out of scope for this issue (Net Liq / `account.equity` only).
- **No multi-account aggregation.** Only the currently active trade account is used.
- **No fallback logic** of any kind (no default cap, no "use fixed if percent fails").
- **No change to the sizing formula** itself beyond substituting the resolved cap.
- **No changes to non-automation flows** (manual order entry, strategy-service) — this issue is
  limited to the automation config / auto-trade engine.
- **No change to how the trade account is selected** (uses the existing active trade account).

## 8. Open Design Questions (for @architect / @ux)

- Exact backend field naming/shape for capital mode + percentage (backward-compatible).
- Whether to store percentage as `float64` (e.g., `60`) with validation, and precision allowed.
- UX for the toggle, the input swap, validation messaging, and how/where the resolved dollar
  figure is displayed at activation and on the dashboard.
- How the activation-time resolved value is returned to the frontend (activation response
  payload vs. a separate preview call).
