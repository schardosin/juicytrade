# Implementation Plan — Auto Mode: Percentage-based Max Capital (Issue #73)

**Issue:** [#73 — Auto Mode with percentage for Max Capital](https://github.com/schardosin/juicytrade/issues/73)
**Requirements:** [requirements.md](./requirements.md) · **Architecture:** [architecture.md](./architecture.md) · **UX:** [ux-design.md](./ux-design.md)
**Author:** @dev
**Status:** Draft — ready for implementation
**Scope:** `trade-backend-go` (Go) + `trade-app` (Vue 3). No provider code changes.

---

## How to use this plan

Each **step** below is one small, self-contained, independently-committable unit of
work: a single module/feature change plus its unit tests. Steps are ordered so the
codebase compiles and all tests pass after every step. Later steps depend on the
symbols introduced in earlier ones (dependencies are called out explicitly).

Conventional-commit message suggestions are given per step (project convention:
lowercase, imperative, optional scope in parentheses).

### Verification commands
- Go: `cd trade-backend-go && go build ./... && go test ./...`
- Go (targeted): `go test ./internal/automation/... ./internal/automation/types/...`
- Vue: `cd trade-app && npx vitest run`
- Vue (targeted): `npx vitest run tests/AutomationConfigForm.test.js tests/AutomationDashboard.test.js`

### Verified anchor points (as of this branch)

Backend (`trade-backend-go/`):
- `TradeConfiguration` struct — `internal/automation/types/types.go:150-168`; `MaxCapital float64 \`json:"max_capital"\`` at `types.go:154`.
- `IronCondorSideConfig` — `types.go:143-147`.
- `NewTradeConfiguration()` — `types.go:480-494` (`MaxCapital: 5000` at `types.go:486`).
- `CalculateUnits() int` — `types.go:567-593` (reads `tc.MaxCapital` at `types.go:588`).
- `ActiveAutomation` — `types.go:236-254` (`Status`, `Message` `:250`, `Logs` `:249`, `ErrorCount` `:248`; no capital field).
- `AutomationLog` — `types.go:228-234`; `AddLog(level, message, details...)` — `types.go:530-545`.
- `Engine` struct — `internal/automation/engine.go:19-30`.
- `(*Engine).Start(id)` — `engine.go:179-220`: builds `ActiveAutomation` (`:199-204`), `AddLog("info","Automation started")` (`:205`), `go runAutomation(...)` (`:214`), `notifyUpdate` (`:217`). Holds `e.mu` for the whole method (`:180-181`).
- `(*Engine).runAutomation` — `engine.go:420-438` (loop; first tick immediate `:427`). `runAutomationTick` — `engine.go:441-485` (state dispatch `:471-484`).
- `(*Engine).handleTradingState` — `engine.go:617-734`: 60s ctx (`:619`), `CalculateUnits()` (`:627`), `units==0` insufficient-capital path (`:628-636`), strike-find failure path w/ `ErrorCount++`/`>=3`/`Recurrence==RecurrenceDaily` (`:641-697`), placement error path (`:699-722`), success → `StatusMonitoring` (`:724-733`).
- Delta-drift re-placement `CalculateUnits()` sites — `engine.go:939` (Iron Condor) and `engine.go:982` (credit spread).
- `(*Engine).notifyUpdate` — `engine.go:74-83`; `persistState` — `engine.go:165-176`.
- `restoreFromPersistentState` — `engine.go:101-162` (`RestoreAutomation` + `go runAutomation` `:145`).
- `PersistedAutomation` — `internal/automation/runtime_state.go:24-35`; `Save` — `runtime_state.go:59-104` (populates from `ActiveAutomation` `:73-83`); `Load` — `:107-133`; `isRecoverableStatus` — `:167-176`; `RestoreAutomation` — `:179-212`.
- `Storage.load()` + migrations — `internal/automation/storage.go:60-94` (`migrateIndicatorIDs`, `migrateIndicatorGroups`).
- Config-save handlers — `internal/api/handlers/automation.go`: `CreateConfig` `:81-138`, `UpdateConfig` `:140-177`.
- `models.Account` — `internal/models/models.go:156-181` (`PortfolioValue *float64` `:163`, `Equity *float64` `:164`).
- `ProviderManager.GetAccount(ctx)` — `internal/providers/manager.go:384`.
- `GET /account` route — `cmd/server/main.go:518` (calls `providerManager.GetAccount`).

Frontend (`trade-app/`):
- Max Capital `.form-field` — `src/components/automation/AutomationConfigForm.vue:425-436` (currency `InputNumber`, `:min="100"`).
- `config` ref default `trade_config` (incl. `max_capital: 5000`) — `AutomationConfigForm.vue:909-927`.
- `errors` ref — `AutomationConfigForm.vue:873`; `validateConfig()` — `:1257-1273`; `saveConfig()` — `:1275+` (spreads `config.value`).
- Pill-toggle idiom — `src/components/strategies/DataImportDialog.vue:36-51` (`.selection-mode-toggle` / `.mode-btn` / `:class="{ active: ... }"`); scoped CSS `:1312-1345`.
- Dashboard `status-details` — `src/components/automation/AutomationDashboard.vue:190-205` (State row `:191-194`, Message row `:203-205`).
- `Max Capital` summary-item — `AutomationDashboard.vue:183-185`.
- Status merge: `loadStatuses` spreads `item.details` — `AutomationDashboard.vue:626`; `handleAutomationUpdate` spreads `message.data` — `:574-585`; `getAutomationStatus` — `:645`; `formatNumber` — `:769`.
- API client `getAccountInfo()` (`GET /account`) — `src/services/api.js:450-457`.
- Tests: `trade-app/tests/AutomationConfigForm.test.js`, `trade-app/tests/AutomationDashboard.test.js`; Go: `internal/automation/types/types_test.go`, `internal/automation/storage_test.go`.

> **Note on line numbers:** the architecture doc cites `Start()` at `:179-220` and `handleTradingState` at `:618-734`; current branch has `handleTradingState` at `:617-734`. Treat line numbers as guidance — anchor edits on the named symbols/strings above, which are exact.

---

## Phase A — Backend types & sizing core (`types` package)

### Step 1 — Add `MaxCapitalMode` type, config fields, and constructor default

**Goal:** Introduce the mode enum and the two new `TradeConfiguration` fields; default new configs to fixed. Pure data-model change, no behavior change yet.

**Files:**
- `trade-backend-go/internal/automation/types/types.go`

**Changes:**
1. Add near `TradeStrategy`/`RecurrenceMode` declarations:
   ```go
   // MaxCapitalMode selects how MaxCapital is interpreted for position sizing.
   // "" (empty) and "fixed" are equivalent (backward compatibility).
   type MaxCapitalMode string

   const (
       MaxCapitalModeFixed   MaxCapitalMode = "fixed"
       MaxCapitalModePercent MaxCapitalMode = "percent"
   )
   ```
2. Add two fields to `TradeConfiguration` (after `MaxCapital` at `types.go:154`):
   ```go
   MaxCapitalMode    MaxCapitalMode `json:"max_capital_mode,omitempty"`    // "" ⇒ fixed
   MaxCapitalPercent float64        `json:"max_capital_percent,omitempty"` // 1..100, percent mode
   ```
3. In `NewTradeConfiguration()` (`types.go:481-494`) set `MaxCapitalMode: MaxCapitalModeFixed`.

**Tests** (`internal/automation/types/types_test.go`, add cases):
- `NewTradeConfiguration()` returns `MaxCapitalMode == MaxCapitalModeFixed`.
- JSON round-trip: a `TradeConfiguration` with only `max_capital` (no mode) unmarshals to `MaxCapitalMode == ""` and `MaxCapitalPercent == 0` (legacy compat).
- JSON round-trip of a percent config preserves `max_capital_mode`/`max_capital_percent`; `omitempty` omits them when zero/empty.

**Verify:** `go build ./... && go test ./internal/automation/types/...`
**Commit:** `feat(automation): add MaxCapitalMode + percent fields to trade config`

---

### Step 2 — Add `ResolveEffectiveCapital` with distinguishable sentinel errors

**Goal:** The single mode→dollars resolver (arch §5.1) plus the two sentinel errors needed later by `handleCapitalFailure` (arch §7.2). Depends on Step 1.

**Files:**
- `trade-backend-go/internal/automation/types/types.go` (add `import "math"`, `import "errors"` if not present)

**Changes:**
1. Define package-level sentinel errors:
   ```go
   // ErrInvalidCapitalPercent is a permanent misconfiguration (percent out of 1..100).
   var ErrInvalidCapitalPercent = errors.New("invalid max_capital_percent (must be 1..100)")
   // ErrNetLiqUnavailable is a transient failure (Net Liq unreadable / non-positive).
   var ErrNetLiqUnavailable = errors.New("net liquidating value unavailable or non-positive")
   ```
2. Add the method:
   ```go
   // ResolveEffectiveCapital maps the configured mode to a dollar capital for CalculateUnits.
   //   fixed  : returns MaxCapital (netLiq ignored); never errors.
   //   percent: returns round(netLiq * pct/100); requires 1<=pct<=100 and a usable netLiq.
   func (tc *TradeConfiguration) ResolveEffectiveCapital(netLiq float64, netLiqOK bool) (float64, error) {
       if tc.MaxCapitalMode != MaxCapitalModePercent { // "" or "fixed"
           return tc.MaxCapital, nil
       }
       if tc.MaxCapitalPercent < 1 || tc.MaxCapitalPercent > 100 {
           return 0, ErrInvalidCapitalPercent
       }
       if !netLiqOK || netLiq <= 0 {
           return 0, ErrNetLiqUnavailable
       }
       return math.Round(netLiq * tc.MaxCapitalPercent / 100.0), nil
   }
   ```

**Tests** (`types_test.go`):
- Fixed mode: any `(netLiq, netLiqOK)` returns `MaxCapital`, `err == nil` (incl. `netLiqOK=false`, `netLiq=0`) — guards AC-4.
- **AC-3:** percent, `pct=60`, `netLiq=10000`, `ok=true` → `6000.0`, no error.
- Rounding (Q2): `pct=33.33`, `netLiq=10001` → `math.Round(3333.6333)=3334`; and a `.5` case to pin `math.Round` behavior.
- Invalid pct: `0`, `100.1`, `-5` → `errors.Is(err, ErrInvalidCapitalPercent)`.
- Net Liq unusable: percent + `ok=false`, and percent + `netLiq<=0` → `errors.Is(err, ErrNetLiqUnavailable)`.
- Empty mode (`""`) behaves as fixed.

**Verify:** `go test ./internal/automation/types/...`
**Commit:** `feat(automation): add ResolveEffectiveCapital with sentinel errors`

---

### Step 3 — Refactor `CalculateUnits` to take effective capital as a parameter

**Goal:** Change the sizing seam so it no longer reads `tc.MaxCapital` directly (arch §5.2). This breaks the three call sites' compilation; Step 4 fixes callers. To keep steps compiling, **do the caller updates in the same commit as this signature change** (this step's "unit" is the sizing seam + all its callers).

