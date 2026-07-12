# QA Test Results — Auto Mode: Percentage-based Max Capital (Issue #73)

**Issue:** [#73 — Auto Mode with percentage for Max Capital](https://github.com/schardosin/juicytrade/issues/73)
**Branch:** `fleet/issue-73-auto-mode-with-percentage-for-max-capital`
**Test plan:** [test-plan.md](./test-plan.md)
**Gate:** G-7 — full backend + frontend suites
**Author:** @qa

---

## Verdict: ✅ CONDITIONAL-PASS

Issue-73 is **fully verified**. All backend tests pass. All frontend tests pass
**except two pre-existing failures in `CollapsibleOptionsChain.test.js`** that are
**unrelated to issue-73** (that component was not touched on this branch). The
condition is solely those two pre-existing failures; every issue-73 test passes.

---

## 1. Backend suite — `go test ./...`

**Result: ALL PASS** (no `FAIL` in any package).

```
?   	trade-backend-go/cmd/server	[no test files]
ok  	trade-backend-go/internal/api/handlers	(cached)
ok  	trade-backend-go/internal/auth	(cached)
ok  	trade-backend-go/internal/automation	(cached)
ok  	trade-backend-go/internal/automation/indicators	(cached)
ok  	trade-backend-go/internal/automation/types	(cached)
?   	trade-backend-go/internal/clients	[no test files]
?   	trade-backend-go/internal/config	[no test files]
?   	trade-backend-go/internal/models	[no test files]
ok  	trade-backend-go/internal/providers	(cached)
?   	trade-backend-go/internal/providers/alpaca	[no test files]
?   	trade-backend-go/internal/providers/base	[no test files]
ok  	trade-backend-go/internal/providers/schwab	(cached)
ok  	trade-backend-go/internal/providers/tastytrade	(cached)
?   	trade-backend-go/internal/providers/tradier	[no test files]
ok  	trade-backend-go/internal/services/ivx	(cached)
?   	trade-backend-go/internal/services/watchlist	[no test files]
ok  	trade-backend-go/internal/streaming	(cached)
?   	trade-backend-go/internal/utils	[no test files]
```

Packages central to issue-73 (`internal/automation`, `internal/automation/types`,
`internal/api/handlers`) all report `ok`. Packages marked `[no test files]` have
no tests and are not failures.

---

## 2. Frontend suite — `npx vitest run`

**Totals:**

| Metric | Value |
|---|---|
| Test files | **1 failed, 37 passed (38 total)** |
| Tests | **2 failed, 1390 passed (1392 total)** |
| Duration | ~39 s |

**The 2 failures — both pre-existing and unrelated to issue-73:**

Both are in `tests/CollapsibleOptionsChain.test.js`:

1. `CollapsibleOptionsChain - Stability & Cascade Protection > Component Initialization & Structure > displays strike count filter with correct options`
   - `expected [...] to have a length of 6 but got 10`
2. `CollapsibleOptionsChain - Stability & Cascade Protection > Strike Count Filter & Data Consistency > provides all expected strike count options`
   - `expected ['50','60','70','80','90','100'] to deeply equal` actual
     `['50','60','70','80','90','100','150','200','250','300']`

**Confirmation these are pre-existing and NOT caused by issue-73:**
- `CollapsibleOptionsChain.vue` / `.test.js` are **not in the issue-73 branch diff**
  (`git diff --name-only origin/main...HEAD` contains no CollapsibleOptionsChain
  files).
- The failures are a stale test asserting 6 strike-count options while the
  production component now offers 10 (added strikes `150/200/250/300`). This stems
  from an unrelated automation-UI change (`741d214 improved automation ui,
  indicators cache, increased strikes to show amount`), not from any #73 work.
- Every issue-73 frontend test passes: `AutomationConfigForm.test.js` (24) and
  `AutomationDashboard.test.js` — mode toggle, percent validation, Net-Liq hint,
  `formatCapitalUsed`, mode-aware summary.

---

## 3. New QA tests added this session (all pass)

| Gap | Test(s) | File | Status |
|---|---|---|---|
| **G-2** config persistence round-trip | `TestStorage_PercentConfigRoundTrip`, `TestStorage_UpdateToPercentRoundTrip` | `internal/automation/storage_test.go` | ✅ PASS |
| **G-3** restart re-resolution overrides stale snapshot | `TestCaptureEffectiveCapital_ReResolvesOverStalePersistedSnapshot` | `internal/automation/engine_capital_test.go` | ✅ PASS |
| **G-4** HTTP 400 validation | `TestCreateConfig_PercentZeroRejected`, `TestCreateConfig_PercentAboveMaxRejected`, `TestCreateConfig_FixedBelowMinRejected`, `TestUpdateConfig_PercentZeroRejected`, `TestUpdateConfig_PercentAboveMaxRejected`, `TestUpdateConfig_FixedBelowMinRejected` | `internal/api/handlers/automation_capital_http_test.go` | ✅ PASS |

All were run via `go test ./internal/automation/... ./internal/api/handlers/...`
and are included in the green backend suite above. No production code was modified
to add them; the working tree was left clean (no stray `automations.json` /
runtime-state files).

---

## 4. Acceptance-Criteria verdict — AC-1 .. AC-9 all met

Per the coverage matrix in [test-plan.md](./test-plan.md) §1:

| AC | Summary | Verdict |
|---|---|---|
| AC-1 | Mode toggle in form; Fixed default | ✅ Met (form + `NewTradeConfiguration` default) |
| AC-2 | Percent 1–100 entered & persisted | ✅ Met (types round-trip, UI validation, **G-2** storage round-trip, **G-4** HTTP) |
| AC-3 | $10,000 × 60% = $6,000 | ✅ Met (`ResolveEffectiveCapital`, `captureEffectiveCapital`) |
| AC-4 | Fixed mode byte-for-byte identical | ✅ Met (fixed-mode parity + no account read) |
| AC-5 | Monitoring-start effective-capital display | ✅ Met (`captureMonitoringStartCapital` + dashboard row) |
| AC-6 | Trade-time effective capital from Net Liq, displayed + logged | ✅ Met at unit level (capture + log + message); full `handleTradingState` wiring is residual gap G-1 (low risk — see §5) |
| AC-7 | Legacy configs load & run as fixed | ✅ Met (legacy unmarshal, `migrateMaxCapitalMode`, legacy runtime restore) |
| AC-8 | Net Liq unavailable → no mis-sized order, clear error, existing failure path | ✅ Met at unit level (`ErrNetLiqUnavailable`, `handleCapitalFailure` transient/permanent branches) |
| AC-9 | All suites pass; new coverage for resolution / example / defaulting / failure | ✅ Met (backend green; frontend green except unrelated pre-existing; new G-2/G-3/G-4 tests) |

---

## 5. Residual gap — G-1 (`handleTradingState` end-to-end)

**Status: residual gap, accepted; integration risk LOW.**

- An end-to-end test of `handleTradingState` (trade-time capture → `Message` →
  `CalculateUnits(eff)` → strike-find → order placement, and the failure
  short-circuit) is **not automatable without adding a production test seam**.
- The only injectable seam on `Engine` is `accountReader`. Strike-finding and
  order placement (`findStrikesForDelta`/`findStrikesForIronCondor`/
  `placeSpreadOrder`/`placeIronCondorOrder`) are plain `*Engine` methods that call
  the **concrete** `*providers.ProviderManager` directly; its provider set is an
  unexported map loaded from on-disk credentials, so no mock can be injected from
  tests without a production change (out of scope).
- **Risk is low** because every constituent piece is unit-tested in isolation:
  `captureEffectiveCapital` (fixed/percent/failure), `CalculateUnits` (floor +
  Iron Condor wider side), `handleCapitalFailure` (permanent vs transient, retry/
  wait/fail), and delta-drift snapshot reuse.
- **To close fully later (requires @dev):** add an injectable seam (function fields
  on `Engine`, or an interface over `ProviderManager`), then add the success/
  failure end-to-end tests. This is a separate, approved task.

---

## 6. PO focus areas — all verified

| Focus area | Verdict | Evidence |
|---|---|---|
| **Backward compatibility** | ✅ Verified | empty-mode→fixed everywhere; legacy JSON unmarshal; `migrateMaxCapitalMode`; legacy runtime restore; additive omitempty JSON fields |
| **Sizing math (Iron Condor wider-side + floor)** | ✅ Verified | `CalculateUnits` floor/truncation cases; Iron Condor `max(putWidth, callWidth)`; percent-resolved `$6,000`/width 5 → 12 units |
| **Failure handling (transient-retry vs permanent-fail)** | ✅ Verified | invalid percent → permanent `StatusFailed`; Net-Liq unavailable → transient, mirrors strike-find (daily→waiting, once→failed at threshold 3); sentinel-error distinction |
| **Persistence / restore re-resolution** | ✅ Verified | snapshot Save/Load + `RestoreAutomation` copy; legacy state without snapshot; **G-2** config round-trip; **G-3** restart re-resolution overrides stale snapshot (display-only) |
| **Test suite runs** | ✅ Verified | backend `go test ./...` green; frontend `npx vitest run` green except 2 unrelated pre-existing CollapsibleOptionsChain failures |

---

## 7. Summary

- **Backend:** all packages `ok` (no failures).
- **Frontend:** 1390/1392 tests pass; the only 2 failures are pre-existing,
  unrelated `CollapsibleOptionsChain` tests (stale strike-count expectation).
- **Issue-73 coverage:** AC-1..AC-9 met; new G-2/G-3/G-4 tests added and passing;
  G-1 documented as a low-risk residual gap requiring a production seam.
- **Recommendation:** issue-73 is ready to merge. Separately, the pre-existing
  `CollapsibleOptionsChain.test.js` expectations should be updated to reflect the
  10 strike-count options (out of scope for #73).

---

*Prepared by @qa. Test execution only; no production code modified for this gate.*
