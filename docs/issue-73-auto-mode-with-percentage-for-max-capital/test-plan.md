# QA Test Plan — Auto Mode: Percentage-based Max Capital (Issue #73)

**Issue:** [#73 — Auto Mode with percentage for Max Capital](https://github.com/schardosin/juicytrade/issues/73)
**Requirements:** [requirements.md](./requirements.md) (AC-1 .. AC-9)
**Design:** [architecture.md](./architecture.md)
**Branch:** `fleet/issue-73-auto-mode-with-percentage-for-max-capital`
**Latest commit at analysis:** `bad6e88 feat(trade-app): make Max Capital summary mode-aware on dashboard`
**Author:** @qa — read-only analysis, no test code written yet.

---

## 0. Analysis Summary

The feature is **fully implemented** and already ships with a substantial test
suite. All existing tests pass at analysis time:

- Backend: `go test ./internal/automation/... ./internal/api/handlers/...` → all `ok`.
- Frontend: `npx vitest run tests/AutomationConfigForm.test.js tests/AutomationDashboard.test.js`
  → **86 tests passed**.

### Files under test (implementation)

| Layer | File | Key symbols |
|---|---|---|
| Types | `trade-backend-go/internal/automation/types/types.go` | `MaxCapitalMode`, `MaxCapitalModeFixed/Percent`, `TradeConfiguration.MaxCapitalMode/MaxCapitalPercent`, `ResolveEffectiveCapital` (`:596`), `CalculateUnits(eff)` (`:610`), `ErrInvalidCapitalPercent`/`ErrNetLiqUnavailable` (`:587`/`:590`), `ActiveAutomation.EffectiveCapital*` (`:268-271`), `NewTradeConfiguration` (`:499`, sets `MaxCapitalModeFixed`) |
| Engine | `trade-backend-go/internal/automation/engine.go` | `readAccount` (`:91`, test seam `accountReader`), `readNetLiq` (`:100`), `captureEffectiveCapital` (`:120`), `handleCapitalFailure` (`:165`), `captureMonitoringStartCapital` (`:559`), `handleTradingState` sizing (`:775-796`), delta-drift re-placement (`:1101`, `:1144`) |
| Persistence | `trade-backend-go/internal/automation/runtime_state.go` | `PersistedAutomation.EffectiveCapital*` (`:36-39`), `Save` (`:90-93`), `RestoreAutomation` (`:204-207`) |
| Migration | `trade-backend-go/internal/automation/storage.go` | `migrateMaxCapitalMode` (empty → fixed) |
| Handler | `trade-backend-go/internal/api/handlers/automation.go` | `validateTradeCapital` (`:17`), called in `CreateConfig` (`:143`) & `UpdateConfig` (`:190`) → HTTP 400 |
| Frontend | `trade-app/src/components/automation/AutomationConfigForm.vue`, `AutomationDashboard.vue` | mode toggle, percent input, `maxCapitalHint`, Net-Liq preview, `formatCapitalUsed`, mode-aware summary |

### Existing test files

| File | Scope |
|---|---|
| `types/types_test.go` | `ResolveEffectiveCapital`, `CalculateUnits`, constructor default, JSON round-trip, snapshot-field marshaling |
| `engine_capital_test.go` | `readNetLiq`, `captureEffectiveCapital` (fixed/percent/failure), `handleCapitalFailure` (permanent vs transient, retry/wait/fail), delta-drift snapshot reuse, `captureMonitoringStartCapital` |
| `runtime_state_test.go` | snapshot Save/Load, `RestoreAutomation` copy, legacy state without snapshot |
| `storage_test.go` | `migrateMaxCapitalMode` (empty→fixed, percent/explicit-fixed untouched) |
| `handlers/automation_capital_test.go` | `validateTradeCapital` (percent 1..100, fixed >=100, empty-defaults-fixed) |
| `trade-app/tests/AutomationConfigForm.test.js` | mode toggle/default, value preservation, hint, Net-Liq preview, percent validation |
| `trade-app/tests/AutomationDashboard.test.js` | `formatCapitalUsed`, Capital-Used row rendering, mode-aware Max Capital summary |

---

## 1. Acceptance-Criteria Coverage Matrix

Legend: **[EXISTS]** already covered by a passing test · **[GAP]** not covered, new test recommended · **[PARTIAL]** partially covered.

### AC-1 — Mode toggle in config form; Fixed is default

| # | Test case | Status | Location |
|---|---|---|---|
| 1.1 | Form defaults `max_capital_mode='fixed'`, currency input rendered, percent input absent | [EXISTS] | `AutomationConfigForm.test.js` "defaults to fixed mode…" (:241) |
| 1.2 | Clicking percent (`setCapitalMode('percent')`) reveals `#maxCapitalPercent` input | [EXISTS] | same file (:249) |
| 1.3 | `NewTradeConfiguration()` sets `MaxCapitalMode = fixed` (backend default) | [EXISTS] | `types_test.go` `TestNewTradeConfiguration_DefaultsToFixedMode` (:134) |
| 1.4 | Toggle Fixed→Percent→Fixed preserves both numeric values (FR-2) | [EXISTS] | `AutomationConfigForm.test.js` (:257, :270) |

### AC-2 — Percent value 1–100 entered and persisted

| # | Test case | Status | Location |
|---|---|---|---|
| 2.1 | Percent JSON round-trips through marshal/unmarshal (`mode=percent`, `percent=60`) | [EXISTS] | `types_test.go` `TestTradeConfiguration_PercentJSONRoundTrip` (:159) |
| 2.2 | `omitempty` omits zero-value mode/percent for fixed configs | [EXISTS] | `types_test.go` `TestTradeConfiguration_OmitemptyOmitsZeroValues` (:189) |
| 2.3 | Frontend percent validation blocks 0 and 150, passes 60 | [EXISTS] | `AutomationConfigForm.test.js` (:355, :362, :369) |
| 2.4 | **Config save round-trip through `storage.go` persists `max_capital_mode`/`max_capital_percent` to `automations.json` and re-loads them** | [GAP] | New `storage_test.go` case: `Create`/`Update` a percent config, reload store, assert fields survive |
| 2.5 | **HTTP `CreateConfig`/`UpdateConfig` accepts a valid percent config (200) and persisted body includes percent fields** | [GAP] | New `handlers/automation_*_test.go` using `httptest` + gin router |

### AC-3 — Net Liq $10,000 × 60% ⇒ $6,000 effective

| # | Test case | Status | Location |
|---|---|---|---|
| 3.1 | `ResolveEffectiveCapital(10000, true)` with pct 60 ⇒ `6000` | [EXISTS] | `types_test.go` `TestResolveEffectiveCapital_PercentAC3` (:247) |
| 3.2 | `captureEffectiveCapital` percent path: account `PortfolioValue=10000`, pct 60 ⇒ `eff=6000`, snapshot fields set | [EXISTS] | `engine_capital_test.go` `TestCaptureEffectiveCapital_PercentSuccess` (:146) |
| 3.3 | `CalculateUnits(6000)` with width 5 ⇒ 12 units | [EXISTS] | `types_test.go` `TestCalculateUnits_BasicFloor` (:333); `engine_capital_test.go` (:307) |
| 3.4 | Rounding: `10001 × 33.33%` ⇒ `round(3333.33)=3333` (Q2) | [EXISTS] | `types_test.go` `TestResolveEffectiveCapital_Rounding` (:259) |

### AC-4 — Fixed mode byte-for-byte identical (no regression)

| # | Test case | Status | Location |
|---|---|---|---|
| 4.1 | `ResolveEffectiveCapital` fixed mode returns `MaxCapital`, never errors (incl. `netLiqOK=false`) | [EXISTS] | `types_test.go` `TestResolveEffectiveCapital_FixedMode` (:211) |
| 4.2 | Empty mode (`""`) behaves as fixed | [EXISTS] | `types_test.go` `TestResolveEffectiveCapital_EmptyModeBehavesAsFixed` (:236) |
| 4.3 | `CalculateUnits(MaxCapital)` reproduces pre-refactor floor math across widths | [EXISTS] | `types_test.go` `TestCalculateUnits_FixedModeRegressionParity` (:353) |
| 4.4 | Fixed mode does **not** read the account (`accountReader` not invoked) | [EXISTS] | `engine_capital_test.go` `TestCaptureEffectiveCapital_FixedMode` (:110), `..._FixedMode` monitoring (:386) |
| 4.5 | Fixed-mode delta-drift re-placement sizes identically (snapshot == MaxCapital) | [EXISTS] | `engine_capital_test.go` `TestCalculateUnits_DriftFixedModeParity` (:319) |

### AC-5 — Monitoring-start shows effective capital

| # | Test case | Status | Location |
|---|---|---|---|
| 5.1 | Percent monitoring-start: snapshot set, message includes `$6000`, status unchanged | [EXISTS] | `engine_capital_test.go` `TestCaptureMonitoringStartCapital_PercentSuccess` (:339) |
| 5.2 | Fixed monitoring-start: eff=5000, message includes `5000`, no account read | [EXISTS] | `engine_capital_test.go` `TestCaptureMonitoringStartCapital_FixedMode` (:386) |
| 5.3 | Missing automation id → no-op / no panic | [EXISTS] | `engine_capital_test.go` `TestCaptureMonitoringStartCapital_MissingAutomationNoOp` (:409) |
| 5.4 | Frontend renders `Capital Used: $6,000 (60% of $10,000)` row from status | [EXISTS] | `AutomationDashboard.test.js` (:911) |

### AC-6 — Trade-time effective capital computed from Net Liq read then; displayed + logged

| # | Test case | Status | Location |
|---|---|---|---|
| 6.1 | Trade-time percent capture: `eff=6000`, snapshot + info log w/ details `mode=percent net_liq=… pct=… effective=…` | [EXISTS] | `engine_capital_test.go` `TestCaptureEffectiveCapital_PercentSuccess` (:146, phase `trade-time`) |
| 6.2 | Fixed trade-time log message contains `(fixed)` and `[trade-time]` | [EXISTS] | `engine_capital_test.go` `TestCaptureEffectiveCapital_FixedMode` (:140-143) |
| 6.3 | **Full `handleTradingState` path: percent config sizes from trade-time capture, sets `Message="Trading - capital $6,000"`, then proceeds to strike-find** | [GAP] | New engine test driving `handleTradingState` with mock provider (strikes + account); assert `active.EffectiveCapital`, `Message`, units passed to order placement |
| 6.4 | Frontend `formatCapitalUsed` percent formatting `$6,000 (60% of $10,000)` | [EXISTS] | `AutomationDashboard.test.js` `formatCapitalUsed` (:885) |

### AC-7 — Legacy configs load and run as fixed

| # | Test case | Status | Location |
|---|---|---|---|
| 7.1 | Legacy JSON (no mode field) unmarshals to `MaxCapitalMode=""`, `MaxCapitalPercent=0` | [EXISTS] | `types_test.go` `TestTradeConfiguration_LegacyJSONUnmarshalsToEmptyMode` (:141) |
| 7.2 | `migrateMaxCapitalMode` normalizes empty → `fixed`; leaves percent/explicit-fixed untouched | [EXISTS] | `storage_test.go` (:238, :261, :283) |
| 7.3 | Legacy runtime state (no snapshot) restores with zero snapshot, no error | [EXISTS] | `runtime_state_test.go` `TestRestoreAutomation_LegacyStateWithoutSnapshot` (:86), `..._LoadFromLegacyFileNoSnapshot` (:112) |
| 7.4 | Frontend dashboard shows `$5,000` for legacy config (no mode) — no `of Net Liq.` | [EXISTS] | `AutomationDashboard.test.js` (:969) |
| 7.5 | **End-to-end: legacy `automations.json` fixture loads via `storage.load()` and a fixed automation sizes with no account read (behavioral parity)** | [PARTIAL] | Covered piecemeal (7.1–7.3 + 4.x). A single fixture-driven load test would strengthen it — optional [GAP] |

### AC-8 — Percent + Net Liq unavailable at trade time → no mis-sized order, clear error, existing failure path

| # | Test case | Status | Location |
|---|---|---|---|
| 8.1 | Provider error → `netLiqOK=false` → `ErrNetLiqUnavailable`, error log, snapshot pct recorded, eff=0 | [EXISTS] | `engine_capital_test.go` `TestCaptureEffectiveCapital_PercentNetLiqUnavailable` (:179) |
| 8.2 | Account returns but Net Liq zero/unusable → `ErrNetLiqUnavailable` | [EXISTS] | `engine_capital_test.go` `TestCaptureEffectiveCapital_PercentZeroNetLiq` (:206) |
| 8.3 | Invalid pct is **permanent** → `StatusFailed`, ErrorCount not incremented | [EXISTS] | `engine_capital_test.go` `TestHandleCapitalFailure_InvalidPercentIsPermanent` (:221) |
| 8.4 | Transient below threshold (<3) → retry (message set, no terminal/waiting transition) | [EXISTS] | `engine_capital_test.go` `TestHandleCapitalFailure_TransientBelowThresholdRetries` (:237) |
| 8.5 | Transient daily @ threshold (3) → `StatusWaiting` + `TradedToday=true` | [EXISTS] | `engine_capital_test.go` `TestHandleCapitalFailure_TransientDailyReachesThresholdWaits` (:253) |
| 8.6 | Transient once @ threshold (3) → `StatusFailed` | [EXISTS] | `engine_capital_test.go` `TestHandleCapitalFailure_TransientOnceReachesThresholdFails` (:271) |
| 8.7 | **`handleTradingState` end-to-end: percent + failing account reader → `handleCapitalFailure` invoked and NO order placed** | [GAP] | New engine test: assert no `placeOrder`/`CurrentOrder` set and status/error follow the failure path |

### AC-9 — All existing tests pass; new tests cover percent resolution, $10k×60%, defaulting, Net-Liq-unavailable

| # | Test case | Status | Location |
|---|---|---|---|
| 9.1 | Backend suite green (`go test ./internal/automation/... ./internal/api/handlers/...`) | [EXISTS] | Verified `ok` at analysis time |
| 9.2 | Frontend suite green (`npx vitest run`) | [EXISTS] | Verified 86 passed |
| 9.3 | Percent resolution / $10k×60% / defaulting / Net-Liq-unavailable covered | [EXISTS] | AC-3, AC-4, AC-8 rows above |
| 9.4 | **Full frontend regression (`npx vitest run`, whole suite) to confirm no cross-component regression** | [GAP-VERIFY] | Run once as a QA gate (no new code) |

---

## 2. PO Focus-Area Coverage

### 2.1 Backward compatibility

| # | Test case | Status |
|---|---|---|
| BC-1 | Empty `max_capital_mode` treated as fixed everywhere (resolve, migrate, validate, UI) | [EXISTS] AC-4.2, AC-7.1/7.2/7.4 |
| BC-2 | Legacy runtime state restores cleanly | [EXISTS] AC-7.3 |
| BC-3 | Additive JSON: existing consumers ignore new `effective_capital*` fields (omitempty) | [EXISTS] `types_test.go` `TestActiveAutomation_EffectiveCapitalOmitemptyOmitsZeroValues` (:425) |
| BC-4 | Fixed sizing numerically unchanged | [EXISTS] AC-4.3 |

### 2.2 Sizing math (Iron Condor wider-side + floor)

| # | Test case | Status |
|---|---|---|
| SM-1 | `CalculateUnits` floors via integer truncation (exact, truncated, below-one, zero, zero-width) | [EXISTS] `TestCalculateUnits_BasicFloor` (:325) |
| SM-2 | Iron Condor uses `max(putWidth, callWidth)`; plain `Width` ignored; symmetric | [EXISTS] `TestCalculateUnits_IronCondorUsesWiderSide` (:377) |
| SM-3 | **Iron Condor + percent-mode floor edge: e.g. wider side 50 & Net-Liq-derived eff that lands just below/above a unit boundary (units floor + wider-side interaction together)** | [PARTIAL/GAP] SM-2 uses eff=6000 fixed value → 1 unit; recommend an explicit percent-derived IC case (e.g. NetLiq 12000 × 50% = 6000, wider 50 ⇒ 1; and 12000×50% with wider 20 ⇒ 6) to jointly exercise resolve→IC-width→floor |
| SM-4 | Floor "just under one unit" returns 0 (feeds insufficient-capital path) | [EXISTS] `TestCalculateUnits_BasicFloor` "below one unit" (:337) |

### 2.3 Failure handling (transient-retry vs permanent-fail)

| # | Test case | Status |
|---|---|---|
| FH-1 | Permanent (invalid pct) → immediate `StatusFailed`, no ErrorCount bump | [EXISTS] AC-8.3 |
| FH-2 | Transient (Net Liq) below threshold → retry | [EXISTS] AC-8.4 |
| FH-3 | Transient daily @ threshold → waiting + TradedToday | [EXISTS] AC-8.5 |
| FH-4 | Transient once @ threshold → failed | [EXISTS] AC-8.6 |
| FH-5 | Sentinel-error distinction (`errors.Is(ErrInvalidCapitalPercent)` vs `ErrNetLiqUnavailable`) | [EXISTS] `types_test.go` (:289, :299, :314), `engine_capital_test.go` (:188, :214) |
| FH-6 | Precedence: invalid pct beats unavailable Net Liq | [EXISTS] `TestResolveEffectiveCapital_InvalidPercentTakesPrecedenceOverNetLiq` (:314) |
| FH-7 | **Failure path fidelity: `handleCapitalFailure` branch structure matches the strike-find path at `engine.go:806-830` (threshold 3, daily→waiting, once→failed)** | [PARTIAL] Unit-tested in isolation (FH-1..4); no test asserts parity against the actual strike-find handler. Recommend a comparison/table test or a comment-anchored assertion |

### 2.4 Persistence / restore re-resolution

| # | Test case | Status |
|---|---|---|
| PR-1 | Snapshot fields Save→Load round-trip in runtime state | [EXISTS] `runtime_state_test.go` (:19) |
| PR-2 | `RestoreAutomation` copies snapshot to `ActiveAutomation` | [EXISTS] `runtime_state_test.go` (:58) |
| PR-3 | Legacy state (no snapshot) restores with zeros | [EXISTS] `runtime_state_test.go` (:86, :112) |
| PR-4 | Config percent fields persist through `automations.json` | [GAP] see AC-2.4 |
| PR-5 | **Re-resolution after restart: restored percent automation re-reads Net Liq at next trade-time capture (persisted snapshot is display-only, not used for sizing)** | [GAP] Design §8.3 promises this but no test asserts that a fresh `captureEffectiveCapital` overrides the restored snapshot. Recommend: restore with stale snapshot 6000, run `captureEffectiveCapital` with account 20000×60% ⇒ assert `EffectiveCapital` becomes 12000 |

### 2.5 Test suite runs (QA gate)

| # | Action | Status |
|---|---|---|
| TS-1 | `cd trade-backend-go && go test ./...` fully green | [GAP-VERIFY] ran automation+handlers subset (green); run full `./...` as a gate |
| TS-2 | `cd trade-app && npx vitest run` fully green | [GAP-VERIFY] ran the 2 relevant files (86 green); run whole suite as a gate |
| TS-3 | `cd strategy-service && python -m pytest` (unrelated, sanity only) | [OPTIONAL] out of scope for #73 |

---

## 3. Identified Gaps → Recommended New Tests

Ordered by priority. **No test code is written yet** — this is the backlog for the
test-writing phase.

### G-1 (High) — `handleTradingState` end-to-end (AC-6.3, AC-8.7)
The two capture points and the failure handler are unit-tested in isolation, but
the actual `handleTradingState` wiring (capture → `Message` → `CalculateUnits(eff)`
→ order placement, and the failure short-circuit) is not exercised.
**New test(s)** in `engine_capital_test.go` (or a new `engine_trading_state_test.go`):
- Percent success: mock `accountReader` (10000) + mock strike/order placement →
  assert `active.EffectiveCapital==6000`, `Message` contains `6000`, order placed
  with `units==12`.
- Percent failure: failing `accountReader` → assert **no order placed**, status/error
  match `handleCapitalFailure`.
Requires stubbing strike-finding/order-placement; check whether existing engine
tests provide seams for `findStrikes*`/`placeOrder` — if not, this may need a
small additional test seam (flag as a note for @dev, not a behavior change).

### G-2 (High) — Config persistence round-trip (AC-2.4, PR-4)
`storage.go` `Create`/`Update` → reload → assert `max_capital_mode` and
`max_capital_percent` survive. Add to `storage_test.go`.

### G-3 (Medium) — Restart re-resolution overrides stale snapshot (PR-5)
Restore an automation with a persisted snapshot, then invoke a fresh
`captureEffectiveCapital` with a **different** Net Liq and assert the snapshot is
replaced (proves persisted value is display-only). Add to `engine_capital_test.go`.

### G-4 (Medium) — HTTP handler validation returns 400 (AC-2.5)
`httptest` + gin: `POST`/`PUT` a percent-out-of-range config → expect HTTP 400 and
`success:false`; valid percent → 200. `validateTradeCapital` is unit-tested, but
the HTTP status wiring in `CreateConfig`/`UpdateConfig` is not. Add to
`handlers/automation_capital_test.go` (or a new handler test).

### G-5 (Low) — Iron Condor + percent-derived floor edge (SM-3)
Table test that resolves capital from Net Liq then feeds an IC `CalculateUnits`,
covering both wider-put and wider-call, at a unit-boundary Net Liq. Add to
`types_test.go`.

### G-6 (Low) — Strike-find parity assertion (FH-7)
Document/assert that `handleCapitalFailure`'s transient branch mirrors the
strike-find handler thresholds. Either a shared helper extraction (dev change) or
a table test mirroring both branches. Add to `engine_capital_test.go`.

### G-7 (Verify only) — Full suite gates (TS-1, TS-2, 9.4)
Run `go test ./...` and full `npx vitest run` as release gates; no new code.

---

## 4. Test Execution Commands

```sh
# Backend — targeted
cd trade-backend-go
go test ./internal/automation/... ./internal/api/handlers/... -v

# Backend — full gate (TS-1)
go test ./...

# Frontend — targeted
cd trade-app
npx vitest run tests/AutomationConfigForm.test.js tests/AutomationDashboard.test.js

# Frontend — full gate (TS-2)
npx vitest run
```

---

## 5. Coverage Scorecard

| Area | Existing | Gap / Verify |
|---|---|---|
| AC-1 mode toggle & default | ✅ full | — |
| AC-2 percent persisted | ✅ types+UI | G-2 (storage), G-4 (HTTP) |
| AC-3 $10k×60%=$6k | ✅ full | — |
| AC-4 fixed regression | ✅ full | — |
| AC-5 monitoring-start display | ✅ full | — |
| AC-6 trade-time display+log | ✅ unit | G-1 (`handleTradingState`) |
| AC-7 legacy compat | ✅ full | — (optional fixture) |
| AC-8 Net-Liq failure path | ✅ unit | G-1 (end-to-end no-order) |
| AC-9 suites + new coverage | ✅ mostly | G-7 (full-suite gate) |
| PO: backward compat | ✅ full | — |
| PO: sizing math (IC+floor) | ✅ core | G-5 (percent-derived IC edge) |
| PO: failure transient/permanent | ✅ full | G-6 (strike-find parity) |
| PO: persistence/restore re-resolution | ✅ snapshot | G-3 (re-resolution override) |
| PO: test suite runs | partial | G-7 (run full gates) |

**Bottom line:** the implementation is well-covered at the unit level (types,
engine helpers, persistence, migration, handler-validation, and both frontend
components). The remaining gaps are integration-level: the full `handleTradingState`
flow (G-1), config-persistence round-trip (G-2), restart re-resolution (G-3), and
HTTP-status validation (G-4), plus two low-priority reinforcement tests (G-5, G-6)
and the full-suite verification gate (G-7).

---

*Prepared by @qa. Read-only analysis; no test code created or modified.*