**Files:**
- `trade-backend-go/internal/automation/types/types.go` — change signature.
- `trade-backend-go/internal/automation/engine.go` — update the three call sites minimally.

**Changes:**
1. `types.go:568` — `func (tc *TradeConfiguration) CalculateUnits(effectiveCapital float64) int`; replace `int(tc.MaxCapital / maxRiskPerUnit)` (`:588`) with `int(effectiveCapital / maxRiskPerUnit)`. Iron Condor wider-side rule and floor math unchanged.
2. `engine.go:627` — `units := active.Config.TradeConfig.CalculateUnits(active.Config.TradeConfig.MaxCapital)` (temporary fixed-mode-equivalent call; replaced by real capture in Step 6).
3. `engine.go:939` and `engine.go:982` — `units := config.CalculateUnits(config.MaxCapital)` (temporary; replaced in Step 7 to reuse the trade-time snapshot).

> This preserves byte-for-byte fixed-mode behavior (passing `MaxCapital` == old behavior) so all existing engine tests keep passing before capture logic lands.

**Tests** (`types_test.go`):
- Update existing `CalculateUnits` tests to pass the capital argument.
- `CalculateUnits(6000)` with `width=5` → `12`; with `width=0` → `0`; `< 1 unit` → `0`.
- **Iron Condor:** `PutSideConfig.Width=50`, `CallSideConfig.Width=20`, `CalculateUnits(eff)` uses `max=50` (unchanged wider-side selection).
- Regression parity: `CalculateUnits(tc.MaxCapital)` equals the pre-refactor result for representative fixed configs (AC-4).

