# Issue #55: IVx Intraday Not Being Calculated

## Problem Description

In the Trade View options chain, the IVx (Implied Volatility Index) for intraday (0DTE) expirations always displays "IVx 0.1%" and expected move of "$0.00" or "$0.01" during market hours. The calculation incorrectly treats same-day expirations as expired.

## Root Cause

The `calculateDaysToExpiration` function in `trade-backend-go/internal/services/ivx/calculator.go` has a timezone bug:

1. **Line 148:** `today := now.Truncate(24 * time.Hour)` — `Truncate` rounds down to midnight **UTC**, not midnight Eastern Time. When `now` is May 1 at 10:00 AM ET (14:00 UTC), `Truncate` produces May 1 00:00:00 UTC, which in ET is **April 30 at 20:00:00 ET** (during EDT).

2. **Line 149:** `expDate = expDate.In(et)` — The parsed expiration date `"2025-05-01"` starts as May 1 00:00:00 UTC. Converting to ET gives **April 30 at 20:00:00 ET**.

3. **Line 152:** `if expDate.Equal(today)` — Both values are April 30 20:00:00 ET, so the 0DTE branch IS entered.

4. **Line 154:** `marketClose := time.Date(today.Year(), today.Month(), today.Day(), 16, 0, 0, 0, et)` — Since `today` represents April 30 at 20:00 ET, this constructs **April 30 at 16:00 ET** as market close.

5. **Line 157:** `if now.After(marketClose)` — The actual current time (May 1 at 10:00 AM ET) is AFTER April 30 at 4:00 PM ET, so the code concludes the option is **expired**.

6. Returns `(0.0001, true)`, causing `adjustedIVx = 0.001` (0.1%) and near-zero expected move.

## Fix Requirements

### FR-1: Correct timezone handling in DTE calculation

Replace the timezone-incorrect `Truncate` approach with proper Eastern Time date construction:

**Current (broken):**
```go
today := now.Truncate(24 * time.Hour)
expDate = expDate.In(et)
```

**Fixed:**
```go
today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, et)
expDateET := time.Date(expDate.Year(), expDate.Month(), expDate.Day(), 0, 0, 0, 0, et)
```

This ensures both "today" and the expiration date are midnight Eastern Time, making:
- The `Equal` comparison correct for same-day detection
- The `marketClose` construction use the correct day
- The `Sub(today)` calculation for future expirations produce correct day counts

### FR-2: Maintain all existing behavior for non-0DTE cases

The fix must not change the behavior for:
- Future expirations (DTE > 0 days)
- Already-expired expirations (past dates)
- After-market-close behavior on expiration day

### FR-3: Unit tests

Add unit tests covering:
- 0DTE during market hours (should return fractional DTE, `isExpired = false`)
- 0DTE after market close (should return 0.0001, `isExpired = true`)
- 1DTE expiration (should return ~1.x days based on time remaining)
- Past expiration (should return small value, `isExpired = true`)
- Various times during the trading day for 0DTE (morning, midday, afternoon)

## Acceptance Criteria

1. **AC-1:** For a 0DTE option during market hours (before 4:00 PM ET), the IVx should reflect the actual ATM IV average (not 0.1%) and the expected move should be calculated using the correct fractional DTE.
2. **AC-2:** For a 0DTE option after market close (after 4:00 PM ET), the option should correctly be marked as expired with IVx near 0.
3. **AC-3:** Future expirations (1+ days out) continue to calculate correctly with no behavioral changes.
4. **AC-4:** All existing tests continue to pass.
5. **AC-5:** New unit tests cover the 0DTE timezone scenarios described in FR-3.

## Scope

### In Scope
- Fix the `calculateDaysToExpiration` function in `calculator.go`
- Add unit tests for the DTE calculation logic

### Out of Scope
- Changes to the frontend display logic
- Changes to the IVx formula itself
- Changes to how IVx is aggregated across expirations