**Verify:** `go build ./... && go test ./internal/automation/... ./internal/automation/types/...`
**Commit:** `refactor(automation): CalculateUnits takes effective capital param`

---

### Step 4 — Add `EffectiveCapital*` snapshot fields to `ActiveAutomation`

**Goal:** Machine-readable snapshot fields for status/persistence (arch §4.2). Pure additive struct change. Depends on Step 1 (`MaxCapitalMode`) only for later population; safe standalone here.

**Files:**
- `trade-backend-go/internal/automation/types/types.go`

**Changes:** Add to `ActiveAutomation` (after `LastTradeDate` at `types.go:253`):
```go
// Effective capital snapshot (percent-mode audit; also populated in fixed mode for uniform display).
EffectiveCapital        float64    `json:"effective_capital,omitempty"`
EffectiveCapitalNetLiq  float64    `json:"effective_capital_net_liq,omitempty"`
EffectiveCapitalPercent float64    `json:"effective_capital_percent,omitempty"`
EffectiveCapitalAt      *time.Time `json:"effective_capital_at,omitempty"`
```

**Tests** (`types_test.go`):
- JSON marshal of an `ActiveAutomation` with the fields set includes `effective_capital*`; zero-valued fields are omitted (`omitempty`); a `nil` `EffectiveCapitalAt` is omitted.

**Verify:** `go test ./internal/automation/types/...`
**Commit:** `feat(automation): add effective-capital snapshot fields to ActiveAutomation`

---

## Phase B — Backend engine (capture, failure, delta-drift)

### Step 5 — Add `readNetLiq` helper

**Goal:** Provider-agnostic Net Liq extraction from `models.Account` (arch §5.3). Standalone, no wiring yet.

**Files:**
- `trade-backend-go/internal/automation/engine.go` (new unexported function; `models` already imported at `engine.go:13`).

**Changes:**
```go
// readNetLiq extracts the account Net Liquidating Value.
// Prefers PortfolioValue, falls back to Equity. Returns (0,false) when neither is usable.
func readNetLiq(acct *models.Account) (float64, bool) {
    if acct == nil {
        return 0, false
    }
    if acct.PortfolioValue != nil && *acct.PortfolioValue > 0 {
        return *acct.PortfolioValue, true
    }
    if acct.Equity != nil && *acct.Equity > 0 {
        return *acct.Equity, true
    }
    return 0, false
}
```

**Tests** (new `internal/automation/engine_capital_test.go`, or existing engine test file):
- `nil` account → `(0,false)`.
- `PortfolioValue=10000` (Equity nil) → `(10000,true)`.
- `PortfolioValue` nil, `Equity=8000` → `(8000,true)`.
- Both nil → `(0,false)`; `PortfolioValue=0` with `Equity=8000` → `(8000,true)` (zero PV falls through); both `<=0` → `(0,false)`.

**Verify:** `go test ./internal/automation/...`
**Commit:** `feat(automation): add readNetLiq account helper`

---

### Step 6 — Add `captureEffectiveCapital` + trade-time capture (FR-6) + `handleCapitalFailure` (FR-8)

**Goal:** Wire the trade-time capture point into `handleTradingState`, populate snapshot fields/log/status, and route capital-resolution failures through a strike-find-mirroring failure path. Depends on Steps 2, 4, 5. (FR-5 monitoring-start capture is Step 8; this step delivers the authoritative trade-time path first.)

**Files:**
- `trade-backend-go/internal/automation/engine.go`

**Changes:**
1. Add the shared capture helper (arch §6.1). Caller holds `e.mu` when mutating `active`:
   ```go
   func (e *Engine) captureEffectiveCapital(ctx context.Context, active *types.ActiveAutomation, phase string) (float64, error) {
       tc := &active.Config.TradeConfig
       var netLiq float64
       var netLiqOK bool
       if tc.MaxCapitalMode == types.MaxCapitalModePercent {
           if acct, err := e.providerManager.GetAccount(ctx); err == nil {
               netLiq, netLiqOK = readNetLiq(acct)
           }
       }
       eff, err := tc.ResolveEffectiveCapital(netLiq, netLiqOK)
       now := time.Now()
       active.EffectiveCapitalAt = &now
       if err != nil {
           active.EffectiveCapitalNetLiq = netLiq
           active.EffectiveCapitalPercent = tc.MaxCapitalPercent
           active.AddLog("error", fmt.Sprintf("[%s] cannot resolve capital: %v", phase, err))
           return 0, err
       }
       active.EffectiveCapital = eff
       if tc.MaxCapitalMode == types.MaxCapitalModePercent {
           active.EffectiveCapitalNetLiq = netLiq
           active.EffectiveCapitalPercent = tc.MaxCapitalPercent
           active.AddLog("info",
               fmt.Sprintf("[%s] capital: $%.0f (%.1f%% of Net Liq $%.0f)", phase, eff, tc.MaxCapitalPercent, netLiq),
               fmt.Sprintf("mode=percent net_liq=%.2f pct=%.2f effective=%.2f", netLiq, tc.MaxCapitalPercent, eff))
       } else {
           active.EffectiveCapitalNetLiq = 0
           active.EffectiveCapitalPercent = 0
           active.AddLog("info", fmt.Sprintf("[%s] capital: $%.0f (fixed)", phase, eff))
       }
       return eff, nil
   }
   ```
   > **Locking:** `captureEffectiveCapital` performs the `GetAccount` network call **without** holding `e.mu`. Because it mutates `active`, callers must serialize mutation. Simplest: call it while **not** holding `e.mu`, then acquire `e.mu` only around the mutation — OR document that the sole caller (`handleTradingState`) is the only writer of these fields on this `active` at this phase. Prefer: read account first, then `e.mu.Lock()` around the field writes + `AddLog`. Keep the network call outside the lock.
2. Add `handleCapitalFailure` (arch §7.2), reusing the exact strike-find branch structure (`ErrorCount++`; `Recurrence == types.RecurrenceDaily && ErrorCount < 3` → `StatusWaiting`+`TradedToday=true`, else `StatusFailed`), and short-circuiting permanent misconfig:
   ```go
   func (e *Engine) handleCapitalFailure(id string, active *types.ActiveAutomation, err error) {
       e.mu.Lock()
       active.AddLog("error", fmt.Sprintf("Cannot size position: %v", err))
       if errors.Is(err, types.ErrInvalidCapitalPercent) {
           active.Status = types.StatusFailed
           active.Message = "Invalid Max Capital percentage configuration"
           e.mu.Unlock()
           e.notifyUpdate(id, active)
           return
       }
       // transient (Net Liq unavailable): mirror strike-find failure handling
       active.ErrorCount++
       if active.ErrorCount >= 3 {
           if active.Config.Recurrence == types.RecurrenceDaily {
               active.TradedToday = true
               active.Status = types.StatusWaiting
               active.Message = "Net Liq unavailable - waiting for next trading day"
           } else {
               active.Status = types.StatusFailed
               active.Message = "Net Liq unavailable - cannot size position"
           }
       } else {
           active.Message = "Net Liq unavailable - will retry"
       }
       e.mu.Unlock()
       e.notifyUpdate(id, active)
   }
   ```
   > Match the existing threshold/branch shape at `engine.go:641-697` verbatim so FR-8 "mirrors existing handling". Add `import "errors"` to `engine.go`.
3. In `handleTradingState`, replace the `CalculateUnits()` call at `engine.go:627` (the temporary from Step 3) with:
   ```go
   eff, capErr := e.captureEffectiveCapital(ctx, active, "trade-time")
   if capErr != nil {
       e.handleCapitalFailure(id, active, capErr) // FR-8
       return
   }
   e.mu.Lock()
   active.Message = fmt.Sprintf("Trading - capital $%.0f", eff)
   e.mu.Unlock()
   units := active.Config.TradeConfig.CalculateUnits(eff)
   if units == 0 { /* existing insufficient-capital path at :628-636, unchanged */ }
   ```
   Keep the existing `units==0` → `StatusFailed` block exactly as-is.

**Tests** (`engine_capital_test.go`; use `httptest.NewServer` mock account per existing engine test patterns):
- Fixed mode: `captureEffectiveCapital(..., "trade-time")` sets `EffectiveCapital == MaxCapital`, `EffectiveCapitalNetLiq==0`, logs `(fixed)`; never errors even if account read would fail (percent-only reads account).
- Percent mode success: account returns `PortfolioValue=10000`, `pct=60` → `eff=6000`; snapshot fields set (`net_liq=10000`, `pct=60`); info log with `mode=percent` details.
- Percent mode, account error/zero → returns `ErrNetLiqUnavailable`; snapshot records net_liq/pct; error log.
- `handleCapitalFailure` with `ErrNetLiqUnavailable`: once-recurrence + `ErrorCount` reaching 3 → `StatusFailed`; daily-recurrence + `ErrorCount>=3` → `StatusWaiting`+`TradedToday`; `ErrorCount<3` → stays, "will retry".
- `handleCapitalFailure` with `ErrInvalidCapitalPercent` → immediate `StatusFailed`, no `ErrorCount` dependence.

**Verify:** `go build ./... && go test ./internal/automation/...`
**Commit:** `feat(automation): capture effective capital at trade time with fail-safe (FR-6/FR-8)`

---

### Step 7 — Reuse trade-time snapshot at delta-drift re-placement sites

**Goal:** Keep sizing stable across re-placements within the same trade; percent mode must **not** re-hit the account here (arch §5.2/§6.3). Depends on Step 6.

**Files:**
- `trade-backend-go/internal/automation/engine.go`

**Changes:**
- `engine.go:939` (Iron Condor drift) and `engine.go:982` (credit-spread drift): replace the Step-3 temporary `config.CalculateUnits(config.MaxCapital)` with `active.Config.TradeConfig.CalculateUnits(active.EffectiveCapital)` (the trade-time snapshot). Leave the `units==0 → StatusFailed` blocks unchanged.

> Rationale: fixed mode still equals `MaxCapital` (snapshot was set from it); percent mode reuses the trade-time dollars without a fresh account read.

**Tests** (`engine_capital_test.go`):
- After a trade-time capture sets `active.EffectiveCapital=6000`, a delta-drift re-placement computes `units` from `6000` (width `5` → `12`), not from `MaxCapital`.
- Fixed-mode drift path uses `MaxCapital`-derived units unchanged (AC-4 regression).

**Verify:** `go test ./internal/automation/...`
**Commit:** `fix(automation): reuse trade-time capital snapshot on delta-drift re-placement`

---

### Step 8 — Monitoring-start capture (FR-5)

**Goal:** Capture/display the effective capital when the automation starts monitoring, without failing on transient Net Liq unavailability (arch §6.2). Depends on Step 6.

**Files:**
- `trade-backend-go/internal/automation/engine.go`

**Changes:**
- Perform the FR-5 capture as the **first action inside the launched `runAutomation` goroutine** (not inside `Start`, which holds `e.mu` for its whole body — see anchor notes). Add a one-shot capture at the top of `runAutomation` (`engine.go:420`), before the first `runAutomationTick`:
  ```go
  func (e *Engine) runAutomation(id string, stopChan chan struct{}) {
      // FR-5: monitoring-start capital capture (does not hold e.mu across the network call).
      e.captureMonitoringStartCapital(id)
      // ... existing ticker setup and loop ...
  }
  ```
- Add `captureMonitoringStartCapital(id string)`:
  - Look up `active` under `e.mu.RLock()`; if absent, return.
  - Create a bounded `context.WithTimeout` (e.g. 30s).
  - Call `captureEffectiveCapital(ctx, active, "monitoring-start")` **without** holding `e.mu` during `GetAccount` (use the same read-then-lock discipline as Step 6).
  - On success: set `active.Message = fmt.Sprintf("Monitoring - capital $%.0f", eff)`.
  - On error (percent, Net Liq unavailable): **do not fail**. Log a `warn`, set `active.Message = "Monitoring - capital pending (Net Liq unavailable)"`. (Invalid-percent config here is still non-fatal at monitoring start; the hard failure is reserved for trade time per arch §6.2 — log warn and continue.)
  - Call `e.notifyUpdate(id, active)`.

> Applies on both fresh `Start` and `restoreFromPersistentState` (both launch `runAutomation`), giving restored percent automations a fresh monitoring-start snapshot.

**Tests** (`engine_capital_test.go`):
- Percent mode, account available: `captureMonitoringStartCapital` sets snapshot + info log + `Message` containing the dollar figure; status **unchanged** (not failed).
- Percent mode, Net Liq unavailable: warn log; `Message` = capital pending; status unchanged (no fail) — satisfies FR-5's non-fatal semantics.
- Fixed mode: sets `EffectiveCapital == MaxCapital`, `(fixed)` log, no account read.

**Verify:** `go test ./internal/automation/...`
**Commit:** `feat(automation): capture effective capital at monitoring start (FR-5)`

---

## Phase C — Backend persistence & validation

### Step 9 — Persist & restore the effective-capital snapshot

**Goal:** Snapshot survives restart for display continuity; sizing is still re-resolved at the next trade time (arch §8.3). Depends on Step 4.

**Files:**
- `trade-backend-go/internal/automation/runtime_state.go`

**Changes:**
1. Add to `PersistedAutomation` (`runtime_state.go:24-35`):
   ```go
   EffectiveCapital        float64    `json:"effective_capital,omitempty"`
   EffectiveCapitalNetLiq  float64    `json:"effective_capital_net_liq,omitempty"`
   EffectiveCapitalPercent float64    `json:"effective_capital_percent,omitempty"`
   EffectiveCapitalAt      *time.Time `json:"effective_capital_at,omitempty"`
   ```
2. Populate in `Save` (`runtime_state.go:73-83`) from `active.*`.
3. Copy back in `RestoreAutomation` (`runtime_state.go:180-191`) into the rebuilt `ActiveAutomation`.

**Tests** (`internal/automation/storage_test.go` or a new `runtime_state_test.go`):
- Round-trip: `Save` an active with snapshot fields → `Load` → fields preserved.
- `RestoreAutomation` copies the four fields onto the restored `ActiveAutomation`.
- Legacy persisted JSON without the fields restores with zero-valued snapshot (no panic).

**Verify:** `go test ./internal/automation/...`
**Commit:** `feat(automation): persist and restore effective-capital snapshot`

---

### Step 10 — Server-side config validation (percent 1..100, fixed >=100)

**Goal:** Reject bad values with HTTP 400 before persistence (arch §10.1). Depends on Step 1.

**Files:**
- `trade-backend-go/internal/api/handlers/automation.go`

**Changes:**
- Add a small `validateTradeCapital(tc types.TradeConfiguration) error` helper (or inline validation) invoked in both `CreateConfig` (`:81-138`, after strategy defaulting `:121-123`) and `UpdateConfig` (`:140-177`, after bind `:153-160`):
  - percent mode: require `1 <= MaxCapitalPercent <= 100`, else 400 `{"success":false,"message":"max_capital_percent must be between 1 and 100"}`.
  - fixed mode (incl. empty mode): require `MaxCapital >= 100`, else 400.
- Return the standard error envelope used elsewhere in this file.

**Tests** (`internal/api/handlers/*_test.go` if present, else a focused handler test using `httptest`/`gin` test context; follow existing handler test style — if none exists, keep validation logic in a pure helper `validateTradeCapital` and unit-test the helper directly):
- percent + `pct=60` → nil.
- percent + `pct=0` / `pct=150` → error.
- fixed + `MaxCapital=5000` → nil; fixed + `MaxCapital=50` → error.
- empty mode + `MaxCapital=5000` → nil (treated as fixed).

**Verify:** `go build ./... && go test ./internal/api/...`
**Commit:** `feat(automation): validate max capital mode/percent on config save`

---

### Step 11 — (Optional) Normalize empty `MaxCapitalMode` on load

**Goal:** Optional cosmetic migration setting `MaxCapitalMode="fixed"` when empty (arch §8.2). Must not change behavior. Depends on Step 1.

**Files:**
- `trade-backend-go/internal/automation/storage.go`

**Changes:**
- Add `migrateMaxCapitalMode()` mirroring `migrateIndicatorIDs`/`migrateIndicatorGroups`: for each config, if `TradeConfig.MaxCapitalMode == ""`, set it to `types.MaxCapitalModeFixed` and mark migrated. Wire into `load()` (`storage.go:79-91`) alongside the existing migrations.

**Tests** (`storage_test.go`):
- Loading a config with empty mode normalizes to `"fixed"` and triggers a save; existing fixed behavior unchanged (AC-7).
- A config already `"percent"` is left untouched.

**Verify:** `go test ./internal/automation/...`
**Commit:** `chore(automation): normalize empty max_capital_mode to fixed on load`

> Skip this step if the team prefers to rely purely on the empty-string-means-fixed rule; it is explicitly optional in the architecture.

---

## Phase D — Frontend config form (`AutomationConfigForm.vue`)

### Step 12 — Config defaults + mode toggle + conditional inputs + hint

**Goal:** Add the Fixed/Percent pill toggle inside the Max Capital `.form-field`, swap inputs by mode, preserve both values across toggles, and show a resolved-dollars hint when Net Liq is known (UX §1). Depends on backend field names from Steps 1–2 (contract only; frontend can proceed independently).

**Files:**
- `trade-app/src/components/automation/AutomationConfigForm.vue`

**Changes:**
1. **Defaults** (`AutomationConfigForm.vue:909-927`): add to `trade_config` `max_capital_mode: 'fixed'` and `max_capital_percent: 60` (alongside `max_capital: 5000`).
2. **Markup** — replace the Max Capital `.form-field` (`:425-436`) per UX §1.3:
   - Pill toggle (`.capital-mode-toggle` / `.mode-btn`, `type="button"`, `:class="{ active: ... }"`, matching `DataImportDialog.vue` idiom) with `@click="setCapitalMode('fixed')"` / `setCapitalMode('percent')`.
   - `InputNumber` currency (`v-if` mode !== 'percent', unchanged: `mode="currency" currency="USD" :min="100"`).
   - `InputNumber` percent (`v-else`: `v-model="config.trade_config.max_capital_percent"`, `suffix="%"`, `:min="1"`, `:max="100"`, `:maxFractionDigits="0"`).
   - `<small class="field-hint">{{ maxCapitalHint }}</small>`.
3. **Script:**
   - `const setCapitalMode = (mode) => { config.value.trade_config.max_capital_mode = mode }` — must NOT touch `max_capital` or `max_capital_percent` (FR-2).
   - Add `netLiq` ref (Step 13 fills it; default `0`).
   - `maxCapitalHint` computed per UX §1.4: fixed → "Maximum capital to risk on this trade"; percent + `netLiq>0` → `"${pct}% of account Net Liq. ≈ $X of $Y"` (using `Math.round(netLiq*pct/100)`, `toLocaleString()`); percent + no Net Liq → `"${pct}% of account Net Liquidating Value"`.
   - Add `setCapitalMode` + `maxCapitalHint` (+ `netLiq`) to the `setup()` return.
4. **Scoped CSS** — add `.capital-mode-toggle` + `.mode-btn` + `.mode-btn.active` per UX §3.1 (theme tokens `--bg-tertiary`, `--border-primary`, `--radius-md/sm`, `--color-brand`, spacing/font tokens).

**Tests** (`trade-app/tests/AutomationConfigForm.test.js`, Vitest + @vue/test-utils):
- Default mode is fixed; currency input rendered, percent input not.
- Clicking "% of Net Liq." sets `max_capital_mode='percent'`, shows percent input (`suffix="%"`, min 1 max 100).
- Toggling Fixed→Percent→Fixed preserves `max_capital` (5000) and `max_capital_percent` (60) — FR-2.
- `maxCapitalHint`: fixed text; percent fallback text when `netLiq=0`; percent "≈ $6,000 of $10,000" when `netLiq=10000`, `pct=60`.

**Verify:** `cd trade-app && npx vitest run tests/AutomationConfigForm.test.js`
**Commit:** `feat(trade-app): add Max Capital fixed/percent toggle to automation form`

---

### Step 13 — Net Liq preview fetch + percent validation

**Goal:** Populate `netLiq` for the hint (best-effort, non-blocking) and add client-side percent validation (UX §1.5/§1.6). Depends on Step 12.

**Files:**
- `trade-app/src/components/automation/AutomationConfigForm.vue`

**Changes:**
1. **Net Liq fetch:** on mount, call `api.getAccountInfo()` (`src/services/api.js:450`) once; set `netLiq.value` from `portfolio_value` (fallback `equity`). Wrap in try/catch; on failure leave `netLiq=0` so the hint degrades gracefully (never block, never add a new endpoint — UX §1.5).
2. **Validation** in `validateConfig()` (`:1257-1273`): if `max_capital_mode === 'percent'`, require `1 <= max_capital_percent <= 100`, else set `errors.value.max_capital_percent` and return false. Render `<small v-if="errors.max_capital_percent" class="p-error">` under the percent input (mirror `errors.name` at `:51`).
3. Surface backend 400 message via existing `saveConfig` error handling (no new plumbing).

**Tests** (`AutomationConfigForm.test.js`):
- Mock `api.getAccountInfo` resolving `{portfolio_value: 10000}` → `netLiq` becomes 10000 and hint shows dollars.
- Mock rejection → `netLiq` stays 0, plain fallback hint, save still allowed.
- Percent mode with `max_capital_percent = 0` (or 150) → `validateConfig()` false, `errors.max_capital_percent` set, save blocked.
- Percent mode with `60` → validates true.

**Verify:** `npx vitest run tests/AutomationConfigForm.test.js`
**Commit:** `feat(trade-app): net-liq preview hint and percent validation for max capital`

---

## Phase E — Frontend dashboard (`AutomationDashboard.vue`)

### Step 14 — "Capital Used" status row + `formatCapitalUsed` helper

**Goal:** Render the resolved capital in `status-details` from `effective_capital*` (UX §2.1). Depends on backend Steps 4/6/8 for the fields at runtime; frontend can be built/tested against mocked status.

**Files:**
- `trade-app/src/components/automation/AutomationDashboard.vue`

**Changes:**
1. Add a `.status-row` **after the State row** (`:191-194`) and **before the Message row** (`:203-205`):
   ```html
   <div v-if="getAutomationStatus(config.id)?.effective_capital" class="status-row">
     <span class="status-label">Capital Used:</span>
     <span class="status-value">{{ formatCapitalUsed(getAutomationStatus(config.id)) }}</span>
   </div>
   ```
2. Add `formatCapitalUsed(status)` (UX §2.1) to `setup()` return, reusing `formatNumber` (`:769`): percent mode (`pct>0 && net_liq>0`) → `"$6,000 (60% of $10,000)"`; fixed → `"$5,000"`; null eff → `"N/A"`. No new CSS (reuse `.status-row`/`.status-label`/`.status-value`).

> No store changes needed: `loadStatuses` spreads `item.details` (`:626`) and `handleAutomationUpdate` spreads `message.data` (`:574-585`), so `effective_capital*` arrive automatically.

**Tests** (`trade-app/tests/AutomationDashboard.test.js`):
- Status with `effective_capital=6000, effective_capital_percent=60, effective_capital_net_liq=10000` → renders "Capital Used: $6,000 (60% of $10,000)".
- Fixed status `effective_capital=5000` (pct/net_liq 0) → "Capital Used: $5,000".
- No `effective_capital` → row hidden (`v-if`), Message row still renders.

**Verify:** `npx vitest run tests/AutomationDashboard.test.js`
**Commit:** `feat(trade-app): show resolved Capital Used on automation dashboard`

---

### Step 15 — Mode-aware "Max Capital" summary line

**Goal:** Make the static config summary reflect mode (UX §2.3). Depends on nothing beyond the config fields.

**Files:**
- `trade-app/src/components/automation/AutomationDashboard.vue`

**Changes:** Update the Max Capital `summary-item` value (`:183-185`) to show `"{pct}% of Net Liq."` when `trade_config.max_capital_mode === 'percent'`, else `"${{ formatNumber(max_capital) }}"` (UX §2.3 markup).

**Tests** (`AutomationDashboard.test.js`):
- Config with `max_capital_mode='percent', max_capital_percent=60` → summary shows "60% of Net Liq.".
- Config fixed / no mode → summary shows "$5,000" (AC-7 default-to-fixed display).

**Verify:** `npx vitest run tests/AutomationDashboard.test.js`
**Commit:** `feat(trade-app): make Max Capital summary mode-aware on dashboard`

---

## Phase F — Full-suite verification

### Step 16 — Run and green the full test suites

**Goal:** Confirm AC-9 (all existing + new tests pass) end-to-end.

**Actions:**
- `cd trade-backend-go && go build ./... && go test ./...`
- `cd trade-app && npx vitest run`
- Fix any incidental fallout (e.g. existing engine tests that constructed `TradeConfiguration` and now need a mode, or existing dashboard/form snapshot expectations).

**Commit:** `test(automation): finalize issue-73 test coverage` (only if fixes were needed)

---

## Acceptance-criteria → step traceability

| AC | Covered by |
|---|---|
| AC-1 (toggle, fixed default) | Step 12 |
| AC-2 (percent 1–100 persisted) | Steps 12, 13 (client) + 10 (server) + existing save plumbing |
| AC-3 ($10k × 60% = $6k) | Step 2 (`ResolveEffectiveCapital`) + Step 6 (trade-time) |
| AC-4 (fixed unchanged) | Steps 2, 3, 6, 7 (fixed path returns `MaxCapital`, never errors) |
| AC-5 (monitoring-start display) | Step 8 + Step 14 |
| AC-6 (trade-time display + log) | Step 6 + Step 14 |
| AC-7 (legacy configs load/run as fixed) | Steps 1, 9, 11 (+ Step 15 display default) |
| AC-8 (Net Liq unavailable → no mis-sized order, error, existing failure path) | Step 6 (`handleCapitalFailure`) |
| AC-9 (all tests pass; new coverage) | Steps 1–15 tests + Step 16 |

## Explicit non-goals (do NOT change)
- Provider code, `ProviderManager.GetAccount`, `GET /account` handler.
- Iron Condor width-selection logic; the units floor math.
- Existing fixed-mode currency input behavior (byte-for-byte parity).
- No new backend endpoints for the form's Net-Liq preview; degrade gracefully.
- Do not clear `max_capital` / `max_capital_percent` when toggling modes.

---

*Prepared by @dev. Ready to implement step-by-step; each step is independently committable and keeps the build green.*
